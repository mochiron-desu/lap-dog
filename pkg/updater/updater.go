package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepo = "mochiron-desu/lap-dog"
)

// GitHubRelease represents the release payload returned by GitHub API
type GitHubRelease struct {
	TagName      string    `json:"tag_name"`
	Name         string    `json:"name"`
	HTMLURL      string    `json:"html_url"`
	Body         string    `json:"body"`
	PublishedAt  time.Time `json:"published_at"`
	Prerelease   bool      `json:"prerelease"`
	Draft        bool      `json:"draft"`
}

// UpdateResult contains the evaluated release information
type UpdateResult struct {
	Repo                string            `json:"repo"`
	CurrentVersion      string            `json:"current_version"`
	LatestVersion       string            `json:"latest_version"`
	UpdateAvailable     bool              `json:"update_available"`
	ReleaseTitle        string            `json:"release_title"`
	ReleaseURL          string            `json:"release_url"`
	ReleaseNotes        string            `json:"release_notes"`
	Platform            string            `json:"platform"`
	Arch                string            `json:"arch"`
	PlatformInstallCmd  string            `json:"platform_install_cmd"`
	AllPlatformCommands map[string]string `json:"all_platform_commands"`
}

// CheckUpdate queries GitHub for the latest release and compares with currentVersion
func CheckUpdate(ctx context.Context, currentVersion, repo string) (*UpdateResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	return CheckUpdateWithClient(ctx, client, currentVersion, repo, "https://api.github.com")
}

// CheckUpdateWithClient allows injecting custom http.Client and baseURL for testing
func CheckUpdateWithClient(ctx context.Context, client *http.Client, currentVersion, repo, baseURL string) (*UpdateResult, error) {
	if repo == "" {
		repo = DefaultRepo
	}

	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(baseURL, "/"), repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create update check request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "lap-dog-updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release information: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned status %d: %s", resp.StatusCode, string(body))
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to decode release response: %w", err)
	}

	latestTag := strings.TrimSpace(rel.TagName)
	updateAvail := IsNewerVersion(currentVersion, latestTag)

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	result := &UpdateResult{
		Repo:                repo,
		CurrentVersion:      currentVersion,
		LatestVersion:       latestTag,
		UpdateAvailable:     updateAvail,
		ReleaseTitle:        rel.Name,
		ReleaseURL:          rel.HTMLURL,
		ReleaseNotes:        rel.Body,
		Platform:            goos,
		Arch:                goarch,
		PlatformInstallCmd:  GetPlatformInstallCommand(repo, goos),
		AllPlatformCommands: GetAllPlatformInstallCommands(repo),
	}

	return result, nil
}

// parseSemver extracts [major, minor, patch] integers and pre-release tag
func parseSemver(v string) ([]int, string) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")

	var prerelease string
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		prerelease = v[idx+1:]
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	nums := make([]int, 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil {
			nums[i] = n
		}
	}
	return nums, prerelease
}

// IsNewerVersion returns true if latest is strictly newer than current
func IsNewerVersion(current, latest string) bool {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)

	if latest == "" || current == latest {
		return false
	}

	if current == "dev" || current == "unknown" || current == "" {
		return true
	}

	currNums, currPre := parseSemver(current)
	latestNums, latestPre := parseSemver(latest)

	for i := 0; i < 3; i++ {
		if latestNums[i] > currNums[i] {
			return true
		}
		if latestNums[i] < currNums[i] {
			return false
		}
	}

	// If semver numbers are identical, a non-prerelease is newer than a prerelease
	if currPre != "" && latestPre == "" {
		return true
	}

	return false
}

// GetPlatformInstallCommand returns the one-liner command to install/update for a given OS
func GetPlatformInstallCommand(repo, goos string) string {
	if repo == "" {
		repo = DefaultRepo
	}

	switch goos {
	case "windows":
		return fmt.Sprintf("irm https://raw.githubusercontent.com/%s/main/scripts/install.ps1 | iex", repo)
	case "darwin", "linux":
		return fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/%s/main/scripts/install.sh | bash", repo)
	default:
		return fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/%s/main/scripts/install.sh | bash", repo)
	}
}

// GetAllPlatformInstallCommands returns instructions for all supported operating systems
func GetAllPlatformInstallCommands(repo string) map[string]string {
	if repo == "" {
		repo = DefaultRepo
	}
	return map[string]string{
		"windows": fmt.Sprintf("irm https://raw.githubusercontent.com/%s/main/scripts/install.ps1 | iex", repo),
		"linux":   fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/%s/main/scripts/install.sh | bash", repo),
		"darwin":  fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/%s/main/scripts/install.sh | bash", repo),
	}
}

// FormatTelegramMessage formats update result as Telegram HTML message
func FormatTelegramMessage(result *UpdateResult, hostname, deviceID string) string {
	if !result.UpdateAvailable {
		return fmt.Sprintf("✅ <b>lap-dog is up to date!</b>\n\n"+
			"<b>Version:</b> <code>%s</code>\n"+
			"<b>Host:</b> <code>%s</code> (<code>%s/%s</code>)\n"+
			"<b>Device ID:</b> <code>%s</code>",
			result.CurrentVersion, hostname, result.Platform, result.Arch, deviceID)
	}

	platformName := "Windows"
	if result.Platform == "linux" {
		platformName = "Linux"
	} else if result.Platform == "darwin" {
		platformName = "macOS"
	}

	return fmt.Sprintf("🚀 <b>lap-dog Update Available: %s!</b>\n\n"+
		"<b>Current:</b> <code>%s</code> ➔ <b>Latest:</b> <code>%s</code>\n"+
		"<b>Host:</b> <code>%s</code> (<code>%s/%s</code>)\n\n"+
		"💻 <b>Upgrade Command (%s):</b>\n"+
		"<code>%s</code>\n\n"+
		"🌐 <b>Other Platforms:</b>\n"+
		"• <b>Windows:</b> <code>%s</code>\n"+
		"• <b>Linux & macOS:</b> <code>%s</code>\n\n"+
		"🔗 <a href=\"%s\">View Release Notes on GitHub</a>",
		result.LatestVersion,
		result.CurrentVersion,
		result.LatestVersion,
		hostname,
		result.Platform,
		result.Arch,
		platformName,
		result.PlatformInstallCmd,
		result.AllPlatformCommands["windows"],
		result.AllPlatformCommands["linux"],
		result.ReleaseURL,
	)
}

// FormatCLIMessage formats update result for terminal CLI output
func FormatCLIMessage(result *UpdateResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🔍 lap-dog Update Check (Repository: %s)\n", result.Repo))
	sb.WriteString(fmt.Sprintf("Current Version: %s (%s/%s)\n", result.CurrentVersion, result.Platform, result.Arch))
	sb.WriteString(fmt.Sprintf("Latest Release:  %s\n\n", result.LatestVersion))

	if result.UpdateAvailable {
		sb.WriteString(fmt.Sprintf("🚀 A newer release of lap-dog is available: %s!\n\n", result.LatestVersion))
		sb.WriteString("To upgrade on this machine, run:\n")
		sb.WriteString(fmt.Sprintf("  %s\n\n", result.PlatformInstallCmd))
		sb.WriteString("Upgrade on other platforms:\n")
		sb.WriteString(fmt.Sprintf("  Windows (PowerShell): %s\n", result.AllPlatformCommands["windows"]))
		sb.WriteString(fmt.Sprintf("  Linux/macOS (Bash):   %s\n\n", result.AllPlatformCommands["linux"]))
		sb.WriteString(fmt.Sprintf("Release page: %s\n", result.ReleaseURL))
	} else {
		sb.WriteString("✅ You are running the latest version of lap-dog.\n")
	}

	return sb.String()
}
