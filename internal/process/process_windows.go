//go:build windows

package process

import (
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Global job object handle
var jobObject windows.Handle

func init() {
	// Create a job object when the package is initialized
	var err error
	jobObject, err = windows.CreateJobObject(nil, nil)
	if err == nil {
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

		windows.SetInformationJobObject(
			jobObject,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
	}
}

// Prepares a command to join the main process group
func SetupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &windows.SysProcAttr{}
}

// Prepares a command to join the a new process group
func SetupNewProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
}

// SIGINT to the whole process group
func TerminateProcessGroup() {
	windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, uint32(os.Getpid()))
}

// SIGINT to the whole process group
func InterruptProcessGroup() {
	TerminateProcessGroup()
}

// SIGINT a process group
func TerminateProcess(cmd *exec.Cmd) { // in Windows, SIGTERM will have the same result as SIGINT
	if cmd == nil || cmd.Process == nil {
		return
	}

	// Be aware this will send a SIGINT to the whole process group, not just the process (this is a Windows limitation)
	windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, uint32(cmd.Process.Pid))
}

// SIGINT a process group
func InterruptProcess(cmd *exec.Cmd) { // in Windows, SIGTERM will have the same result as SIGINT
	TerminateProcess(cmd)
}

// SIGKILL a process
func KillProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/F", "/T").Run()
	cmd.Process.Kill()
}

// RegisterWithJobObject adds a process to the job object after it has started
func RegisterWithJobObject(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil || jobObject == 0 {
		return
	}

	handle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		return
	}

	windows.AssignProcessToJobObject(jobObject, handle)
	windows.CloseHandle(handle)
}

// TerminateJobObject kills all processes in the job object
func TerminateJobObject() {
	if jobObject != 0 {
		windows.TerminateJobObject(jobObject, 0) // Exit code 0
	}
}

// SetupSignalHandling sets up a channel to receive OS signals
func SetupSignalHandling() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, windows.SIGINT, windows.SIGTERM, os.Interrupt)
	return sigChan
}
