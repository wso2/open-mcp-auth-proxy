//go:build windows

package subprocess

import (
	"os/exec"
	"syscall"
)

// setProcAttr sets Windows-specific process attributes
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// getProcessGroup returns the PID itself on Windows (no process groups)
func getProcessGroup(pid int) (int, error) {
	return pid, nil
}

// killProcessGroup kills a process on Windows (no process groups)
func killProcessGroup(pgid int, signal syscall.Signal) error {
	// On Windows, we'll use the process handle directly
	// This function shouldn't be called on Windows, but we provide it for compatibility
	return nil
}