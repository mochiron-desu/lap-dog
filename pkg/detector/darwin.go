package detector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"lap-dog/pkg/config"
	"lap-dog/pkg/event"
)

type DarwinDetector struct {
	cfg *config.Config
}

func NewDarwinDetector(cfg *config.Config) *DarwinDetector {
	return &DarwinDetector{
		cfg: cfg,
	}
}

func (d *DarwinDetector) Name() string {
	return "macOS Unified Logging Detector"
}

type macOSLogEntry struct {
	Timestamp       string `json:"timestamp"`
	Process         string `json:"processImagePath"`
	EventMessage    string `json:"eventMessage"`
	MessageType     string `json:"messageType"`
}

func (d *DarwinDetector) Start(ctx context.Context, eventChan chan<- event.SecurityEvent) error {
	predicate := `process CONTAINS "loginwindow" OR process CONTAINS "sshd" OR process CONTAINS "sudo" OR process CONTAINS "authorizationd"`
	cmd := exec.CommandContext(ctx, "log", "stream", "--style", "json", "--predicate", predicate)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("macOS log pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("macOS log stream start error: %w", err)
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

			var entry macOSLogEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				continue
			}

			if evt, ok := d.parseDarwinEntry(&entry); ok {
				eventChan <- evt
			}
		}
	}
}

func (d *DarwinDetector) parseDarwinEntry(entry *macOSLogEntry) (event.SecurityEvent, bool) {
	msg := entry.EventMessage
	if msg == "" {
		return event.SecurityEvent{}, false
	}

	now := time.Now()
	baseEvt := event.SecurityEvent{
		ID:        fmt.Sprintf("mac-%d", now.UnixNano()),
		Timestamp: now,
		DeviceID:  d.cfg.DeviceID,
		Hostname:  d.cfg.Hostname,
		Platform:  "darwin",
		Source:    "UnifiedLog",
		Severity:  event.SeverityMedium,
		Details:   msg,
	}

	reLogin := regexp.MustCompile(`sessionDidLogin|User logged in`)
	reUnlock := regexp.MustCompile(`screenUnlocked|Screen unlocked`)
	reSSHFail := regexp.MustCompile(`Failed password for (\S+) from (\S+)`)
	reSSHOk := regexp.MustCompile(`Accepted password for (\S+) from (\S+)`)
	reSudoFail := regexp.MustCompile(`3 incorrect password attempts|authentication failure`)

	if reLogin.MatchString(msg) {
		// Ignore automated daemon logins or periodic background tasks
		if strings.Contains(msg, "com.apple.periodic") || strings.Contains(msg, "cron") {
			return event.SecurityEvent{}, false
		}
		baseEvt.EventType = event.LoginSuccess
		baseEvt.Severity = event.SeverityCritical
		baseEvt.Username = "macUser"
		return baseEvt, true
	}

	if reUnlock.MatchString(msg) {
		baseEvt.EventType = event.Unlock
		baseEvt.Severity = event.SeverityCritical
		baseEvt.Username = "macUser"
		return baseEvt, true
	}

	if matches := reSSHFail.FindStringSubmatch(msg); len(matches) >= 3 {
		baseEvt.EventType = event.LoginFailure
		baseEvt.Username = matches[1]
		baseEvt.SourceIP = matches[2]
		baseEvt.Source = "SSH"
		baseEvt.Severity = event.SeverityMedium
		return baseEvt, true
	}

	if matches := reSSHOk.FindStringSubmatch(msg); len(matches) >= 3 {
		baseEvt.EventType = event.RemoteLogin
		baseEvt.Username = matches[1]
		baseEvt.SourceIP = matches[2]
		baseEvt.Source = "SSH"
		baseEvt.Severity = event.SeverityHigh
		return baseEvt, true
	}

	if reSudoFail.MatchString(msg) {
		baseEvt.EventType = event.LoginFailure
		baseEvt.Username = "sudo"
		baseEvt.Severity = event.SeverityHigh
		return baseEvt, true
	}

	return event.SecurityEvent{}, false
}
