//go:build windows

package osutil

import (
	"os/exec"
	"syscall"
)

const (
	// CREATE_NO_WINDOW prevents Windows from creating a new console window for the child process
	CREATE_NO_WINDOW = 0x08000000
)

// SilentSysProcAttr returns SysProcAttr configured to suppress visible console windows
func SilentSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: CREATE_NO_WINDOW,
		HideWindow:    true,
	}
}

// PrepareSilentCommand attaches window suppression attributes to an exec.Cmd
func PrepareSilentCommand(cmd *exec.Cmd) *exec.Cmd {
	if cmd == nil {
		return nil
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = SilentSysProcAttr()
	} else {
		cmd.SysProcAttr.CreationFlags |= CREATE_NO_WINDOW
		cmd.SysProcAttr.HideWindow = true
	}
	return cmd
}
