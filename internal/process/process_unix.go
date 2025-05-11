//go:build !windows
// +build !windows

package process

import (
	"os"
	"os/exec"
	"os/signal"

	"golang.org/x/sys/unix"
)

// Prepares a command to join the main process group
func SetupProcessGroup(cmd *exec.Cmd) {
	pgid, err := unix.Getpgid(os.Getpid())
	if err != nil {
		return
	}

	cmd.SysProcAttr = &unix.SysProcAttr{
		Setpgid: false,
		Pgid:    pgid,
	}
}

// Prepares a command to join a new process group
func SetupNewProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &unix.SysProcAttr{
		Setpgid: true,
	}
}

// SIGTERM to all processes
func TerminateProcessGroup() {
	for _, cmd := range ProcessRegistry {
		TerminateProcess(cmd)
	}
}

// SIGINT to all processes
func InterruptProcessGroup() {
	for _, cmd := range ProcessRegistry {
		InterruptProcess(cmd)
	}
}

// SIGTERM a process group
func TerminateProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	unix.Kill(-cmd.Process.Pid, unix.SIGTERM)
}

// SIGINT a process group
func InterruptProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	unix.Kill(-cmd.Process.Pid, unix.SIGINT)
}

// SIGKILL a process
func KillProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	unix.Kill(cmd.Process.Pid, unix.SIGKILL)
	cmd.Process.Kill()
}

func RegisterWithJobObject(cmd *exec.Cmd) {
	// For Windows
}

func TerminateJobObject() {
	// For Windows
}

// SetupSignalHandling sets up a channel to receive OS signals
func SetupSignalHandling() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, unix.SIGINT, unix.SIGTERM, os.Interrupt)
	return sigChan
}
