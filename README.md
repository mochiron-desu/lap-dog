# lap-dog: Cross-Platform Laptop Security Agent 🐕

**lap-dog** is a lightweight, cross-platform background security agent written in Go. It monitors laptop authentication and access events (logins, unlock attempts, remote sessions, USB insertions) across **Linux**, **Windows**, and **macOS**, sending instant alerts and interactive controls via a Telegram bot.

---

## 🚀 Quick Start (No Compiler Required)

Install `lap-dog` instantly using zero-dependency prebuilt binaries from GitHub Releases:

### 🐧 Linux & 🍎 macOS
```bash
curl -fsSL https://raw.githubusercontent.com/mochiron-desu/lap-dog/main/scripts/install.sh | bash
```

### 🪟 Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/mochiron-desu/lap-dog/main/scripts/install.ps1 | iex
```

### 🧙‍♂️ Interactive Setup
Once installed, run the setup wizard to link your Telegram bot and enable auto-start in under 30 seconds:
```bash
lapdog setup
```

---

## ⚙️ Updating Configuration in the Future

If you ever need to update your **Telegram Bot Token**, **Chat ID**, **thresholds**, or **Away Mode preferences** later on:

### Option 1: Re-run Interactive Setup (Recommended)
Run the setup wizard anytime to interactively modify settings:
```bash
lapdog setup
```

### Option 2: Direct Configuration File Edit
You can directly edit `config.yaml` using any text editor:
- **Linux & macOS**: `nano ~/.lapdog/config.yaml`
- **Windows**: `notepad $env:USERPROFILE\.lapdog\config.yaml`

### 🔄 Restarting the Daemon After Configuration Changes
If `lap-dog` is actively running as a background service, you **must restart the service** for new settings to take effect:

- **Linux (systemd User Service)**:
  ```bash
  systemctl --user restart lapdog
  ```
- **Linux (systemd System Service)**:
  ```bash
  sudo systemctl restart lapdog
  ```
- **macOS (launchd)**:
  ```bash
  launchctl unload ~/Library/LaunchAgents/com.lapdog.agent.plist
  launchctl load ~/Library/LaunchAgents/com.lapdog.agent.plist
  ```
- **Windows (Task Scheduler / PowerShell)**:
  ```powershell
  schtasks /Run /TN "lapdog"
  # or if running as a Windows Service:
  Restart-Service lapdog
  ```
- **Manual CLI Execution**: Press `Ctrl+C` to stop the daemon, then re-run `lapdog run`.

### 🧪 Verify Updated Configuration
After restarting, confirm your changes and test alert delivery:
```bash
lapdog status
lapdog test
```

---

## 🌟 Key Features

- **Cross-Platform Event Sources**:
  - **Linux**: systemd-journald (`journalctl`), PAM authentication logs (`/var/log/auth.log`), SSH logins, lock/unlock events.
  - **Windows**: Windows Security Event Log (ETW/PowerShell 4624 login, 4625 failed login, 4800/4801 lock/unlock, 4778 RDP session).
  - **macOS**: Unified Logging (`os_log` stream for `loginwindow`, `sshd`, `sudo`, `authorizationd`).
  - **Mock Simulator**: Built-in test event generator for unprivileged testing (`lapdog run --mock`).
- **Rule & Escalation Engine**:
  - **Away Mode Toggle**: Suppresses non-critical alerts when user is at the machine; fires alerts when Away Mode is ARMED.
  - **Brute Force Escalation**: Automatically escalates multiple failed logins in a short window to `CRITICAL` severity.
  - **Rate Limiting & Deduplication**: Throttles alert bursts and discards duplicate events.
- **Webcam Snapshot on Failed Authentication**:
  - Captures a temporary webcam photo on failed login attempts and stores it in `~/.lapdog/captures/`.
  - Dispatches as a high-resolution photo with HTML caption via Telegram `sendPhoto`.
  - **Graceful Antivirus & Exception Fallback**: If antivirus software, OS privacy permissions, or another application (busy camera) prevents access, `lap-dog` falls back immediately without dropping the alert, and diagnoses the exact block reason to the user on Telegram.
  - **Offline Storage & Auto-Cleanup**: The photo persists safely in the local queue while offline and is automatically deleted from disk once delivered.
- **Offline-First Storage Queue**: Local JSON/queue store ensures zero alert loss during network drops with automatic retry backoff.
- **Telegram Bot Integration**:
  - HTML alerts with emoji indicators (🔴, 🔒, ⚠️).
  - Interactive bot commands: `/away` (Arm), `/back` (Disarm), `/status` (Metrics), `/test`, `/help`.
- **Background Daemon & Service Installer**: Built-in `lapdog service install` for systemd, launchd, and Windows Services.

---

## 📦 Prebuilt Release Binaries & Supply Chain Verification

Every tag release (`v*`) automatically builds and publishes static binaries for 6 platform architectures:

| Operating System | Architecture | Binary Asset Name |
| :--- | :--- | :--- |
| **Linux** | x86_64 / amd64 | `lapdog-linux-amd64` |
| **Linux** | ARM64 / aarch64 | `lapdog-linux-arm64` |
| **Windows** | x86_64 / amd64 | `lapdog-windows-amd64.exe` |
| **Windows** | ARM64 | `lapdog-windows-arm64.exe` |
| **macOS** | Intel (amd64) | `lapdog-darwin-amd64` |
| **macOS** | Apple Silicon (arm64) | `lapdog-darwin-arm64` |

### 🔒 Verify Checksums
All releases include a signed/hashed `checksums.txt` file. You can verify your downloaded binary manually:

```bash
# Linux
sha256sum -c checksums.txt --ignore-missing

# macOS
shasum -a 256 -c checksums.txt

# Windows PowerShell
Get-FileHash -Path lapdog-windows-amd64.exe -Algorithm SHA256
```

---

## 🍏 Note for macOS Users

On macOS, downloading unsigned binaries directly from GitHub may trigger Apple Gatekeeper warnings on first execution (*"Apple cannot verify that 'lapdog' is free of malware"*).

To allow execution on macOS:
```bash
# Remove quarantine attribute on downloaded binary
xattr -d com.apple.quarantine lapdog-darwin-arm64  # or lapdog-darwin-amd64
```
Alternatively, go to **System Settings > Privacy & Security** and click **"Allow Anyway"**.

---

## 🛠️ CLI Commands Reference

| Command | Description |
| :--- | :--- |
| `lapdog setup` | Interactive wizard for Telegram bot token, chat ID, and background service setup |
| `lapdog run` | Starts the security daemon (use `--background` to run silently in background, `--mock` for simulator) |
| `lapdog away` | Arms Away Mode (alerts enabled for access events) |
| `lapdog back` | Disarms Away Mode (suppresses non-critical alerts) |
| `lapdog status` | Displays dashboard status, version build metadata, and queue metrics |
| `lapdog test` | Enqueues and sends a test alert notification |
| `lapdog simulate` | Injects synthetic test events (`--event LOGIN_FAILURE --user attacker`) |
| `lapdog update` | Checks for newer releases on GitHub and displays platform upgrade instructions |
| `lapdog service install` | Registers lap-dog as a background system service (systemd / launchd / Windows Service) |
| `lapdog service uninstall` | Unregisters background system service |
| `lapdog uninstall` | Uninstalls lap-dog, stops background service, and removes binary (`--purge` removes `~/.lapdog`) |
| `lapdog version` | Displays version, commit hash, build date, and platform architecture |

---

## 🗑️ Uninstalling lap-dog

To completely uninstall `lap-dog`, stop its background service, and remove the binary executable:

### Option 1: Built-in CLI Command
```bash
# Uninstall lap-dog service and binary
lapdog uninstall

# Or uninstall and purge configuration & stored database state (~/.lapdog)
lapdog uninstall --purge
```

### Option 2: One-Line Uninstall Script
- **Linux & macOS**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/mochiron-desu/lap-dog/main/scripts/uninstall.sh | bash
  ```
  *(Add `--purge` to script arguments to also remove config & database history)*

- **Windows (PowerShell)**:
  ```powershell
  irm https://raw.githubusercontent.com/mochiron-desu/lap-dog/main/scripts/uninstall.ps1 | iex
  ```
  *(Pass `-Purge` parameter to also remove config & database history)*

### Option 3: From Source (Makefile)
```bash
make uninstall
```

---

## 🤖 Telegram Bot Commands

Send commands directly to your Telegram bot:
- `/away` - Arm monitoring mode (alerts ON)
- `/back` - Disarm monitoring mode (suppresses non-critical alerts)
- `/status` - Query current agent state, pending queue, and uptime
- `/update` - Check for latest release & receive platform-specific upgrade commands
- `/test` - Trigger test alert notification
- `/help` - Show command reference

---

## 🔄 Checking for Updates & Upgrading

`lap-dog` includes built-in update checking and notification mechanisms:

1. **Automatic Background Notifications**: While running in the background, `lap-dog` checks for newly published releases every 12 hours. When an update is available, it sends a Telegram alert containing the exact upgrade command for your laptop's operating system.
2. **On-Demand Telegram Check**: Send `/update` to your Telegram bot anytime to check your current version against GitHub releases.
3. **On-Demand CLI Check**: Run `lapdog update` from the terminal.

### 💻 Platform Upgrade One-Liners

- **Linux & macOS (Bash)**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/mochiron-desu/lap-dog/main/scripts/install.sh | bash
  ```
- **Windows (PowerShell)**:
  ```powershell
  irm https://raw.githubusercontent.com/mochiron-desu/lap-dog/main/scripts/install.ps1 | iex
  ```

---

## 🔒 Security Architecture & Least Privilege

`lap-dog` is built with a zero-trust, privacy-focused security model:
1. **No Root Requirement**: Uses Linux `systemd-journal` group permissions to read authentication events without granting full root/sudo access:
   ```bash
   sudo usermod -aG systemd-journal,adm $USER
   ```
2. **Systemd Sandboxing**: The installed systemd unit uses `NoNewPrivileges=true` and `ProtectSystem=strict` to prevent process privilege escalation or unauthorized file modifications.
3. **100% Open Source**: Written in pure Go with **zero CGO dependencies** and static compilation.

---

## 💻 Developer Guide (Building from Source)

If you have Go 1.24+ installed and wish to build locally:

```bash
# Local build
make build

# Build cross-platform matrix & generate checksums
make checksums

# Run tests
make test
```
