package process

import (
	"os/exec"
	"sync"
)

var TerminatingProcesses = false
var ProcessRegistry = make(map[int]*exec.Cmd)
var processRegistryLock sync.Mutex
var afterProcessesRegistry = make(map[int]*exec.Cmd)
var afterProcessesRegistryLock sync.Mutex

// Register adds a running process to the registry
func Register(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	RegisterWithJobObject(cmd)
	processRegistryLock.Lock()
	ProcessRegistry[cmd.Process.Pid] = cmd
	processRegistryLock.Unlock()
}

// Register adds an after command running process to the registry
func RegisterAfter(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	RegisterWithJobObject(cmd)
	afterProcessesRegistryLock.Lock()
	afterProcessesRegistry[cmd.Process.Pid] = cmd
	afterProcessesRegistryLock.Unlock()
}

// Try to gracefully terminate all processes
func TerminateAll() {
	TerminatingProcesses = true
	TerminateProcessGroup()
}

// Try to interrupt of all processes
func InterruptAll() {
	TerminatingProcesses = true
	InterruptProcessGroup()
}

// Force shutdown of all processes
func KillAll() {
	processRegistryLock.Lock()
	for pid, cmd := range ProcessRegistry {
		KillProcess(cmd)
		delete(ProcessRegistry, pid)
	}
	processRegistryLock.Unlock()

	afterProcessesRegistryLock.Lock()
	for pid, cmd := range afterProcessesRegistry {
		KillProcess(cmd)
		delete(afterProcessesRegistry, pid)
	}
	afterProcessesRegistryLock.Unlock()

	TerminateJobObject()
}
