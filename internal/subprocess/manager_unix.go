//go:build !windows

package subprocess

import (
	"os/exec"
	"syscall"
)

// setProcAttr sets Unix-specific process attributes
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// getProcessGroup gets the process group ID on Unix systems
func getProcessGroup(pid int) (int, error) {
	return syscall.Getpgid(pid)
}

// killProcessGroup kills a process group on Unix systems
func killProcessGroup(pgid int, signal syscall.Signal) error {
	return syscall.Kill(-pgid, signal)
}
