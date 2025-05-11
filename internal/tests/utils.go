package tests

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-navi/navi/internal/logger"
	"github.com/go-navi/navi/internal/port"
	"github.com/go-navi/navi/internal/process"
	"github.com/go-navi/navi/internal/utils"
)

// Global variables for test coordination and control
var (
	testWaitGroup              sync.WaitGroup
	activeCommand              *exec.Cmd
	displayOutputInRealtime    bool
	TestShutdownInProgress     = false
	ErrCommandShouldHaveFailed = errors.New("Expected command to fail, but got no error")
)

// TestResult contains the output and metadata from a test execution
type TestResult struct {
	T             *testing.T
	CommandArgs   []string
	WorkingDir    string
	ExecutionTime time.Duration
	CommandOutput string
}

// CleanupFunctions stores and manages functions that revert test changes
type CleanupFunctions struct {
	Actions []func()
}

// Add registers a cleanup function
func (cf *CleanupFunctions) Add(fn func()) {
	cf.Actions = append(cf.Actions, fn)
}

// ExecuteAll runs all cleanup functions in reverse order
func (cf *CleanupFunctions) ExecuteAll() {
	for i := len(cf.Actions) - 1; i >= 0; i-- {
		cf.Actions[i]()
	}
	cf.Actions = []func(){}
}

// captureCommandOutput sets up pipes to collect stdout and stderr from a command
func captureCommandOutput(cmd *exec.Cmd) (func() string, error) {
	capturedText := ""
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve stdout logs: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve stderr logs: %v", err)
	}

	var streamWaitGroup sync.WaitGroup
	streamWaitGroup.Add(2)

	// Process a single output stream (either stdout or stderr)
	processOutputStream := func(stream io.Reader, done func()) {
		defer done()
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()
			capturedText += line + "\n"
			if displayOutputInRealtime {
				fmt.Printf("\033[0;32moutput ⟫\033[0m %s\n", line)
			}
		}
	}

	// Start capturing both output streams
	go processOutputStream(stdout, streamWaitGroup.Done)
	go processOutputStream(stderr, streamWaitGroup.Done)

	// Return a function that waits for all output and returns it
	return func() string {
		streamWaitGroup.Wait()
		return capturedText
	}, nil
}

// GracefullyKillOrTerminate handles process termination based on OS
func GracefullyKillOrTerminate(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		process.KillProcess(cmd)
	} else {
		process.TerminateProcess(cmd)
	}
}

// ShutdownAllTests terminates all ongoing test processes
func ShutdownAllTests() {
	if TestShutdownInProgress {
		return
	}

	TestShutdownInProgress = true
	GracefullyKillOrTerminate(activeCommand)

	// Wait for processes to finish or force termination after timeout
	completionChannel := make(chan struct{})
	go func() {
		testWaitGroup.Wait()
		close(completionChannel)
	}()

	select {
	case <-completionChannel:
		// Tests shut down properly
	case <-time.After(5 * time.Second):
		logger.Warn("Child processes took too long to terminate. Forcing shutdown...")
	}

	process.KillProcess(activeCommand)
}

// isShutdownInProgress checks if tests are being terminated
func isShutdownInProgress() bool {
	if TestShutdownInProgress {
		testWaitGroup.Wait()
		time.Sleep(500 * time.Millisecond)
		return true
	}
	return false
}

// assertCondition checks if a condition is true and fails the test if not
func (result *TestResult) assertCondition(condition bool, format string, args ...any) {
	if isShutdownInProgress() {
		return
	}
	if !condition {
		logger.Error(format+"\nOutput:\n%s", append(args, result.CommandOutput)...)
		result.T.FailNow()
	}
}

// AssertContains verifies output contains all specified substrings
func (result *TestResult) AssertContains(substr ...string) {
	for _, s := range substr {
		result.assertCondition(
			strings.Contains(strings.ToLower(result.CommandOutput), strings.ToLower(s)),
			"Expected output to contain `%v`", s,
		)
	}
}

// AssertNotContains verifies output does not contain specified substrings
func (result *TestResult) AssertNotContains(substr ...string) {
	for _, s := range substr {
		result.assertCondition(
			!strings.Contains(strings.ToLower(result.CommandOutput), strings.ToLower(s)),
			"Expected output to not contain `%v`", s,
		)
	}
}

// AssertMinOccurrences checks if a substring appears at least N times
func (result *TestResult) AssertMinOccurrences(substr string, min int) {
	result.assertCondition(
		strings.Count(strings.ToLower(result.CommandOutput), strings.ToLower(substr)) >= min,
		"Expected output `%v` to appear at least `%d` time(s)", substr, min,
	)
}

// AssertOccurrences checks if a substring appears exactly N times
func (result *TestResult) AssertOccurrences(substr string, count int) {
	result.assertCondition(
		strings.Count(strings.ToLower(result.CommandOutput), strings.ToLower(substr)) == count,
		"Expected output `%v` to appear `%d` time(s)", substr, count,
	)
}

// AssertSequentialOrder verifies items appear in the output in the given order
func (result *TestResult) AssertSequentialOrder(items ...string) {
	if len(items) < 2 {
		result.assertCondition(false, "Could not check output order. Needs at least 2 items to compare")
		return
	}

	currentPos := 0

	for i, item := range items {
		pos := strings.Index(result.CommandOutput[currentPos:], item)

		result.assertCondition(pos != -1, "Could not check output order. Item `%s` (at position %d in sequence) not found after position %d in output",
			item, i, currentPos)

		currentPos += pos + len(item)
	}
}

// AssertMinDuration checks if command execution took at least the specified time
func (result *TestResult) AssertMinDuration(min time.Duration) {
	result.assertCondition(result.ExecutionTime >= min, "Expected execution duration to be at least %v. Finished in %v", min, result.ExecutionTime)
}

// AssertPortTimeoutError verifies port timeout error message is as expected
func (result *TestResult) AssertPortTimeoutError(portNumber int, timeout float64) {
	portErr := port.VerifyPortAvailability(portNumber, timeout, func() string { return "" })
	if portErr != nil {
		errorMessage := portErr.Error()
		expectedSubstr := "Timeout reached after " + utils.FormatDurationValue(timeout) +
			" seconds waiting for port " + strconv.Itoa(portNumber) + " to become ready for connection"
		result.assertCondition(
			strings.Contains(strings.ToLower(errorMessage), strings.ToLower(expectedSubstr)),
			"Expected error to contain `%s`\nError: %s", expectedSubstr, errorMessage,
		)
	}
}

// executeFileOperation handles common file operation pattern and error reporting
func executeFileOperation(t *testing.T, operation func() (func(), error), errorMessage string) func() {
	isShutdownInProgress()
	cleanup, err := operation()
	if err != nil {
		logger.Error(errorMessage+" `%v`", err)
		t.FailNow()
	}
	return cleanup
}

// FileMoveOperation moves a file and returns a function to restore it
func FileMoveOperation(src, dst string, overwrite bool) (func(), error) {
	// Verify source exists and isn't a directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("Source file error: %v", err)
	}
	if srcInfo.IsDir() {
		return nil, fmt.Errorf("Source is a directory, not a file")
	}

	// Check destination status
	_, err = os.Stat(dst)
	if err == nil && !overwrite {
		return nil, fmt.Errorf("Destination file already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Destination check error: %v", err)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return nil, fmt.Errorf("Failed to create destination directory: %v", err)
	}

	// Move the file
	if err := os.Rename(src, dst); err != nil {
		return nil, err
	}

	// Return cleanup function
	return func() {
		if err := os.Rename(dst, src); err != nil {
			logger.Error("Failed to restore moved file: %v", err)
		}
	}, nil
}

// MoveTestFile moves a file with test error handling
func MoveTestFile(t *testing.T, src, dst string, overwrite bool) func() {
	return executeFileOperation(t, func() (func(), error) {
		return FileMoveOperation(src, dst, overwrite)
	}, "Moving file has failed with error")
}

// FileCopyOperation copies a file and returns a function to delete the copy
func FileCopyOperation(src, dst string, overwrite bool) (func(), error) {
	// Verify source exists and isn't a directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("Source file error: %v", err)
	}
	if srcInfo.IsDir() {
		return nil, fmt.Errorf("Source is a directory, not a file")
	}

	// Check destination status
	_, err = os.Stat(dst)
	if err == nil && !overwrite {
		return nil, fmt.Errorf("Destination file already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Destination check error: %v", err)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return nil, fmt.Errorf("Failed to create destination directory: %v", err)
	}

	// Open source file
	source, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("Failed to open source file: %v", err)
	}
	defer source.Close()

	// Create destination file
	destination, err := os.Create(dst)
	if err != nil {
		return nil, fmt.Errorf("Failed to create destination file: %v", err)
	}
	defer destination.Close()

	// Copy file content
	if _, err = io.Copy(destination, source); err != nil {
		return nil, err
	}
	if err = os.Chmod(dst, srcInfo.Mode()); err != nil {
		return nil, fmt.Errorf("Failed to set file permissions: %v", err)
	}

	// Return cleanup function
	return func() {
		if err := os.Remove(dst); err != nil {
			logger.Error("Failed to remove copied file during restore: %v", err)
		}
	}, nil
}

// CopyTestFile copies a file with test error handling
func CopyTestFile(t *testing.T, src, dst string, overwrite bool) func() {
	return executeFileOperation(t, func() (func(), error) {
		return FileCopyOperation(src, dst, overwrite)
	}, "Copying file has failed with error")
}

// FileUpdateOperation adds a temp marker to a file with restoration function
func FileUpdateOperation(filePath string) (func(), error) {
	_, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("File error: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file: %v", err)
	}

	originalContent := make([]byte, len(content))
	copy(originalContent, content)

	if len(content) > 0 && content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}
	newContent := append(content, []byte("/// temp ///\n")...)

	if err := os.WriteFile(filePath, newContent, 0644); err != nil {
		return nil, fmt.Errorf("Failed to write to file: %v", err)
	}

	return func() {
		if err := os.WriteFile(filePath, originalContent, 0644); err != nil {
			logger.Error("Failed to restore original file content: %v", err)
		}
	}, nil
}

// ModifyTestFile updates a file for testing with temp marker
func ModifyTestFile(t *testing.T, filePath string) func() {
	return executeFileOperation(t, func() (func(), error) {
		return FileUpdateOperation(filePath)
	}, "Adding temp marker to file has failed with error")
}

// FileDeleteOperation deletes a file and provides a restoration function
func FileDeleteOperation(filePath string) (func(), error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("File error: %v", err)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("Path is a directory, not a file")
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file content: %v", err)
	}
	fileMode := fileInfo.Mode()

	if err := os.Remove(filePath); err != nil {
		return nil, fmt.Errorf("Failed to delete file: %v", err)
	}

	return func() {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Error("Failed to create directory during restore: %v", err)
			return
		}
		if err := os.WriteFile(filePath, fileContent, fileMode); err != nil {
			logger.Error("Failed to restore file content: %v", err)
		}
	}, nil
}

// DeleteTestFile safely deletes a file for testing
func DeleteTestFile(t *testing.T, filePath string) func() {
	return executeFileOperation(t, func() (func(), error) {
		return FileDeleteOperation(filePath)
	}, "Deleting file has failed with error")
}

// DirectoryCreationOperation creates a directory with cleanup function
func DirectoryCreationOperation(dirPath string) (func(), error) {
	_, err := os.Stat(dirPath)
	if err == nil {
		return nil, fmt.Errorf("Directory already exists: %s", dirPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Directory check error: %v", err)
	}

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("Failed to create directory: %v", err)
	}

	return func() {
		if err := os.RemoveAll(dirPath); err != nil {
			logger.Error("Failed to remove directory during restore: %v", err)
		}
	}, nil
}

// CreateTestDirectory creates a directory for testing with cleanup handling
func CreateTestDirectory(t *testing.T, dirPath string) func() {
	return executeFileOperation(t, func() (func(), error) {
		return DirectoryCreationOperation(dirPath)
	}, "Creating directory has failed with error")
}

// ExecuteTestCommand runs a navi command with specified parameters and timeout
func ExecuteTestCommand(
	t *testing.T,
	workingDir string,
	args []string,
	timeout time.Duration,
	asyncCallback func(terminate func()),
	expectError bool,
	identifier string,
) TestResult {
	if isShutdownInProgress() {
		return TestResult{T: t, WorkingDir: workingDir, CommandArgs: args}
	}

	// Log the command being executed
	executingCmd := strings.TrimSpace("navi " + strings.Join(args, " "))

	if identifier != "" {
		logger.Info("Running `%s` (%s)", executingCmd, identifier)
	} else {
		logger.Info("Running `%s`", executingCmd)
	}

	// Get current directory and construct binary path
	originalDir, err := os.Getwd()
	naviExecutable := filepath.Join(originalDir, "navi")

	if err != nil {
		logger.Error("Cannot get current dir: %v", err)
		t.FailNow()
	}

	// Change to test working directory
	if err = os.Chdir(workingDir); err != nil {
		logger.Error("Cannot change dir: %v", err)
		t.FailNow()
	}

	// Restore original directory when function completes
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			logger.Error("Failed to restore dir: %v", err)
			t.FailNow()
		}
	}()

	// Set up command with environment
	cmd := exec.Command(naviExecutable, args...)
	process.SetupNewProcessGroup(cmd)
	activeCommand = cmd
	cmd.Env = append(os.Environ(), "NAVI_TEST_MODE=1")

	// Prepare to capture execution time and output
	startTime := time.Now()
	testWaitGroup.Add(1)
	getOutputFunc, err := captureCommandOutput(cmd)
	if err != nil {
		logger.Error("Failed to setup command output: %v", err)
		t.FailNow()
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start command: %v", err)
		t.FailNow()
	}

	// Process command completion in background
	commandCompletionChannel := make(chan error)
	go func() {
		err := cmd.Wait()
		testWaitGroup.Done()

		if err == nil && expectError {
			commandCompletionChannel <- ErrCommandShouldHaveFailed
			return
		}
		commandCompletionChannel <- err
	}()

	// Set default timeout if none provided
	var executionTime time.Duration
	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	// Handle async callback if provided
	if asyncCallback != nil {
		go func() {
			asyncCallback(func() {
				GracefullyKillOrTerminate(cmd)
				logger.Warn("Forcing async test `navi %s` to finish", strings.Join(args, " "))
			})
		}()
	}

	// Wait for command completion or timeout
	select {
	case <-time.After(timeout):
		// Command timed out
		GracefullyKillOrTerminate(cmd)
		executionTime = timeout
		logger.Warn("Forced timeout reached after %s", executionTime)

	case cmdErr := <-commandCompletionChannel:
		// Command completed
		if errors.Is(cmdErr, ErrCommandShouldHaveFailed) {
			output := getOutputFunc()
			logger.Error("%s\nOutput:\n%s", cmdErr.Error(), output)
			t.FailNow()
		}
		executionTime = time.Since(startTime)
	}

	// Wait for test process to fully complete
	completionSignal := make(chan struct{})
	go func() {
		testWaitGroup.Wait()
		close(completionSignal)
	}()

	select {
	case <-completionSignal:
		// Normal completion
	case <-time.After(15 * time.Second):
		logger.Warn("Running test took too long to terminate. Forcing termination...")
	}

	// Ensure process is killed
	process.KillProcess(cmd)

	// Return test result
	return TestResult{
		T:             t,
		CommandArgs:   args,
		WorkingDir:    workingDir,
		ExecutionTime: executionTime,
		CommandOutput: getOutputFunc(),
	}
}

// CreateTesterWithTimeout returns a function to run tests with custom timeouts
func CreateTesterWithTimeout(t *testing.T, dir string) func(timeout time.Duration, args ...string) TestResult {
	return func(timeout time.Duration, args ...string) TestResult {
		return ExecuteTestCommand(t, dir, args, timeout, nil, false, "")
	}
}

// CreateStandardTester returns a function to run basic tests with default timeout
func CreateStandardTester(t *testing.T, dir string) func(args ...string) TestResult {
	return func(args ...string) TestResult {
		return ExecuteTestCommand(t, dir, args, 0, nil, false, "")
	}
}

// CreateCLITester returns a function to run commands against the CLI
func CreateCLITester(t *testing.T, dir string, args ...string) func(identifier string, terminalWidth, terminalHeight int, cmds ...any) TestResult {
	return func(identifier string, terminalWidth, terminalHeight int, cmds ...any) TestResult {
		os.Setenv("NAVI_TEST_CLI_TERM_WIDTH", strconv.Itoa(terminalWidth))
		os.Setenv("NAVI_TEST_CLI_TERM_HEIGHT", strconv.Itoa(terminalHeight))
		cliCommands := []string{}

		for _, cmd := range cmds {
			switch cmdVal := cmd.(type) {
			case string:
				cliCommands = append(cliCommands, cmdVal)
			case int:
				cliCommands = append(cliCommands, strconv.Itoa(cmdVal))
			default:
				logger.Error("Invalid CLI command type: %v", cmdVal)
				os.Unsetenv("NAVI_TEST_CLI_TERM_WIDTH")
				os.Unsetenv("NAVI_TEST_CLI_TERM_HEIGHT")
				t.FailNow()
			}
		}

		os.Setenv("NAVI_TEST_CLI_COMMANDS", strings.Join(cliCommands, "|"))
		result := ExecuteTestCommand(t, dir, args, 0, nil, false, identifier)
		os.Unsetenv("NAVI_TEST_CLI_TERM_WIDTH")
		os.Unsetenv("NAVI_TEST_CLI_TERM_HEIGHT")
		os.Unsetenv("NAVI_TEST_CLI_COMMANDS")
		return result
	}
}

// CreateErrorExpectingTester returns a function for tests that should fail
func CreateErrorExpectingTester(t *testing.T, dir string) func(args ...string) TestResult {
	return func(args ...string) TestResult {
		return ExecuteTestCommand(t, dir, args, 0, nil, true, "")
	}
}

// CreateAsyncTester returns a function for asynchronous test preparation
func CreateAsyncTester(t *testing.T, dir string) func(args ...string) TestResult {
	return func(args ...string) TestResult {
		return TestResult{T: t, CommandArgs: args, WorkingDir: dir}
	}
}

// ExecuteAsync runs a prepared test asynchronously with callback
func (result *TestResult) ExecuteAsync(timeout time.Duration, callback func(terminate func())) {
	*result = ExecuteTestCommand(result.T, result.WorkingDir, result.CommandArgs, timeout, callback, false, "")
}

// ExecuteAsyncExpectingError runs a test that should fail asynchronously
func (result *TestResult) ExecuteAsyncExpectingError(timeout time.Duration, callback func(terminate func())) {
	*result = ExecuteTestCommand(result.T, result.WorkingDir, result.CommandArgs, timeout, callback, true, "")
}

// Initialize configuration for test output display
func init() {
	displayOutputInRealtime = (os.Getenv("NAVI_TEST_OUTPUT") == "1" || os.Getenv("NAVI_TEST_OUTPUT") == "true")
}
