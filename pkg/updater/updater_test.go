package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"v0.1.0", "v0.1.0", false},
		{"v0.1.0", "v0.2.0", true},
		{"v0.1.0", "v0.1.1", true},
		{"v1.0.0", "v2.0.0", true},
		{"v0.2.0", "v0.1.0", false},
		{"0.1.0", "v0.2.0", true},
		{"v0.1.0", "0.2.0", true},
		{"dev", "v0.1.0", true},
		{"unknown", "v0.1.0", true},
		{"", "v0.1.0", true},
		{"v0.1.0-alpha", "v0.1.0", true},
		{"v0.1.0", "v0.1.0-alpha", false},
	}

	for _, tc := range tests {
		result := IsNewerVersion(tc.current, tc.latest)
		if result != tc.expected {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tc.current, tc.latest, result, tc.expected)
		}
	}
}

func TestGetPlatformInstallCommand(t *testing.T) {
	repo := "testuser/testrepo"

	winCmd := GetPlatformInstallCommand(repo, "windows")
	if !strings.Contains(winCmd, "install.ps1") || !strings.Contains(winCmd, repo) {
		t.Errorf("unexpected windows install command: %s", winCmd)
	}

	linuxCmd := GetPlatformInstallCommand(repo, "linux")
	if !strings.Contains(linuxCmd, "install.sh") || !strings.Contains(linuxCmd, repo) {
		t.Errorf("unexpected linux install command: %s", linuxCmd)
	}

	darwinCmd := GetPlatformInstallCommand(repo, "darwin")
	if !strings.Contains(darwinCmd, "install.sh") || !strings.Contains(darwinCmd, repo) {
		t.Errorf("unexpected darwin install command: %s", darwinCmd)
	}
}

func TestCheckUpdateWithClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"tag_name": "v0.2.0",
			"name": "v0.2.0 - Security Update",
			"html_url": "https://github.com/mochiron-desu/lap-dog/releases/tag/v0.2.0",
			"body": "Fixed critical security issues"
		}`))
	}))
	defer ts.Close()

	result, err := CheckUpdateWithClient(context.Background(), ts.Client(), "v0.1.0", "mochiron-desu/lap-dog", ts.URL)
	if err != nil {
		t.Fatalf("CheckUpdateWithClient failed: %v", err)
	}

	if result.LatestVersion != "v0.2.0" {
		t.Errorf("expected latest version v0.2.0, got %s", result.LatestVersion)
	}
	if !result.UpdateAvailable {
		t.Errorf("expected UpdateAvailable to be true")
	}

	tgMsg := FormatTelegramMessage(result, "test-host", "device-123")
	if !strings.Contains(tgMsg, "Update Available") || !strings.Contains(tgMsg, "v0.2.0") {
		t.Errorf("unexpected telegram message: %s", tgMsg)
	}

	cliMsg := FormatCLIMessage(result)
	if !strings.Contains(cliMsg, "A newer release of lap-dog is available") {
		t.Errorf("unexpected cli message: %s", cliMsg)
	}
}
