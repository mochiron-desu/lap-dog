//go:build windows

package service

import (
	"syscall"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	user32            = syscall.NewLazyDLL("user32.dll")
	procGetConsoleWnd = kernel32.NewProc("GetConsoleWindow")
	procShowWindow    = user32.NewProc("ShowWindow")
)

const (
	swHide = 0
)

// HideConsoleWindow hides the console window attached to this process on Windows.
func HideConsoleWindow() {
	hwnd, _, _ := procGetConsoleWnd.Call()
	if hwnd != 0 {
		procShowWindow.Call(hwnd, uintptr(swHide))
	}
}
