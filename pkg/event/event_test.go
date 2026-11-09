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
	if !strings.Contains(html, "What happened:") {
		t.Errorf("expected 'What happened:' in html, got: %s", html)
	}
	if !strings.Contains(html, "System Log:") {
		t.Errorf("expected 'System Log:' in html, got: %s", html)
	}

	// Test webcam status formatting
	evtWithPhoto := evt
	evtWithPhoto.ImagePath = "/tmp/snap.jpg"
	evtWithPhoto.WebcamStatus = "Snapshot captured successfully"
	htmlPhoto := evtWithPhoto.FormatTelegramHTML()
	if !strings.Contains(htmlPhoto, "📸 <b>Webcam Snapshot:</b> Snapshot captured successfully") {
		t.Errorf("expected success webcam status in html, got: %s", htmlPhoto)
	}

	evtBlocked := evt
	evtBlocked.ImagePath = ""
	evtBlocked.WebcamStatus = "Blocked by Antivirus, security policy, or OS privacy permissions (Access Denied)"
	htmlBlocked := evtBlocked.FormatTelegramHTML()
	if !strings.Contains(htmlBlocked, "📷 <b>Webcam Snapshot:</b> ⚠️ Blocked by Antivirus") {
		t.Errorf("expected blocked webcam status warning in html, got: %s", htmlBlocked)
	}
}

func TestHumanExplanationDictionary(t *testing.T) {
	tests := []struct {
		name     string
		event    SecurityEvent
		expected string
	}{
		{
			name: "GKR PAM Keyring Unlock",
			event: SecurityEvent{
				Platform:  "linux",
				EventType: Unlock,
				Details:   "gkr-pam: unlocked login keyring",
			},
			expected: "Screen unlocked with valid password (GNOME credentials vault unlocked)",
		},
		{
			name: "SSH Failed Password",
			event: SecurityEvent{
				Platform:  "linux",
				EventType: LoginFailure,
				Details:   "Failed password for invalid user admin from 10.0.0.1 port 22",
			},
			expected: "Remote SSH connection attempt failed (incorrect password or missing key)",
		},
		{
			name: "PAM Auth Failure",
			event: SecurityEvent{
				Platform:  "linux",
				EventType: LoginFailure,
				Details:   "pam_unix(sudo:auth): authentication failure; logname=user uid=1000",
			},
			expected: "Incorrect password entered during authentication (lock screen, sudo, or login)",
		},
		{
			name: "Windows Logon Type 2",
			event: SecurityEvent{
				Platform:  "windows",
				EventType: LoginSuccess,
				LogonType: "2",
			},
			expected: "Interactive console login (physically typed password at keyboard)",
		},
		{
			name: "Fallback by EventType",
			event: SecurityEvent{
				Platform:  "linux",
				EventType: USBInsert,
			},
			expected: "A physical USB device or peripheral was plugged in",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.event.HumanExplanation()
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
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
