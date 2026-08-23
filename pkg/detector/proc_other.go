//go:build !windows

package detector

import (
	"os/exec"
)

func setHiddenProcAttr(cmd *exec.Cmd) {
	// No-op on non-Windows platforms
}
