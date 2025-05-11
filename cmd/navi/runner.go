package navi

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-navi/navi/internal/logger"
	portUtils "github.com/go-navi/navi/internal/port"
	"github.com/go-navi/navi/internal/process"
	"github.com/go-navi/navi/internal/utils"
	"github.com/kballard/go-shellquote"
)

// Maps to track unique log prefixes for commands
var logPrefixCounterMap = make(map[string]int)
var logPrefixCounterMutex sync.Mutex

// executeRunner processes runner configurations and executes commands sequentially or in parallel
func executeRunner(contextCmd Ctx, commandArgs []string, commandLineRunnerFlags RunnerFlags) error {
	runnerCommandsList, runnerName, runnerFlagStrings, _, isInlineRunner, err := findMatchingRunnerConfiguration(commandArgs, commandLineRunnerFlags, true)
	if err != nil {
		return err
	}

	parsedRunnerFlags := convertStringFlagsToRunnerFlags(runnerFlagStrings)

	if len(runnerFlagStrings) > 0 {
		if isInlineRunner {
			logger.Info("Starting inline runner with flags [%s] and %d command(s)", strings.Join(runnerFlagStrings, ", "), len(runnerCommandsList))
		} else {
			logger.Info("Starting runner `%s` with flags [%s]", runnerName, strings.Join(runnerFlagStrings, ", "))
		}
	} else {
		if isInlineRunner {
			logger.Info("Starting inline runner with %d command(s)", len(runnerCommandsList))
		} else {
			logger.Info("Starting runner `%s`", runnerName)
		}
	}

	// Process and execute all runner commands
	return executeRunnerCommands(contextCmd, runnerName, runnerCommandsList, parsedRunnerFlags)
}

// normalizeCommandList converts various command formats to a standard list
func normalizeCommandList(commandsRaw any, runnerName string) (commands []map[string]any, err error) {
	switch value := commandsRaw.(type) {
	case string:
		commands = append(commands, map[string]any{"cmd": value})

	case []any:
		for _, cmd := range value {
			switch typedCmd := cmd.(type) {
			case map[string]any:
				if _, ok := typedCmd["cmd"]; !ok {
					return nil, fmt.Errorf("Runner command in runner `%s` must have a `cmd` key", runnerName)
				}

				commands = append(commands, typedCmd)

			case string:
				commands = append(commands, map[string]any{"cmd": typedCmd})
			}
		}
	}

	if len(commands) == 0 {
		return nil, fmt.Errorf("Runner `%s` must be defined as a command or a list of commands", runnerName)
	}

	return commands, nil
}

// sanitizeCommandList ensures `project:*` format is correctly handled
func sanitizeCommandList(commandList []map[string]any) ([]map[string]any, error) {
	yamlConfig, _, err := getYamlConfiguration(true)
	if err != nil {
		return nil, fmt.Errorf("Failed to load configuration from YAML file: %v", err)
	}

	newCommandList := []map[string]any{}

	for _, command := range commandList {
		if cmdStr, ok := command["cmd"].(string); ok {
			if !strings.HasSuffix(cmdStr, ":*") {
				newCommandList = append(newCommandList, command)
				continue
			}

			// Handle `project:*` format
			projectName := strings.TrimSuffix(cmdStr, ":*")

			if projectName == "" {
				continue
			}

			// Check if project exists in configuration
			if _, projectExists := yamlConfig.Projects[projectName]; !projectExists {
				continue
			}

			// Add all commands from the project
			for cmdName := range yamlConfig.Projects[projectName].Cmds {
				if cmdName == "*:*" {
					continue
				}

				newCommand := map[string]any{
					"cmd": projectName + ":" + cmdName,
				}

				for key, value := range command {
					if key != "cmd" {
						newCommand[key] = value
					}
				}

				newCommandList = append(newCommandList, newCommand)
			}
		}
	}

	return newCommandList, nil
}

// executeRunnerCommands processes all commands in a runner
func executeRunnerCommands(contextCmd Ctx, runnerName string, commandsList []map[string]any, runnerFlags RunnerFlags) error {
	var waitGroup sync.WaitGroup

	// Create channel for sequential execution
	previousCommandChannel := make(chan struct{})
	close(previousCommandChannel) // first goroutine can start immediately

	// Process each command
	runnerExecutions, err := prepareRunnerExecutions(commandsList, runnerName, runnerFlags)
	if err != nil {
		return err
	}

	// Define handlers for command failure/completion
	handlers := CommandHandlers{
		serialFailure: func(cmdConfig RunnerCommand) {
			if cmdConfig.Serial || runnerFlags.Serial {
				logger.Error("A serial command in runner `%s` has failed", runnerName)
				gracefulShutdown(contextCmd, "")
			}
		},
		dependentCompletion: func(cmdConfig RunnerCommand) {
			if cmdConfig.Dependent || runnerFlags.Dependent {
				logger.Error("A dependent command in runner `%s` has failed or finished", runnerName)
				gracefulShutdown(contextCmd, "")
			}
		},
	}

	// Launch all runner commands with proper sequencing
	for _, executionConfig := range runnerExecutions {
		nextCommandChannel := make(chan struct{})
		waitGroup.Add(1)

		// Runner command execution
		launchRunnerCommand(
			contextCmd,
			executionConfig,
			previousCommandChannel,
			nextCommandChannel,
			&waitGroup,
			handlers,
		)

		previousCommandChannel = nextCommandChannel // Chain the next command
	}

	waitGroup.Wait() // Wait for all commands to complete
	return nil
}

// prepareRunnerExecutions processes command configurations into execution structures
func prepareRunnerExecutions(commandsList []map[string]any, runnerName string, runnerFlags RunnerFlags) ([]RunnerExecution, error) {
	runnerExecutions := []RunnerExecution{}

	for _, command := range commandsList {
		// Extract command string
		commandString, ok := command["cmd"].(string)
		if !ok {
			return nil, fmt.Errorf("Runner command in runner `%s` must have a `cmd` key", runnerName)
		}

		var runnerCmd RunnerCommand
		runnerCmd.Cmd = commandString

		// Parse custom name if provided
		if name, ok := command["name"].(string); ok && strings.TrimSpace(name) != "" {
			runnerCmd.Name = name
		}

		// Parse execution flags
		parseCommandFlags(&runnerCmd, command, runnerFlags)

		// Parse delay before execution
		if delaySeconds, ok := utils.ToFloat64(command["delay"]); ok {
			runnerCmd.Delay = delaySeconds
		}

		// Parse restart configuration
		enableRestart, maxRetries, restartCondition, retryInterval := parseRestartConfig(command)
		runnerCmd.Restart = enableRestart

		// Validate restart condition if restart is enabled
		if enableRestart && restartCondition != "always" && restartCondition != "failure" && restartCondition != "success" {
			return nil, fmt.Errorf(
				"Invalid value for parameter `condition` in runner `%s`. Must be `always`, `failure`, or `success`",
				runnerName,
			)
		}

		// Parse ports to await
		if awaits, ok := command["awaits"]; ok {
			runnerCmd.Awaits = awaits
		}

		// Setup project command
		commandTokens, err := shellquote.Split(runnerCmd.Cmd)
		if err != nil {
			return nil, fmt.Errorf("Invalid format for command `%s` in runner `%s`", commandString, runnerName)
		}

		projectCmd, notFound, err := getProjectCommand(commandTokens)
		if err != nil {
			if notFound {
				projectCmd = createFallbackProjectCommand(runnerName, runnerCmd.Cmd, commandTokens)
			} else {
				return nil, err
			}
		}

		setupCommandLogPrefix(projectCmd, projectCmd.Identifier, runnerCmd.Name)

		runnerExecutions = append(runnerExecutions, RunnerExecution{
			projectCmd:       projectCmd,
			runnerCmd:        &runnerCmd,
			runnerCmdStr:     runnerCmd.Cmd,
			maxRetries:       maxRetries,
			retryInterval:    retryInterval,
			enableRestart:    enableRestart,
			restartCondition: restartCondition,
		})
	}

	return runnerExecutions, nil
}

// parseCommandFlags extracts serial and dependent flags from command config
func parseCommandFlags(runnerCmd *RunnerCommand, commandConfig map[string]any, runnerFlags RunnerFlags) {
	// Parse serial execution flag
	if isSerial, ok := commandConfig["serial"].(bool); ok {
		runnerCmd.Serial = isSerial
	}

	// Parse serial execution flag from runner flags
	if runnerFlags.Serial {
		runnerCmd.Serial = true
	}

	// Parse dependent execution flag
	if isDependent, ok := commandConfig["dependent"].(bool); ok {
		runnerCmd.Dependent = isDependent
	}

	// Parse dependent execution flag from runner flags
	if runnerFlags.Dependent {
		runnerCmd.Dependent = true
	}
}

// parseRestartConfig extracts restart settings from command config
func parseRestartConfig(commandConfig map[string]any) (bool, int, string, float64) {
	var enableRestart bool
	var maxRetries int
	var restartCondition = "failure" // Default
	var retryInterval float64 = 1.0  // Default

	if restartConfig, ok := commandConfig["restart"]; ok {
		switch value := restartConfig.(type) {
		case bool:
			enableRestart = value

		case map[string]any:
			enableRestart = true

			if retries, ok := utils.ToInt(value["retries"]); ok {
				maxRetries = retries
			}

			if condition, ok := value["condition"].(string); ok {
				restartCondition = condition
			}

			if interval, ok := utils.ToFloat64(value["interval"]); ok {
				retryInterval = interval
			}
		}
	}

	return enableRestart, maxRetries, restartCondition, retryInterval
}

// createFallbackProjectCommand creates a generic project command for non-project commands
func createFallbackProjectCommand(runnerName, commandStr string, tokens []string) *ProjectCommand {
	return &ProjectCommand{
		Identifier:  runnerName,
		Dir:         applicationRootPath,
		EnvVars:     []string{},
		CommandList: []string{commandStr},
	}
}

// setupCommandLogPrefix configures unique log prefix for a command
func setupCommandLogPrefix(projectCmd *ProjectCommand, defaultPrefix, customName string) {
	projectCmd.LogPrefix = defaultPrefix
	if customName != "" && strings.TrimSpace(customName) != "" {
		projectCmd.LogPrefix = customName
	}

	logPrefixCounterMutex.Lock()
	logPrefixCounterMap[projectCmd.LogPrefix]++
	projectCmd.LogPrefixId = strconv.Itoa(logPrefixCounterMap[projectCmd.LogPrefix])
	logPrefixCounterMutex.Unlock()
	projectCmd.LogPrefixColor = logger.GetLogPrefixColor()
}

// launchRunnerCommand starts a goroutine for runner command execution
func launchRunnerCommand(
	contextCmd Ctx,
	execution RunnerExecution,
	startSignal, nextStartSignal chan struct{},
	waitGroup *sync.WaitGroup,
	handlers CommandHandlers,
) {
	go func() {
		defer waitGroup.Done()
		<-startSignal // Wait for previous command if serial

		projectCmd := execution.projectCmd
		cmdConfig := *execution.runnerCmd

		if !cmdConfig.Serial && !process.TerminatingProcesses {
			close(nextStartSignal) // Allow next command to start immediately
		}

		if execution.enableRestart {
			executeRestartableCommand(
				contextCmd,
				projectCmd,
				cmdConfig,
				execution.maxRetries,
				execution.retryInterval,
				execution.restartCondition,
				handlers,
			)
		} else {
			executeOneTimeCommand(
				contextCmd,
				projectCmd,
				cmdConfig,
				nextStartSignal,
				handlers,
			)
		}

		if cmdConfig.Serial && !process.TerminatingProcesses {
			close(nextStartSignal) // Allow next command to start after this one completes
		}
	}()
}

// executeRestartableCommand handles command execution with auto-restart capability
func executeRestartableCommand(
	contextCmd Ctx,
	projectCmd *ProjectCommand,
	cmdConfig RunnerCommand,
	maxRetries int,
	retryDelay float64,
	restartCondition string,
	handlers CommandHandlers,
) {
	retryCount := 0

	if maxRetries > 0 {
		logger.InfoWithPrefix(projectCmd.GetLogPrefix(), "Starting with auto-restart (max %d retries)", maxRetries)
	} else {
		logger.InfoWithPrefix(projectCmd.GetLogPrefix(), "Starting with auto-restart")
	}

	for {
		// Wait for required ports
		if err := tryWaitForPorts(cmdConfig, projectCmd.GetLogPrefix); err != nil {
			if process.TerminatingProcesses {
				return
			}

			shouldContinue := handlePortWaitError(
				err, projectCmd.GetLogPrefix, maxRetries, retryDelay, &retryCount,
			)

			if !shouldContinue {
				handlers.serialFailure(cmdConfig)
				handlers.dependentCompletion(cmdConfig)
				break
			}
			continue
		}

		// Apply command delay
		applyCommandDelay(cmdConfig, projectCmd.GetLogPrefix)
		if process.TerminatingProcesses {
			return
		}

		// Execute the command
		err := executeCommandWithAfterHandling(contextCmd, projectCmd)

		// Handle execution result
		if err != nil {
			if errors.Is(err, ErrProcessTerminated) {
				return
			}

			if shouldRestartOnCondition("failure", restartCondition) {
				if scheduleRetry(projectCmd.GetLogPrefix, maxRetries, retryDelay, &retryCount) {
					continue
				}
			}

			handlers.serialFailure(cmdConfig)
		} else if shouldRestartOnCondition("success", restartCondition) {
			if scheduleRetry(projectCmd.GetLogPrefix, maxRetries, retryDelay, &retryCount) {
				continue
			}
		}

		handlers.dependentCompletion(cmdConfig)
		break
	}
}

// shouldRestartOnCondition checks if command should restart based on condition
func shouldRestartOnCondition(outcome, condition string) bool {
	return condition == outcome || condition == "always"
}

// executeOneTimeCommand handles command execution without restart
func executeOneTimeCommand(
	contextCmd Ctx,
	projectCmd *ProjectCommand,
	cmdConfig RunnerCommand,
	nextStartSignal chan struct{},
	handlers CommandHandlers,
) {
	// Wait for required ports
	if err := tryWaitForPorts(cmdConfig, projectCmd.GetLogPrefix); err != nil {
		if process.TerminatingProcesses {
			return
		}

		if !errors.Is(err, ErrProcessTerminated) {
			logger.ErrorWithPrefix(projectCmd.GetLogPrefix(), "%v", err)
		}

		handlers.serialFailure(cmdConfig)
		handlers.dependentCompletion(cmdConfig)

		if cmdConfig.Serial && !process.TerminatingProcesses {
			close(nextStartSignal)
		}
		return
	}

	// Apply command delay
	applyCommandDelay(cmdConfig, projectCmd.GetLogPrefix)
	if process.TerminatingProcesses {
		return
	}

	// Execute command
	err := executeCommandWithAfterHandling(contextCmd, projectCmd)

	// Handle execution result
	if err != nil && !errors.Is(err, ErrProcessTerminated) {
		handlers.serialFailure(cmdConfig)
		handlers.dependentCompletion(cmdConfig)
	}
}

// tryWaitForPorts tries to wait for specified ports to become available
func tryWaitForPorts(cmdConfig RunnerCommand, getLogPrefix func() string) error {
	if process.TerminatingProcesses {
		return ErrProcessTerminated
	}

	return waitForRequiredPorts(cmdConfig, getLogPrefix)
}

// handlePortWaitError handles errors from port waiting
func handlePortWaitError(
	err error,
	getLogPrefix func() string,
	maxRetries int,
	retryDelay float64,
	retryCount *int,
) bool {
	if process.TerminatingProcesses {
		return false
	}

	if !errors.Is(err, ErrProcessTerminated) {
		logger.ErrorWithPrefix(getLogPrefix(), "%v", err)
	}

	return scheduleRetry(getLogPrefix, maxRetries, retryDelay, retryCount)
}

// executeCommandWithAfterHandling runs a command and its 'after' commands
func executeCommandWithAfterHandling(contextCmd Ctx, projectCmd *ProjectCommand) error {
	err := projectCmd.execute(contextCmd)
	if err != nil && !errors.Is(err, ErrProcessTerminated) {
		logger.ErrorWithPrefix(projectCmd.GetLogPrefix(), "%v", err)
	}

	// Execute after-commands if needed
	if !projectCmd.WatchExecuted {
		afterErr := projectCmd.executeAfterHooks(err, contextCmd, false)
		if afterErr != nil {
			logger.ErrorWithPrefix(projectCmd.GetLogPrefix(), "Fail during execution of after command(s): %v", afterErr)
			gracefulShutdown(contextCmd, "")
		}
	}

	return err
}

// isRunnerConfigured checks if the given command matches a runner in configuration
func isRunnerConfigured(commandArgs []string) bool {
	_, _, _, runnerFound, _, err := findMatchingRunnerConfiguration(commandArgs, RunnerFlags{}, false)
	return runnerFound && err == nil
}

// waitForRequiredPorts waits until specified ports are available
func waitForRequiredPorts(commandConfig RunnerCommand, getLogPrefix func() string) error {
	if commandConfig.Awaits != nil {
		portsToWait, timeoutSeconds, parseErr := portUtils.ParsePortConfiguration(commandConfig.Awaits)
		if parseErr != nil {
			return parseErr
		}

		return portUtils.WaitForMultiplePorts(portsToWait, getLogPrefix, timeoutSeconds)
	}

	return nil
}

// applyCommandDelay pauses execution for specified seconds
func applyCommandDelay(commandConfig RunnerCommand, getLogPrefix func() string) {
	if commandConfig.Delay > 0 {
		formattedDelay := utils.FormatDurationValue(commandConfig.Delay)
		logger.InfoWithPrefix(getLogPrefix(), "Waiting %s seconds before execution...", formattedDelay)
		time.Sleep(time.Duration(commandConfig.Delay) * time.Second)
	}
}

// scheduleRetry manages retry logic and delay between attempts
func scheduleRetry(getLogPrefix func() string, maxRetries int, retryDelay float64, currentRetryCount *int) bool {
	*currentRetryCount++

	if maxRetries > 0 {
		// Limited retries
		if *currentRetryCount > maxRetries {
			logger.WarnWithPrefix(getLogPrefix(), "Maximum retry attempts (%d) reached. Terminating", maxRetries)
			return false
		}

		logger.InfoWithPrefix(getLogPrefix(), "Restarting in %s seconds... (attempt %d/%d)",
			utils.FormatDurationValue(retryDelay), *currentRetryCount, maxRetries)
	} else {
		// Infinite retries
		logger.InfoWithPrefix(getLogPrefix(), "Restarting in %s seconds...",
			utils.FormatDurationValue(retryDelay))
	}

	time.Sleep(time.Duration(retryDelay) * time.Second)
	return true
}

// extractRunnerNameAndFlags parses runner key in format: name[flag1,flag2,...]
func extractRunnerNameAndFlags(keyString string, existingFlags []string) (string, []string) {
	bracketIdx := strings.LastIndex(keyString, "[")

	if bracketIdx <= 0 || !strings.HasSuffix(keyString, "]") {
		return keyString, existingFlags
	}

	runnerName := strings.TrimSpace(keyString[:bracketIdx])
	flagsPart := strings.TrimSpace(keyString[bracketIdx+1 : utf8.RuneCountInString(keyString)-1])

	if flagsPart == "" {
		return runnerName, existingFlags
	}

	// Parse flags from bracket notation
	flagsList := strings.Split(flagsPart, ",")
	for i, flagName := range flagsList {
		if utils.SliceContainsValue(existingFlags, flagName) {
			continue // Skip duplicate flags
		}

		flagsList[i] = strings.TrimSpace(flagName)
	}

	return runnerName, flagsList
}

// convertStringFlagsToRunnerFlags converts string flag names to RunnerFlags struct
func convertStringFlagsToRunnerFlags(flagStrings []string) RunnerFlags {
	flagsConfig := RunnerFlags{}

	for _, flagName := range flagStrings {
		switch flagName {
		case "serial":
			flagsConfig.Serial = true

		case "dependent":
			flagsConfig.Dependent = true
		}
	}

	return flagsConfig
}

// findMatchingRunnerConfiguration finds the appropriate runner config based on input key
func findMatchingRunnerConfiguration(
	commandArgs []string,
	commandLineFlags RunnerFlags,
	retrieveCommandList bool,
) (
	commandsList []map[string]any,
	runnerName string,
	flagStrings []string,
	found bool,
	isInlineRunner bool,
	err error,
) {
	yamlConfig, _, configErr := getYamlConfiguration(true)
	if configErr != nil {
		return nil, "", nil, false, false, fmt.Errorf("Failed to load configuration from YAML file: %v", configErr)
	}

	// Extract base name from input key
	flagStrings = commandLineFlags.GetFlags()

	// Try exact match first
	if _, exactMatch := yamlConfig.Runners[commandArgs[0]]; exactMatch {
		if retrieveCommandList {
			commandsList, err = normalizeCommandList(yamlConfig.Runners[commandArgs[0]], commandArgs[0])
			if err != nil {
				return nil, "", nil, false, false, err
			}

			commandsList, err = sanitizeCommandList(commandsList)
			if err != nil {
				return nil, "", nil, false, false, err
			}
		}

		return commandsList, commandArgs[0], flagStrings, true, false, nil
	}

	inputBaseName, _ := extractRunnerNameAndFlags(commandArgs[0], []string{})

	// Try base name match
	if _, baseNameMatch := yamlConfig.Runners[inputBaseName]; baseNameMatch {
		if retrieveCommandList {
			commandsList, err = normalizeCommandList(yamlConfig.Runners[inputBaseName], inputBaseName)
			if err != nil {
				return nil, "", nil, false, false, err
			}

			commandsList, err = sanitizeCommandList(commandsList)
			if err != nil {
				return nil, "", nil, false, false, err
			}
		}

		return commandsList, inputBaseName, flagStrings, true, false, nil
	}

	// Try matching configuration keys that have the same base name
	for configKey := range yamlConfig.Runners {
		configKeyString := fmt.Sprintf("%v", configKey)
		configBaseName, configFlags := extractRunnerNameAndFlags(configKeyString, flagStrings)

		if configBaseName == inputBaseName {
			if retrieveCommandList {
				commandsList, err = normalizeCommandList(yamlConfig.Runners[configKey], configBaseName)
				if err != nil {
					return nil, "", nil, false, false, err
				}

				commandsList, err = sanitizeCommandList(commandsList)
				if err != nil {
					return nil, "", nil, false, false, err
				}
			}

			return commandsList, configBaseName, configFlags, true, false, nil
		}
	}

	// Check for inline runner match
	cmdNames := make(map[string]bool)

	for name, project := range yamlConfig.Projects {
		if len(project.Cmds) > 0 {
			for cmdName := range project.Cmds {
				cmdNames[name+":"+cmdName] = true
			}

			cmdNames[name+":*"] = true
		}
	}

	for name := range yamlConfig.Commands {
		cmdNames[name] = true
	}

	for _, arg := range commandArgs {
		if cmdNames[arg] {
			commandsList = append(commandsList, map[string]any{
				"cmd": arg,
			})
		} else {
			return nil, "", nil, false, false, fmt.Errorf("Found invalid command `%s` while building inline runner", arg)
		}
	}

	forceRunner := false
	if len(commandArgs) == 1 {
		if command, exists := commandsList[0]["cmd"].(string); exists {
			if strings.HasSuffix(command, ":*") {
				// If the command ends with `:*`, it is meant to run all commands
				// in the project and should be forced to execute as a runner
				forceRunner = true
			}
		}
	}

	commandsList, err = sanitizeCommandList(commandsList)
	if err != nil {
		return nil, "", nil, false, false, err
	}

	if len(commandsList) <= 1 && !forceRunner {
		// Ignoring runner with only one command. Will execute as a normal command
		return nil, "", nil, false, false, fmt.Errorf("Could not find any runner command")
	}

	// return inline runner name and flags
	return commandsList, "inline", flagStrings, true, true, nil
}

// GetFlags returns all active flags as string slice
func (runnerFlags *RunnerFlags) GetFlags() []string {
	var flagsList []string

	if runnerFlags.Serial {
		flagsList = append(flagsList, "serial")
	}

	if runnerFlags.Dependent {
		flagsList = append(flagsList, "dependent")
	}

	return flagsList
}
