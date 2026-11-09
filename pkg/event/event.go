package event

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

type EventType string

const (
	LoginSuccess EventType = "LOGIN_SUCCESS"
	LoginFailure EventType = "LOGIN_FAILURE"
	Unlock       EventType = "UNLOCK"
	RemoteLogin  EventType = "REMOTE_LOGIN"
	USBInsert    EventType = "USB_INSERT"
	Heartbeat    EventType = "HEARTBEAT"
)

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

// SecurityEvent represents a normalized cross-platform security event
type SecurityEvent struct {
	ID        string    `json:"id" yaml:"id"`
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
	DeviceID  string    `json:"device_id" yaml:"device_id"`
	Hostname  string    `json:"hostname" yaml:"hostname"`
	Platform  string    `json:"platform" yaml:"platform"`
	EventType EventType `json:"event_type" yaml:"event_type"`
	Username  string    `json:"username" yaml:"username"`
	Source    string    `json:"source" yaml:"source"`
	SourceIP  string    `json:"source_ip,omitempty" yaml:"source_ip,omitempty"`
	LogonType string    `json:"logon_type,omitempty" yaml:"logon_type,omitempty"`
	Severity     Severity  `json:"severity" yaml:"severity"`
	Details      string    `json:"details,omitempty" yaml:"details,omitempty"`
	ImagePath    string    `json:"image_path,omitempty" yaml:"image_path,omitempty"`
	WebcamStatus string    `json:"webcam_status,omitempty" yaml:"webcam_status,omitempty"`
}

// Fingerprint generates a deterministic hash to identify duplicate events in a short window
func (e *SecurityEvent) Fingerprint() string {
	if e.ID != "" {
		return e.ID
	}
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		e.Platform, e.EventType, e.Username, e.Source, e.SourceIP, e.Timestamp.Format(time.RFC3339))
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:12])
}

// Emoji returns a visual icon based on EventType and Severity
func (e *SecurityEvent) Emoji() string {
	switch e.Severity {
	case SeverityCritical:
		return "🔴"
	case SeverityHigh:
		return "⚠️"
	case SeverityMedium:
		return "🔒"
	default:
		return "ℹ️"
	}
}

// FormatTelegramHTML renders the event into a Telegram HTML message
func (e *SecurityEvent) FormatTelegramHTML() string {
	var title string
	switch e.EventType {
	case LoginSuccess:
		title = "<b>SUCCESSFUL LOGIN ALERT</b>"
	case LoginFailure:
		title = "<b>FAILED LOGIN ATTEMPT</b>"
	case Unlock:
		title = "<b>WORKSTATION UNLOCKED</b>"
	case RemoteLogin:
		title = "<b>REMOTE ACCESS DETECTED</b>"
	case USBInsert:
		title = "<b>USB DEVICE INSERTED</b>"
	default:
		title = fmt.Sprintf("<b>SECURITY EVENT: %s</b>", e.EventType)
	}

	timeStr := e.Timestamp.Format("2006-01-02 15:04:05 MST")
	
	msg := fmt.Sprintf("%s %s\n\n"+
		"<b>Device:</b> <code>%s</code> (%s)\n"+
		"<b>User:</b> <code>%s</code>\n"+
		"<b>Source:</b> %s\n",
		e.Emoji(), title,
		escapeHTML(e.Hostname), strings.ToUpper(e.Platform),
		escapeHTML(e.Username),
		escapeHTML(e.Source))

	if e.SourceIP != "" {
		msg += fmt.Sprintf("<b>Source IP:</b> <code>%s</code>\n", escapeHTML(e.SourceIP))
	}
	if e.LogonType != "" {
		msg += fmt.Sprintf("<b>Logon Type:</b> %s\n", escapeHTML(e.LogonType))
	}

	msg += fmt.Sprintf("<b>Severity:</b> %s\n", e.Severity)
	msg += fmt.Sprintf("<b>Time:</b> %s\n", timeStr)

	explanation := e.HumanExplanation()
	if explanation != "" {
		msg += fmt.Sprintf("\n💡 <b>What happened:</b> %s\n", escapeHTML(explanation))
	}

	if e.Details != "" {
		details := e.Details
		if len(details) > 300 {
			details = details[:297] + "..."
		}
		msg += fmt.Sprintf("🔍 <b>System Log:</b> <code>%s</code>\n", escapeHTML(details))
	}

	if e.WebcamStatus != "" {
		if e.ImagePath != "" {
			msg += fmt.Sprintf("\n📸 <b>Webcam Snapshot:</b> %s", escapeHTML(e.WebcamStatus))
		} else {
			msg += fmt.Sprintf("\n📷 <b>Webcam Snapshot:</b> ⚠️ %s", escapeHTML(e.WebcamStatus))
		}
	}

	return msg
}

func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}
