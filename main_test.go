package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-navi/navi/cmd/navi"
	"github.com/go-navi/navi/internal/logger"
	"github.com/go-navi/navi/internal/process"
	"github.com/go-navi/navi/internal/term"
	utils "github.com/go-navi/navi/internal/tests"
)

var fixturesDir string
var cleanUpFunctions utils.CleanupFunctions

func cleanUp() {
	os.Unsetenv("TEST_ENV_VAR")
	os.Unsetenv("TEST_NUM_ENV_VAR")
	os.Unsetenv("SPECIAL_CHARS")
	cleanUpFunctions.ExecuteAll()
}

func TestErrors(t *testing.T) {
	var result utils.TestResult
	tester := utils.CreateErrorExpectingTester(t, fixturesDir)
	asyncTester := utils.CreateAsyncTester(t, fixturesDir)

	result = tester("-f", "not-found.yml")
	result.AssertContains("ERROR: Configuration file not found. Path: not-found.yml")

	result = tester("-f", "../")
	result.AssertContains("ERROR: Configuration file not found. Path: ../")

	result = tester("-f", "./flags/")
	result.AssertContains("ERROR: Configuration file not found. Path: ./flags/")

	result = tester("--file", "./xyz")
	result.AssertContains("ERROR: Configuration file not found. Path: ./xyz")

	cleanUpFunctions.Add(utils.MoveTestFile(t, filepath.Join(fixturesDir, "navi.yml"), filepath.Join(fixturesDir, "yml", "navi.yml"), true))
	result = tester()
	result.AssertContains("ERROR: Configuration file not found")
	cleanUpFunctions.ExecuteAll()

	result = tester("not-found-proj:cmd")
	result.AssertContains("ERROR: Could not find `not-found-proj:cmd` in yaml configuration")

	result = tester("--file", "./yml/test_1.yml")
	result.AssertContains(
		"Opening Interactive CLI...",
		"ERROR: No commands, project commands or runners found in configuration",
	)

	result = tester("--file", "./yml/test_2.yml", "cmd-2")
	result.AssertContains("ERROR: Could not find `cmd-2` in yaml configuration")

	result = tester("--file", "./yml/test_2.yml", "proj", "arg1")
	result.AssertContains("ERROR: Could not find `proj` in yaml configuration")

	result = tester("--file", "./yml/test_2.yml", "proj:cmd")
	result.AssertContains("ERROR: Could not find `proj:cmd` in yaml configuration")

	result = tester("--file", "./yml/test_2.yml", "projects")
	result.AssertContains("ERROR: Could not find `projects` in yaml configuration")

	result = tester("--file", "./yml/test_2.yml", "runners")
	result.AssertContains("ERROR: Could not find `runners` in yaml configuration")

	result = tester("--file", "./yml/test_3.yml", "test-1")
	result.AssertContains("ERROR: Could not find `test-1` in yaml configuration")

	result = tester("--file", "./yml/test_3.yml", "proj")
	result.AssertContains("ERROR: Missing command to execute in project `proj`")

	if runtime.GOOS != "windows" { // unix
		result = tester("--file", "./yml/test_3.yml", "proj", "test")
		result.AssertContains("ERROR: Command `test` failed with exit code exit status 1")
	}

	result = tester("--file", "./yml/test_3.yml", "proj:test-2")
	result.AssertContains("ERROR: Command `test-2` not found in project `proj`")

	result = tester("--file", "./yml/test_3.yml", "commands")
	result.AssertContains("ERROR: Could not find `commands` in yaml configuration")

	result = tester("--file", "./yml/test_3.yml", "runners")
	result.AssertContains("ERROR: Could not find `runners` in yaml configuration")

	result = tester("--file", "./yml/test_4.yml", "runner-2")
	result.AssertContains("ERROR: Could not find `runner-2` in yaml configuration")

	result = tester("--file", "./yml/test_4.yml", "proj", "runner-1")
	result.AssertContains("ERROR: Could not find `proj` in yaml configuration")

	result = tester("--file", "./yml/test_4.yml", "proj:runner-1")
	result.AssertContains("ERROR: Could not find `proj:runner-1` in yaml configuration")

	result = tester("--file", "./yml/test_4.yml", "commands")
	result.AssertContains("ERROR: Could not find `commands` in yaml configuration")

	result = tester("--file", "./yml/test_4.yml", "projects")
	result.AssertContains("ERROR: Could not find `projects` in yaml configuration")

	result = tester("--file", "./yml/test_5.yml")
	result.AssertContains(
		"Opening Interactive CLI...",
		"ERROR: No commands, project commands or runners found in configuration",
	)

	result = tester("--file", "./yml/test_5.yml", "cmd-name")
	result.AssertContains("Could not find `cmd-name` in yaml configuration")

	result = tester("--file", "./yml/test_6.yml")
	result.AssertContains(
		"Opening Interactive CLI...",
		"ERROR: No commands, project commands or runners found in configuration",
	)

	result = tester("error-proj-1")
	result.AssertContains("ERROR: Missing command to execute in project `error-proj-1`")

	result = tester("error-proj-1", "\\")
	result.AssertContains("ERROR: Invalid format for command `\\`")

	result = tester("error-proj-1", ":missing-env-var")
	result.AssertContains("ERROR: Command `:missing-env-var` has failed with error `exec: \":missing-env-var\"")

	result = tester("error-proj-1::missing-env-var")
	result.AssertContains("ERROR: Environment variable `ENV_VAR6` not found in file `" + filepath.Join(fixturesDir, ".env") + "`")

	result = tester("error-proj-1:missing-run")
	result.AssertContains("ERROR: Missing required `run` field for command `missing-run` in project `error-proj-1`")

	result = tester("error-proj-2:test-1")
	result.AssertContains("ERROR: Project `error-proj-2` is missing required `dir` property in configuration")

	result = tester("error-proj-3:node", "main.js")
	result.AssertContains("ERROR: Command `node` not found in project `error-proj-3`")

	if runtime.GOOS == "windows" {
		result = tester("error-proj-3:no-shell-win-1")
		result.AssertContains("ERROR: Command `echo \"cwd => $PWD\"` has failed with error `exec: \"echo\": executable file not found in %PATH%`")

		result = tester("error-proj-3:no-shell-win-2")
		result.AssertContains("ERROR: Command `echo \"cwd => %CD%\"` has failed with error `exec: \"echo\": executable file not found in %PATH%`")

		result = tester("error-proj-3", "unknown")
		result.AssertContains("ERROR: Command `unknown` has failed with error `exec: \"unknown\": executable file not found in %PATH%`")

	} else {
		result = tester("error-proj-3", "unknown")
		result.AssertContains("ERROR: Command `unknown` has failed with error `exec: \"unknown\": executable file not found in $PATH`")
	}

	result = tester("error-proj-4:test")
	result.AssertContains("ERROR: Project `pre` command failed: Command `node exit.js` failed with exit code exit status 1")

	result = tester("error-proj-5:test")
	result.AssertContains("ERROR: Command `pre` command failed: Command `node exit.js` failed with exit code exit status 1")

	result = tester("error-proj-6:test")
	result.AssertContains("ERROR: Command `post` command failed: Command `node exit.js` failed with exit code exit status 1")

	result = tester("error-proj-7:test")
	result.AssertContains("ERROR: Project `post` command failed: Command `node exit.js` failed with exit code exit status 1")
	result.AssertNotContains("Fail during execution of after command(s)")

	if runtime.GOOS != "windows" { // unix
		result = tester("error-proj-8:shell-test")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `array=(111, 222, 333); echo \"${array[1]}\"`",
			"Running main command...",
			"Executing `array=(111, 222, 333); echo \"${array[1]}\"`",
			"ERROR: Command `array=(111, 222, 333); echo \"${array[1]}\"`",
		)
	}

	result = tester("error-proj-9:test-1")
	result.AssertContains("ERROR: Command `test-1` in project `error-proj-9` must be a command or a list of commands")

	result = tester("error-proj-9:test-2")
	result.AssertContains("ERROR: The `run` field of command `test-2` in project `error-proj-9` must be a command or a list of commands")

	result = tester("error-proj-10:test")
	result.AssertContains("ERROR: Failed to load environment file `" + filepath.Join(fixturesDir, "node", "not_exist.env") + "`")

	result = tester("error-proj-11:test")
	result.AssertContains("ERROR: Environment variable `NO_KEY` not found in file `" + filepath.Join(fixturesDir, "node", ".node1.env") + "`")

	result = tester("error-proj-12:test-1")
	result.AssertContains("ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("error-proj-12:test-2")
	result.AssertContains("ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("error-proj-13:test")
	result.AssertContains("ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("error-cmd-1")
	result.AssertContains("ERROR: Command `error-cmd-1` must be a command or a list of commands")

	result = tester("error-cmd-2")
	result.AssertContains("ERROR: Command `node exit.js` failed with exit code exit status 1")

	result = tester("error-cmd-3")
	result.AssertContains("ERROR: The `run` field for command `error-cmd-3` must be a command or a list of commands")

	result = tester("error-cmd-4")
	result.AssertContains("ERROR: Command `node exit.js` failed with exit code exit status 1")

	result = tester("error-cmd-5")
	result.AssertContains("ERROR: Parameter `watch` in command `error-cmd-5` must be a list of Glob patterns")

	result = tester("error-cmd-6")
	result.AssertContains("ERROR: Missing required `run` field for command `error-cmd-6`")

	result = tester("general-1:")
	result.AssertContains("ERROR: Missing command to execute in project `general-1`")

	result = tester("general-2:general-1")
	result.AssertContains("Could not find `general-2:general-1` in yaml configuration")

	result = tester("watch-mode-error-1:test")
	result.AssertContains("ERROR: Parameter `watch` in project `watch-mode-error-1` must be a list of Glob patterns")

	result = tester("watch-mode-error-2:test")
	result.AssertContains("ERROR: Parameter `watch.include` in project `watch-mode-error-2` must be a list of Glob patterns")

	result = tester("watch-mode-error-3:test")
	result.AssertContains("ERROR: Parameter `watch.exclude` in project `watch-mode-error-3` must be a list of Glob patterns")

	result = tester("watch-mode-error-4:test")
	result.AssertContains("ERROR: Invalid format for `watch` config: syntax error in pattern")

	result = tester("watch-mode-error-5:test")
	result.AssertContains(
		"Starting in watch mode",
		"Executing `node exit.js`",
		"timeout from exit.js",
		"ERROR: Command `node exit.js` failed with exit code exit status 1",
	)
	result.AssertNotContains("Fail during execution of after command(s)")

	result = tester("watch-mode-error-6:test-1")
	result.AssertContains("ERROR: Fail during execution of after command(s) in watch mode: Command `node ./node/exit.js` failed with exit code exit status 1")

	result = asyncTester("watch-mode-error-6:test-2")
	result.ExecuteAsyncExpectingError(10*time.Second, func(_ func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "index.css")))
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(2 * time.Second)
	result.AssertContains(
		"Running `after.change` command...",
		"ERROR: Fail during execution of after command(s) in watch mode:",
		"WARNING: Shutting down processes... (don't close the terminal)",
	)

	result = tester("watch-mode-error-7:test")
	result.AssertContains(
		"Running `after.failure` command...",
		"Running `after.always` command...",
		"ERROR: Fail during execution of after command(s) in watch mode: Command `node exit.js` failed with exit code exit status 1",
	)
	result.AssertNotContains("Running `after.success` command...")

	result = tester("runner-error-1")
	result.AssertContains("ERROR: Invalid value for parameter `condition` in runner `runner-error-1`. Must be `always`, `failure`, or `success`")

	result = tester("runner-error-2")
	result.AssertContains("ERROR: Runner `runner-error-2` must be defined as a command or a list of commands")

	result = tester("runner-error-3")
	result.AssertContains("ERROR: Invalid format for command `\\` in runner `runner-error-3`")

	result = tester("runner-error-4")
	result.AssertContains("ERROR: Invalid format for command `\\` in runner `runner-error-4`")

	result = tester("runner-error-5")
	result.AssertContains("ERROR: Invalid format for command `\\` in runner `runner-error-5`")

	result = tester("runner-error-6")
	result.AssertContains("ERROR: Runner command in runner `runner-error-6` must have a `cmd` key")

	result = tester("-d", "runner-error-7")
	result.AssertContains("error-proj-12:test-1 ⟫ ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("-d", "runner-error-8")
	result.AssertContains("error-proj-13:test ⟫ ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("-d", "runner-error-9")
	result.AssertOccurrences("error-proj-12:test-1 ⟫ Running `after.failure` command...", 1)
	result.AssertContains("error-proj-12:test-1 ⟫ ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("-d", "runner-error-10")
	result.AssertOccurrences("error-proj-13:test ⟫ Running project-level `after` command...", 1)
	result.AssertContains("error-proj-13:test ⟫ ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("-d", "runner-error-11")
	result.AssertOccurrences("error-proj-13:test ⟫ Running project-level `after` command...", 1)
	result.AssertContains("error-proj-13:test ⟫ ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("-d", "runner-error-12")
	result.AssertOccurrences("error-proj-12:test-1 ⟫ Running `after.failure` command...", 1)
	result.AssertContains("error-proj-12:test-1 ⟫ ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("-d", "runner-error-13")
	result.AssertOccurrences("error-proj-13:test ⟫ Running project-level `after` command...", 1)
	result.AssertContains("error-proj-13:test ⟫ ERROR: Fail during execution of after command(s): Command `node exit.js` failed with exit code exit status 1")

	result = tester("runner-56")
	result.AssertContains("ERROR: Could not find `runner-56` in yaml configuration")

	result = tester("runner-57")
	result.AssertContains("ERROR: Could not find `runner-57` in yaml configuration")

	result = tester("runner-58")
	result.AssertContains("ERROR: Could not find `runner-58` in yaml configuration")
}

func TestWatchModeExecution(t *testing.T) {
	var result utils.TestResult
	asyncTester := utils.CreateAsyncTester(t, fixturesDir)

	result = asyncTester("cmd-6")
	result.ExecuteAsync(14*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "main.js")))
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_2.css")))
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_1.css")))
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "inner2", "index_3.css")))
		time.Sleep(4 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(12 * time.Second)
	result.AssertOccurrences("Starting in watch mode", 1)
	result.AssertSequentialOrder(
		"Running `pre` command...",
		"Executing `npm run env-vars`",
		"main ENV_VAR1: 888",
		"main ENV_VAR2: 999",
		"main ENV_VAR3: undefined",
		"main ENV_VAR4: undefined",
		"main ENV_VAR5: undefined",
		"Running main command...",
		"Executing `npm run main`",
		"running main.js",
		"Running `post` command...",
		"Executing `python3 env_vars_post.py`",
		"post ENV_VAR1: 888",
		"post ENV_VAR2: 999",
		"post ENV_VAR3: None",
		"post ENV_VAR4: None",
		"Command(s) completed successfully",
		"Running `after.always` command...",
		"Executing `python3 ../python/env_vars.py`",
		"main ENV_VAR1: 777",
		"main ENV_VAR2: 999",
		"main ENV_VAR3: None",
		"main ENV_VAR4: None",
		"After command(s) completed successfully",
	)
	result.AssertOccurrences("File change detected. Stopping running command...", 2)
	result.AssertOccurrences("Running `pre` command...", 3)
	result.AssertOccurrences("Running main command...", 3)
	result.AssertOccurrences("Running `post` command...", 3)
	result.AssertOccurrences("Running `after.always` command...", 3)

	result = asyncTester("watch-mode-1:test")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "main.js")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "inner", "env_vars_pre.js")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_2.css")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "inner2", "index_3.css")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(5 * time.Second)
	result.AssertOccurrences("Starting in watch", 1)
	result.AssertOccurrences("Executing `node continuous.js`", 3)
	result.AssertOccurrences("File change detected. Stopping running command...", 2)
	result.AssertMinOccurrences("1 second passed", 4)

	result = asyncTester("watch-mode-1:test")
	result.ExecuteAsync(10*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "go.mod")))
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "main.go")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_1.css")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "package.json")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "package-lock.json")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "inner", "package-lock.json")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_1.py")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "inside", "index.css")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertOccurrences("Executing `node continuous.js`", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-1:test")
	result.ExecuteAsync(10*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.MoveTestFile(
			t,
			filepath.Join(fixturesDir, "python", "inside", "env_vars_post.py"),
			filepath.Join(fixturesDir, "python", "env_vars_post_tmp.py"),
			true,
		))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.MoveTestFile(
			t,
			filepath.Join(fixturesDir, "python", "env_vars_post_tmp.py"),
			filepath.Join(fixturesDir, "python", "inside", "env_vars_post.py"),
			true,
		))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.CopyTestFile(
			t,
			filepath.Join(fixturesDir, "python", "inside", "env_vars_post.py"),
			filepath.Join(fixturesDir, "python", "env_vars_post_tmp.py"),
			true,
		))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.CopyTestFile(
			t,
			filepath.Join(fixturesDir, "python", "timeout_1.py"),
			filepath.Join(fixturesDir, "python", "inside", "timeout_1.py"),
			true,
		))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.DeleteTestFile(t, filepath.Join(fixturesDir, "python", "inside", "timeout_1.py")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(8 * time.Second)
	result.AssertOccurrences("Executing `node continuous.js`", 5)
	result.AssertOccurrences("File change detected. Stopping running command...", 4)

	result = asyncTester("watch-mode-2:test-1")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "main.js")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_1.css")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "inner", "env_vars_pre.js")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_2.css")))
		time.Sleep(150 * time.Millisecond)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "inner2", "index_3.css")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertContains("Starting in watch mode")
	result.AssertOccurrences("Executing `node continuous.js`", 3)
	result.AssertOccurrences("File change detected. Stopping running command...", 2)
	result.AssertMinOccurrences("1 second passed", 3)

	result = asyncTester("watch-mode-2:test-1")
	result.ExecuteAsync(17*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "go.mod")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "main.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_1.css")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_2.css")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "inner2", "index_3.css")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_1.html")))
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_2.html")))
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "inner", "index_2.html")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_1.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "inside", "index.css")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(15 * time.Second)
	result.AssertOccurrences("Executing `node continuous.js`", 6)
	result.AssertOccurrences("File change detected. Stopping running command...", 5)

	result = asyncTester("watch-mode-3:test-1")
	result.ExecuteAsync(9*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_A.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_1.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_2.py")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(7 * time.Second)
	result.AssertOccurrences("Executing `python3 continuous.py`", 3)
	result.AssertOccurrences("File change detected. Stopping running command...", 2)

	result = asyncTester("watch-mode-3:test-2")
	result.ExecuteAsync(9*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_A.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_1.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_2.py")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(7 * time.Second)
	result.AssertOccurrences("Executing `python3 continuous.py`", 4)
	result.AssertOccurrences("File change detected. Stopping running command...", 3)

	result = asyncTester("watch-mode-4:test-1")
	result.ExecuteAsync(7*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "misc_1.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "misc_2.py", "index.css")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(5 * time.Second)
	result.AssertOccurrences("Executing `python3 continuous.py`", 3)
	result.AssertOccurrences("File change detected. Stopping running command...", 2)

	result = asyncTester("watch-mode-4:test-2")
	result.ExecuteAsync(7*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.CopyTestFile(
			t,
			filepath.Join(fixturesDir, "python", "timeout_.py"),
			filepath.Join(fixturesDir, "python", "empty", "timeout_.py"),
			true,
		))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(5 * time.Second)
	result.AssertOccurrences("Executing `python3 ./python/continuous.py`", 3)
	result.AssertOccurrences("File change detected. Stopping running command...", 2)

	result = asyncTester("watch-mode-4:test-3")
	result.ExecuteAsync(9*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.CopyTestFile(
			t,
			filepath.Join(fixturesDir, "python", "timeout_.py"),
			filepath.Join(fixturesDir, "python", "empty", "timeout_.py"),
			true,
		))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "index.css")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.CopyTestFile(
			t,
			filepath.Join(fixturesDir, "python", "index.css"),
			filepath.Join(fixturesDir, "python", "empty", "index.css"),
			true,
		))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(7 * time.Second)
	result.AssertOccurrences("Executing `python3 ./python/continuous.py`", 3)
	result.AssertOccurrences("File change detected. Stopping running command...", 2)

	result = asyncTester("watch-mode-4:test-4")
	result.ExecuteAsync(7*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.CopyTestFile(
			t,
			filepath.Join(fixturesDir, "python", "index.css"),
			filepath.Join(fixturesDir, "python", "empty", "index.css"),
			true,
		))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(5 * time.Second)
	result.AssertOccurrences("Executing `python3 ./python/continuous.py`", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-4:test-5")
	result.ExecuteAsync(7*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "timeout_.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.CopyTestFile(
			t,
			filepath.Join(fixturesDir, "python", "index.css"),
			filepath.Join(fixturesDir, "python", "empty", "index.css"),
			true,
		))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(5 * time.Second)
	result.AssertOccurrences("Executing `python3 ./python/continuous.py`", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-5:test")
	result.ExecuteAsync(7*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "misc_1.py")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "python", "misc_2.py", "index.css")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(5 * time.Second)
	result.AssertOccurrences("Executing `python3 continuous.py`", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-6:test-1")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "main.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "file.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "inner", "file.go")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertContains("WARNING: Skipping directories [\"build\"] by default. Add them to `watch` parameter to monitor")
	result.AssertOccurrences("by default. Add them to `watch` parameter to monitor", 1)
	result.AssertOccurrences("Executing `go run main.go`", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-6:test-2")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "main.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "file.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "inner", "file.go")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertNotContains("by default. Add them to `watch` parameter to monitor")
	result.AssertOccurrences("Executing `go run main.go`", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-6:test-3")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "main.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "file.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "inner", "file.go")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertNotContains("by default. Add them to `watch` parameter to monitor")
	result.AssertOccurrences("Executing `go run main.go`", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-6:test-4")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "main.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "file.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "inner", "file.go")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertNotContains("by default. Add them to `watch` parameter to monitor")
	result.AssertOccurrences("Executing `go run main.go`", 3)
	result.AssertOccurrences("File change detected. Stopping running command...", 2)

	result = asyncTester("watch-mode-6:test-5")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "main.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "file.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "inner", "file.go")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertContains("WARNING: Skipping directories [\"build\"] by default. Add them to `watch` parameter to monitor")
	result.AssertOccurrences("by default. Add them to `watch` parameter to monitor", 1)
	result.AssertNotContains("File change detected. Stopping running command...")
	result.AssertContains("WARNING: No directories found to watch for changes")

	result = asyncTester("watch-mode-6:test-6")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "main.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "file.go")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "go", "build", "inner", "file.go")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertNotContains("by default. Add them to `watch` parameter to monitor")
	result.AssertOccurrences("Executing `go run main.go`", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-6:test-7")
	result.ExecuteAsync(4*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		terminate()
	})
	result.AssertMinDuration(2 * time.Second)
	result.AssertContains("WARNING: Skipping directories [\"build\", \"dist\"] by default. Add them to `watch` parameter to monitor")
	result.AssertOccurrences("by default. Add them to `watch` parameter to monitor", 1)

	result = asyncTester("watch-mode-6:test-7")
	result.ExecuteAsync(4*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		terminate()
	})
	result.AssertMinDuration(2 * time.Second)
	result.AssertContains("WARNING: Skipping directories [\"build\", \"dist\"] by default. Add them to `watch` parameter to monitor")
	result.AssertOccurrences("by default. Add them to `watch` parameter to monitor", 1)

	if runtime.GOOS != "windows" { // unix
		result = asyncTester("watch-mode-7:test")
		result.ExecuteAsync(10*time.Second, func(terminate func()) {
			time.Sleep(2 * time.Second)
			terminate()
		})
		result.AssertMinDuration(2 * time.Second)
		result.AssertContains(
			"WARNING: Received `terminated` signal",
			"WARNING: Shutting down processes... (don't close the terminal)",
			"Received SIGTERM. Shutting down in 5 seconds...",
			"Shutdown timeout reached. Forcing exit now.",
		)
	}

	result = asyncTester("watch-mode-8:test-1")
	result.ExecuteAsync(6*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "main.js")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(4 * time.Second)
	result.AssertOccurrences("\nExecuting", 2)
	result.AssertNotContains("Running `post` command...")
	result.AssertOccurrences("Running `pre` command...", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-8:test-2")
	result.ExecuteAsync(8*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "index.css")))
		time.Sleep(4 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(6 * time.Second)
	result.AssertOccurrences("\nExecuting", 4)
	result.AssertNotContains("Running `post` command...")
	result.AssertOccurrences("Running `pre` command...", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)

	result = asyncTester("watch-mode-8:test-3")
	result.ExecuteAsync(10*time.Second, func(terminate func()) {
		time.Sleep(2 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "index.css")))
		time.Sleep(6 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(8 * time.Second)
	result.AssertOccurrences("\nExecuting", 6)
	result.AssertOccurrences("Running `pre` command...", 2)
	result.AssertOccurrences("Running `post` command...", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)
}

func TestRunnerExecution(t *testing.T) {
	var result utils.TestResult
	tester := utils.CreateStandardTester(t, fixturesDir)
	testerWithTTL := utils.CreateTesterWithTimeout(t, fixturesDir)
	asyncTester := utils.CreateAsyncTester(t, fixturesDir)

	// inline runners
	result = tester("cmd-9", "cmd-9")
	result.AssertContains(
		"Starting inline runner with 2 command(s)",
		"1 cmd-9 ⟫ Executing `node timeout_1.js`",
		"2 cmd-9 ⟫ Executing `node timeout_1.js`",
	)

	result = tester("proj-2:test-1", "proj-2:test-1")
	result.AssertContains(
		"Starting inline runner with 2 command(s)",
		"1 proj-2:test-1 ⟫ Executing `npm run main`",
		"2 proj-2:test-1 ⟫ Executing `npm run main`",
	)

	result = tester("proj-2:args", "proj-2:test-1", "proj-3:env-vars")
	result.AssertContains(
		"Starting inline runner with 3 command(s)",
		"proj-2:args ⟫ Executing `node args.js`",
		"proj-2:test-1 ⟫ Executing `npm run main`",
		"proj-3:env-vars ⟫ Executing `npm run env-vars-pre`",
	)
	result.AssertContains(
		"proj-2:args ⟫ No arguments found",
		"proj-2:test-1 ⟫ Command(s) completed successfully",
		"proj-3:env-vars ⟫ Command(s) completed successfully",
	)

	result = tester("cmd-11", "cmd-10", "cmd-9")
	result.AssertContains(
		"cmd-10 ⟫ Executing `node ./node/timeout_2.js`",
		"cmd-11 ⟫ Executing `node ./node/args.js`",
		"cmd-9 ⟫ Executing `node timeout_1.js`",
	)
	result.AssertContains(
		"cmd-11 ⟫ No arguments found",
		"cmd-10 ⟫ Command(s) completed successfully",
		"cmd-9 ⟫ Command(s) completed successfully",
	)

	result = tester("proj-2:test-1", "proj-2:test-2", "invalid")
	result.AssertSequentialOrder(
		"Executing `npm run main proj-2:test-2 invalid`",
		"running main.js",
		"Command(s) completed successfully",
	)

	result = tester("cmd-11", "cmd-10", "invalid")
	result.AssertSequentialOrder(
		"Executing `node ./node/args.js cmd-10 invalid`",
		"1 => cmd-10",
		"2 => invalid",
	)

	result = tester("-s", "proj-2:timeout-1", "cmd-9", "proj-2:timeout-2", "cmd-10")
	result.AssertSequentialOrder(
		"Starting inline runner with flags [serial] and 4 command(s)",
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"cmd-9 ⟫ Executing `node timeout_1.js`",
		"cmd-9 ⟫ Command(s) completed successfully",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"proj-2:timeout-2 ⟫ Command(s) completed successfully",
		"cmd-10 ⟫ Executing `node ./node/timeout_2.js`",
		"cmd-10 ⟫ Command(s) completed successfully",
	)

	result = tester("--serial", "proj-2:timeout-1", "proj-2:exit", "cmd-10")
	result.AssertNotContains("proj-2:timeout-2")
	result.AssertSequentialOrder(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"proj-2:exit ⟫ Executing `npm run exit`",
		"ERROR: A serial command in runner `inline` has failed",
	)

	result = tester("-d", "cmd-9", "proj-2:exit", "proj-2:timeout-2")
	result.AssertContains(
		"proj-2:exit ⟫ Executing `npm run exit`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"cmd-9 ⟫ Executing `node timeout_1.js`",
		"ERROR: A dependent command in runner `inline` has failed",
	)

	result = tester("--dependent", "--serial", "cmd-9", "proj-2:exit", "proj-2:timeout-2")
	result.AssertNotContains("proj-2:timeout-2")
	result.AssertSequentialOrder(
		"Starting inline runner with flags [serial, dependent] and 3 command(s)",
		"cmd-9 ⟫ Executing `node timeout_1.js`",
		"proj-2:exit ⟫ Executing `npm run exit`",
		"ERROR: A serial command in runner `inline` has failed",
	)

	result = tester("general-1:*")
	result.AssertContains(
		"Starting inline runner with 2 command(s)",
		"general-1:general-1 ⟫ Executing `node args.js project`",
		"general-1:general-2 ⟫ Executing `node args.js project`",
		"general-1:general-1 ⟫ 1 => project",
		"general-1:general-2 ⟫ 1 => project",
		"general-1:general-1 ⟫ Command(s) completed successfully",
		"general-1:general-2 ⟫ Command(s) completed successfully",
	)

	result = tester("cmd-1", "general-1:*", "other-1")
	result.AssertContains(
		"Starting inline runner with 4 command(s)",
		"cmd-1 ⟫ Executing `node ./node/timeout_1.js`",
		"cmd-1 ⟫ running timeout_1.js",
		"cmd-1 ⟫ Executing `node ./node/timeout_2.js`",
		"cmd-1 ⟫ running timeout_2.js",
		"general-1:general-1 ⟫ Executing `node args.js project`",
		"general-1:general-2 ⟫ Executing `node args.js project`",
		"general-1:general-1 ⟫ 1 => project",
		"general-1:general-2 ⟫ 1 => project",
		"other-1 ⟫ Executing `python3 ./python/env_vars.py`",
		"other-1 ⟫ main ENV_VAR1: 123",
	)

	result = tester("other:format:*")
	result.AssertContains(
		"Starting inline runner with 1 command(s)",
		"other:format:command:format ⟫ Executing `go run main.go`",
		"other:format:command:format ⟫ running main.go",
	)

	result = tester("-s", "-d", "other:format:*", "proj-50:*", "other-1")
	result.AssertNotContains("other-1")
	result.AssertContains(
		"Starting inline runner with flags [serial, dependent] and 4 command(s)",
		"other:format:command:format ⟫ Executing `go run main.go`",
		"other:format:command:format ⟫ Command(s) completed successfully",
		"proj-50:test ⟫ Executing `python3 exit.py`",
		"proj-50:test ⟫ ERROR: Command `python3 exit.py` failed with exit code exit status 1",
		"proj-50:test ⟫ Running `after` command...",
		"proj-50:test ⟫ Executing `python3 main.py`",
		"proj-50:test ⟫ Running project-level `after.failure` command...",
		"proj-50:test ⟫ Executing `python3 main.py`",
		"proj-50:test ⟫ After command(s) completed successfully",
		"ERROR: A serial command in runner `inline` has failed",
	)

	// common runners
	result = tester("general-1")
	result.AssertContains(
		"general-1 ⟫ 1 => runner",
	)

	result = tester("runner-1")
	result.AssertContains(
		"Starting runner `runner-1`",
		"proj-2:test-1 ⟫ running main.js",
		"Executing `npm run main`",
	)

	result = tester("runner-2")
	result.AssertContains(
		"node-test ⟫ running main.js",
		"Executing `npm run main`",
	)

	result = tester("runner-3")
	result.AssertContains(
		"1 proj-2:test-1 ⟫ running main.js",
		"2 proj-2:test-1 ⟫ running main.js",
		"1 proj-33:test ⟫ running main.py",
		"2 proj-33:test ⟫ running main.py",
		"proj-2:test-2 ⟫ running main.js",
		"Executing `python3 main.py`",
		"Executing `npm run main`",
	)

	result = tester("runner-4")
	result.AssertContains(
		"1 proj-2:test-1 ⟫ running main.js",
		"2 proj-2:test-1 ⟫ running main.js",
		"proj-33:test ⟫ running main.py",
		"Executing `python3 main.py`",
		"Executing `npm run main`",
	)

	result = tester("runner-5")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR2: None",
		"pre ENV_VAR3: None",
		"pre ENV_VAR4: false",
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: None",
		"main ENV_VAR3: None",
		"main ENV_VAR4: false",
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR2: None",
		"post ENV_VAR3: None",
		"post ENV_VAR4: false",
		"Executing `python3 env_vars.py`",
		"Executing `python3 env_vars_pre.py`",
		"Executing `python3 env_vars_post.py`",
		"Executing `npm run main`",
	)

	result = tester("runner-6")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR2: '|@%&*'",
		"pre ENV_VAR3: None",
		"pre ENV_VAR4: false",
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: '|@%&*'",
		"main ENV_VAR3: None",
		"main ENV_VAR4: false",
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR2: 789",
		"post ENV_VAR3: None",
		"post ENV_VAR4: false",
		"Executing `python3 env_vars.py`",
		"Executing `python3 env_vars_pre.py`",
		"Executing `python3 env_vars_post.py`",
		"Executing `npm run main`",
	)

	result = tester("runner-7", "ignored", "input")
	result.AssertNotContains("ignored", "input")
	result.AssertOccurrences("1 => arg1", 3)
	result.AssertOccurrences("2 => arg2", 2)
	result.AssertOccurrences("3 => arg3", 1)
	result.AssertContains(
		"2 proj-2:npm ⟫ Executing `npm run args \"single arg\"`",
		"2 proj-2:npm ⟫ 1 => single arg",

		"3 proj-2:npm ⟫ Executing `npm run args arg \"single arg\"`",
		"3 proj-2:npm ⟫ 1 => arg",
		"3 proj-2:npm ⟫ 2 => single arg",

		"2 proj-2:args ⟫ Executing `node args.js \"single arg\"`",
		"2 proj-2:args ⟫ 1 => single arg",

		"3 proj-2:args ⟫ Executing `node args.js arg \"single arg\"`",
		"3 proj-2:args ⟫ 1 => arg",
		"3 proj-2:args ⟫ 2 => single arg",

		"2 proj-2 ⟫ Executing `node args.js \"single arg\"`",
		"2 proj-2 ⟫ 1 => single arg",
		"",
		"3 proj-2 ⟫ Executing `node args.js arg \"single arg\"`",
		"3 proj-2 ⟫ 1 => arg",
		"3 proj-2 ⟫ 2 => single arg",
	)

	result = testerWithTTL(5*time.Second, "runner-8")
	result.AssertContains(
		"1 runner-8 ⟫ Executing `./navi runner-4`",
		"2 runner-8 ⟫ Executing `./navi runner-5`",
		"2 runner-8 ⟫ proj-33:env-vars-2 ⟫ Executing `python3 env_vars_pre.py`",
		"2 runner-8 ⟫ proj-2:test-1 ⟫ Executing `npm run main`",
		"1 runner-8 ⟫ 1 proj-2:test-1 ⟫ Executing `npm run main`",
		"1 runner-8 ⟫ 2 proj-2:test-1 ⟫ Executing `npm run main`",
		"1 runner-8 ⟫ proj-33:test ⟫ Executing `python3 main.py`",
		"1 runner-8 ⟫ 1 proj-2:test-1 ⟫ running main.js",
		"1 runner-8 ⟫ 2 proj-2:test-1 ⟫ running main.js",
		"1 runner-8 ⟫ proj-33:test ⟫ running main.py",
		"2 runner-8 ⟫ proj-33:env-vars-2 ⟫ pre ENV_VAR1: \"|@%&*\"",
		"2 runner-8 ⟫ proj-33:env-vars-2 ⟫ post ENV_VAR1: \"|@%&*\"",
	)

	result = tester("runner-9")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"runner-9 ⟫ Executing `node "+filepath.ToSlash(filepath.Join(fixturesDir, "node", "timeout_2.js"))+"`",
		"proj-2:timeout-1 ⟫ running timeout_1.js",
		"runner-9 ⟫ running timeout_2.js",
		"proj-2:timeout-1 ⟫ timeout from timeout_1.js",
		"runner-9 ⟫ timeout from timeout_2.js",
	)

	result = tester("runner-10")
	result.AssertContains(
		"runner-10 ⟫ Executing `node "+filepath.ToSlash(filepath.Join(fixturesDir, "node", "timeout_1.js"))+"`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"runner-10 ⟫ running timeout_1.js",
		"proj-2:timeout-2 ⟫ running timeout_2.js",
		"runner-10 ⟫ timeout from timeout_1.js",
		"proj-2:timeout-2 ⟫ timeout from timeout_2.js",
	)

	result = testerWithTTL(7*time.Second, "runner-11")
	result.AssertMinDuration(5 * time.Second)
	result.AssertContains(
		"running main.js",
		"Executing `npm run main`",
	)

	result = testerWithTTL(5*time.Second, "runner-12")
	result.AssertMinDuration(1 * time.Second)
	result.AssertContains(
		"1 proj-2:test-1 ⟫ running main.js",
		"2 proj-2:test-1 ⟫ running main.js",
		"3 proj-2:test-1 ⟫ running main.js",
		"1 proj-2:test-1 ⟫ Executing `npm run main`",
		"2 proj-2:test-1 ⟫ Executing `npm run main`",
		"3 proj-2:test-1 ⟫ Executing `npm run main`",
	)

	result = testerWithTTL(12*time.Second, "runner-13")
	result.AssertMinDuration(10 * time.Second)
	result.AssertContains(
		"1 proj-2:test-1 ⟫ running main.js",
		"2 proj-2:test-1 ⟫ running main.js",
		"1 proj-2:test-1 ⟫ Executing `npm run main`",
		"2 proj-2:test-1 ⟫ Executing `npm run main`",
	)

	result = testerWithTTL(9*time.Second, "runner-14")
	result.AssertMinOccurrences("timeout from exit.js", 4)
	result.AssertContains("Executing `npm run exit`")

	result = testerWithTTL(16*time.Second, "runner-15")
	result.AssertMinDuration(11 * time.Second)
	result.AssertMinOccurrences("timeout from exit.js", 4)
	result.AssertContains("Executing `npm run exit`")

	result = testerWithTTL(8*time.Second, "runner-16")
	result.AssertOccurrences("proj-2:test-1 ⟫ running main.js", 1)
	result.AssertMinOccurrences("proj-2:exit ⟫ timeout from exit.js", 4)
	result.AssertMinOccurrences("node-wait-2 ⟫ timeout from exit.js", 4)
	result.AssertContains(
		"Executing `npm run main`",
		"Executing `npm run exit`",
	)

	result = testerWithTTL(8*time.Second, "runner-17")
	result.AssertMinOccurrences("timeout from exit.js", 4)

	result = testerWithTTL(15*time.Second, "runner-18")
	result.AssertMinOccurrences("1 proj-2:exit ⟫ Waiting 0.2 seconds before execution...", 4)
	result.AssertMinOccurrences("2 proj-2:exit ⟫ Waiting 3 seconds before execution...", 3)
	result.AssertMinDuration(6 * time.Second)
	result.AssertContains(
		"Starting runner `runner-18`",
		"2 proj-2:exit ⟫ Waiting 3 seconds before execution...",
		"1 proj-2:exit ⟫ Waiting 0.2 seconds before execution...",
		"2 proj-2:exit ⟫ timeout from exit.js",
	)

	result = testerWithTTL(8*time.Second, "runner-19")
	result.AssertMinDuration(3 * time.Second)
	result.AssertPortTimeoutError(5001, 2.0)
	result.AssertContains(
		"proj-33 ⟫ Waiting 3 seconds before execution...",
		"proj-2:test-1 ⟫ Checking if port 5001 is ready for connection... (timeout in 30 seconds)",
		"proj-2:test-1 ⟫ running main.js",
	)

	result = testerWithTTL(5*time.Second, "runner-20")
	result.AssertPortTimeoutError(5001, 2.0)
	result.AssertContains(
		"runner-20 ⟫ Executing `python3 -m http.server 5001`",
		"proj-2:test-1 ⟫ Checking if port 5001 is ready for connection... (timeout in 30 seconds)",
		"proj-2:test-1 ⟫ running main.js",
	)

	result = testerWithTTL(12*time.Second, "runner-21")
	result.AssertMinDuration(10 * time.Second)
	result.AssertContains("Checking if port 5001 is ready for connection... (timeout in 10 seconds)")

	result = testerWithTTL(12*time.Second, "runner-22")
	result.AssertMinDuration(10 * time.Second)
	result.AssertPortTimeoutError(5001, 2.0)
	result.AssertPortTimeoutError(5002, 2.0)
	result.AssertPortTimeoutError(5003, 2.0)
	result.AssertContains(
		"Checking if port 5001 is ready for connection... (timeout in 30 seconds)",
		"Checking if port 5001 is ready for connection... (timeout in 50 seconds)",
		"Checking if port 5002 is ready for connection... (timeout in 50 seconds)",
		"Checking if port 5003 is ready for connection... (timeout in 50 seconds)",
		"1 proj-2:test-1 ⟫ running main.js",
		"2 proj-2:test-1 ⟫ running main.js",
	)

	result = testerWithTTL(15*time.Second, "runner-23")
	result.AssertMinDuration(1 * time.Second)
	result.AssertContains("Checking if port 5001 is ready for connection... (timeout in 30 seconds)")
	result.AssertMinOccurrences("timeout from exit.js", 4)
	result.AssertPortTimeoutError(5001, 2.0)

	result = testerWithTTL(15*time.Second, "runner-24")
	result.AssertMinDuration(6 * time.Second)
	result.AssertContains(
		"Checking if port 5001 is ready for connection... (timeout in 2 seconds)",
		"ERROR: Timeout reached after 2 seconds waiting for port 5001 to become ready for connection",
		"Restarting in 1 seconds... (attempt 1/3)",
		"Restarting in 1 seconds... (attempt 2/3)",
		"Restarting in 1 seconds... (attempt 3/3)",
	)

	result = testerWithTTL(19*time.Second, "runner-25")
	result.AssertMinDuration(17 * time.Second)
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Checking if port 5001 is ready for connection... (timeout in 15.5 seconds)",
		"node-wait-port ⟫ Checking if port 5001 is ready for connection... (timeout in 30 seconds)",
	)

	result = testerWithTTL(23*time.Second, "runner-26")
	result.AssertMinDuration(21 * time.Second)
	result.AssertMinOccurrences("proj-2:exit ⟫ Executing `npm run exit`", 4)
	result.AssertMinOccurrences("proj-2:exit ⟫ Checking if port 5001 is ready for connection... (timeout in 30 seconds)", 4)
	result.AssertMinOccurrences("proj-2:exit ⟫ Checking if port 5002 is ready for connection... (timeout in 30 seconds)", 4)
	result.AssertPortTimeoutError(5001, 2.0)
	result.AssertPortTimeoutError(5002, 2.0)

	result = testerWithTTL(12*time.Second, "runner-27")
	result.AssertMinDuration(10 * time.Second)
	result.AssertMinOccurrences("timeout from exit.js", 3)
	result.AssertMinOccurrences("Restarting in 4 seconds...", 3)

	result = testerWithTTL(15*time.Second, "runner-28")
	result.AssertMinDuration(9 * time.Second)
	result.AssertMinOccurrences("timeout from exit.js", 4)
	result.AssertContains(
		"Restarting in 3 seconds... (attempt 1/3)",
		"Restarting in 3 seconds... (attempt 2/3)",
		"Restarting in 3 seconds... (attempt 3/3)",
	)

	result = testerWithTTL(15*time.Second, "runner-29")
	result.AssertMinDuration(11 * time.Second)
	result.AssertMinOccurrences("1 proj-2:exit ⟫ timeout from exit.js", 3)
	result.AssertMinOccurrences("2 proj-2:exit ⟫ timeout from exit.js", 4)

	result = testerWithTTL(6*time.Second, "runner-30")
	result.AssertMinDuration(4 * time.Second)
	result.AssertOccurrences("1 runner-30 ⟫ Executing `node npm`", 1)
	result.AssertMinOccurrences("2 runner-30 ⟫ Executing `node --version`", 3)

	result = testerWithTTL(6*time.Second, "runner-31")
	result.AssertMinDuration(4 * time.Second)
	result.AssertMinOccurrences("1 runner-31 ⟫ Executing `node npm`", 3)
	result.AssertMinOccurrences("2 runner-31 ⟫ Executing `node --version`", 3)

	result = testerWithTTL(6*time.Second, "runner-32")
	result.AssertMinDuration(4 * time.Second)
	result.AssertMinOccurrences("1 runner-32 ⟫ Executing `node npm`", 3)
	result.AssertOccurrences("2 runner-32 ⟫ Executing `node --version`", 1)

	result = testerWithTTL(8*time.Second, "runner-33")
	result.AssertMinDuration(7 * time.Second)
	result.AssertMinOccurrences("proj-2:exit ⟫ Executing `npm run exit`", 3)
	result.AssertMinOccurrences("proj-2:test-1 ⟫ Executing `npm run main`", 1)

	result = testerWithTTL(5*time.Second, "runner-34")
	result.AssertSequentialOrder(
		"1 proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"2 proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"1 proj-2:timeout-2 ⟫ Command(s) completed successfully",
		"2 proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
	)

	result = testerWithTTL(5*time.Second, "runner-35")
	result.AssertSequentialOrder(
		"1 proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"2 proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"1 proj-2:timeout-2 ⟫ Command(s) completed successfully",
		"2 proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
	)

	result = testerWithTTL(5*time.Second, "-s", "runner-36")
	result.AssertSequentialOrder(
		"1 proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"2 proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"1 proj-2:timeout-2 ⟫ Command(s) completed successfully",
		"2 proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
	)

	result = testerWithTTL(5*time.Second, "runner-37")
	result.AssertNotContains("proj-2:timeout-2")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"ERROR: A serial command in runner `runner-37` has failed",
	)

	result = testerWithTTL(5*time.Second, "runner-38")
	result.AssertNotContains("proj-2:timeout-2")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"ERROR: A serial command in runner `runner-38` has failed",
	)

	result = testerWithTTL(5*time.Second, "--serial", "runner-39")
	result.AssertNotContains("proj-2:timeout-2")
	result.AssertSequentialOrder(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:exit ⟫ Executing `npm run exit`",
		"ERROR: A serial command in runner `runner-39` has failed",
	)

	result = testerWithTTL(5*time.Second, "runner-40")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:exit ⟫ Executing `npm run exit`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"ERROR: A dependent command in runner `runner-40` has failed",
	)

	result = testerWithTTL(5*time.Second, "runner-41")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:exit ⟫ Executing `npm run exit`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"ERROR: A dependent command in runner `runner-41` has failed",
	)

	result = testerWithTTL(5*time.Second, "-d", "runner-42")
	result.AssertContains(
		"Starting runner `runner-42` with flags [dependent]",
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"proj-2:exit ⟫ Executing `npm run exit`",
		"ERROR: A dependent command in runner `runner-42` has failed",
	)

	result = testerWithTTL(5*time.Second, "--dependent", "runner-42")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"ERROR: A dependent command in runner `runner-42` has failed",
	)

	result = testerWithTTL(5*time.Second, "--dependent", "--serial", "runner-42")
	result.AssertNotContains("proj-2:timeout-2")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"ERROR: A serial command in runner `runner-42` has failed",
	)

	result = testerWithTTL(5*time.Second, "runner-43")
	result.AssertNotContains("proj-2:timeout-2")
	result.AssertContains(
		"Starting runner `runner-43` with flags [serial, dependent]",
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"ERROR: A serial command in runner `runner-43` has failed",
	)

	result = testerWithTTL(5*time.Second, "runner-44")
	result.AssertContains("proj-2:timeout-2 ⟫ ERROR: Parameter `awaits` must be a list of port numbers, or have the nested fields `ports` or `timeout`")

	result = testerWithTTL(5*time.Second, "runner-45")
	result.AssertContains("proj-2:timeout-2 ⟫ ERROR: Invalid port specification in `awaits`: invalid")

	result = testerWithTTL(5*time.Second, "runner-46")
	result.AssertContains("proj-2:timeout-2 ⟫ ERROR: Invalid port specification in `awaits.ports`: invalid")

	result = testerWithTTL(5*time.Second, "runner-47")
	result.AssertContains("proj-2:timeout-2 ⟫ ERROR: Parameter `awaits.ports` must be a list of port numbers")

	result = testerWithTTL(5*time.Second, "runner-48")
	result.AssertContains("Parameter `awaits.timeout` must be a number")

	result = testerWithTTL(8*time.Second, "runner-49")
	result.AssertOccurrences("1 watch-mode:error ⟫ Starting in watch mode", 4)
	result.AssertOccurrences("2 watch-mode:test ⟫ Starting in watch mode", 1)
	result.AssertOccurrences("1 watch-mode:error ⟫ ERROR: Command `node exit.js` failed with exit code exit status 1", 4)
	result.AssertNotContains("2 watch-mode:test ⟫ Restarting in")
	result.AssertContains(
		"1 watch-mode:error ⟫ Restarting in 1 seconds... (attempt 1/3)",
		"1 watch-mode:error ⟫ Restarting in 1 seconds... (attempt 2/3)",
		"1 watch-mode:error ⟫ Restarting in 1 seconds... (attempt 3/3)",
		"1 watch-mode:error ⟫ WARNING: Maximum retry attempts (3) reached. Terminating",
	)

	result = asyncTester("runner-50")
	result.ExecuteAsync(10*time.Second, func(terminate func()) {
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_1.html")))
		time.Sleep(1 * time.Second)
		cleanUpFunctions.Add(utils.ModifyTestFile(t, filepath.Join(fixturesDir, "node", "index_2.html")))
		time.Sleep(2 * time.Second)
		terminate()
	})
	cleanUpFunctions.ExecuteAll()
	result.AssertMinDuration(4 * time.Second)
	result.AssertOccurrences("watch-mode-1:test ⟫ Starting in watch mode", 1)
	result.AssertOccurrences("watch-mode-2:test-2 ⟫ Starting in watch mode", 1)
	result.AssertOccurrences("watch-mode-1:test ⟫ Executing `node continuous.js`", 1)
	result.AssertOccurrences("watch-mode-2:test-2 ⟫ running main.js", 2)
	result.AssertOccurrences("File change detected. Stopping running command...", 1)
	result.AssertNotContains("Restarting in")

	result = testerWithTTL(6*time.Second, "runner-51")
	result.AssertMinDuration(6 * time.Second)
	result.AssertMinOccurrences("runner-51 ⟫ 1 second passed", 4)
	result.AssertSequentialOrder(
		"proj-50:test ⟫ Executing `python3 exit.py`",
		"proj-50:test ⟫ ERROR: Command `python3 exit.py` failed with exit code exit status 1",
		"proj-50:test ⟫ Running `after` command...",
		"proj-50:test ⟫ Running project-level `after.failure` command...",
		"proj-50:test ⟫ After command(s) completed successfully",
	)

	result = testerWithTTL(11*time.Second, "runner-52")
	result.AssertMinDuration(6 * time.Second)
	result.AssertMinOccurrences("runner-52 ⟫ 1 second passed", 8)
	result.AssertOccurrences("proj-50:test ⟫ Restarting in 1 seconds...", 2)
	result.AssertOccurrences("proj-50:test ⟫ Executing `python3 exit.py`", 3)
	result.AssertOccurrences("proj-50:test ⟫ ERROR: Command `python3 exit.py` failed with exit code exit status 1", 3)
	result.AssertOccurrences("proj-50:test ⟫ Running `after` command...", 3)
	result.AssertOccurrences("proj-50:test ⟫ Running project-level `after.failure` command...", 3)
	result.AssertOccurrences("proj-50:test ⟫ After command(s) completed successfully", 3)

	result = testerWithTTL(9*time.Second, "runner-53")
	result.AssertMinDuration(8 * time.Second)
	result.AssertMinOccurrences("runner-53 ⟫ 1 second passed", 6)
	result.AssertSequentialOrder(
		"proj-50:test ⟫ Waiting 2 seconds before execution...",
		"proj-50:test ⟫ Executing `python3 exit.py`",
		"proj-50:test ⟫ ERROR: Command `python3 exit.py` failed with exit code exit status 1",
		"proj-50:test ⟫ Running `after` command...",
		"proj-50:test ⟫ Running project-level `after.failure` command...",
		"proj-50:test ⟫ After command(s) completed successfully",
	)

	result = tester("runner-54")
	result.AssertContains(
		"Starting runner `runner-54`",
		"other[dependent]:command[serial] ⟫ Executing `npm run timeout-3`",
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"proj-2:timeout-2 ⟫ Command(s) completed successfully",
		"other[dependent]:command[serial] ⟫ Command(s) completed successfully",
	)

	result = tester("runner-55")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"other[serial]format:command[dependent]format ⟫ Executing `npm run exit`",
		"proj-2 ⟫ Executing `npm run timeout-3`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"other[serial]format:command[dependent]format ⟫ ERROR: Command `npm run exit` failed with exit code exit status 1",
		"proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"proj-2:timeout-2 ⟫ Command(s) completed successfully",
		"proj-2 ⟫ Command(s) completed successfully",
	)

	result = tester("runner-56[serial]format")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"proj-2:timeout-2 ⟫ Command(s) completed successfully",
	)

	result = tester("runner-57[dependent]format")
	result.AssertContains(
		"proj-2:exit ⟫ Executing `npm run exit`",
		"proj-2 ⟫ Executing `npm run timeout-3`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"proj-2:exit ⟫ ERROR: Command `npm run exit` failed with exit code exit status 1",
		"proj-2:timeout-2 ⟫ Command(s) completed successfully",
		"proj-2 ⟫ Command(s) completed successfully",
	)

	result = tester("runner-58[serial,dependent]format")
	result.AssertContains(
		"proj-2:timeout-1 ⟫ Executing `npm run timeout-1`",
		"proj-2:timeout-2 ⟫ Executing `npm run timeout-2`",
		"proj-2:exit ⟫ Executing `npm run exit`",
		"proj-2:timeout-1 ⟫ Command(s) completed successfully",
	)
	result.AssertContains(
		"proj-2:exit ⟫ ERROR: Command `npm run exit` failed with exit code exit status 1",
		"proj-2:timeout-1 ⟫ Command(s) completed successfully",
		"proj-2:timeout-2 ⟫ Command(s) completed successfully",
		"proj-2 ⟫ Command(s) completed successfully",
	)

	result = tester("runner-59")
	result.AssertContains(
		"Starting runner `runner-59`",
		"1 other-loooooooooooooooong-project-name:command-... ⟫ Executing `node -v`",
		"2 other-loooooooooooooooong-project-name:command-... ⟫ Executing `node -v`",
	)

	result = tester("runner-60")
	result.AssertContains(
		"Starting runner `runner-60`",
		"general-2 ⟫ 1 => command",
		"general-2 ⟫ Command(s) completed successfully",
		"proj-2:test-1 ⟫ running main.js",
		"proj-2:test-1 ⟫ Command(s) completed successfully",
	)

	result = tester("runner-61")
	result.AssertContains(
		"Starting runner `runner-61`",
		"1 cmd-9 ⟫ running timeout_1.js",
		"1 cmd-9 ⟫ Command(s) completed successfully",
		"2 cmd-9 ⟫ running main.js",
		"2 cmd-9 ⟫ Command(s) completed successfully",
	)

	result = testerWithTTL(7*time.Second, "runner-62")
	result.AssertOccurrences("cmd-9 ⟫ Restarting in 1 seconds...", 2)
	result.AssertOccurrences("cmd-9 ⟫ Executing `node timeout_1.js`", 2)
	result.AssertOccurrences("cmd-9 ⟫ Waiting 2 seconds before execution...", 3)
	result.AssertOccurrences("proj-2:test-1 ⟫ Executing `npm run main`", 1)

	result = testerWithTTL(3*time.Second, "runner-63")
	result.AssertContains(
		"Starting runner `runner-63`",
		"runner-63 ⟫ Waiting 1 seconds before execution...",
		"cmd-9 ⟫ Checking if port 5001 is ready for connection... (timeout in 30 seconds)",
	)
	result.AssertSequentialOrder(
		"runner-63 ⟫ Executing `python3 -m http.server 5001`",
		"cmd-9 ⟫ Port 5001 is ready for connection",
		"cmd-9 ⟫ Executing `node timeout_1.js`",
		"cmd-9 ⟫ Command(s) completed successfully",
	)

	result = tester("runner-64")
	result.AssertContains(
		"Starting runner `runner-64`",
		"general-1:general-1 ⟫ Executing `node args.js project`",
		"general-1:general-2 ⟫ Executing `node args.js project`",
		"general-1:general-1 ⟫ 1 => project",
		"general-1:general-2 ⟫ 1 => project",
		"general-1:general-1 ⟫ Command(s) completed successfully",
		"general-1:general-2 ⟫ Command(s) completed successfully",
	)

	result = tester("runner-65")
	result.AssertContains(
		"Starting runner `runner-65`",
		"cmd-1 ⟫ Executing `node ./node/timeout_1.js`",
		"cmd-1 ⟫ timeout from timeout_1.js",
		"cmd-1 ⟫ Executing `node ./node/timeout_2.js`",
		"cmd-1 ⟫ timeout from timeout_2.js",
		"general-1:general-1 ⟫ Executing `node args.js project`",
		"general-1:general-2 ⟫ Executing `node args.js project`",
		"general-1:general-1 ⟫ 1 => project",
		"general-1:general-2 ⟫ 1 => project",
	)

	result = testerWithTTL(7*time.Second, "runner-66")
	result.AssertContains(
		"Starting runner `runner-66`",
		"cmd-1 ⟫ Executing `node ./node/timeout_1.js`",
		"cmd-1 ⟫ timeout from timeout_1.js",
		"cmd-1 ⟫ Executing `node ./node/timeout_2.js`",
		"cmd-1 ⟫ timeout from timeout_2.js",
		"general-1:general-1 ⟫ Executing `node args.js project`",
		"general-1:general-2 ⟫ Executing `node args.js project`",
		"general-1:general-1 ⟫ 1 => project",
		"general-1:general-2 ⟫ 1 => project",
	)
	result.AssertOccurrences("general-1:general-1 ⟫ Waiting 2 seconds before execution...", 3)
	result.AssertOccurrences("general-1:general-2 ⟫ Waiting 2 seconds before execution...", 3)
	result.AssertOccurrences("general-1:general-1 ⟫ Restarting in 1 seconds...", 2)
	result.AssertOccurrences("general-1:general-2 ⟫ Restarting in 1 seconds...", 2)
	result.AssertOccurrences("general-1:general-1 ⟫ Executing `node args.js project`", 2)
	result.AssertOccurrences("general-1:general-2 ⟫ Executing `node args.js project`", 2)

	result = testerWithTTL(7*time.Second, "runner-67")
	result.AssertNotContains("other-1")
	result.AssertContains(
		"Starting runner `runner-67` with flags [serial, dependent]",
		"other:format:command:format ⟫ Executing `go run main.go`",
		"other:format:command:format ⟫ Command(s) completed successfully",
		"proj-50:test ⟫ Executing `python3 exit.py`",
		"proj-50:test ⟫ ERROR: Command `python3 exit.py` failed with exit code exit status 1",
		"proj-50:test ⟫ Running `after` command...",
		"proj-50:test ⟫ Executing `python3 main.py`",
		"proj-50:test ⟫ Running project-level `after.failure` command...",
		"proj-50:test ⟫ Executing `python3 main.py`",
		"proj-50:test ⟫ After command(s) completed successfully",
		"ERROR: A serial command in runner `runner-67` has failed",
	)

	result = testerWithTTL(7*time.Second, "runner-68")
	result.AssertSequentialOrder(
		"Starting runner `runner-68`",
		"other:format:command:format ⟫ Executing `go run main.go`",
		"other:format:command:format ⟫ running main.go",
		"other:format:command:format ⟫ Command(s) completed successfully",
	)
}

func TestProjectExecution(t *testing.T) {
	var result utils.TestResult
	tester := utils.CreateStandardTester(t, fixturesDir)
	testerWithTTL := utils.CreateTesterWithTimeout(t, fixturesDir)
	helpTextArr := strings.Split(navi.HelpText, "\n")

	result = tester("-h")
	result.AssertSequentialOrder(helpTextArr...)

	result = tester("--help")

	result = tester("-v")
	result.AssertContains(navi.NaviVersion)

	result = tester("--version")
	result.AssertContains(navi.NaviVersion)

	cleanUpFunctions.Add(utils.MoveTestFile(t, filepath.Join(fixturesDir, "navi.yml"), filepath.Join(fixturesDir, "yml", "navi.yml"), true))
	result = tester("-f", "./flags/navi.yml", "proj-1:file")
	result.AssertContains("file in flags/")
	cleanUpFunctions.ExecuteAll()

	result = tester("--file", "./flags/test.yml", "proj-1:test")
	result.AssertContains("\ncwd => " + filepath.Join(fixturesDir, "flags"))

	result = tester("cmd-1")
	result.AssertSequentialOrder(
		"Executing `node ./node/timeout_1.js`",
		"running timeout_1.js",
		"timeout from timeout_1.js",
		"Executing `node ./node/timeout_2.js`",
		"running timeout_2.js",
		"timeout from timeout_2.js",
		"Command(s) completed successfully",
	)

	result = tester("cmd-2")
	result.AssertSequentialOrder(
		"Executing `node ./node/timeout_1.js`",
		"running timeout_1.js",
		"timeout from timeout_1.js",
		"Executing `node "+filepath.ToSlash(filepath.Join(fixturesDir, "node", "timeout_2.js"))+"`",
		"running timeout_2.js",
		"timeout from timeout_2.js",
		"Executing `node ./node/env_vars.js`",
		"main ENV_VAR1: undefined",
		"main ENV_VAR2: 999",
		"main ENV_VAR3: undefined",
		"main ENV_VAR4: false",
		"main ENV_VAR5: undefined",
	)

	result = tester("cmd-3")
	result.AssertSequentialOrder(
		"Executing `python3 timeout_1.py`",
		"Starting `timeout_1.py`",
		"Finished after 2 seconds",
		"Executing `python3 timeout_2.py`",
		"Starting `timeout_2.py`",
		"Finished after 3 seconds",
		"Executing `python3 env_vars.py`",
		"main ENV_VAR1: None",
		"main ENV_VAR2: 888",
		"main ENV_VAR3: None",
		"main ENV_VAR4: */-+.!#@$&_%",
	)

	if runtime.GOOS == "windows" {
		result = tester("cmd-4")
		result.AssertSequentialOrder(
			"Executing `echo \"cwd => %CD%\"`",
			"cwd => "+filepath.Join(fixturesDir),
		)
	} else { // unix
		result = tester("cmd-5")
		result.AssertSequentialOrder(
			"Executing `echo \"cwd => $PWD\"`",
			"cwd => "+filepath.Join(fixturesDir),
		)
	}

	result = tester("cmd-7", "arg1", "arg2")
	result.AssertSequentialOrder(
		"Executing `node "+filepath.ToSlash(filepath.Join(fixturesDir, "node", "args.js"))+"`",
		"No arguments found",
		"Executing `node node/args.js arg1 arg2`",
		"1 => arg1",
		"2 => arg2",
	)

	result = tester("cmd-8", "arg1", "arg2")
	result.AssertSequentialOrder(
		"Executing `node "+filepath.ToSlash(filepath.Join(fixturesDir, "node", "args.js"))+"`",
		"No arguments found",
		"Executing `node args.js arg1 arg2`",
		"1 => arg1",
		"2 => arg2",
	)

	result = tester("general-1:general-1")
	result.AssertContains("1 => project")

	result = tester("general-2")
	result.AssertContains("1 => command")

	result = tester("general-1:general-2")
	result.AssertContains("1 => project")

	result = tester("other:format:command:format")
	result.AssertContains("Executing `go run main.go`")

	result = tester("other:format", "node", "--version")
	result.AssertContains(
		"Executing `node --version`",
		"Command(s) completed successfully",
	)

	result = tester("other:format:", "python3", "--version")
	result.AssertContains(
		"Executing `python3 --version`",
		"Command(s) completed successfully",
	)

	result = tester("other:format::command:format:")
	result.AssertContains("Executing `python3 main.py`")

	result = tester("other[dependent]:command[serial]")
	result.AssertContains("Executing `npm run timeout-3`")

	result = tester("other[serial]format:command[dependent]format")
	result.AssertContains("Executing `npm run exit`")

	result = tester("proj-1:test")
	result.AssertContains("\ncwd => " + filepath.Join(fixturesDir))

	result = tester("proj-1:file")
	result.AssertContains("file in fixtures/")

	result = tester("proj-2:test-1")
	result.AssertContains(
		"Executing `npm run main`",
		"running main.js",
	)

	result = tester("proj-2:test-2")
	result.AssertContains(
		"Executing `npm run main`",
		"running main.js",
	)

	result = tester("proj-2", "npm", "version")
	result.AssertContains(
		"npm: ", "node: ",
		"Executing `npm version`",
	)

	result = tester("proj-2:env-vars-1")
	result.AssertContains(
		"pre ENV_VAR3: qwe",
		"pre ENV_VAR5: undefined",
		"post ENV_VAR1: a/b",
		"post ENV_VAR3: qwe",
		"post ENV_VAR4: undefined",
		"main ENV_VAR1: a/b",
		"main ENV_VAR3: qwe",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR5: undefined",
		"main ENV_VAR1: a/b",
		"post ENV_VAR4: undefined",
	)

	result = tester("proj-2:env-vars-2")
	result.AssertContains(
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: '|@%&*'",
		"main ENV_VAR3: abc",
		"main ENV_VAR4: false",
		"main ENV_VAR5: undefined",
		"Executing `npm run env-vars`",
	)

	result = tester("proj-2:env-vars-3")
	result.AssertContains(
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: '|@%&*'",
		"main ENV_VAR3: abc",
		"main ENV_VAR4: false",
		"main ENV_VAR5: undefined",
		"Executing `npm run env-vars`",
	)

	result = tester("proj-2:env-vars-4")
	result.AssertContains(
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: '|@%&*'",
		"main ENV_VAR3: abc",
		"main ENV_VAR4: false",
		"main ENV_VAR5: undefined",
		"Executing `npm run env-vars`",
	)

	result = tester("proj-2:env-vars-5")
	result.AssertContains(
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: '|@%&*'",
		"main ENV_VAR3: qwe",
		"main ENV_VAR4: false",
		"main ENV_VAR5: undefined",
		"Executing `npm run env-vars`",
	)

	result = tester("proj-2:multi-1")
	result.AssertOccurrences("Command(s) completed successfully", 1)
	result.AssertContains(
		"Executing `npm run timeout-1`",
		"running timeout_1.js",
		"timeout from timeout_1.js",
		"Executing `npm run timeout-2`",
		"running timeout_2.js",
		"timeout from timeout_2.js",
	)

	result = tester("proj-2:multi-2", "node", "--version")
	result.AssertOccurrences("Executing `npm run args`", 2)
	result.AssertOccurrences("Executing `node args.js`", 1)
	result.AssertOccurrences("Executing `node args.js node --version`", 1)
	result.AssertOccurrences("No arguments found", 3)
	result.AssertOccurrences("1 => node", 1)
	result.AssertOccurrences("2 => --version", 1)
	result.AssertOccurrences("Command(s) completed successfully", 1)

	if runtime.GOOS == "windows" {
		result = tester("proj-2:cwd-cmd")
		result.AssertContains("cwd => " + filepath.Join(fixturesDir, "node"))

		result = tester("proj-2:cwd-cmd-2")
		result.AssertContains("cwd => " + filepath.Join(fixturesDir, "node"))

		result = tester("proj-2:cwd-powershell")
		result.AssertContains("cwd => " + filepath.Join(fixturesDir, "node"))

		result = tester("proj-2:cwd-powershell-2")
		result.AssertContains("cwd => " + filepath.Join(fixturesDir, "node"))
	} else { // unix
		result = testerWithTTL(2*time.Second, "proj-2:shutdown-1")
		result.AssertOccurrences("Received SIGTERM", 1)
		result.AssertContains(
			"WARNING: Received `terminated` signal",
			"WARNING: Shutting down processes... (don't close the terminal)",
			"Received SIGTERM. Shutting down in 5 seconds...",
			"Shutdown timeout reached. Forcing exit now.",
		)

		result = testerWithTTL(2*time.Second, "proj-2:shutdown-2")
		result.AssertOccurrences("Received SIGTERM", 1)
		result.AssertContains(
			"WARNING: Received `terminated` signal",
			"WARNING: Shutting down processes... (don't close the terminal)",
			"Received SIGTERM. Shutting down in 5 seconds...",
			"Shutdown timeout reached. Forcing exit now.",
			"Running `after` command...",
			"timeout from timeout_3.js",
			"After command(s) completed successfully",
		)

		result = tester("proj-2:cwd-bash")
		result.AssertContains("cwd => " + filepath.Join(fixturesDir, "node"))

		result = tester("proj-2:cwd-bash-2")
		result.AssertContains("cwd => " + filepath.Join(fixturesDir, "node"))

		result = tester("proj-2:cwd-no-shell-unix")
		result.AssertContains("cwd => $PWD")
	}

	result = tester("proj-3:env-vars")
	result.AssertContains(
		"pre ENV_VAR3: qwe",
		"pre ENV_VAR5: undefined",
		"post ENV_VAR1: a/b",
		"post ENV_VAR3: qwe",
		"post ENV_VAR4: */-+.!#@$&_%",
		"main ENV_VAR1: a/b",
		"main ENV_VAR3: qwe",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR5: undefined",
		"main ENV_VAR1: a/b",
		"post ENV_VAR4: */-+.!#@$&_%",
	)

	result = tester("proj-4:env-vars")
	result.AssertContains(
		"pre ENV_VAR3: undefined",
		"pre ENV_VAR5: undefined",
		"main ENV_VAR1: a/b",
		"main ENV_VAR3: qwe",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR5: undefined",
		"main ENV_VAR1: a/b",
	)

	result = tester("proj-5:env-vars-1")
	result.AssertContains(
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR3: abc",
		"main ENV_VAR1: a/b",
		"main ENV_VAR3: qwe",
		"main ENV_VAR4: */-+.!#@$&_%",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"main ENV_VAR1: a/b",
		"post ENV_VAR1: \"|@%&*\"",
	)

	result = tester("proj-5:env-vars-2")
	result.AssertContains(
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR3: abc",
		"main ENV_VAR1: a/b",
		"main ENV_VAR3: qwe",
		"main ENV_VAR4: */-+.!#@$&_%",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"main ENV_VAR1: a/b",
		"post ENV_VAR1: \"|@%&*\"",
	)

	result = tester("proj-6:env-vars")
	result.AssertContains(
		"main ENV_VAR1: a/b",
		"main ENV_VAR3: qwe",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: qwe",
		"main ENV_VAR1: a/b",
	)

	result = tester("proj-7:env-vars")
	result.AssertContains(
		"pre ENV_VAR3: undefined",
		"pre ENV_VAR5: undefined",
		"post ENV_VAR1: undefined",
		"post ENV_VAR3: undefined",
		"post ENV_VAR4: undefined",
		"main ENV_VAR1: undefined",
		"main ENV_VAR3: undefined",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: undefined",
		"main ENV_VAR3: undefined",
		"post ENV_VAR1: undefined",
	)

	result = tester("proj-8:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: 888",
		"pre ENV_VAR2: 999",
		"pre ENV_VAR3: ddd",
		"pre ENV_VAR4: 1\\2",
		"pre ENV_VAR5: 3/4",
		"main ENV_VAR1: a/b",
		"main ENV_VAR2: c\\d",
		"main ENV_VAR3: qwe",
		"main ENV_VAR4: */-+.!#@$&_%",
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR3: qwe",
		"post ENV_VAR4: undefined",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: ddd",
		"main ENV_VAR3: qwe",
		"post ENV_VAR1: \"|@%&*\"",
	)

	result = tester("proj-9:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR2: None",
		"pre ENV_VAR3: qwe",
		"pre ENV_VAR4: None",
		"pre ENV_VAR5: None",
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: None",
		"main ENV_VAR3: qwe",
		"main ENV_VAR4: None",
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR2: None",
		"post ENV_VAR3: qwe",
		"post ENV_VAR4: None",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: qwe",
		"main ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR4: None",
	)

	result = tester("proj-10:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR2: None",
		"pre ENV_VAR3: qwe",
		"pre ENV_VAR4: None",
		"pre ENV_VAR5: None",
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: None",
		"main ENV_VAR3: qwe",
		"main ENV_VAR4: None",
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR2: None",
		"post ENV_VAR3: true",
		"post ENV_VAR4: \"quoted test",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: qwe",
		"main ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR4: \"quoted test",
	)

	result = tester("proj-11:env-vars")
	result.AssertContains(
		"pre ENV_VAR3: abc 123",
		"pre ENV_VAR5: undefined",
		"post ENV_VAR1: false",
		"post ENV_VAR3: abc 123",
		"post ENV_VAR4: undefined",
		"main ENV_VAR1: false",
		"main ENV_VAR2: undefined",
		"main ENV_VAR3: abc 123",
		"main ENV_VAR4: undefined",
		"main ENV_VAR5: undefined",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: abc 123",
		"main ENV_VAR3: abc 123",
		"post ENV_VAR3: abc 123",
	)

	result = tester("proj-12:env-vars")
	result.AssertContains(
		"pre ENV_VAR3: abc \" \\' \\\"  abc",
		"pre ENV_VAR5: undefined",
		"post ENV_VAR1: false",
		"post ENV_VAR3: abc \" \\' \\\"  abc",
		"post ENV_VAR4: undefined",
		"main ENV_VAR1: false",
		"main ENV_VAR2: undefined",
		"main ENV_VAR3: abc \" \\' \\\"  abc",
		"main ENV_VAR4: undefined",
		"main ENV_VAR5: undefined",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: abc \" \\' \\\"  abc",
		"main ENV_VAR3: abc \" \\' \\\"  abc",
		"post ENV_VAR3: abc \" \\' \\\"  abc",
	)

	result = tester("proj-13:env-vars")
	result.AssertContains(
		"pre ENV_VAR3: abc \" abc",
		"pre ENV_VAR5: undefined",
		"post ENV_VAR1: false",
		"post ENV_VAR3: abc \" abc",
		"post ENV_VAR4: undefined",
		"main ENV_VAR1: false",
		"main ENV_VAR2: undefined",
		"main ENV_VAR3: abc \" abc",
		"main ENV_VAR4: undefined",
		"main ENV_VAR5: undefined",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: abc \" abc",
		"main ENV_VAR3: abc \" abc",
		"post ENV_VAR3: abc \" abc",
	)

	result = tester("proj-14:env-vars")
	result.AssertContains(
		"pre ENV_VAR3: 5000",
		"pre ENV_VAR5: undefined",
		"post ENV_VAR1: undefined",
		"post ENV_VAR3: 5000",
		"post ENV_VAR4: undefined",
		"main ENV_VAR1: undefined",
		"main ENV_VAR2: undefined",
		"main ENV_VAR3: 5000",
		"main ENV_VAR4: undefined",
		"main ENV_VAR5: undefined",
		"Executing `npm run env-vars`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR3: 5000",
		"main ENV_VAR3: 5000",
		"post ENV_VAR3: 5000",
	)

	if runtime.GOOS == "windows" {
		result = tester("proj-15:shell-test")
		result.AssertContains(
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)
		result.AssertSequentialOrder(
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)

		result = tester("proj-16:shell-test")
		result.AssertContains(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)
		result.AssertSequentialOrder(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)

		result = tester("proj-17:shell-test")
		result.AssertContains(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)
		result.AssertSequentialOrder(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)

		result = tester("proj-18:shell-test")
		result.AssertContains(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)
		result.AssertSequentialOrder(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)

		result = tester("proj-19:shell-test")
		result.AssertContains(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)
		result.AssertSequentialOrder(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)
	} else { // unix
		result = tester("proj-20:shell-test")
		result.AssertContains("main path " + filepath.Join(fixturesDir, "node"))

		result = tester("proj-21:shell-test")
		result.AssertContains("main path " + filepath.Join(fixturesDir, "node"))

		result = tester("proj-22:shell-test")
		result.AssertContains(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"222,",
			"Executing `array=(111, 222, 333); echo \"${array[1]}\"`",
			"post path "+filepath.Join(fixturesDir, "node"),
		)
		result.AssertSequentialOrder(
			"pre path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)

		result = tester("proj-23:shell-test")
		result.AssertContains(
			"222,",
			"Executing `array=(111, 222, 333); echo \"${array[1]}\"`",
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)
		result.AssertSequentialOrder(
			"main path "+filepath.Join(fixturesDir, "node"),
			"post path "+filepath.Join(fixturesDir, "node"),
		)
	}

	result = tester("proj-24", "npm", "run", "env-vars")
	result.AssertSequentialOrder(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR3: abc",
		"pre ENV_VAR5: abcde",
		"main ENV_VAR1: a/b",
		"main ENV_VAR3: qwe",
		"main ENV_VAR5: abcde",
		"post ENV_VAR1: a/b",
		"post ENV_VAR3: qwe",
		"post ENV_VAR5: abcde",
	)

	if runtime.GOOS == "windows" {
		result = tester("proj-25:test")
		result.AssertSequentialOrder(
			"pre ENV_VAR1: \"|@%&*\"",
			"pre ENV_VAR3: abc",
			"pre ENV_VAR5: custom5",
			"main ENV_VAR1: abc123",
			"main ENV_VAR3: qwe",
			"main ENV_VAR5: custom5",
			"post ENV_VAR1: a/b",
			"post ENV_VAR3: qwe",
			"post ENV_VAR5: custom5",
		)
	} else { // unix
		result = tester("proj-26", "echo", "main exec")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `array=(111, 222, 333); echo \"${array[1]} ENV_VAR1=${ENV_VAR1} ENV_VAR3=${ENV_VAR3} ENV_VAR5=${ENV_VAR5}\"`",
			"222, ENV_VAR1=\"|@%&*\" ENV_VAR3=abc ENV_VAR5=abcde",
			"Executing `echo \"main exec\"`",
			"\nmain exec",
			"Running project-level `post` command...",
			"Executing `array=(111, 222, 333); echo \"${array[1]} ENV_VAR1=${ENV_VAR1} ENV_VAR5=${ENV_VAR5}\"`",
			"222, ENV_VAR1=a/b ENV_VAR5=5000",
			"Command(s) completed successfully",
		)

		result = tester("proj-27:test")
		result.AssertSequentialOrder(
			"pre ENV_VAR1: \"|@%&*\"",
			"pre ENV_VAR3: abc",
			"pre ENV_VAR5: custom5",
			"main ENV_VAR1: abc123",
			"main ENV_VAR3: qwe",
			"main ENV_VAR5: custom5",
			"post ENV_VAR1: a/b",
			"post ENV_VAR3: qwe",
			"post ENV_VAR5: custom5",
		)
	}

	result = tester("proj-28:test", "node", "--version")
	result.AssertSequentialOrder(
		"No arguments found",
		"Executing `node node/args.js node --version`",
		"Provided arguments:",
		"1 => node",
		"2 => --version",
	)

	result = tester("proj-29:test", "node", "--version")
	result.AssertSequentialOrder(
		"Executing `node "+filepath.ToSlash(filepath.Join(fixturesDir, "node", "args.js"))+"`",
		"Provided arguments:",
		"No arguments found",
		"Executing `node args.js node --version`",
		"Provided arguments:",
		"1 => node",
		"2 => --version",
		"Command(s) completed successfully",
	)

	result = tester("proj-29:test", "single arg")
	result.AssertSequentialOrder(
		"Provided arguments:",
		"No arguments found",
		"Executing `node args.js \"single arg\"",
		"Provided arguments:",
		"1 => single arg",
	)

	result = tester("proj-29:test", "arg", "single arg")
	result.AssertSequentialOrder(
		"Provided arguments:",
		"No arguments found",
		"Executing `node args.js arg \"single arg\"",
		"Provided arguments:",
		"1 => arg",
		"2 => single arg",
	)

	if runtime.GOOS == "windows" {
		result = tester("proj-30:test")
		result.AssertSequentialOrder(
			"Running main command...",
			"Executing `node timeout_1.js`",
			"running timeout_1.js",
			"timeout from timeout_1.js",
			"Running main command...",
			"Executing `npm run timeout-2`",
			"running timeout_2.js",
			"timeout from timeout_2.js",
			"Running `post` command...",
			"Executing `npm run main`",
			"running main.js",
			"Executing `node args.js`",
			"Provided arguments:",
			"No arguments found",
			"Command(s) completed successfully",
		)
	} else { // unix
		result = tester("proj-31:test")
		result.AssertSequentialOrder(
			"Running `pre` command...",
			"Executing `npm run main`",
			"running main.js",
			"Executing `node args.js`",
			"Provided arguments:",
			"No arguments found",
			"Running main command...",
			"Executing `node timeout_1.js`",
			"running timeout_1.js",
			"timeout from timeout_1.js",
			"Running main command...",
			"Executing `npm run timeout-2`",
			"running timeout_2.js",
			"timeout from timeout_2.js",
			"Command(s) completed successfully",
		)

		result = tester("proj-32:env-vars")
		result.AssertSequentialOrder(
			"Running `pre` command...",
			"pre ENV_VAR3: "+filepath.Join(fixturesDir, "node"),
			"pre ENV_VAR5: undefined",
			"Running main command...",
			"main ENV_VAR1: undefined",
			"main ENV_VAR2: undefined",
			"main ENV_VAR3: "+filepath.Join(fixturesDir, "node"),
			"main ENV_VAR4: undefined",
			"main ENV_VAR5: undefined",
			"Running `post` command...",
			"post ENV_VAR1: undefined",
			"post ENV_VAR3: "+filepath.Join(fixturesDir, "node"),
			"post ENV_VAR4: undefined",
		)
	}

	result = tester("proj-33:test")
	result.AssertContains(
		"running main.py",
		"Executing `python3 main.py`",
	)

	result = tester("proj-33", "python3", "main.py")
	result.AssertContains(
		"running main.py",
		"Executing `python3 main.py`",
	)

	result = tester("proj-33:env-vars-1")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR3: abc",
		"pre ENV_VAR4: false",
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR4: false",
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR2: 789",
		"post ENV_VAR4: false",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR4: false",
		"post ENV_VAR4: false",
	)

	result = tester("proj-33:env-vars-2")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR3: None",
		"post ENV_VAR2: None",
		"post ENV_VAR4: false",
		"Executing `python3 env_vars.py`",
		"Executing `python3 env_vars_pre.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR4: false",
	)

	result = tester("proj-33:env-vars-3")
	result.AssertContains(
		"pre ENV_VAR1: None",
		"pre ENV_VAR3: None",
		"Executing `python3 env_vars.py`",
		"Executing `python3 env_vars_pre.py`",
	)

	result = tester("proj-33:env-vars-4")
	result.AssertContains(
		"post ENV_VAR2: None",
		"post ENV_VAR4: None",
		"Executing `python3 env_vars.py`",
	)

	result = tester("proj-33:env-vars-5")
	result.AssertContains(
		"main ENV_VAR3: abc",
		"Executing `python3 env_vars.py`",
	)

	result = tester("proj-33:env-vars-6")
	result.AssertContains(
		"main ENV_VAR3: abc",
		"Executing `python3 env_vars.py`",
	)

	result = tester("proj-34:env-vars")
	result.AssertContains(
		"main ENV_VAR2: None",
		"main ENV_VAR3: None",
		"Executing `python3 env_vars.py`",
	)

	result = tester("proj-35:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: None",
		"post ENV_VAR1: None",
		"main ENV_VAR1: None",
		"pre ENV_VAR3: None",
		"post ENV_VAR2: None",
		"pre ENV_VAR4: None",
		"post ENV_VAR4: None",
		"main ENV_VAR4: None",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR1: None",
		"main ENV_VAR4: None",
		"post ENV_VAR4: None",
	)

	result = tester("proj-36:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: 456",
		"pre ENV_VAR3: def",
		"pre ENV_VAR4: true",
		"main ENV_VAR1: 456",
		"main ENV_VAR4: true",
		"post ENV_VAR1: 456",
		"post ENV_VAR2: 789",
		"post ENV_VAR4: true",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR1: 456",
		"main ENV_VAR1: 456",
		"post ENV_VAR4: true",
	)

	result = tester("proj-37:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: 456",
		"pre ENV_VAR3: def",
		"main ENV_VAR1: 456",
		"main ENV_VAR3: None",
		"main ENV_VAR4: true",
		"post ENV_VAR1: 456",
		"post ENV_VAR2: 789",
		"post ENV_VAR4: true",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR1: 456",
		"main ENV_VAR1: 456",
		"post ENV_VAR4: true",
	)

	result = tester("proj-38:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: None",
		"pre ENV_VAR2: '|@%&*'",
		"pre ENV_VAR3: None",
		"pre ENV_VAR4: false",
		"main ENV_VAR1: None",
		"main ENV_VAR2: '|@%&*'",
		"main ENV_VAR3: None",
		"main ENV_VAR4: false",
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR2: '|@%&*'",
		"post ENV_VAR3: None",
		"post ENV_VAR4: false",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR1: None",
		"main ENV_VAR2: '|@%&*'",
		"post ENV_VAR4: false",
	)

	result = tester("proj-39:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR2: '|@%&*'",
		"pre ENV_VAR3: None",
		"pre ENV_VAR4: false",
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: '|@%&*'",
		"main ENV_VAR3: None",
		"main ENV_VAR4: false",
		"post ENV_VAR1: \"|@%&*\"",
		"post ENV_VAR2: 789",
		"post ENV_VAR3: None",
		"post ENV_VAR4: false",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR4: false",
		"main ENV_VAR4: false",
		"post ENV_VAR1: \"|@%&*\"",
	)

	result = tester("proj-40:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: 456",
		"pre ENV_VAR2: 789",
		"pre ENV_VAR3: def",
		"pre ENV_VAR4: false",
		"main ENV_VAR1: 456",
		"main ENV_VAR2: 789",
		"main ENV_VAR3: def",
		"main ENV_VAR4: false",
		"post ENV_VAR1: 456",
		"post ENV_VAR2: 789",
		"post ENV_VAR3: def",
		"post ENV_VAR4: true",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR4: false",
		"main ENV_VAR4: false",
		"post ENV_VAR1: 456",
	)

	result = tester("proj-41:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR2: '|@%&*'",
		"pre ENV_VAR3: abc",
		"pre ENV_VAR4: false",
		"pre ENV_VAR1: 456",
		"pre ENV_VAR2: 789",
		"pre ENV_VAR3: def",
		"pre ENV_VAR4: true",
		"main ENV_VAR1: 456",
		"main ENV_VAR2: 789",
		"main ENV_VAR3: def",
		"main ENV_VAR4: true",
		"post ENV_VAR1: 456",
		"post ENV_VAR2: '|@%&*'",
		"post ENV_VAR3: def",
		"post ENV_VAR4: true",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR1: 456",
		"main ENV_VAR2: 789",
		"post ENV_VAR2: '|@%&*'",
	)

	result = tester("proj-42:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: \"|@%&*\"",
		"pre ENV_VAR2: '|@%&*'",
		"pre ENV_VAR3: abc",
		"pre ENV_VAR4: None",
		"pre ENV_VAR1: 456",
		"pre ENV_VAR2: 789",
		"pre ENV_VAR3: def",
		"pre ENV_VAR4: true",
		"main ENV_VAR1: \"|@%&*\"",
		"main ENV_VAR2: '|@%&*'",
		"main ENV_VAR3: abc",
		"main ENV_VAR4: false",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR4: None",
		"pre ENV_VAR4: true",
		"main ENV_VAR1: \"|@%&*\"")

	result = tester("proj-43:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: 888",
		"pre ENV_VAR2: 999",
		"pre ENV_VAR3: ddd",
		"pre ENV_VAR4: None",
		"main ENV_VAR1: 888",
		"main ENV_VAR2: 999",
		"main ENV_VAR3: ddd",
		"main ENV_VAR4: None",
		"post ENV_VAR1: 456",
		"post ENV_VAR2: 789",
		"post ENV_VAR3: def",
		"post ENV_VAR4: true",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR2: 999",
		"main ENV_VAR1: 888",
		"post ENV_VAR1: 456",
	)

	result = tester("proj-44:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: None",
		"pre ENV_VAR2: 999",
		"pre ENV_VAR3: ddd",
		"pre ENV_VAR4: 1\\2",
		"pre ENV_VAR5: 3/4",
		"main ENV_VAR1: None",
		"main ENV_VAR2: 999",
		"main ENV_VAR3: ddd",
		"main ENV_VAR4: 1\\2",
		"post ENV_VAR1: None",
		"post ENV_VAR2: 999",
		"post ENV_VAR3: ddd",
		"post ENV_VAR4: 1\\2",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR2: 999",
		"main ENV_VAR4: 1\\2",
		"post ENV_VAR1: None",
	)

	result = tester("proj-45:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: true",
		"pre ENV_VAR2: 12.34",
		"pre ENV_VAR3: ddd",
		"pre ENV_VAR4: 1\\2",
		"pre ENV_VAR5: 3/4",
		"main ENV_VAR1: true",
		"main ENV_VAR2: 12.34",
		"main ENV_VAR3: ddd",
		"main ENV_VAR4: 1\\2",
		"post ENV_VAR1: true",
		"post ENV_VAR2: 12.34",
		"post ENV_VAR3: ddd",
		"post ENV_VAR4: 1\\2",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR2: 12.34",
		"main ENV_VAR4: 1\\2",
		"post ENV_VAR1: true",
	)

	result = tester("proj-46:env-vars")
	result.AssertContains(
		"pre ENV_VAR1: true string",
		"pre ENV_VAR2: false",
		"pre ENV_VAR3: ddd",
		"pre ENV_VAR4: 1\\2",
		"pre ENV_VAR5: 3/4",
		"main ENV_VAR1: None",
		"main ENV_VAR2: 999",
		"main ENV_VAR3: ddd",
		"main ENV_VAR4: 1\\2",
		"post ENV_VAR1: None",
		"post ENV_VAR2: 999",
		"post ENV_VAR3: ddd",
		"post ENV_VAR4: 1\\2",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR2: false",
		"main ENV_VAR4: 1\\2",
		"post ENV_VAR1: None",
	)

	result = tester("proj-47:env-vars")
	// Replace newlines for consistent testing across platforms
	result.CommandOutput = strings.ReplaceAll(result.CommandOutput, "\r\n", "[nl]") // Handle Windows
	result.CommandOutput = strings.ReplaceAll(result.CommandOutput, "\n", "[nl]")   // Handle Unix
	result.AssertContains(
		"pre ENV_VAR_STRING1: abc \"de fg\" hijk",
		"pre ENV_VAR_STRING2: abc'd efghijk",
		"pre ENV_VAR_STRING3: abc 'defg' hijk",
		"main ENV_VAR_STRING4: line1[nl]line2[nl]line3",
		"main ENV_VAR_STRING5: line1[nl]line2[nl]line3",
		"post ENV_VAR_STRING5: line1[nl]line2[nl]line3",
		"Executing `python3 env_vars.py`",
	)
	result.AssertSequentialOrder(
		"pre ENV_VAR_STRING3: abc 'defg' hijk",
		"main ENV_VAR_STRING4: line1[nl]line2[nl]line3",
		"post ENV_VAR_STRING5: line1[nl]line2[nl]line3",
	)

	result = tester("proj-48:env-vars")
	result.AssertSequentialOrder(
		"Executing `python3 main.py`",
		"running main.py",
		"Command(s) completed successfully",
		"Running `after.success` command...",
		"Executing `python3 env_vars_pre.py`",
		"pre ENV_VAR1: abc123",
		"pre ENV_VAR2: 999",
		"pre ENV_VAR3: ddd",
		"pre ENV_VAR4: 000",
		"Running `after.always` command...",
		"Executing `python3 env_vars.py`",
		"main ENV_VAR1: abc123",
		"main ENV_VAR2: def456",
		"main ENV_VAR3: ddd",
		"main ENV_VAR4: 000",
		"Running project-level `after` command...",
		"Executing `python3 env_vars_post.py`",
		"post ENV_VAR1: 888",
		"post ENV_VAR2: 999",
		"post ENV_VAR3: xyz789",
		"post ENV_VAR4: 000",
		"After command(s) completed successfully",
	)

	result = tester("proj-49:env-vars")
	result.AssertSequentialOrder(
		"Executing `python3 main.py`",
		"running main.py",
		"Command(s) completed successfully",
		"Running `after` command...",
		"Executing `python3 env_vars.py`",
		"main ENV_VAR1: a/b",
		"main ENV_VAR2: c\\d",
		"main ENV_VAR3: qwe",
		"main ENV_VAR4: */-+.!#@$&_%",
		"Running project-level `after` command...",
		"Executing `python3 env_vars_post.py`",
		"post ENV_VAR1: 888",
		"post ENV_VAR2: 999",
		"post ENV_VAR3: ddd",
		"post ENV_VAR4: 1\\2",
		"After command(s) completed successfully",
	)

	result = tester("proj-50:test")
	result.AssertSequentialOrder(
		"Executing `python3 exit.py`",
		"ERROR: Command `python3 exit.py` failed with exit code exit status 1",
		"Running `after` command...",
		"Running project-level `after.failure` command...",
		"After command(s) completed successfully",
		"WARNING: Shutting down processes... (don't close the terminal)",
	)

	result = tester("proj-51::list")
	result.AssertContains(
		"test/local",
		"Executing `go list`",
	)

	result = tester("proj-51", "npm", "version")
	result.AssertContains(
		"npm: ",
		"node: ",
		"Executing `npm version`",
	)

	result = tester("proj-51", "go", "run", "main.go")
	result.AssertContains(
		"running main.go",
		"Executing `go run main.go`",
	)

	result = tester("proj-51:npm", "version")
	result.AssertContains(
		"npm: ",
		"node: ",
		"Executing `npm version`",
	)

	result = tester("proj-52", "go", "run", "main.go")
	result.AssertContains(
		"running main.go",
		"Executing `go run main.go`",
	)

	result = tester("proj-53:list", "node", "--version")
	result.AssertSequentialOrder(
		"Running project-level `pre` command...",
		"Executing `node "+filepath.ToSlash(filepath.Join(fixturesDir, "node", "args.js"))+"`",
		"Provided arguments:",
		"No arguments found",
		"Running `pre` command...",
		"Executing `node args.js`",
		"Executing `node args.js node --version`",
		"1 => node",
		"2 => --version",
		"Running `post` command...",
		"Running project-level `post` command...",
	)

	result = tester("proj-53:list-2", "node", "--version")
	result.AssertSequentialOrder(
		"Running project-level `pre` command...",
		"Executing `node "+filepath.ToSlash(filepath.Join(fixturesDir, "node", "args.js"))+"`",
		"Provided arguments:",
		"No arguments found",
		"Running `pre` command...",
		"Executing `node args.js`",
		"Executing `node args.js node --version`",
		"1 => node",
		"2 => --version",
		"Running `post` command...",
		"Running project-level `post` command...",
	)

	if runtime.GOOS == "windows" {
		result = tester("proj-54:steps")
		result.AssertSequentialOrder(
			"echo \"pre 1\"",
			"echo \"pre 2\"",
			"echo \"main\"",
			"echo \"post 1\"",
			"echo \"post 2\"",
		)
	} else { // unix
		result = tester("proj-55:steps")
		result.AssertSequentialOrder(
			"echo \"pre 1\"",
			"echo \"pre 2\"",
			"echo \"main\"",
			"echo \"post 1\"",
			"echo \"post 2\"",
		)
	}

	if runtime.GOOS == "windows" {
		os.Setenv("SPECIAL_CHARS", "/\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|")
		result = tester("proj-56:run")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `echo \"special chars => %SPECIAL_CHARS%\"`",
			"\\\"special chars => /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|\\\"",
			"Executing `echo \"special chars => /\\\\@!#$&%*)\\\"-\\\"_(}]'[']{+=^~?:;.,<>|\"`",
			"\\\"special chars => /\\\\@!#$&%*)\\\\\\\"-\\\\\\\"_(}]'[']{+=^~?:;.,<>|\\\"",
			"Running project-level `post` command...",
			"Executing `node env_vars.js`",
			"main ENV_VAR1: /@!#$&%*)-_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR2: /@!#$&%*)-_(}]\"[\"]{+=^~?:;.,<>|",
			"main ENV_VAR3: /@!$&%*-_+=^~?:.,<>|",
			"main ENV_VAR4: %SPECIAL_CHARS%",
			"main ENV_VAR5: /\\\\@!#$&%*)\\\"-\\\"_(}]'[']{+=^~?:;.,<>|",
			"Command(s) completed successfully",
		)

		result = tester("proj-57:run")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `echo \"special chars => /\\\\@!#$&%*)\\\"-\\\"_(}]'[']{+=^~?:;.,<>|\"`",
			"\nspecial chars => /\\\\@!#$&%*)\\",
			"\n-\\_(}]'[']{+=^~?:;.,<>|",
			"Running project-level `post` command...",
			"Executing `node env_vars.js`",
			"main ENV_VAR1: /@!#$&%*)-_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR2: /@!#$&%*)-_(}]\"[\"]{+=^~?:;.,<>|",
			"main ENV_VAR3: /@!$&%*-_+=^~?:.,<>|",
			"main ENV_VAR4: /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR5: undefined",
			"Command(s) completed successfully",
		)

		result = tester("proj-58:test")
		result.AssertSequentialOrder(
			"Running `pre` command...",
			"Executing `echo \"/\\@!#$&%*)\"-\"_(}][]{+=^~?:;.,<>|\"`",
			"\\\"/\\@!#$&%*)\\\"-\\\"_(}][]{+=^~?:;.,<>|\\\"",
			"Executing `node --version`",
			"v20.10.0",
			"Running `post` command...",
			"Executing `node env_vars.js`",
			"main ENV_VAR1: /@!#$&%*)-_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR2: /@!#$&%*)-_(}]\"[\"]{+=^~?:;.,<>|",
			"main ENV_VAR3: /@!$&%*-_+=^~?:.,<>|",
			"main ENV_VAR4: /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR5: /\\@!#$&%*)\"-\"_(}][]{+=^~?:;.,<>|",
			"Command(s) completed successfully",
		)

		result = tester("proj-59:test")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `echo \"/\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|\"`",
			"\n/\\@!#$&%*)",
			"\n-_(}]'[']{+=^~?:;.,<>|",
			"Running project-level `post` command...",
			"Executing `node env_vars.js`",
			"main ENV_VAR1: /@!#$&%*)-_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR2: /@!#$&%*)-_(}]\"[\"]{+=^~?:;.,<>|",
			"main ENV_VAR3: /@!$&%*-_+=^~?:.,<>|",
			"main ENV_VAR4: /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR5: /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|",
			"Command(s) completed successfully",
		)
	} else { // unix
		result = tester("proj-60:run")
		result.AssertSequentialOrder("#1", "#2", "#3", "#4", "#5", "#6", "#7", "#8", "#9", "#10")

		result = tester("proj-61:run")
		result.AssertSequentialOrder(
			"#1", "#2", "#3", "#4", "#5", "#6", "#7", "#8", "#9", "#10",
			"Command(s) completed successfully",
			"Running `after` command...",
			"#11", "#12", "#13",
			"Running project-level `after` command...",
			"#14", "#15", "#16",
		)

		os.Setenv("TEST_ENV_VAR", "testing")
		result = tester("proj-62:run")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `node args.js testing`",
			"Provided arguments:",
			"1 => testing",
			"Running `pre` command...",
			"Executing `node args.js "+os.Getenv("PWD")+"`",
			"1 => "+os.Getenv("PWD"),
			"Running `post` command...",
			"Running project-level `post` command...",
		)

		os.Setenv("TEST_ENV_VAR", "testing2")
		os.Setenv("TEST_NUM_ENV_VAR", "123")
		result = tester("proj-63:run")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `node args.js testing2`",
			"Provided arguments:",
			"1 => testing2",
			"Running `pre` command...",
			"Executing `node args.js "+os.Getenv("PWD")+"`",
			"1 => "+os.Getenv("PWD"),
			"Running `post` command...",
			"Running project-level `post` command...",
		)

		os.Setenv("TEST_ENV_VAR", "testing3")
		os.Setenv("TEST_NUM_ENV_VAR", "12.3")
		result = tester("proj-64:run")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `node args.js testing3`",
			"Provided arguments:",
			"1 => testing3",
			"Running `pre` command...",
			"Executing `node args.js "+os.Getenv("PWD")+"`",
			"1 => "+os.Getenv("PWD"),
			"Running `post` command...",
			"Running project-level `post` command...",
		)

		os.Setenv("SPECIAL_CHARS", "/\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|")
		result = tester("proj-65:run")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `echo \"special chars => /\\\\@!#$&%*)\\\"-\\\"_(}]'[']{+=^~?:;.,<>|\"`",
			"special chars => /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|",
			"Running project-level `post` command...",
			"Executing `node env_vars.js`",
			"main ENV_VAR1: /@!#$&%*)-_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR2: /@!#$&%*)-_(}]\"[\"]{+=^~?:;.,<>|",
			"main ENV_VAR3: /@!$&%*-_+=^~?:.,<>|",
			"main ENV_VAR4: /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR5: undefined",
			"Command(s) completed successfully",
		)

		result = tester("proj-66:test")
		result.AssertSequentialOrder(
			"Running project-level `pre` command...",
			"Executing `echo '/\\@!#$&%*)\"-\"_(}][]{+=^~?:;.,<>|'`",
			"\n/\\@!#$&%*)\"-\"_(}][]{+=^~?:;.,<>|",
			"Executing `echo \"/\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|\"`",
			"/\\@!#$&%*)-_(}]'[']{+=^~?:;.,<>|",
			"Running project-level `post` command...",
			"Executing `node env_vars.js`",
			"main ENV_VAR1: /@!#$&%*)-_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR2: /@!#$&%*)-_(}]\"[\"]{+=^~?:;.,<>|",
			"main ENV_VAR3: /@!$&%*-_+=^~?:.,<>|",
			"main ENV_VAR4: /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|",
			"main ENV_VAR5: /\\@!#$&%*)\"-\"_(}][]{+=^~?:;.,<>|",
			"Command(s) completed successfully",
		)
	}
}

func TestCLI(t *testing.T) {
	var result utils.TestResult
	tester := utils.CreateCLITester(t, fixturesDir)

	result = tester("cli-1", 80, 12, "proj-2:test-1", term.KEY_ENTER)
	result.AssertContains(
		"Selected `proj-2:test-1`",
		"running main.js",
	)

	result = tester("cli-2", 60, 12, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-1`")

	result = tester("cli-3", 40, 12, "-2 test-1", term.KEY_ENTER)
	result.AssertContains("Selected `proj-2:test-1`")

	result = tester("cli-4", 20, 12, "-2 test-1", term.KEY_UP, term.KEY_ENTER)
	result.AssertContains("Selected `proj-2:test-1`")

	result = tester("cli-5", 10, 12, " test-1 :", term.KEY_DOWN, term.KEY_DOWN, term.KEY_ENTER)
	result.AssertContains("Selected `proj-2:test-1`")

	result = tester("cli-6", 1, 12, term.KEY_PAGE_DOWN, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-3`")

	result = tester("cli-7", 80, 12, term.KEY_PAGE_DOWN, term.KEY_UP, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-2`")

	result = tester("cli-8", 60, 12, term.KEY_PAGE_DOWN, term.KEY_PAGE_DOWN, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-7`")

	result = tester("cli-9", 40, 12, term.KEY_END, term.KEY_ENTER)
	result.AssertContains(
		"Selected `runner-error-9`",
		"Starting runner `runner-error-9`",
	)

	result = tester("cli-10", 20, 12, term.KEY_END, term.KEY_HOME, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-1`")

	result = tester("cli-11", 10, 12, ":args", term.KEY_CTRL_SPACE, "arg1 arg2 arg3", term.KEY_ENTER)
	result.AssertContains(
		"Selected `proj-2:args arg1 arg2 arg3`",
		"1 => arg1",
		"2 => arg2",
		"3 => arg3",
	)

	result = tester(
		"cli-12",
		1, 12,
		":args",
		term.KEY_CTRL_SPACE,
		"arg1 arg2 arg3",
		term.KEY_BACKSPACE,
		term.KEY_BACKSPACE,
		term.KEY_BACKSPACE,
		term.KEY_BACKSPACE,
		term.KEY_ENTER,
	)
	result.AssertContains(
		"Selected `proj-2:args arg1 arg2`",
		"1 => arg1",
		"2 => arg2",
	)

	result = tester("cli-13", 80, 12, ":args", term.KEY_CTRL_SPACE, term.KEY_CTRL_C)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-14", 80, 12, ":args", term.KEY_CTRL_SPACE, "arg1 arg2 arg3", term.KEY_CTRL_C)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-15", 80, 12, ":args", term.KEY_CTRL_SPACE, "arg1 arg2 arg3", term.KEY_ESC, term.KEY_ENTER)
	result.AssertContains(
		"Selected `proj-2:args`",
		"No arguments found",
	)

	result = tester("cli-16", 60, 12, ":args", term.KEY_CTRL_SPACE, "arg1 arg2 arg3", term.KEY_ESC, term.KEY_ESC, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-1`")

	result = tester("cli-17", 40, 12, ":args", term.KEY_CTRL_SPACE, "arg1 arg2 arg3", term.KEY_ESC, term.KEY_ESC, term.KEY_ESC)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-18", 20, 12, "runner-1", term.KEY_CTRL_SPACE, "arg1 arg2 arg3", term.KEY_ENTER)
	result.AssertContains("ERROR: Failed to read input: No more test commands available")

	result = tester("cli-19", 10, 12, "runner-1", term.KEY_CTRL_SPACE, term.KEY_ESC, term.KEY_ESC)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-20", 10, 12, "runner-1", term.KEY_CTRL_SPACE, term.KEY_CTRL_C)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-21", 1, 12, "proj-2:test-1", term.KEY_ESC, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-1`")

	result = tester("cli-22", 80, 12, "proj-2:test-1", term.KEY_ESC, term.KEY_ESC)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-23", 80, 12, "proj-2:test-1", term.KEY_CTRL_C)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-24", 80, 12, term.KEY_CTRL_C)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-25", 80, 12, term.KEY_ESC)
	result.AssertContains("ERROR: CLI cancelled by user")

	result = tester("cli-26", 80, 12, "nonexistent", term.KEY_ENTER, term.KEY_ESC, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-1`")

	result = tester("cli-27", 80, 12, "runn", term.KEY_ENTER)
	result.AssertContains("Starting runner `runner-1`")

	result = tester("cli-28", 80, 12, "runn", term.KEY_BACKSPACE, term.KEY_BACKSPACE, term.KEY_BACKSPACE, term.KEY_ENTER)
	result.AssertContains("ERROR: Command `error-cmd-1` must be a command or a list of commands")

	result = tester("cli-29", 80, 9, term.KEY_TAB, term.KEY_DOWN, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-1`")

	result = tester("cli-30", 80, 9, term.KEY_TAB, term.KEY_UP, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-1`")

	result = tester("cli-31", 80, 9, term.KEY_SHIFT_TAB, term.KEY_DOWN, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-1`")

	result = tester("cli-32", 80, 9, term.KEY_TAB, term.KEY_TAB, term.KEY_DOWN, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-10`")

	result = tester("cli-33", 80, 14, term.KEY_TAB, term.KEY_DOWN, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-10`")

	result = tester("cli-34", 80, 14, term.KEY_SHIFT_TAB, term.KEY_DOWN, term.KEY_ENTER)
	result.AssertContains("Selected `cmd-10`")

	result = tester("cli-35", 80, 12, term.KEY_DOWN, " ", term.KEY_ENTER)
	result.AssertContains("Selected `cmd-10`")

	result = tester("cli-36", 80, 12, term.KEY_PAGE_DOWN, "test", term.KEY_ENTER)
	result.AssertContains("ERROR: Failed to load environment file `" + filepath.Join(fixturesDir, "node", "not_exist.env") + "`")

	result = tester("cli-37", 10, 12, ":args", term.KEY_CTRL_SPACE, "\\", term.KEY_ENTER)
	result.AssertContains("ERROR: Invalid command arguments format `\\`")

	result = tester("cli-38", 80, 12, "other:format::command:format:", term.KEY_ENTER)
	result.AssertContains(
		"Selected `other:format::command:format:`",
		"Executing `python3 main.py`",
	)

	result = tester("cli-39", 80, 12, "other[dependent]:command[serial]", term.KEY_ENTER)
	result.AssertContains(
		"Selected `other[dependent]:command[serial]`",
		"Executing `npm run timeout-3`",
	)

	result = tester("cli-40", 80, 12, "other[serial]format:command[dependent]format", term.KEY_ENTER)
	result.AssertContains(
		"Selected `other[serial]format:command[dependent]format`",
		"Executing `npm run exit`",
	)

	result = tester("cli-41", 80, 12, "runner-56[serial]format", term.KEY_ENTER)
	result.AssertContains(
		"Selected `runner-56[serial]format`",
		"Starting runner `runner-56[serial]format`",
	)

	result = tester("cli-42", 80, 12, "runner-57[dependent]format", term.KEY_ENTER)
	result.AssertContains(
		"Selected `runner-57[dependent]format`",
		"Starting runner `runner-57[dependent]format`",
	)

	result = tester("cli-43", 80, 12, "runner-58[serial,dependent]format", term.KEY_ENTER)
	result.AssertContains(
		"Selected `runner-58[serial,dependent]format`",
		"Starting runner `runner-58[serial,dependent]format`",
	)

	result = tester("cli-44", 10, 12, ":args", term.KEY_CTRL_SPACE, "\"single arg\"", term.KEY_ENTER)
	result.AssertContains(
		"Selected `proj-2:args \"single arg\"`",
		"Executing `node args.js \"single arg\"`",
		"Provided arguments:",
		"1 => single arg",
	)

	result = tester("cli-45", 10, 12, ":args", term.KEY_CTRL_SPACE, "arg \"single arg\"", term.KEY_ENTER)
	result.AssertContains(
		"Selected `proj-2:args arg \"single arg\"`",
		"Executing `node args.js arg \"single arg\"`",
		"Provided arguments:",
		"1 => arg",
		"2 => single arg",
	)

	result = tester("cli-46", 80, 12, "runner-35[serial]", term.KEY_ENTER)
	result.AssertContains(
		"Selected `runner-35`",
		"Starting runner `runner-35` with flags [serial]",
	)

	result = tester("cli-47", 80, 12, "runner-43", term.KEY_ENTER)
	result.AssertContains(
		"Selected `runner-43`",
		"Starting runner `runner-43` with flags [serial, dependent]",
	)

	result = tester("snapshot-1", 67, 14, "error-proj-", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                             Navi CLI",
		"-------------------------------------------------------------------",
		"Filter: error-proj-    [1 of 18] ¦ Definition:",
		"                                 ¦",
		"error-proj-10:test       Project ¦ error-proj-10:",
		"error-proj-11:test       Project ¦   dir: node",
		"error-proj-12:test-1     Project ¦   cmds:",
		"error-proj-12:test-2     Project ¦     ...",
		"error-proj-13:test       Project ¦     test:",
		"error-proj-1::missing....Project ¦       dotenv: not_exist.env",
		"error-proj-1:missing-run Project ¦       run: node --version",
		"               ↓                 ¦",
		"-------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Clear filter",
	)

	result = tester("snapshot-2", 66, 14, term.KEY_DOWN, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                            Navi CLI",
		"------------------------------------------------------------------",
		"Filter:              [2 of 255] ¦ Definition:",
		"                                ¦",
		"cmd-1                   Command ¦ cmd-10: node ./node/timeout_2.js",
		"cmd-10                  Command ¦",
		"cmd-11                  Command ¦",
		"cmd-2                   Command ¦",
		"cmd-3                   Command ¦",
		"cmd-4                   Command ¦",
		"cmd-5                   Command ¦",
		"              ↓                 ¦",
		"------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Exit",
	)

	result = tester("snapshot-3", 50, 14, " shell proj", term.KEY_PAGE_DOWN, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                    Navi CLI",
		"--------------------------------------------------",
		"Filter:  proj [7 of 15] ¦ Definition:",
		"                        ¦",
		"error-proj-3....Project ¦ proj-18:",
		"error-proj-3....Project ¦   dir: node",
		"error-proj-8:...Project ¦   shell: cmd",
		"proj-15:shel....Project ¦   pre:",
		"proj-16:shel....Project ¦     shell: powershell",
		"proj-17:shel....Project ¦     run: echo \"pre pa...",
		"proj-18:shell...Project ¦   cmds:",
		"          ↓             ¦             ↓",
		"--------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args ...",
	)

	result = tester("snapshot-4", 80, 14, " shell proj  ", term.KEY_PAGE_DOWN, term.KEY_DOWN, term.KEY_TAB, term.KEY_DOWN, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                   Navi CLI",
		"--------------------------------------------------------------------------------",
		"Filter: shell proj           [8 of 15] ¦ Definition:",
		"                  ↑                    ¦                    ↑",
		"error-proj-3:no-shell-win-2    Project ¦   dir: node",
		"error-proj-8:shell-test        Project ¦   shell: powershell",
		"proj-15:shell-test             Project ¦   post:",
		"proj-16:shell-test             Project ¦     shell: cmd",
		"proj-17:shell-test             Project ¦     run: echo \"post path %CD%\"",
		"proj-18:shell-test             Project ¦   cmds:",
		"proj-19:shell-test             Project ¦     ...",
		"                  ↓                    ¦                    ↓",
		"--------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Tab] Navigate definition   ...",
	)

	result = tester("snapshot-5", 67, 12, " -7", term.KEY_END, term.KEY_UP, term.KEY_UP, term.KEY_SHIFT_TAB, term.KEY_END, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                             Navi CLI",
		"-------------------------------------------------------------------",
		"Filter: -7              [6 of 8] ¦ Definition:",
		"               ↑                 ¦                 ↑",
		"watch-mode-6:test-7      Project ¦       run: node exit.js",
		"watch-mode-7:test        Project ¦       after:",
		"watch-mode-error-7:test  Project ¦         success: node main.js",
		"runner-7                  Runner ¦         failure: node main.js",
		"runner-error-7            Runner ¦         always: node exit.js",
		"                                 ¦",
		"-------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Tab] Navigate ...",
	)

	result = tester("snapshot-6", 66, 12, " -7", term.KEY_END, term.KEY_UP, term.KEY_UP, term.KEY_SHIFT_TAB, term.KEY_END, term.KEY_UP, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                            Navi CLI",
		"------------------------------------------------------------------",
		"Filter: -7             [6 of 8] ¦ Definition:",
		"              ↑                 ¦                 ↑",
		"watch-mode-6:test-7     Project ¦     test:",
		"watch-mode-7:test       Project ¦       run: node exit.js",
		"watch-mode-error-7:test Project ¦       after:",
		"runner-7                 Runner ¦         success: node main.js",
		"runner-error-7           Runner ¦         failure: node main.js",
		"                                ¦                 ↓",
		"------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Tab] Navigate...",
		"ERROR: Failed to read input: No more test commands available",
	)

	result = tester("snapshot-7", 58, 12, term.KEY_END, "runner-11", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                        Navi CLI",
		"----------------------------------------------------------",
		"Filter: runner-11  [1 of 1] ¦ Definition:",
		"                            ¦",
		"runner-11            Runner ¦ multiline runner description",
		"                            ¦",
		"                            ¦ runner-11:",
		"                            ¦   - cmd: proj-2:test-1",
		"                            ¦     delay: 5",
		"                            ¦",
		"----------------------------------------------------------",
		"[Enter] Execute   [Esc] Clear filter",
	)

	result = tester("snapshot-8", 57, 12, term.KEY_END, "runner-11", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                        Navi CLI",
		"---------------------------------------------------------",
		"Filter: runner-11  [1 of 1] ¦ Definition:",
		"                            ¦",
		"runner-11            Runner ¦ multiline runner",
		"                            ¦ description",
		"                            ¦",
		"                            ¦ runner-11:",
		"                            ¦   - cmd: proj-2:test-1",
		"                            ¦              ↓",
		"---------------------------------------------------------",
		"[Enter] Execute   [Tab] Navigate definition   [Esc] Cl...",
	)

	result = tester("snapshot-9", 57, 17, "3:env", term.KEY_PAGE_DOWN, term.KEY_UP, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                        Navi CLI",
		"---------------------------------------------------------",
		"Filter: 3:env      [8 of 9] ¦ Definition:",
		"                            ¦",
		"proj-13:env-vars    Project ¦ project command description",
		"proj-33:env-vars-1  Project ¦",
		"proj-33:env-vars-2  Project ¦ proj-3:",
		"proj-33:env-vars-3  Project ¦   dir: node/",
		"proj-33:env-vars-4  Project ¦   cmds:",
		"proj-33:env-vars-5  Project ¦     ...",
		"proj-33:env-vars-6  Project ¦     env-vars:",
		"proj-3:env-vars     Project ¦       dir: ./",
		"proj-43:env-vars    Project ¦       dotenv: ./.node1.env",
		"                            ¦       pre: npm run env-v...",
		"                            ¦              ↓",
		"---------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Tab]...",
	)

	result = tester("snapshot-10", 58, 19, term.KEY_DOWN, term.KEY_DOWN, term.KEY_DOWN, term.KEY_CTRL_SPACE, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                        Navi CLI",
		"----------------------------------------------------------",
		"Filter:          [4 of 255] ¦ Definition:",
		"                            ¦",
		"cmd-1               Command ¦ cmd-2:",
		"cmd-10              Command ¦   dotenv: .env | ENV_VAR4",
		"cmd-11",
		"cmd-2    ┌─────── Enter Command Arguments ──────┐",
		"cmd-3    │                                      │",
		"cmd-4    │ > navi cmd-2                         │   out...",
		"cmd-5    │                                      │   de/...",
		"cmd-6    └─── [Enter] Confirm   [Esc] Cancel ───┘   var...",
		"cmd-7",
		"cmd-8               Command ¦",
		"cmd-9               Command ¦",
		"error-cmd-1         Command ¦",
		"            ↓               ¦",
		"----------------------------------------------------------",
		" ",
	)

	result = tester("snapshot-11", 57, 19, term.KEY_DOWN, term.KEY_DOWN, term.KEY_DOWN, term.KEY_CTRL_SPACE, "command extra input arguments", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                        Navi CLI",
		"---------------------------------------------------------",
		"Filter:          [4 of 255] ¦ Definition:",
		"                            ¦",
		"cmd-1               Command ¦ cmd-2:",
		"cmd-10              Command ¦   dotenv: .env | ENV_VAR4",
		"cmd-11",
		"cmd-2    ┌────── Enter Command Arguments ──────┐",
		"cmd-3    │                                     │",
		"cmd-4    │ > navi cmd-2 command extra input ar │   eou...",
		"cmd-5    │ guments                             │   ode...",
		"cmd-6    │                                     │   _va...",
		"cmd-7    └── [Enter] Confirm   [Esc] Cancel ───┘",
		"cmd-8",
		"cmd-9               Command ¦",
		"error-cmd-1         Command ¦",
		"            ↓               ¦",
		"---------------------------------------------------------",
		" ",
	)

	result = tester("snapshot-12", 57, 14, "long", term.KEY_CTRL_SPACE, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                        Navi CLI",
		"---------------------------------------------------------",
		"Filter: long       [1 of 1] ¦ Definition:",
		" ",
		"long-p   ┌────── Enter Command Arguments ──────┐",
		"         │                                     │",
		"         │ > navi long-project-name:long-proje │",
		"         │ ct-command-name                     │",
		"         │                                     │   and...",
		"         └── [Enter] Confirm   [Esc] Cancel ───┘",
		" ",
		"                            ¦",
		"---------------------------------------------------------",
		" ",
	)

	result = tester("snapshot-13", 57, 10, "long", term.KEY_CTRL_SPACE, "command extra arguments", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                        Navi CLI",
		"------                                             ------",
		"Filter   ┌────── Enter Command Arguments ──────┐",
		"         │                                     │",
		"long-p   │ > navi long-project-name:long-proje │",
		"         │ ct-command-name command extra argum │",
		"         │ ents                                │",
		"         │                                     │",
		"------   └── [Enter] Confirm   [Esc] Cancel ───┘   ------",
		" ",
	)

	result = tester("snapshot-14", 84, 10, "long", term.KEY_CTRL_SPACE, "command extra arguments", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                     Navi CLI",
		"----------                                                                ----------",
		"Filter: lo   ┌──────────────── Enter Command Arguments ───────────────┐",
		"             │                                                        │",
		"long-proje   │ > navi long-project-name:long-project-command-name com │",
		"             │ mand extra arguments                                   │",
		"             │                                                        │",
		"             └──────────── [Enter] Confirm   [Esc] Cancel ────────────┘",
		"----------                                                                ----------",
		" ",
	)

	result = tester("snapshot-15", 84, 10, "nonexistent", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                     Navi CLI",
		"------------------------------------------------------------------------------------",
		"Filter: nonexistent             [0 of 0] ¦ Definition:",
		"                                         ¦",
		"No matching items found                  ¦ No item selected",
		"                                         ¦",
		"                                         ¦",
		"                                         ¦",
		"------------------------------------------------------------------------------------",
		"[Esc] Clear filter",
	)

	result = tester("snapshot-16", 84, 19, "proj-5:env-vars-1", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                     Navi CLI",
		"------------------------------------------------------------------------------------",
		"Filter: proj-5:env-vars-1       [1 of 1] ¦ Definition:",
		"                                         ¦",
		"proj-5:env-vars-1                Project ¦ proj-5:",
		"                                         ¦   dir: node/",
		"                                         ¦   post:",
		"                                         ¦     dir: ./",
		"                                         ¦     dotenv: ../.env | ENV_VAR1, ENV_VAR3",
		"                                         ¦     run: npm run env-vars-post",
		"                                         ¦   cmds:",
		"                                         ¦     ...",
		"                                         ¦     env-vars-1:",
		"                                         ¦       dir: __ROOT__/node/",
		"                                         ¦       dotenv: .node1.env",
		"                                         ¦       run: npm run env-vars",
		"                                         ¦",
		"------------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Clear filter",
	)

	result = tester("snapshot-17", 84, 14, "proj-15", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                     Navi CLI",
		"------------------------------------------------------------------------------------",
		"Filter: proj-15                 [1 of 1] ¦ Definition:",
		"                                         ¦",
		"proj-15:shell-test               Project ¦ proj-15:",
		"                                         ¦   dir: node",
		"                                         ¦   shell: powershell",
		"                                         ¦   cmds:",
		"                                         ¦     ...",
		"                                         ¦     shell-test:",
		"                                         ¦       run: echo \"main path \\$PWD\"",
		"                                         ¦                     ↓",
		"------------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Tab] Navigate definition   [Esc...",
	)

	result = tester("snapshot-18", 96, 20, "proj-56", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                           Navi CLI",
		"------------------------------------------------------------------------------------------------",
		"Filter: proj-56                       [1 of 1] ¦ Definition:",
		"                                               ¦",
		"proj-56:run                            Project ¦ proj-56:",
		"                                               ¦   dir: node",
		"                                               ¦   shell: cmd",
		"                                               ¦   pre: echo \"special chars => %SPECIAL_CHARS%\"",
		"                                               ¦   post:",
		"                                               ¦     dotenv: __ROOT__/.special.chars.env",
		"                                               ¦     env:",
		"                                               ¦       ENV_VAR4: %SPECIAL_CHARS%",
		"                                               ¦       ENV_VAR5: ${SPECIAL_CHARS}",
		"                                               ¦     run: node env_vars.js",
		"                                               ¦   cmds:",
		"                                               ¦     ...",
		"                                               ¦     run: echo \"special chars => $SPECIAL_CHARS\"",
		"                                               ¦",
		"------------------------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Clear filter",
	)

	result = tester("snapshot-19", 96, 22, "proj-62", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                           Navi CLI",
		"------------------------------------------------------------------------------------------------",
		"Filter: proj-62                       [1 of 1] ¦ Definition:",
		"                                               ¦",
		"proj-62:run                            Project ¦ proj-62:",
		"                                               ¦   dir: node",
		"                                               ¦   shell: bash",
		"                                               ¦   pre:",
		"                                               ¦     run: node args.js $TEST_ENV_VAR",
		"                                               ¦   post:",
		"                                               ¦     run: node args.js $PWD",
		"                                               ¦   cmds:",
		"                                               ¦     ...",
		"                                               ¦     run:",
		"                                               ¦       pre:",
		"                                               ¦         run: node args.js $PWD",
		"                                               ¦       run: node args.js $TEST_ENV_VAR",
		"                                               ¦       post:",
		"                                               ¦         run: node args.js $PWD",
		"                                               ¦",
		"------------------------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Clear filter",
	)

	result = tester("snapshot-20", 96, 22, "proj-63", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                           Navi CLI",
		"------------------------------------------------------------------------------------------------",
		"Filter: proj-63                       [1 of 1] ¦ Definition:",
		"                                               ¦",
		"proj-63:run                            Project ¦ proj-63:",
		"                                               ¦   dir: node",
		"                                               ¦   shell: bash",
		"                                               ¦   pre: node args.js ${TEST_ENV_VAR}",
		"                                               ¦   post:",
		"                                               ¦     env:",
		"                                               ¦       ENV_VAR1: ${TEST_ENV_VAR}",
		"                                               ¦       ENV_VAR2: ${TEST_NUM_ENV_VAR}",
		"                                               ¦     run: node env_vars.js",
		"                                               ¦   cmds:",
		"                                               ¦     ...",
		"                                               ¦     run:",
		"                                               ¦       pre: node args.js ${PWD}",
		"                                               ¦       run: node args.js ${TEST_ENV_VAR}",
		"                                               ¦       post: node args.js ${PWD}",
		"                                               ¦",
		"------------------------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Clear filter",
	)

	result = tester("snapshot-21", 110, 22, "proj-58", term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                                  Navi CLI",
		"--------------------------------------------------------------------------------------------------------------",
		"Filter: proj-58                              [1 of 1] ¦ Definition:",
		"                                                      ¦",
		"proj-58:test                                  Project ¦ proj-58:",
		"                                                      ¦   dir: node",
		"                                                      ¦   cmds:",
		"                                                      ¦     ...",
		"                                                      ¦     test:",
		"                                                      ¦       pre:",
		"                                                      ¦         shell: cmd",
		"                                                      ¦         run: echo \"/\\@!#$&%*)\"-\"_(}][]{+=^~?:;.,<>|\"",
		"                                                      ¦       run: node --version",
		"                                                      ¦       post:",
		"                                                      ¦         dotenv: __ROOT__/.special.chars.env",
		"                                                      ¦         env:",
		"                                                      ¦           ENV_VAR4: /\\@!#$&%*)\"-\"_(}]'[']{+=^~?:;.,<>|",
		"                                                      ¦           ENV_VAR5: /\\@!#$&%*)\"-\"_(}][]{+=^~?:;.,<>|",
		"                                                      ¦         run: node env_vars.js",
		"                                                      ¦",
		"--------------------------------------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Clear filter",
	)

	result = tester(
		"snapshot-22",
		57, 17,
		"cmd-",
		term.KEY_BACKSPACE,
		term.KEY_DOWN,
		term.KEY_DOWN,
		term.KEY_DOWN,
		term.KEY_DOWN,
		term.TEST_SNAPSHOT,
	)
	result.AssertSequentialOrder(
		"                        Navi CLI",
		"---------------------------------------------------------",
		"Filter: cmd       [5 of 19] ¦ Definition:",
		"                            ¦",
		"cmd-1               Command ¦ command CLI description",
		"cmd-10              Command ¦",
		"cmd-11              Command ¦ cmd-3:",
		"cmd-2               Command ¦   dir: python/",
		"cmd-3               Command ¦   dotenv: __ROOT__/node/...",
		"cmd-4               Command ¦   env:",
		"cmd-5               Command ¦     ENV_VAR2: 888",
		"cmd-6               Command ¦   run:",
		"cmd-7               Command ¦     - python3 timeout_1.py",
		"cmd-8               Command ¦     - python3 timeout_2.py",
		"            ↓               ¦              ↓",
		"---------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Tab]...",
	)

	result = utils.CreateCLITester(t, fixturesDir, "-f", "./yml/test_2.yml")("snapshot-23", 80, 14, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                   Navi CLI",
		"--------------------------------------------------------------------------------",
		"Filter:                       [1 of 1] ¦ Definition:",
		"                                       ¦",
		"cmd-1                          Command ¦ cmd-1:",
		"                                       ¦   dir: ../go",
		"                                       ¦   run: go run main.go",
		"                                       ¦",
		"                                       ¦",
		"                                       ¦",
		"                                       ¦",
		"                                       ¦",
		"--------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Exit",
	)

	result = utils.CreateCLITester(t, fixturesDir, "-f", "./yml/test_3.yml")("snapshot-24", 80, 14, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                   Navi CLI",
		"--------------------------------------------------------------------------------",
		"Filter:                       [1 of 1] ¦ Definition:",
		"                                       ¦",
		"proj:test-1                    Project ¦ proj:",
		"                                       ¦   dir: ../node",
		"                                       ¦   cmds:",
		"                                       ¦     ...",
		"                                       ¦     test-1: node main.js",
		"                                       ¦",
		"                                       ¦",
		"                                       ¦",
		"--------------------------------------------------------------------------------",
		"[Enter] Execute   [Ctrl+Space] Execute w/ args   [Esc] Exit",
	)

	result = utils.CreateCLITester(t, fixturesDir, "-f", "./yml/test_4.yml")("snapshot-25", 80, 14, term.TEST_SNAPSHOT)
	result.AssertSequentialOrder(
		"                                   Navi CLI",
		"--------------------------------------------------------------------------------",
		"Filter:                       [1 of 1] ¦ Definition:",
		"                                       ¦",
		"runner-1                        Runner ¦ runner-1:",
		"                                       ¦   - python3 ../python/main.py",
		"                                       ¦",
		"                                       ¦",
		"                                       ¦",
		"                                       ¦",
		"                                       ¦",
		"                                       ¦",
		"--------------------------------------------------------------------------------",
		"[Enter] Execute   [Esc] Exit",
	)
}

func TestMain(m *testing.M) {
	var err error
	fixturesDir, err = filepath.Abs("./fixtures")
	if err != nil {
		logger.Error("Error while getting the fixtures directory: %v", err)
		os.Exit(1)
	}

	utils.DirectoryCreationOperation(filepath.Join(fixturesDir, "python", "empty"))

	if runtime.GOOS == "windows" {
		utils.FileCopyOperation("./navi.exe", filepath.Join(fixturesDir, "navi.exe"), true)
	} else {
		utils.FileCopyOperation("./navi", filepath.Join(fixturesDir, "navi"), true)
	}

	sigChan := process.SetupSignalHandling()

	// handle SIGINT and SIGTERM
	go func() {
		sig := <-sigChan

		if utils.TestShutdownInProgress {
			return
		}

		fmt.Print("\n")
		logger.Warn("Received `%v` signal. Shutting down running tests...", sig)
		utils.ShutdownAllTests()
		cleanUp()
		os.Exit(1)
	}()

	exitCode := m.Run()
	cleanUp()
	os.Exit(exitCode)
}
