package osutil

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestPrepareSilentCommand(t *testing.T) {
	cmd := exec.Command("echo", "test")
	cmd = PrepareSilentCommand(cmd)

	if runtime.GOOS == "windows" {
		if cmd.SysProcAttr == nil {
			t.Fatalf("expected SysProcAttr to be set on Windows")
		}
	} else {
		if cmd.SysProcAttr != nil {
			t.Fatalf("expected nil SysProcAttr on non-Windows")
		}
	}
}
