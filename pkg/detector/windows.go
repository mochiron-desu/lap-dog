package detector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
)

type WindowsDetector struct {
	cfg *config.Config
}

func NewWindowsDetector(cfg *config.Config) *WindowsDetector {
	return &WindowsDetector{
		cfg: cfg,
	}
}

func (w *WindowsDetector) Name() string {
	return "Windows Event Log Detector"
}

type winEventData struct {
	Id          int       `json:"Id"`
	TimeCreated time.Time `json:"TimeCreated"`
	Message     string    `json:"Message"`
}

func (w *WindowsDetector) Start(ctx context.Context, eventChan chan<- event.SecurityEvent) error {
	// Script to continuously query PowerShell Get-WinEvent for new Security events
	psScript := `
$lastTime = [DateTime]::Now
while ($true) {
    Start-Sleep -Seconds 2
    $events = Get-WinEvent -FilterHashtable @{LogName='Security'; StartTime=$lastTime; Id=4624,4625,4800,4801,4778,4779} -ErrorAction SilentlyContinue
    if ($events) {
        $lastTime = [DateTime]::Now
        foreach ($e in $events) {
            [PSCustomObject]@{
                Id = $e.Id
                TimeCreated = $e.TimeCreated
                Message = $e.Message
            } | ConvertTo-Json -Compress
        }
    }
}
`

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", psScript)
	setHiddenProcAttr(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("powershell pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("powershell start error: %w", err)
	}

	reader := bufio.NewReader(stdout)
	for {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}

			var entry winEventData
			if err := json.Unmarshal(line, &entry); err != nil {
				continue
			}

			if evt, ok := w.parseWinEvent(&entry); ok {
				eventChan <- evt
			}
		}
	}
}

func (w *WindowsDetector) parseWinEvent(entry *winEventData) (event.SecurityEvent, bool) {
	now := time.Now()
	if !entry.TimeCreated.IsZero() {
		now = entry.TimeCreated
	}

	baseEvt := event.SecurityEvent{
		ID:        fmt.Sprintf("win-%d-%d", entry.Id, now.UnixNano()),
		Timestamp: now,
		DeviceID:  w.cfg.DeviceID,
		Hostname:  w.cfg.Hostname,
		Platform:  "windows",
		Source:    "Windows EventLog",
		Details:   entry.Message,
	}

	switch entry.Id {
	case 4625: // Login Failure
		baseEvt.EventType = event.LoginFailure
		baseEvt.Severity = event.SeverityMedium
		baseEvt.Username = extractWinField(entry.Message, "Account Name:")
		baseEvt.SourceIP = extractWinField(entry.Message, "Source Network Address:")
		return baseEvt, true

	case 4624: // Login Success
		logonType := extractWinField(entry.Message, "Logon Type:")
		username := extractWinField(entry.Message, "Account Name:")

		// Ignore background services (LogonType 5), scheduled tasks (LogonType 4), and system startups (LogonType 0)
		if logonType == "0" || logonType == "4" || logonType == "5" {
			return event.SecurityEvent{}, false
		}

		// Ignore Windows system and machine accounts
		upperUser := strings.ToUpper(username)
		if upperUser == "SYSTEM" || upperUser == "LOCAL SERVICE" || upperUser == "NETWORK SERVICE" || upperUser == "ANONYMOUS LOGON" || strings.HasSuffix(username, "$") {
			return event.SecurityEvent{}, false
		}

		baseEvt.Username = username
		baseEvt.LogonType = logonType
		baseEvt.SourceIP = extractWinField(entry.Message, "Source Network Address:")

		if logonType == "10" || logonType == "3" { // RDP or Network
			baseEvt.EventType = event.RemoteLogin
			baseEvt.Severity = event.SeverityHigh
			baseEvt.Source = "RDP/Network"
		} else {
			baseEvt.EventType = event.LoginSuccess
			baseEvt.Severity = event.SeverityCritical
		}
		return baseEvt, true

	case 4801: // Workstation Unlocked
		baseEvt.EventType = event.Unlock
		baseEvt.Severity = event.SeverityCritical
		baseEvt.Username = extractWinField(entry.Message, "Account Name:")
		return baseEvt, true

	case 4778: // RDP Connect
		baseEvt.EventType = event.RemoteLogin
		baseEvt.Severity = event.SeverityHigh
		baseEvt.Source = "RDP Session"
		baseEvt.Username = extractWinField(entry.Message, "Account Name:")
		return baseEvt, true
	}

	return event.SecurityEvent{}, false
}

func extractWinField(msg, fieldName string) string {
	idx := strings.Index(msg, fieldName)
	if idx == -1 {
		return "unknown"
	}
	sub := msg[idx+len(fieldName):]
	lines := strings.Split(sub, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return "unknown"
}
