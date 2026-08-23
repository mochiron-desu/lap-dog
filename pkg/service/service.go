package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		serviceContent := fmt.Sprintf(`[Unit]
Description=lap-dog Security Agent
After=network.target auditd.service

[Service]
Type=simple
ExecStart=%s run --config %s
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
`, absExec, absCfg)

		unitPath := "/etc/systemd/system/lapdog.service"
		if err := os.WriteFile(unitPath, []byte(serviceContent), 0644); err != nil {
			// Try user systemd directory if root fails
			home, _ := os.UserHomeDir()
			userUnitDir := filepath.Join(home, ".config", "systemd", "user")
			_ = os.MkdirAll(userUnitDir, 0755)
			unitPath = filepath.Join(userUnitDir, "lapdog.service")
			if userErr := os.WriteFile(unitPath, []byte(serviceContent), 0644); userErr != nil {
				return fmt.Errorf("failed to write systemd service (root & user): %w", err)
			}
			fmt.Printf("Systemd user service written to %s\nRun 'systemctl --user enable --now lapdog'\n", unitPath)
			return nil
		}
		fmt.Printf("Systemd service written to %s\nRun 'sudo systemctl daemon-reload && sudo systemctl enable --now lapdog'\n", unitPath)
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
		_ = os.Remove("/etc/systemd/system/lapdog.service")
		home, _ := os.UserHomeDir()
		_ = os.Remove(filepath.Join(home, ".config", "systemd", "user", "lapdog.service"))
		fmt.Println("Systemd service files removed.")
		return nil
	case "darwin":
		home, _ := os.UserHomeDir()
		plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.lapdog.agent.plist")
		_ = exec.Command("launchctl", "unload", plistPath).Run()
		_ = os.Remove(plistPath)
		fmt.Println("LaunchAgent removed.")
		return nil
	case "windows":
		out, err := exec.Command("sc.exe", "delete", "lapdog").CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete Windows service: %w (%s)", err, string(out))
		}
		fmt.Println("Windows service 'lapdog' deleted.")
		return nil
	default:
		return fmt.Errorf("unsupported OS")
	}
}
