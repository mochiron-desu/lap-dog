package event

import (
	"regexp"
	"strings"
)

// LogTranslation represents an entry in the security log dictionary
type LogTranslation struct {
	Pattern     *regexp.Regexp
	Explanation string
}

// logDictionary maps technical system log patterns across Linux, macOS, and Windows to plain English
var logDictionary = []LogTranslation{
	// GNOME Keyring & PAM
	{
		Pattern:     regexp.MustCompile(`(?i)gkr-pam:\s*unlocked login keyring`),
		Explanation: "Screen unlocked with valid password (GNOME credentials vault unlocked)",
	},
	{
		Pattern:     regexp.MustCompile(`(?i)gkr-pam`),
		Explanation: "GNOME keyring password vault accessed via PAM authentication",
	},

	// Screen lock/unlock
	{
		Pattern:     regexp.MustCompile(`(?i)(screen unlocked|session unlocked|unlocked screen|is now unlocked|screenUnlocked)`),
		Explanation: "Workstation lock screen dismissed after successful authentication",
	},

	// SSH logins
	{
		Pattern:     regexp.MustCompile(`(?i)Accepted (password|publickey) for (\S+) from (\S+)`),
		Explanation: "Remote SSH terminal session connected and authenticated",
	},
	{
		Pattern:     regexp.MustCompile(`(?i)Failed (password|publickey) for (invalid user )?(\S+) from (\S+)`),
		Explanation: "Remote SSH connection attempt failed (incorrect password or missing key)",
	},

	// Password / PAM authentication failures
	{
		Pattern:     regexp.MustCompile(`(?i)3 incorrect password attempts`),
		Explanation: "Multiple consecutive incorrect password attempts",
	},
	{
		Pattern:     regexp.MustCompile(`(?i)(pam_unix\(.*:auth\):\s*authentication failure|authentication failure)`),
		Explanation: "Incorrect password entered during authentication (lock screen, sudo, or login)",
	},
	{
		Pattern:     regexp.MustCompile(`(?i)FAILED SU`),
		Explanation: "Failed attempt to switch user or acquire root permissions via su",
	},

	// Sessions (systemd-logind / pam_unix)
	{
		Pattern:     regexp.MustCompile(`(?i)New session \S+ of user (\S+)`),
		Explanation: "New interactive desktop or console login session started",
	},
	{
		Pattern:     regexp.MustCompile(`(?i)pam_unix\(.*:session\):\s*session opened for user`),
		Explanation: "User login session initialized by authentication system",
	},

	// USB Hardware
	{
		Pattern:     regexp.MustCompile(`(?i)New USB device found`),
		Explanation: "A physical USB hardware device or accessory was connected",
	},

	// Linux Kernel Audit
	{
		Pattern:     regexp.MustCompile(`(?i)type=USER_AUTH.*res=failed`),
		Explanation: "Kernel audit: User authentication failed",
	},
	{
		Pattern:     regexp.MustCompile(`(?i)type=USER_AUTH.*res=success`),
		Explanation: "Kernel audit: User authentication succeeded",
	},

	// macOS Unified Logs
	{
		Pattern:     regexp.MustCompile(`(?i)(sessionDidLogin|User logged in)`),
		Explanation: "User successfully logged into macOS desktop session",
	},
}

// HumanExplanation returns a clear, human-readable description of what actually happened
func (e *SecurityEvent) HumanExplanation() string {
	// 1. Windows specific logon types
	if strings.ToLower(e.Platform) == "windows" && e.LogonType != "" {
		switch e.LogonType {
		case "2":
			return "Interactive console login (physically typed password at keyboard)"
		case "7":
			return "Workstation unlocked (lock screen dismissed)"
		case "10":
			return "Remote Desktop (RDP) login from external network host"
		case "3":
			return "Network login (accessed shared folder or network service)"
		case "11":
			return "Cached credentials login (unlocked while disconnected from network domain)"
		}
	}

	// 2. Search log dictionary for matching details
	if e.Details != "" {
		for _, entry := range logDictionary {
			if entry.Pattern.MatchString(e.Details) {
				return entry.Explanation
			}
		}
	}

	// 3. Fallback to clean explanations based on normalized EventType
	switch e.EventType {
	case Unlock:
		return "Workstation screen was unlocked"
	case LoginSuccess:
		return "User successfully authenticated and logged in"
	case LoginFailure:
		return "Failed login attempt (incorrect password or unknown username)"
	case RemoteLogin:
		return "Remote access session connected to this workstation"
	case USBInsert:
		return "A physical USB device or peripheral was plugged in"
	case Heartbeat:
		return "Agent status heartbeat check"
	default:
		return "Security event recorded"
	}
}
