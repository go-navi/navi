package navi

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-navi/navi/internal/logger"
	"github.com/go-navi/navi/internal/process"
	"github.com/go-navi/navi/internal/utils"
)

var processWg sync.WaitGroup

var NaviVersion = "1.0.0"

var HelpText = `Navi - Lightweight Command Runner

Usage:

1: Run 'navi' in the terminal to open the interactive CLI.

2: Run commands directly from the command line:
  navi [options] <runner-name> [args...]
  navi [options] <command-name> [args...]
  navi [options] <project:command> [args...]
  navi [options] <project> [args...]
  navi [options] [<command>, <project:command>, ...]

Examples:
  navi lint              Run predefined 'lint' single command
  navi web:dev           Run 'dev' command of the 'web' project
  navi web:*             Run all commands of the 'web' project
  navi web go build      Run 'go build' on the 'web' project folder
  navi start-all         Run predefined 'start-all' runner
  navi lint web:dev ...  Run multiple commands or project commands

Options:
  -f, --file <path>      Specify path to config file (default: ./navi.yml)
  -s, --serial           Run all runner commands sequentially
  -d, --dependent        Make all runner commands dependent
  -h, --help             Display this help message
  -v, --version          Display current version

See https://github.com/go-navi/navi for more information.`

// displayHelp prints usage instructions and exits the program
func displayHelp() {
	logger.Info(HelpText)
	os.Exit(0)
}

// displayVersion prints current version and exits the program
func displayVersion() {
	logger.Info(NaviVersion)
	os.Exit(0)
}

// gracefulShutdown attempts to cleanly terminate all running processes
func gracefulShutdown(ctx Ctx, signal string) {
	if process.TerminatingProcesses {
		return
	}

	process.TerminatingProcesses = true

	// Exit immediately if no processes are running
	if len(process.ProcessRegistry) == 0 {
		os.Exit(1)
	}

	logger.Warn("Shutting down processes... (don't close the terminal)")

	// Only terminate processes normally on non-Windows or when not triggered by a signal
	if runtime.GOOS != "windows" || signal == "" {
		if signal == "interrupt" {
			process.InterruptAll()
		} else {
			process.TerminateAll()
		}
	}

	// Wait for processes with a timeout
	waitWithTimeout(10*time.Second, func() {
		processWg.Wait()
	})

	suppressNewAfterCommands = true
	ctx.Cancel()
	process.KillAll()
	os.Exit(1)
}

// waitWithTimeout waits for a function to complete with a timeout
func waitWithTimeout(timeout time.Duration, fn func()) {
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()

	select {
	case <-done:
		// Function completed in time
	case <-time.After(timeout):
		logger.Warn("Child processes took too long to terminate. Forcing shutdown...")
	}
}

// globalVarsInit initializes global variables based on provided flags
func globalVarsInit(fileFlag string) error {
	// Handle explicit config file path
	if strings.TrimSpace(fileFlag) != "" {
		if info, err := os.Stat(fileFlag); os.IsNotExist(err) || info.IsDir() {
			return fmt.Errorf("Configuration file not found. Path: %s", fileFlag)
		}

		path, err := filepath.Abs(fileFlag)
		if err != nil {
			return fmt.Errorf("Failed to determine configuration file path: %w", err)
		}

		logger.Info("Using configuration file: %s", path)
		configurationPath = path
	}

	// Try to use default config in current directory
	if configurationPath == "" {
		path, err := filepath.Abs("navi.yml")
		if err != nil {
			return fmt.Errorf("Failed to determine configuration file on path `./navi.yml`")
		}

		if info, err := os.Stat(path); os.IsNotExist(err) || info.IsDir() {
			return fmt.Errorf("Configuration file not found: Path: %s", path)
		}

		configurationPath = path
	}

	applicationRootPath = strings.TrimSuffix(filepath.ToSlash(filepath.Dir(configurationPath)), "/")
	return nil
}

// processCommandError handles errors and executes after commands
func processCommandError(err error, projectCmd *ProjectCommand, ctx Ctx) {
	if err != nil && !errors.Is(err, ErrProcessTerminated) {
		logger.Error("%v", err)
	}

	// Execute after commands if applicable
	var afterErr error
	if projectCmd != nil && !projectCmd.AfterExecuted {
		afterErr = projectCmd.executeAfterHooks(err, ctx, false)
		if afterErr != nil {
			logger.Error("Fail during execution of after command(s): %v", afterErr)
		}
	}

	if errors.Is(err, ErrProcessTerminated) {
		return
	}

	if err != nil || afterErr != nil {
		gracefulShutdown(ctx, "")
	}
}

// entry point of the application
func Main() {
	// Parse command-line flags - consolidate flags with shared variables
	var fileFlag string
	var serialFlag, dependentFlag, helpFlag, versionFlag bool

	flag.StringVar(&fileFlag, "f", "", "")
	flag.StringVar(&fileFlag, "file", "", "Specify path to config file")
	flag.BoolVar(&serialFlag, "s", false, "")
	flag.BoolVar(&serialFlag, "serial", false, "Run all runner commands serially")
	flag.BoolVar(&dependentFlag, "d", false, "")
	flag.BoolVar(&dependentFlag, "dependent", false, "Make all runner commands dependent")
	flag.BoolVar(&helpFlag, "h", false, "")
	flag.BoolVar(&helpFlag, "help", false, "Display help information")
	flag.BoolVar(&versionFlag, "v", false, "")
	flag.BoolVar(&versionFlag, "version", false, "Display current version")
	flag.Parse()

	if helpFlag {
		displayHelp()
	}

	if versionFlag {
		displayVersion()
	}

	// Initialize global variables
	if err := globalVarsInit(fileFlag); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) == 0 {
		// Start interactive CLI
		var err error
		args, err = startCLI()
		if err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}

		if len(args) == 0 {
			logger.Error("Missing command arguments")
			os.Exit(1)
		}

		logger.Info("Selected `%s`", strings.Join(utils.AddQuotesToArgsWithSpaces(args), " "))
	}

	// Set up context and signal handling
	commandContext := createContext(context.Background())
	defer commandContext.Cancel()
	sigChan := process.SetupSignalHandling()

	// Handle OS signals in a goroutine
	go func() {
		sig := <-sigChan
		if !process.TerminatingProcesses {
			fmt.Print("\n")
			logger.Warn("Received `%s` signal", sig.String())
			gracefulShutdown(commandContext, sig.String())
		}
	}()

	// Runner flags from CLI options
	cliRunnerFlags := RunnerFlags{
		Serial:    serialFlag,
		Dependent: dependentFlag,
	}

	// Execute runner command if applicable
	if isRunnerConfigured(args) {
		err := executeRunner(commandContext, args, cliRunnerFlags)
		processCommandError(err, nil, commandContext)
	} else {
		// Find and execute a global command, a project command or a project with args
		_, _, found, _, _ := findGlobalCommandOrProjectOrProjectCommand(args)
		if !found {
			processCommandError(fmt.Errorf("Could not find `%s` in yaml configuration", args[0]), nil, commandContext)
		}

		projectCmd, _, err := getProjectCommand(args)
		processCommandError(err, nil, commandContext)

		err = projectCmd.execute(commandContext)
		processCommandError(err, projectCmd, commandContext)
	}

	// Wait for all processes to finish
	if process.TerminatingProcesses {
		<-make(chan any)
	}
}
