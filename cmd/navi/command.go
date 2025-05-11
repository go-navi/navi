package navi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-navi/navi/internal/logger"
	"github.com/go-navi/navi/internal/process"
	"github.com/go-navi/navi/internal/utils"
	"github.com/go-navi/navi/internal/watcher"
	"github.com/kballard/go-shellquote"
)

var suppressNewAfterCommands = false                             // Flag to prevent new after commands from starting
var logExecId = "naviLogId" + strconv.Itoa(rand.Intn(100000000)) // ID for command execution logs

// execute runs a command with automatic watch mode detection
func (cmd *ProjectCommand) execute(ctx Ctx) error {
	if hasFilesToWatch(cmd.WatchPatterns) {
		return cmd.executeWithFileWatcher(ctx)
	}
	return cmd.executeOneTime(ctx, nil)
}

// executeAfterHooks runs after-commands based on execution result
func (cmd *ProjectCommand) executeAfterHooks(executionErr error, ctx Ctx, isAfterChange bool) error {
	if (cmd.AfterCommand == nil && cmd.ProjAfterCommand == nil) || suppressNewAfterCommands {
		return nil
	}

	cmd.AfterExecuted = true
	defer processWg.Done()

	// Helper function to execute a set of after commands with common logic
	executeAfterCommandSet := func(afterCommandSet *ProjectCommand, isProjectLevel bool) error {
		if afterCommandSet == nil {
			return nil
		}

		executeRootAfter := true

		// Helper to run a specific after hook type
		runAfterHook := func(hookCmd *ProjectCommand, hookType string) error {
			if hookCmd == nil {
				return nil
			}

			hookCmd.LogPrefix = cmd.LogPrefix
			hookCmd.LogPrefixId = cmd.LogPrefixId
			hookCmd.LogPrefixColor = cmd.LogPrefixColor
			executeRootAfter = false

			if isProjectLevel {
				logger.InfoWithPrefix(cmd.GetLogPrefix(), "Running project-level `after%s` command...", hookType)
			} else {
				logger.InfoWithPrefix(cmd.GetLogPrefix(), "Running `after%s` command...", hookType)
			}

			return hookCmd.executeCommand(ctx, nil, true, true)
		}

		// Execute success hooks if the command succeeded
		if executionErr == nil {
			if afterErr := runAfterHook(afterCommandSet.AfterSuccessCommand, ".success"); afterErr != nil {
				return afterErr
			}
		}

		// Execute failure hooks if the command failed
		if executionErr != nil {
			if afterErr := runAfterHook(afterCommandSet.AfterFailureCommand, ".failure"); afterErr != nil {
				return afterErr
			}
		}

		// Execute file change hooks if command if a file change occurred in watch mode
		if isAfterChange {
			if afterErr := runAfterHook(afterCommandSet.AfterChangeCommand, ".change"); afterErr != nil {
				return afterErr
			}
		}

		// Always execute these hooks regardless of success/failure
		if afterErr := runAfterHook(afterCommandSet.AfterAlwaysCommand, ".always"); afterErr != nil {
			return afterErr
		}

		// If no specific hook matched, run the root after command
		if executeRootAfter {
			if afterErr := runAfterHook(afterCommandSet, ""); afterErr != nil {
				return afterErr
			}
		}

		return nil
	}

	// Execute command-level after hooks
	if afterErr := executeAfterCommandSet(cmd.AfterCommand, false); afterErr != nil {
		return afterErr
	}

	// Execute project-level after hooks
	if afterErr := executeAfterCommandSet(cmd.ProjAfterCommand, true); afterErr != nil {
		return afterErr
	}

	logger.InfoWithPrefix(cmd.GetLogPrefix(), "After command(s) completed successfully")
	return nil
}

// executeOneTime runs the command once and manages process lifecycle
func (cmd *ProjectCommand) executeOneTime(ctx Ctx, watchData *ExecuteWatchData) error {
	processWg.Add(1)
	defer processWg.Done()

	// Add waitgroup for after commands if they exist
	if cmd.AfterCommand != nil || cmd.ProjAfterCommand != nil {
		processWg.Add(1)
	}

	execError := cmd.executeCommand(ctx, watchData, false, true)
	if execError == nil {
		logger.InfoWithPrefix(cmd.GetLogPrefix(), "Command(s) completed successfully")
	}

	return execError
}

// preserveWatchModeErrorState retains watch mode restart errors
func preserveWatchModeErrorState(currentErr error, newError error) error {
	if errors.Is(currentErr, ErrWatchModeRestart) || errors.Is(currentErr, ErrProcessTerminated) {
		return currentErr
	}
	return newError
}

// executeCommand runs a command with its pre/post hooks
func (cmd *ProjectCommand) executeCommand(ctx Ctx, watchData *ExecuteWatchData, isAfterCmd, isMainCommand bool) error {
	// Determine if we have hooks to run
	hasHooks := false
	if isAfterCmd {
		hasHooks = cmd.PreCommand != nil || cmd.PostCommand != nil
	} else {
		hasHooks = cmd.PreCommand != nil || cmd.PostCommand != nil ||
			cmd.ProjPreCommand != nil || cmd.ProjPostCommand != nil
	}

	// Run pre-hooks first
	if err := cmd.executePreHooks(ctx, watchData, isAfterCmd); err != nil {
		return err
	}

	// Log main command execution
	if hasHooks && isMainCommand {
		logger.InfoWithPrefix(cmd.GetLogPrefix(), "Running main command...")
	}

	// Execute commands
	if err := cmd.executeSingleCommand(ctx, watchData, isAfterCmd, cmd.CommandList); err != nil {
		return err
	}

	// Run post-hooks last
	return cmd.executePostHooks(ctx, watchData, isAfterCmd)
}

// executePreHooks runs pre-execution hooks in proper order
func (cmd *ProjectCommand) executePreHooks(ctx Ctx, watchData *ExecuteWatchData, isAfterCmd bool) error {
	// Project-level pre commands run first
	if cmd.ProjPreCommand != nil && !isAfterCmd {
		cmd.ProjPreCommand.copyLogConfiguration(cmd)
		logger.InfoWithPrefix(cmd.GetLogPrefix(), "Running project-level `pre` command...")
		if err := cmd.ProjPreCommand.executeCommand(ctx, watchData, isAfterCmd, false); err != nil {
			return preserveWatchModeErrorState(err, fmt.Errorf("Project `pre` command failed: %v", err))
		}
	}

	// Command-level pre commands run next
	if cmd.PreCommand != nil {
		cmd.PreCommand.copyLogConfiguration(cmd)
		logger.InfoWithPrefix(cmd.GetLogPrefix(), "Running `pre` command...")
		if err := cmd.PreCommand.executeCommand(ctx, watchData, isAfterCmd, false); err != nil {
			return preserveWatchModeErrorState(err, fmt.Errorf("Command `pre` command failed: %v", err))
		}
	}

	return nil
}

// executePostHooks runs post-execution hooks in proper order
func (cmd *ProjectCommand) executePostHooks(ctx Ctx, watchData *ExecuteWatchData, isAfterCmd bool) error {
	// Command-level post commands run first
	if cmd.PostCommand != nil {
		cmd.PostCommand.copyLogConfiguration(cmd)
		logger.InfoWithPrefix(cmd.GetLogPrefix(), "Running `post` command...")
		if err := cmd.PostCommand.executeCommand(ctx, watchData, isAfterCmd, false); err != nil {
			return preserveWatchModeErrorState(err, fmt.Errorf("Command `post` command failed: %v", err))
		}
	}

	// Project-level post commands run last
	if cmd.ProjPostCommand != nil && !isAfterCmd {
		cmd.ProjPostCommand.copyLogConfiguration(cmd)
		logger.InfoWithPrefix(cmd.GetLogPrefix(), "Running project-level `post` command...")
		if err := cmd.ProjPostCommand.executeCommand(ctx, watchData, isAfterCmd, false); err != nil {
			return preserveWatchModeErrorState(err, fmt.Errorf("Project `post` command failed: %v", err))
		}
	}

	return nil
}

// copyLogConfiguration copies logging settings from source to target
func (targetCmd *ProjectCommand) copyLogConfiguration(sourceCmd *ProjectCommand) {
	targetCmd.LogPrefix = sourceCmd.LogPrefix
	targetCmd.LogPrefixId = sourceCmd.LogPrefixId
	targetCmd.LogPrefixColor = sourceCmd.LogPrefixColor
}

// executeSingleCommand runs a single system command
func (cmd *ProjectCommand) executeSingleCommand(ctx Ctx, watchData *ExecuteWatchData, isAfterCmd bool, cmdArgs []string) error {
	var execLogMap map[string]string
	var err error
	var cmdShell = cmd.Shell

	// Prepare command with default shell if not specified
	if strings.TrimSpace(cmd.Shell) == "" {
		if runtime.GOOS == "windows" {
			cmdShell = "cmd" // default windows shell
		} else {
			cmdShell = os.Getenv("SHELL") // default unix shell

			if strings.TrimSpace(cmdShell) == "" {
				if runtime.GOOS == "darwin" {
					cmdShell = "zsh" // On macOS, try to use zsh
				} else {
					cmdShell = "bash" // fallback to bash for other Unix systems
				}
			}
		}
	}

	cmdArgs, execLogMap, err = prepareShellCommands(cmdShell, cmdArgs)
	if err != nil {
		return err
	}

	// Create the OS command
	processCmd := exec.CommandContext(ctx.Ctx, cmdArgs[0], cmdArgs[1:]...)

	// Configure process group based on OS
	if runtime.GOOS == "windows" {
		process.SetupProcessGroup(processCmd)
	} else {
		process.SetupNewProcessGroup(processCmd)
	}

	// Set up working directory and environment
	processCmd.Dir = cmd.Dir
	processCmd.Env = append(os.Environ(), cmd.EnvVars...)
	processCmd.Env = append(processCmd.Env, "FORCE_COLOR=1") // Enable colors in output

	// Configure output handling
	waitForOutput, err := cmd.setupCommandOutputHandling(processCmd, execLogMap)
	if err != nil {
		return err
	}

	// Update watch group if needed
	if watchData != nil {
		watchData.ProcessWatchWg.Add(1)
	}

	// Start the command
	if err := processCmd.Start(); err != nil {
		if watchData != nil {
			watchData.ProcessWatchWg.Done()
		}

		// Handle special error cases
		if process.TerminatingProcesses && !isAfterCmd {
			return ErrProcessTerminated
		}

		if watchData != nil && errors.Is(watchData.ErrStatus, ErrWatchModeRestart) {
			return ErrWatchModeRestart
		}

		return fmt.Errorf("The command has failed with error `%v`", err)
	}

	// Register the process for tracking
	if isAfterCmd {
		process.RegisterAfter(processCmd)
	} else {
		process.Register(processCmd)
	}

	if watchData != nil {
		watchData.RunningCmd = processCmd
	}

	// Wait for command to complete
	if err := processCmd.Wait(); err != nil {
		if watchData != nil {
			watchData.ProcessWatchWg.Done()
		}

		waitForOutput()

		// Handle special error cases
		if process.TerminatingProcesses && !isAfterCmd {
			return ErrProcessTerminated
		}

		if watchData != nil && errors.Is(watchData.ErrStatus, ErrWatchModeRestart) {
			return ErrWatchModeRestart
		}

		return fmt.Errorf("The command has failed with exit code %v", err)
	}

	// Cleanup after successful execution
	if watchData != nil {
		watchData.ProcessWatchWg.Done()
	}

	waitForOutput()

	// Check for global termination
	if process.TerminatingProcesses && !isAfterCmd {
		return ErrProcessTerminated
	}

	if watchData != nil && errors.Is(watchData.ErrStatus, ErrWatchModeRestart) {
		return ErrWatchModeRestart
	}

	return nil
}

// executeWithFileWatcher runs a command with file watching capability
func (cmd *ProjectCommand) executeWithFileWatcher(parentCtx Ctx) error {
	logger.InfoWithPrefix(cmd.GetLogPrefix(), "Starting in watch mode")
	cmd.WatchExecuted = true

	// Create file watcher
	fileWatcher, err := watcher.NewFileWatcher(cmd.WatchPatterns, cmd.GetLogPrefix)
	if err != nil {
		return err
	}

	defer fileWatcher.Stop()

	// Start watching for file changes
	fileChangeEvents, err := fileWatcher.Start(cmd.GetLogPrefix)
	if err != nil {
		return err
	}

	// Variables for command execution management
	var (
		cancelCurrentCmd   context.CancelFunc
		debounceTimer      *time.Timer
		debounceDelayMs    = 500 * time.Millisecond // Delay to avoid multiple restarts
		isDebouncingActive = false
		isRestartInProcess = false
		commandErrorChan   = make(chan error, 1)
		watchExecutionData = ExecuteWatchData{
			ProcessWatchWg: sync.WaitGroup{},
			RunningCmd:     nil,
			ErrStatus:      nil,
		}
	)

	// Function to start or restart the command
	startOrRestartCommand := func() {
		if isRestartInProcess {
			return
		}

		isRestartInProcess = true
		previousCancel := cancelCurrentCmd
		cmdContext := createContext(parentCtx.Ctx)
		cancelCurrentCmd = cmdContext.Cancel

		// Stop previous command if it's running
		if previousCancel != nil && watchExecutionData.RunningCmd != nil {
			watchExecutionData.ErrStatus = ErrWatchModeRestart

			// Terminate the process based on OS
			if runtime.GOOS != "windows" {
				process.TerminateProcess(watchExecutionData.RunningCmd)
			} else {
				process.KillProcess(watchExecutionData.RunningCmd)
			}

			// Wait for process to terminate with timeout
			processDoneSignal := make(chan struct{})
			go func() {
				watchExecutionData.ProcessWatchWg.Wait()
				close(processDoneSignal)
			}()

			select {
			case <-processDoneSignal:
			case <-time.After(5 * time.Second):
				logger.Warn("Command took too long to stop. Forcing stop...")
			}

			// Force kill if still running
			previousCancel()
			process.KillProcess(watchExecutionData.RunningCmd)
		}

		// Start the command in a new goroutine
		go func() {
			watchExecutionData.RunningCmd = nil
			watchExecutionData.ErrStatus = nil
			isRestartInProcess = false

			// Execute the command and any after hooks
			execErr := cmd.executeOneTime(cmdContext, &watchExecutionData)
			afterErr := cmd.executeAfterHooks(execErr, cmdContext, true)
			if afterErr != nil {
				logger.ErrorWithPrefix(cmd.GetLogPrefix(), "Fail during execution of after command(s) in watch mode: %v", afterErr)
				gracefulShutdown(parentCtx, "")
			}

			// Handle command errors
			if execErr != nil {
				if errors.Is(execErr, ErrWatchModeRestart) {
					watchExecutionData.ErrStatus = nil
				} else {
					watchExecutionData.RunningCmd = nil
					watchExecutionData.ErrStatus = nil
					select {
					case commandErrorChan <- execErr:
						// Error sent to channel
					default:
						// Chanel is full, ignore the error
					}
				}
			} else {
				watchExecutionData.RunningCmd = nil
				watchExecutionData.ErrStatus = nil
			}
		}()
	}

	// Start the command initially
	startOrRestartCommand()
	defer cancelCurrentCmd()

	// Main event loop
	for {
		select {
		case <-fileChangeEvents:
			if !process.TerminatingProcesses {
				// Handle file change events with debouncing
				if isDebouncingActive {
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
				} else {
					isDebouncingActive = true

					if !isRestartInProcess {
						logger.InfoWithPrefix(cmd.GetLogPrefix(), "File change detected. Stopping running command...")
					}
				}

				// Restart command after debounce period
				debounceTimer = time.AfterFunc(debounceDelayMs, func() {
					isDebouncingActive = false
					startOrRestartCommand()
				})
			}

		case err := <-commandErrorChan:
			// Handle command errors
			if cancelCurrentCmd != nil {
				cancelCurrentCmd()
			}

			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			return err

		case <-parentCtx.Done():
			// Handle parent context cancellation (exit)
			if cancelCurrentCmd != nil {
				cancelCurrentCmd()
			}

			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			return nil
		}
	}
}

// hasFilesToWatch checks if there are files to watch
func hasFilesToWatch(patterns watcher.FilePatterns) bool {
	return len(patterns.Include) > 0
}

// getProjectCommand creates a ProjectCommand from command arguments
func getProjectCommand(args []string) (projectCmd *ProjectCommand, projectNotFound bool, err error) {
	var mainCommand any
	var projectConfig ProjectConfig
	var commandIdentifier string
	var extraArgs []string

	if len(args) > 1 {
		extraArgs = args[1:]
	}

	yamlConfig, _, err := getYamlConfiguration(true)
	if err != nil {
		return nil, false, fmt.Errorf("Failed to load configuration from YAML file: %v", err)
	}

	projectName, commandName, found, isGlobalCommand, err := findGlobalCommandOrProjectOrProjectCommand(args)
	if err != nil {
		return nil, !found, err
	}

	if isGlobalCommand {
		mainCommand = yamlConfig.Commands[commandName]
		commandIdentifier = commandName
	} else {
		projectConfig = yamlConfig.Projects[projectName]

		if strings.TrimSpace(projectConfig.Dir) == "" {
			return nil, false, fmt.Errorf("Project `%s` is missing required `dir` property in configuration", projectName)
		}

		commandIdentifier = projectName

		if commandName != "" {
			// Handle `project:command` format
			definedCommand, exists := projectConfig.Cmds[commandName]
			if !exists {
				return nil, false, fmt.Errorf("Command `%s` not found in project `%s`", commandName, projectName)
			}

			mainCommand = definedCommand
			commandIdentifier = projectName + ":" + commandName
		} else {
			// Handle `project args` format
			if len(extraArgs) == 0 {
				return nil, false, fmt.Errorf("Missing command to execute in project `%s`", projectName)
			}

			mainCommand = strings.Join(utils.AddQuotesToArgsWithSpaces(extraArgs), " ")
			extraArgs = nil
		}
	}

	// Resolve project directory path
	if projectConfig.Dir == "" {
		projectConfig.Dir = "."
	}

	projectConfig.Dir = resolveFilePath(projectConfig.Dir, applicationRootPath)

	// Build the main command
	projectCommand, err := buildProjectCommand(
		mainCommand, projectConfig.Env, projectConfig.Dotenv, []string{}, projectConfig.Watch,
		projectConfig.Shell, projectConfig.Dir, commandName, projectName, false, isGlobalCommand,
	)
	if err != nil {
		return nil, false, err
	}

	// Build pre, post and after commands
	if projectConfig.Pre != nil {
		projectCommand.ProjPreCommand, err = buildProjectCommand(
			projectConfig.Pre, projectConfig.Env, projectConfig.Dotenv, []string{}, nil,
			projectConfig.Shell, projectConfig.Dir, "pre", projectName, false, isGlobalCommand,
		)
		if err != nil {
			return nil, false, err
		}
	}

	if projectConfig.Post != nil {
		projectCommand.ProjPostCommand, err = buildProjectCommand(
			projectConfig.Post, projectConfig.Env, projectConfig.Dotenv, []string{}, nil,
			projectConfig.Shell, projectConfig.Dir, "post", projectName, false, isGlobalCommand,
		)
		if err != nil {
			return nil, false, err
		}
	}

	if projectConfig.After != nil {
		projectCommand.ProjAfterCommand, err = buildProjectCommand(
			projectConfig.After, projectConfig.Env, projectConfig.Dotenv, []string{}, nil,
			projectConfig.Shell, projectConfig.Dir, "after", projectName, true, isGlobalCommand,
		)
		if err != nil {
			return nil, false, err
		}
	}

	// Apply extra args if any
	if len(extraArgs) > 0 {
		lastIdx := len(projectCommand.CommandList) - 1
		projectCommand.CommandList[lastIdx] += " " + strings.Join(utils.AddQuotesToArgsWithSpaces(extraArgs), " ")
	}

	projectCommand.Identifier = commandIdentifier
	return projectCommand, false, nil
}

// findGlobalCommandOrProjectOrProjectCommand checks for existing, in order:
// 1. Global command (e.g., `mycli`)
// 2. Project command (e.g., `myproject:mycommand`)
// 3. Project  (e.g., `myproject`)
func findGlobalCommandOrProjectOrProjectCommand(commandArgs []string) (projectName, commandName string, found, isGlobalCommand bool, err error) {
	// look for the command in the global commands
	yamlConfig, _, err := getYamlConfiguration(true)
	if err != nil {
		return "", "", true, false, fmt.Errorf("Failed to load configuration from YAML file: %v", err)
	}

	if _, ok := yamlConfig.Commands[commandArgs[0]]; ok {
		return "", commandArgs[0], true, true, nil
	}

	// look for project or project command
	for name, cfg := range yamlConfig.Projects {
		if projectName != "" || commandName != "" {
			break
		}

		if name == commandArgs[0] {
			projectName = name
			break
		}

		for cmdName := range cfg.Cmds {
			if commandArgs[0] == name+":"+cmdName {
				projectName = name
				commandName = cmdName
				break
			}
		}
	}

	if projectName == "" && commandName == "" {
		for name := range yamlConfig.Projects {
			if strings.HasPrefix(commandArgs[0], name+":") {
				projectName = name
				commandName = strings.TrimPrefix(commandArgs[0], name+":")
				break
			}
		}
	}

	if projectName == "" && commandName == "" {
		parts := strings.SplitN(commandArgs[0], ":", 2)

		if len(parts) >= 1 {
			projectName = parts[0]
		}

		if len(parts) == 2 {
			commandName = parts[1]
		}
	}

	if _, hasProject := yamlConfig.Projects[projectName]; hasProject {
		return projectName, commandName, true, false, nil
	}

	return "", "", false, false, fmt.Errorf("Command, project or project command `%s` was not found in configuration", commandArgs[0])
}

// buildProjectCommand creates a ProjectCommand from configuration
func buildProjectCommand(
	commandRaw any,
	commandEnv map[string]string,
	commandDotEnv any,
	envVars []string,
	commandWatch any,
	commandShell string,
	commandPath string,
	cmdName string,
	projName string,
	isAfterCmd bool,
	isGlobalCommand bool,
) (*ProjectCommand, error) {
	// Load environment variables from dotenv files
	envVarsFromDotEnv, err := loadEnvironmentVariables(parseDotEnvConfiguration(commandDotEnv, commandPath))
	if err != nil {
		return nil, err
	}

	// Combine environment variables
	combinedEnvVars := append(envVars, envVarsFromDotEnv...)
	combinedEnvVars = append(combinedEnvVars, formatEnvironmentMap(commandEnv)...)
	effectiveShell := commandShell
	effectivePath := commandPath

	// Parse file watch patterns
	includePatterns, excludePatterns, err := parseWatchPatterns(commandWatch, projName, cmdName, isGlobalCommand)
	if err != nil {
		return nil, err
	}

	watchPatterns := watcher.FilePatterns{
		Include: []string{},
		Exclude: []string{},
	}

	// Process include/exclude patterns
	if len(includePatterns) > 0 {
		normalizedIncludes := processGlobPatterns(includePatterns, effectivePath)
		watchPatterns.Include = append(watchPatterns.Include, normalizedIncludes...)
	}

	if len(excludePatterns) > 0 {
		normalizedExcludes := processGlobPatterns(excludePatterns, effectivePath)
		watchPatterns.Exclude = append(watchPatterns.Exclude, normalizedExcludes...)
	}

	// Handle command types
	switch command := commandRaw.(type) {
	case map[string]any: // Complex command configuration
		return buildCommandFromMap(
			command, commandEnv, commandDotEnv, envVars, combinedEnvVars,
			commandWatch, commandShell, commandPath, effectivePath,
			watchPatterns, cmdName, projName, isAfterCmd, isGlobalCommand,
		)
	case any: // Simple command string or list
		commandList, ok := convertToStringList(command)
		if !ok {
			if isGlobalCommand {
				return nil, fmt.Errorf("Command `%s` must be a command or a list of commands", cmdName)
			}
			return nil, fmt.Errorf("Command `%s` in project `%s` must be a command or a list of commands", cmdName, projName)
		}

		return &ProjectCommand{
			Dir:           effectivePath,
			EnvVars:       combinedEnvVars,
			Shell:         effectiveShell,
			WatchPatterns: watchPatterns,
			CommandList:   commandList,
		}, nil
	}

	if strings.TrimSpace(cmdName) == "" && !isGlobalCommand {
		return nil, fmt.Errorf("Invalid command format in project `%s`", projName)
	}

	if isGlobalCommand {
		return nil, fmt.Errorf("Invalid format for `%s` command", cmdName)
	} else {
		return nil, fmt.Errorf("Invalid format for `%s` command in project `%s`", cmdName, projName)
	}
}

// processGlobPatterns normalizes a list of glob patterns
func processGlobPatterns(patterns []string, basePath string) []string {
	result := []string{}
	for _, pattern := range patterns {
		fullPattern := normalizeGlobPattern(pattern, basePath)
		result = append(result, []string{fullPattern}...)
	}
	return result
}

// buildCommandFromMap creates a command from a complex configuration map
func buildCommandFromMap(
	commandMap map[string]any,
	commandEnv map[string]string,
	commandDotEnv any,
	envVars []string,
	parentEnvVars []string,
	commandWatch any,
	commandShell string,
	commandPath string,
	parentPath string,
	parentWatchPatterns watcher.FilePatterns,
	cmdName string,
	projName string,
	isAfterCmd bool,
	isGlobalCommand bool,
) (*ProjectCommand, error) {
	projectCmd := &ProjectCommand{}
	var err error

	// Special handling for after commands
	if isAfterCmd {
		buildAfterCommandHook := func(cmdRaw any, afterCmdName string) (*ProjectCommand, error) {
			return buildProjectCommand(
				cmdRaw, commandEnv, commandDotEnv, envVars,
				commandWatch, commandShell, commandPath,
				afterCmdName, projName, false, isGlobalCommand,
			)
		}

		// Process different types of after hooks
		afterHookTypes := []struct {
			key         string
			destination **ProjectCommand
		}{
			{"success", &projectCmd.AfterSuccessCommand},
			{"failure", &projectCmd.AfterFailureCommand},
			{"change", &projectCmd.AfterChangeCommand},
			{"always", &projectCmd.AfterAlwaysCommand},
		}

		// Build each defined after hook
		hasAfterSubcommand := false
		for _, hookType := range afterHookTypes {
			if hookCmd, exists := commandMap[hookType.key]; exists {
				*hookType.destination, err = buildAfterCommandHook(hookCmd, "after."+hookType.key)
				if err != nil {
					return nil, err
				}
				hasAfterSubcommand = true
			}
		}

		if hasAfterSubcommand {
			return projectCmd, nil
		}
	}

	// Parse command configuration
	cmdConfig, err := parseCommandMap(commandMap, cmdName, projName, isGlobalCommand)
	if err != nil {
		return nil, err
	}

	// Apply shell override if specified
	effectiveShell := commandShell
	if strings.TrimSpace(cmdConfig.Shell) != "" {
		effectiveShell = cmdConfig.Shell
	}

	// Resolve command directory
	commandWorkingDir := resolveFilePath(cmdConfig.Dir, parentPath)

	// Load dotenv variables specific to this command
	envVarsFromDotEnv, err := loadEnvironmentVariables(parseDotEnvConfiguration(cmdConfig.Dotenv, commandWorkingDir))
	if err != nil {
		return nil, err
	}

	// Process command-specific watch patterns
	effectiveWatchPatterns := parentWatchPatterns
	if len(cmdConfig.WatchPatterns.Include) > 0 {
		normalizedIncludes := processGlobPatterns(cmdConfig.WatchPatterns.Include, commandWorkingDir)
		effectiveWatchPatterns.Include = append(effectiveWatchPatterns.Include, normalizedIncludes...)
	}

	if len(cmdConfig.WatchPatterns.Exclude) > 0 {
		normalizedExcludes := processGlobPatterns(cmdConfig.WatchPatterns.Exclude, commandWorkingDir)
		effectiveWatchPatterns.Exclude = append(effectiveWatchPatterns.Exclude, normalizedExcludes...)
	}

	// Combine all environment variables
	combinedEnvVars := append(parentEnvVars, envVarsFromDotEnv...)
	combinedEnvVars = append(combinedEnvVars, formatEnvironmentMap(cmdConfig.Env)...)

	// Set up the project command
	projectCmd.Dir = commandWorkingDir
	projectCmd.EnvVars = combinedEnvVars
	projectCmd.WatchPatterns = effectiveWatchPatterns
	projectCmd.Shell = effectiveShell
	projectCmd.CommandList = cmdConfig.Run

	// Process hooks: after, pre, and post
	if !isAfterCmd && cmdConfig.After != nil {
		projectCmd.AfterCommand, err = buildProjectCommand(
			cmdConfig.After, cmdConfig.Env, cmdConfig.Dotenv, combinedEnvVars,
			nil, effectiveShell, commandWorkingDir, "after", projName, true, isGlobalCommand,
		)
		if err != nil {
			return nil, err
		}
	}

	if cmdConfig.Pre != nil {
		projectCmd.PreCommand, err = buildProjectCommand(
			cmdConfig.Pre, cmdConfig.Env, cmdConfig.Dotenv, combinedEnvVars,
			nil, effectiveShell, commandWorkingDir, "pre", projName, false, isGlobalCommand,
		)
		if err != nil {
			return nil, err
		}
	}

	if cmdConfig.Post != nil {
		projectCmd.PostCommand, err = buildProjectCommand(
			cmdConfig.Post, cmdConfig.Env, cmdConfig.Dotenv, combinedEnvVars,
			nil, effectiveShell, commandWorkingDir, "post", projName, false, isGlobalCommand,
		)
		if err != nil {
			return nil, err
		}
	}

	return projectCmd, nil
}

// setupCommandOutputHandling configures stdout/stderr for command execution
func (cmd *ProjectCommand) setupCommandOutputHandling(processCmd *exec.Cmd, execLogMap map[string]string) (func(), error) {
	// Create pipes for stdout and stderr
	stdoutPipe, err := processCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve stdout logs from a command being executed: %v", err)
	}

	stderrPipe, err := processCmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve stderr logs from a command being executed: %v", err)
	}

	// Set up goroutines to process output
	var outputWg sync.WaitGroup
	outputWg.Add(2)

	// Process stdout with prefix
	go func() {
		defer outputWg.Done()
		scanner := bufio.NewScanner(stdoutPipe)

		for scanner.Scan() {
			log := scanner.Text()
			logId := strings.TrimSpace(log)

			if execLog, exists := execLogMap[logId]; exists {
				if !utils.IsRunningInTestMode() {
					colorGreen := "\033[0;32m"
					colorReset := "\033[0m"
					execLog = colorGreen + execLog + colorReset
				}

				if cmd.LogPrefix == "" {
					fmt.Println(execLog)
				} else {
					fmt.Println(cmd.GetLogPrefix() + " " + execLog)
				}
			} else if cmd.LogPrefix == "" {
				fmt.Println(log)
			} else {
				fmt.Println(cmd.GetLogPrefix() + " " + log)
			}
		}
	}()

	// Process stderr with prefix
	go func() {
		defer outputWg.Done()
		scanner := bufio.NewScanner(stderrPipe)

		for scanner.Scan() {
			if cmd.LogPrefix == "" {
				fmt.Printf("%s\n", scanner.Text())
			} else {
				fmt.Printf("%s %s\n", cmd.GetLogPrefix(), scanner.Text())
			}
		}
	}()

	return outputWg.Wait, nil
}

// prepareShellCommands wraps a command with the appropriate shell
func prepareShellCommands(shell string, cmdArgs []string) (result []string, execLogMap map[string]string, err error) {
	shellName := strings.TrimSpace(shell)
	cmdArgsWithLogIds := []string{}
	execLogMap = make(map[string]string)

	for idx, arg := range cmdArgs {
		id := logExecId + "_" + strconv.Itoa(idx+1)
		execLogMap[id] = "Executing `" + arg + "`"

		if shell == "cmd" && runtime.GOOS == "windows" {
			if idx > 0 {
				cmdArgsWithLogIds = append(cmdArgsWithLogIds, "&&")
			}

			cmdArgsWithLogIds = append(cmdArgsWithLogIds, "echo", id, "&&")
			splitQuotedArgs, err := shellquote.Split(arg)
			if err != nil {
				return nil, nil, fmt.Errorf("Invalid format for command `%s`", arg)
			}

			splitArgs := strings.Split(arg, " ")
			if len(splitArgs) == len(splitQuotedArgs) {
				cmdArgsWithLogIds = append(cmdArgsWithLogIds, splitArgs...)
			} else {
				cmdArgsWithLogIds = append(cmdArgsWithLogIds, splitQuotedArgs...)
			}
		} else {
			cmdArgsWithLogIds = append(cmdArgsWithLogIds, "echo "+id, arg)
		}
	}

	cmdArgs = cmdArgsWithLogIds

	if runtime.GOOS == "windows" {
		if shellName == "powershell" {
			result = []string{"powershell", "-Command", strings.Join(cmdArgs, " ; ")}
		} else {
			result = append([]string{shellName, "/C"}, cmdArgs...)
		}
	} else { // unix
		result = []string{shellName, "-c", strings.Join(cmdArgs, " && ")}
	}

	return result, execLogMap, nil
}

// parseCommandMap extracts command settings from a YAML map structure
func parseCommandMap(cmdData map[string]any, cmdName, projName string, isGlobalCommand bool) (CommandConfig, error) {
	cmdConfig := CommandConfig{}

	// Check for required run field
	rawCmd, exists := cmdData["run"]
	if !exists {
		if isGlobalCommand {
			return cmdConfig, fmt.Errorf("Missing required `run` field for command `%s`", cmdName)
		}
		return cmdConfig, fmt.Errorf("Missing required `run` field for command `%s` in project `%s`", cmdName, projName)
	}

	// Parse the command(s) to run
	commandList, isValidFormat := convertToStringList(rawCmd)
	if !isValidFormat {
		if isGlobalCommand {
			return cmdConfig, fmt.Errorf("The `run` field for command `%s` must be a command or a list of commands", cmdName)
		}
		return cmdConfig, fmt.Errorf("The `run` field of command `%s` in project `%s` must be a command or a list of commands", cmdName, projName)
	}

	cmdConfig.Run = commandList

	// Initialize watch patterns
	cmdConfig.WatchPatterns = struct {
		Include []string
		Exclude []string
	}{
		Include: []string{},
		Exclude: []string{},
	}

	// Parse watch patterns if present
	if watch, exists := cmdData["watch"]; exists {
		include, exclude, err := parseWatchPatterns(watch, projName, cmdName, isGlobalCommand)
		if err != nil {
			return cmdConfig, err
		}
		cmdConfig.WatchPatterns.Exclude = exclude
		cmdConfig.WatchPatterns.Include = include
	}

	// Parse simple string fields
	if dir, exists := cmdData["dir"].(string); exists {
		cmdConfig.Dir = dir
	}

	if shell, exists := cmdData["shell"].(string); exists {
		cmdConfig.Shell = shell
	}

	// Parse dotenv field
	if dotenv, exists := cmdData["dotenv"]; exists {
		cmdConfig.Dotenv = dotenv
	}

	// Parse environment variables
	if env, exists := cmdData["env"]; exists {
		cmdConfig.Env = make(map[string]string)
		if envData, exists := env.(map[string]any); exists {
			for keyStr, envVal := range envData {
				cmdConfig.Env[keyStr] = convertYamlValueToString(envVal)
			}
		}
	}

	// Parse hook commands
	if pre, exists := cmdData["pre"]; exists {
		cmdConfig.Pre = pre
	}

	if post, exists := cmdData["post"]; exists {
		cmdConfig.Post = post
	}

	if after, exists := cmdData["after"]; exists {
		cmdConfig.After = after
	}

	return cmdConfig, nil
}

// parseWatchPatterns parses file watch patterns from configuration
func parseWatchPatterns(watchData any, projName, cmdName string, isGlobalCommand bool) (includePatterns []string, excludePatterns []string, err error) {
	if watchData == nil {
		return nil, nil, nil
	}

	// Handle map format with include/exclude keys
	if watchMap, ok := watchData.(map[string]any); ok {
		includePatterns := []string{}
		if includeData, hasInclude := watchMap["include"]; hasInclude {
			includePatterns, ok = convertToStringList(includeData)
			if !ok {
				if isGlobalCommand {
					return nil, nil, fmt.Errorf("Parameter `watch.include` in command `%s` must be a list of Glob patterns", cmdName)
				}
				return nil, nil, fmt.Errorf("Parameter `watch.include` in project `%s` must be a list of Glob patterns", projName)
			}
		}

		excludePatterns := []string{}
		if excludeData, hasExclude := watchMap["exclude"]; hasExclude {
			excludePatterns, ok = convertToStringList(excludeData)
			if !ok {
				if isGlobalCommand {
					return nil, nil, fmt.Errorf("Parameter `watch.exclude` in command `%s` must be a list of Glob patterns", cmdName)
				}
				return nil, nil, fmt.Errorf("Parameter `watch.exclude` in project `%s` must be a list of Glob patterns", projName)
			}
		}

		return includePatterns, excludePatterns, nil
	}

	// Handle simple string or array format
	if patterns, ok := convertToStringList(watchData); ok {
		return patterns, nil, nil
	}

	if isGlobalCommand {
		return nil, nil, fmt.Errorf("Parameter `watch` in command `%s` must be a list of Glob patterns", cmdName)
	}
	return nil, nil, fmt.Errorf("Parameter `watch` in project `%s` must be a list of Glob patterns", projName)
}

// convertToStringList converts YAML values to string list
func convertToStringList(value any) ([]string, bool) {
	// Handle single string
	if strValue, isString := value.(string); isString {
		return []string{strValue}, true
	}

	// Handle array of strings
	if listValue, isList := value.([]any); isList {
		stringList := []string{}
		for _, item := range listValue {
			if strItem, isString := item.(string); isString {
				stringList = append(stringList, strItem)
			}
		}
		return stringList, true
	}

	return nil, false
}

// normalizeGlobPattern converts a glob pattern to a platform-independent format
func normalizeGlobPattern(pattern, parentPath string) string {
	hasTrailingSlash := strings.HasSuffix(pattern, "/") || strings.HasSuffix(pattern, "\\")
	cleanPattern := filepath.Clean(pattern)
	resolvedPattern := resolveFilePath(cleanPattern, parentPath)
	normalizedPattern := filepath.ToSlash(resolvedPattern)

	// Handle patterns that should end with a trailing slash
	if hasTrailingSlash {
		normalizedPattern = utils.EnsureSuffix(normalizedPattern, "/")
		return normalizedPattern
	}

	// Handle special directory references
	if pattern == "." || strings.HasSuffix(pattern, "/.") || strings.HasSuffix(pattern, "\\.") ||
		pattern == ".." || strings.HasSuffix(pattern, "/..") || strings.HasSuffix(pattern, "\\..") {
		normalizedPattern = utils.EnsureSuffix(normalizedPattern, "/")
		return normalizedPattern
	}

	// Handle wildcard patterns
	if pattern == "*" || strings.HasSuffix(pattern, "/*") || strings.HasSuffix(pattern, "\\*") {
		if strings.HasSuffix(normalizedPattern, "/*") {
			normalizedPattern = strings.TrimSuffix(normalizedPattern, "*")
		} else {
			normalizedPattern = utils.EnsureSuffix(normalizedPattern, "/")
		}
		return normalizedPattern
	}

	return normalizedPattern
}

// GetLogPrefix returns a formatted log prefix for this command
func (cmd *ProjectCommand) GetLogPrefix() string {
	if strings.TrimSpace(cmd.LogPrefixId) == "" ||
		strings.TrimSpace(cmd.LogPrefix) == "" ||
		strings.TrimSpace(cmd.LogPrefixColor) == "" {
		return ""
	}

	// Add ID to prefix only when multiple commands share same prefix
	logPrefixCounterMutex.Lock()
	showId := logPrefixCounterMap[cmd.LogPrefix] > 1
	logPrefixCounterMutex.Unlock()
	logPrefix := cmd.LogPrefix

	// Truncate prefix if it's too long (over 50 chars)
	if utf8.RuneCountInString(logPrefix) > 50 {
		logPrefix = logPrefix[:47] + "..."
	}

	return logger.GetColorizedPrefix(cmd.LogPrefixId, logPrefix, cmd.LogPrefixColor, showId)
}
