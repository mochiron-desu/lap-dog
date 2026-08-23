//go:build !windows

package service

// HideConsoleWindow is a no-op on non-Windows platforms.
func HideConsoleWindow() {
}
