# lap-dog: Cross-Platform Laptop Security Agent 🐕

**lap-dog** is a lightweight, cross-platform background security agent written in Go. It monitors laptop authentication and access events (logins, unlock attempts, remote sessions, USB insertions) across **Windows**, **Linux**, and **macOS**, sending instant alerts and interactive controls via a Telegram bot.

---

## 🌟 Features

- **Cross-Platform Event Sources**:
  - **Linux**: systemd-journald (`journalctl`), PAM logs (`/var/log/auth.log`), SSH logins, lock/unlock events.
  - **Windows**: Windows Security Event Log (ETW/PowerShell 4624 login, 4625 failed login, 4800/4801 lock/unlock, 4778 RDP session).
  - **macOS**: Unified Logging (`os_log` stream for `loginwindow`, `sshd`, `sudo`, `authorizationd`).
  - **Mock Simulator**: Built-in test event generator for unprivileged testing.
- **Event Normalization**: All native logs map to standard `SecurityEvent` schema (`LOGIN_SUCCESS`, `LOGIN_FAILURE`, `UNLOCK`, `REMOTE_LOGIN`, `USB_INSERT`).
- **Rule & Escalation Engine**:
  - **Away Mode Toggle**: Suppresses non-critical alerts when user is at the machine; fires alerts when Away Mode is ARMED.
  - **Brute Force Escalation**: Automatically escalates multiple failed logins in a short window to `CRITICAL` severity.
  - **Rate Limiting & Deduplication**: Throttles alert bursts and discards duplicate events.
- **Offline-First Storage Queue**: Local JSON/queue store ensures zero alert loss during network drops with automatic retry backoff.
- **Telegram Bot Integration**:
  - HTML alerts with emoji indicators (🔴, 🔒, ⚠️).
  - Interactive bot commands: `/away` (Arm), `/back` (Disarm), `/status` (Metrics), `/test`, `/help`.
- **Background Daemon & Service Installer**: Built-in `lapdog service install` for systemd, launchd, and Windows Services.

---

## 🚀 Quick Start & Setup Guide

### 1. Build from Source
```bash
export PATH=$PATH:$HOME/.local/go/bin
make build
# Binary created at bin/lapdog
```

### 2. Configure Telegram Credentials
1. Open Telegram and search for `@BotFather`.
2. Send `/newbot`, follow prompts, and copy the **HTTP API Token**.
3. Search for your new bot in Telegram and click **START** (or send `/start`).
4. Search for `@userinfobot` to get your Telegram **Chat ID**.
5. Copy default config and edit:
```bash
mkdir -p ~/.lapdog
cp config.example.yaml ~/.lapdog/config.yaml
nano ~/.lapdog/config.yaml
```
Set `telegram_bot_token` and `telegram_chat_id`.

### 3. Linux Permissions & Security (Least Privilege)
To allow `lap-dog` to monitor Linux authentication logs **without needing `sudo` or root privileges**:
```bash
sudo usermod -aG systemd-journal,adm $USER
```
*(Log out and back in for group permissions to take effect).*

### 4. Test Notification
```bash
./bin/lapdog test
./bin/lapdog run --mock
```

### 5. Install Background Service (Auto-Start on Boot)
```bash
./bin/lapdog service install
systemctl --user enable --now lapdog
```

---

## 🛠️ CLI Commands

| Command | Description |
| :--- | :--- |
| `lapdog run` | Starts the security daemon (use `--mock` for simulator mode) |
| `lapdog away` | Arms Away Mode (alerts enabled for access events) |
| `lapdog back` | Disarms Away Mode (suppresses non-critical alerts) |
| `lapdog status` | Displays dashboard status and queue metrics |
| `lapdog test` | Enqueues and sends a test alert notification |
| `lapdog simulate` | Injects synthetic test events (`--event LOGIN_FAILURE --user attacker`) |
| `lapdog service install` | Registers lap-dog as a background system service |
| `lapdog service uninstall` | Unregisters system service |
| `lapdog version` | Shows version information |

---

## 🤖 Telegram Bot Commands

Send commands directly to your Telegram bot:
- `/away` - Arm monitoring mode
- `/back` - Disarm monitoring mode
- `/status` - Query current agent state, pending queue, and uptime
- `/test` - Trigger test alert
- `/help` - Show command reference

---

## 🔒 Security Architecture & Least Privilege

`lap-dog` is built with a zero-trust, privacy-focused security model:
1. **No Root Requirement**: Uses Linux `systemd-journal` group permissions to read authentication events without granting full root/sudo access.
2. **Systemd Sandboxing**: The installed systemd unit uses `NoNewPrivileges=true` and `ProtectSystem=strict` to prevent process privilege escalation or unauthorized file modifications.
3. **100% Open Source**: Written in pure Go with **zero CGO dependencies** and no dynamic binary blobs.

---

## 📦 Building Cross-Platform Release Binaries

To cross-compile binaries for all supported platforms without CGO dependencies:
```bash
make cross-build
```

Generated binaries in `bin/`:
- `lapdog-linux-amd64`
- `lapdog-linux-arm64`
- `lapdog-windows-amd64.exe`
- `lapdog-darwin-amd64`
- `lapdog-darwin-arm64`

---

## 🧪 Testing

Run the full Go unit test suite:
```bash
make test
```
