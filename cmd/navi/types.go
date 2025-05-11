package navi

import (
	"context"
	"os/exec"
	"sync"

	"github.com/go-navi/navi/internal/watcher"
)

// YamlConfig represents the top-level navi.yml structure
type YamlConfig struct {
	Projects map[string]ProjectConfig // Project definitions
	Runners  map[string]any           // Runner definitions
	Commands map[string]any           // Command definitions
}

// ProjectConfig defines a project's settings in the YAML file
type ProjectConfig struct {
	Dir    string            // Project directory
	Cmds   map[string]any    // Available commands
	Pre    any               // Commands before main execution
	Post   any               // Commands after main execution
	After  any               // Commands after completion
	Dotenv any               // Environment file settings
	Watch  any               // Files to watch for changes
	Env    map[string]string // Environment variables
	Shell  string            // Shell for execution
}

// RunnerFlags controls command execution flow behavior
type RunnerFlags struct {
	Serial    bool // Execute commands sequentially
	Dependent bool // All commands stop if any fails
}

// RunnerCommand defines command execution parameters
type RunnerCommand struct {
	Cmd       string  // Command to execute
	Name      string  // Display name
	Delay     float64 // Pre-execution delay in seconds
	Restart   any     // Restart settings
	Awaits    any     // Ports to wait for
	Serial    bool    // Block subsequent commands
	Dependent bool    // Stop all on failure
}

// RunnerExecution manages command execution state
type RunnerExecution struct {
	projectCmd       *ProjectCommand // Processed command
	runnerCmd        *RunnerCommand  // Runner configuration
	runnerCmdStr     string          // Original command string
	maxRetries       int             // Retry count (0 = infinite)
	retryInterval    float64         // Seconds between retries
	enableRestart    bool            // Auto-restart flag
	restartCondition string          // Restart condition
}

// ProjectCommand contains a fully parsed command ready for execution
type ProjectCommand struct {
	Identifier          string               // Command identifier
	Dir                 string               // Working directory
	Tokens              [][]string           // Parsed command arguments
	EnvVars             []string             // Environment variables
	ProjPreCommand      *ProjectCommand      // Project pre-hook
	PreCommand          *ProjectCommand      // Command pre-hook
	PostCommand         *ProjectCommand      // Command post-hook
	ProjPostCommand     *ProjectCommand      // Project post-hook
	AfterCommand        *ProjectCommand      // Command after-hook
	AfterSuccessCommand *ProjectCommand      // Success after-hook
	AfterFailureCommand *ProjectCommand      // Failure after-hook
	AfterAlwaysCommand  *ProjectCommand      // Always after-hook
	AfterChangeCommand  *ProjectCommand      // File change after-hook
	AfterExecuted       bool                 // After hooks executed flag
	ProjAfterCommand    *ProjectCommand      // Project after-hook
	Shell               string               // Shell for execution
	WatchPatterns       watcher.FilePatterns // File watch patterns
	WatchExecuted       bool                 // Watch mode executed flag
	OriginalCmds        []string             // Raw command strings
	LogPrefix           string               // Log prefix text
	LogPrefixId         string               // Log prefix ID
	LogPrefixColor      string               // Log prefix color
}

// CommandConfig is an intermediate representation during command building
type CommandConfig struct {
	Run           []string             // Commands to run
	Dir           string               // Working directory
	Pre           any                  // Pre-command hooks
	Post          any                  // Post-command hooks
	After         any                  // After-command hooks
	Dotenv        any                  // Environment files
	WatchPatterns watcher.FilePatterns // Watch patterns
	Env           map[string]string    // Environment variables
	Shell         string               // Shell for execution
}

// DotEnvConfig defines environment file loading configuration
type DotEnvConfig struct {
	Files []DotEnvFile // Environment files to process
	Valid bool         // Config validation status
}

// DotEnvFile specifies an environment file with optional key filtering
type DotEnvFile struct {
	Path string   // Path to the .env file
	Keys []string // Specific keys to load (empty = all)
}

// Ctx wraps a context with its cancel function
type Ctx struct {
	Ctx    context.Context    // Context object
	Cancel context.CancelFunc // Cancellation function
}

// ExecuteWatchData tracks state for watch-mode execution
type ExecuteWatchData struct {
	ProcessWatchWg sync.WaitGroup // Process synchronization
	RunningCmd     *exec.Cmd      // Active command
	ErrStatus      error          // Error state
}

// CommandHandlers contains functions to handle different command events
type CommandHandlers struct {
	serialFailure       func(RunnerCommand)
	dependentCompletion func(RunnerCommand)
}
