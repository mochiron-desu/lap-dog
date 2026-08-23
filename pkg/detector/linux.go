package detector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
)

type LinuxDetector struct {
	cfg *config.Config
}

func NewLinuxDetector(cfg *config.Config) *LinuxDetector {
	return &LinuxDetector{
		cfg: cfg,
	}
}

func (l *LinuxDetector) Name() string {
	return "Linux (journald/auth.log) Detector"
}

type JournalEntry struct {
	Message          string `json:"MESSAGE"`
	SyslogIdentifier string `json:"SYSLOG_IDENTIFIER"`
	AuditType        string `json:"_AUDIT_TYPE"`
	AuditTypeNum     string `json:"_AUDIT_TYPE_NAME"`
	PID              string `json:"_PID"`
	TimestampMicro   string `json:"__REALTIME_TIMESTAMP"`
}

func (l *LinuxDetector) Start(ctx context.Context, eventChan chan<- event.SecurityEvent) error {
	// Attempt journalctl first
	if _, err := exec.LookPath("journalctl"); err == nil {
		err := l.streamJournalctl(ctx, eventChan)
		if err == nil || ctx.Err() != nil {
			return err
		}
		log.Printf("[LinuxDetector] journalctl stream ended (%v), falling back to auth.log tailing", err)
	}

	// Fallback to tailing /var/log/auth.log or /var/log/secure
	return l.tailAuthLog(ctx, eventChan)
}

func (l *LinuxDetector) streamJournalctl(ctx context.Context, eventChan chan<- event.SecurityEvent) error {
	cmd := exec.CommandContext(ctx, "journalctl", "-o", "json", "-f", "-n", "0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("journalctl pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("journalctl start error: %w", err)
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

			var entry JournalEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				continue
			}

			if evt, ok := l.parseJournalEntry(&entry); ok {
				eventChan <- evt
			}
		}
	}
}

func (l *LinuxDetector) parseJournalEntry(entry *JournalEntry) (event.SecurityEvent, bool) {
	msg := entry.Message
	if msg == "" {
		return event.SecurityEvent{}, false
	}

	now := time.Now()
	baseEvt := event.SecurityEvent{
		ID:        fmt.Sprintf("linux-%d", now.UnixNano()),
		Timestamp: now,
		DeviceID:  l.cfg.DeviceID,
		Hostname:  l.cfg.Hostname,
		Platform:  "linux",
		Source:    "journald",
		Severity:  event.SeverityMedium,
	}

	// Regex patterns for Linux auth events
	var (
		reSSHFail       = regexp.MustCompile(`Failed (password|publickey) for (invalid user )?(\S+) from (\S+) port \d+`)
		reSSHOk         = regexp.MustCompile(`Accepted (password|publickey) for (\S+) from (\S+) port \d+`)
		rePAMFail       = regexp.MustCompile(`pam_unix\(\S+:auth\): authentication failure`)
		reLogindSession = regexp.MustCompile(`New session \S+ of user (\S+)\.`)
		rePAMSession    = regexp.MustCompile(`pam_unix\(\S+:session\): session opened for user (\S+)`)
		reUnlock        = regexp.MustCompile(`(?i)(screen unlocked|session unlocked|unlocked screen|unlocked|is now unlocked)`)
		reUSB           = regexp.MustCompile(`New USB device found, idVendor=\S+`)
		reAuditAuth     = regexp.MustCompile(`type=USER_AUTH.*acct="([^"]+)".*res=(success|failed)`)
	)

	// 1. SSH Failure
	if matches := reSSHFail.FindStringSubmatch(msg); len(matches) >= 5 {
		baseEvt.EventType = event.LoginFailure
		baseEvt.Username = matches[3]
		baseEvt.SourceIP = matches[4]
		baseEvt.Source = "SSH"
		baseEvt.Severity = event.SeverityMedium
		baseEvt.Details = msg
		return baseEvt, true
	}

	// 2. SSH Success
	if matches := reSSHOk.FindStringSubmatch(msg); len(matches) >= 4 {
		baseEvt.EventType = event.RemoteLogin
		baseEvt.Username = matches[2]
		baseEvt.SourceIP = matches[3]
		baseEvt.Source = "SSH"
		baseEvt.Severity = event.SeverityHigh
		baseEvt.Details = msg
		return baseEvt, true
	}

	// 3. PAM Authentication Failure (sudo, su, gdm, lockscreen)
	if rePAMFail.MatchString(msg) || strings.Contains(msg, "authentication failure") || strings.Contains(msg, "FAILED SU") {
		user := "unknown"
		if idx := strings.Index(msg, "user="); idx != -1 {
			fields := strings.Fields(msg[idx+5:])
			if len(fields) > 0 && fields[0] != "" {
				user = fields[0]
			}
		} else if idx := strings.Index(msg, "ruser="); idx != -1 {
			fields := strings.Fields(msg[idx+6:])
			if len(fields) > 0 && fields[0] != "" {
				user = fields[0]
			}
		}
		baseEvt.EventType = event.LoginFailure
		baseEvt.Username = user
		baseEvt.Source = "PAM"
		baseEvt.Severity = event.SeverityMedium
		baseEvt.Details = msg
		return baseEvt, true
	}

	// 4. Kernel Audit Authentication event
	if matches := reAuditAuth.FindStringSubmatch(msg); len(matches) >= 3 {
		username := matches[1]
		result := matches[2]
		if result == "failed" {
			baseEvt.EventType = event.LoginFailure
			baseEvt.Username = username
			baseEvt.Source = "Audit"
			baseEvt.Severity = event.SeverityMedium
			baseEvt.Details = msg
			return baseEvt, true
		} else if result == "success" {
			baseEvt.EventType = event.LoginSuccess
			baseEvt.Username = username
			baseEvt.Source = "Audit"
			baseEvt.Severity = event.SeverityCritical
			baseEvt.Details = msg
			return baseEvt, true
		}
	}

	// 5. New Session / Login (systemd-logind or pam_unix session)
	if matches := reLogindSession.FindStringSubmatch(msg); len(matches) >= 2 {
		baseEvt.EventType = event.LoginSuccess
		baseEvt.Username = matches[1]
		baseEvt.Source = "systemd-logind"
		baseEvt.Severity = event.SeverityCritical
		baseEvt.Details = msg
		return baseEvt, true
	}
	if matches := rePAMSession.FindStringSubmatch(msg); len(matches) >= 2 {
		// Ignore automated background cron and system daemon sessions
		if strings.Contains(msg, "cron:session") || strings.Contains(msg, "systemd:session") || strings.Contains(msg, "systemd-user:session") || strings.Contains(msg, "polkit-1:session") {
			return event.SecurityEvent{}, false
		}
		baseEvt.EventType = event.LoginSuccess
		baseEvt.Username = matches[1]
		baseEvt.Source = "pam_unix"
		baseEvt.Severity = event.SeverityCritical
		baseEvt.Details = msg
		return baseEvt, true
	}

	// 6. Screen Unlock
	if reUnlock.MatchString(msg) {
		baseEvt.EventType = event.Unlock
		baseEvt.Username = "user"
		baseEvt.Source = "DisplayManager"
		baseEvt.Severity = event.SeverityCritical
		baseEvt.Details = msg
		return baseEvt, true
	}

	// 7. USB Insertion
	if reUSB.MatchString(msg) {
		baseEvt.EventType = event.USBInsert
		baseEvt.Username = "system"
		baseEvt.Source = "kernel"
		baseEvt.Severity = event.SeverityHigh
		baseEvt.Details = msg
		return baseEvt, true
	}

	return event.SecurityEvent{}, false
}

func (l *LinuxDetector) tailAuthLog(ctx context.Context, eventChan chan<- event.SecurityEvent) error {
	logPaths := []string{"/var/log/auth.log", "/var/log/secure"}
	var logFile string
	for _, p := range logPaths {
		if _, err := os.Stat(p); err == nil {
			logFile = p
			break
		}
	}

	if logFile == "" {
		return fmt.Errorf("no accessible auth log file found in /var/log")
	}

	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", logFile, err)
	}
	defer file.Close()

	// Seek to end
	_, _ = file.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(file)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				entry := JournalEntry{Message: strings.TrimSpace(line)}
				if evt, ok := l.parseJournalEntry(&entry); ok {
					eventChan <- evt
				}
			}
		}
	}
}
