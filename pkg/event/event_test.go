package event

import (
	"strings"
	"testing"
	"time"
)

func TestSecurityEventFormatting(t *testing.T) {
	now := time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC)
	evt := SecurityEvent{
		ID:        "evt-123",
		Timestamp: now,
		DeviceID:  "device-uuid-1",
		Hostname:  "Mochi-Laptop",
		Platform:  "linux",
		EventType: LoginFailure,
		Username:  "root",
		Source:    "SSH",
		SourceIP:  "192.168.1.100",
		Severity:  SeverityMedium,
		Details:   "3 failed password attempts",
	}

	if evt.Emoji() != "🔒" {
		t.Errorf("expected emoji 🔒, got %s", evt.Emoji())
	}

	html := evt.FormatTelegramHTML()
	if !strings.Contains(html, "FAILED LOGIN ATTEMPT") {
		t.Errorf("expected title in html, got: %s", html)
	}
	if !strings.Contains(html, "Mochi-Laptop") {
		t.Errorf("expected hostname in html, got: %s", html)
	}
	if !strings.Contains(html, "192.168.1.100") {
		t.Errorf("expected source IP in html, got: %s", html)
	}
}

func TestFingerprint(t *testing.T) {
	evt1 := SecurityEvent{
		ID:        "unique-1",
		Platform:  "linux",
		EventType: LoginSuccess,
		Username:  "alice",
		Timestamp: time.Now(),
	}
	if evt1.Fingerprint() != "unique-1" {
		t.Errorf("expected ID fingerprint 'unique-1', got %s", evt1.Fingerprint())
	}

	evt2 := SecurityEvent{
		Platform:  "windows",
		EventType: LoginFailure,
		Username:  "bob",
		Timestamp: time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC),
	}
	fp2 := evt2.Fingerprint()
	if len(fp2) == 0 {
		t.Errorf("expected non-empty generated fingerprint")
	}
}
