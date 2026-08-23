package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func ServiceInstall(execPath, cfgPath string) error {
	absExec, err := filepath.Abs(execPath)
	if err != nil {
		return err
	}
	absCfg, err := filepath.Abs(cfgPath)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "linux":
		unitPath := "/etc/systemd/system/lapdog.service"
		target := "multi-user.target"

		// Try root directory first, fallback to user systemd directory
		if err := os.WriteFile(unitPath, []byte(""), 0644); err != nil {
			home, _ := os.UserHomeDir()
			userUnitDir := filepath.Join(home, ".config", "systemd", "user")
			_ = os.MkdirAll(userUnitDir, 0755)
			unitPath = filepath.Join(userUnitDir, "lapdog.service")
			target = "default.target"
		}

		serviceContent := fmt.Sprintf(`[Unit]
Description=lap-dog Security Agent
After=network.target auditd.service

[Service]
Type=simple
ExecStart=%s run --config %s
Restart=always
RestartSec=5s

[Install]
WantedBy=%s
`, absExec, absCfg, target)

		if err := os.WriteFile(unitPath, []byte(serviceContent), 0644); err != nil {
			return fmt.Errorf("failed to write systemd service: %w", err)
		}

		if target == "default.target" {
			fmt.Printf("Systemd user service written to %s\nRun 'systemctl --user enable --now lapdog'\n", unitPath)
		} else {
			fmt.Printf("Systemd service written to %s\nRun 'sudo systemctl daemon-reload && sudo systemctl enable --now lapdog'\n", unitPath)
		}
		return nil

	case "darwin":
		home, _ := os.UserHomeDir()
		plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.lapdog.agent.plist")
		plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.lapdog.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>run</string>
        <string>--config</string>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>`, absExec, absCfg)

		_ = os.MkdirAll(filepath.Dir(plistPath), 0755)
		if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
			return err
		}
		fmt.Printf("macOS LaunchAgent created at %s\nRun 'launchctl load %s'\n", plistPath, plistPath)
		return nil

	case "windows":
		cmdStr := fmt.Sprintf("sc.exe create lapdog binPath= \"%s run --config %s\" start= auto", absExec, absCfg)
		cmd := exec.Command("powershell", "-Command", cmdStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install Windows service: %w (%s)", err, string(out))
		}
		fmt.Println("Windows service 'lapdog' created successfully. Run 'sc.exe start lapdog'")
		return nil

	default:
		return fmt.Errorf("unsupported platform for service installation: %s", runtime.GOOS)
	}
}

func ServiceUninstall() error {
	switch runtime.GOOS {
	case "linux":
		// Stop and disable user service if present
		_ = exec.Command("systemctl", "--user", "stop", "lapdog").Run()
		_ = exec.Command("systemctl", "--user", "disable", "lapdog").Run()
		home, _ := os.UserHomeDir()
		userUnit := filepath.Join(home, ".config", "systemd", "user", "lapdog.service")
		if _, err := os.Stat(userUnit); err == nil {
			_ = os.Remove(userUnit)
			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		}

		// Stop and disable system service if present
		_ = exec.Command("systemctl", "stop", "lapdog").Run()
		_ = exec.Command("systemctl", "disable", "lapdog").Run()
		sysUnit := "/etc/systemd/system/lapdog.service"
		if _, err := os.Stat(sysUnit); err == nil {
			_ = os.Remove(sysUnit)
			_ = exec.Command("systemctl", "daemon-reload").Run()
		}

		fmt.Println("Systemd service stopped and unit files removed.")
		return nil
	case "darwin":
		home, _ := os.UserHomeDir()
		plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.lapdog.agent.plist")
		_ = exec.Command("launchctl", "unload", plistPath).Run()
		_ = os.Remove(plistPath)
		fmt.Println("macOS LaunchAgent stopped and removed.")
		return nil
	case "windows":
		_ = exec.Command("sc.exe", "stop", "lapdog").Run()
		out, err := exec.Command("sc.exe", "delete", "lapdog").CombinedOutput()
		if err != nil && !strings.Contains(string(out), "1060") {
			return fmt.Errorf("failed to delete Windows service: %w (%s)", err, string(out))
		}
		fmt.Println("Windows service 'lapdog' stopped and deleted.")
		return nil
	default:
		return fmt.Errorf("unsupported OS")
	}
}
