//go:build !windows

package osutil

import (
	"os/exec"
	"syscall"
)

// SilentSysProcAttr returns nil on non-Windows platforms
func SilentSysProcAttr() *syscall.SysProcAttr {
	return nil
}

// PrepareSilentCommand is a no-op on non-Windows platforms
func PrepareSilentCommand(cmd *exec.Cmd) *exec.Cmd {
	return cmd
}
