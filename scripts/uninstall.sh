#!/usr/bin/env bash
set -e

PURGE=false
FORCE=false

for arg in "$@"; do
    case $arg in
        --purge)
            PURGE=true
            shift
            ;;
        -y|--force)
            FORCE=true
            shift
            ;;
    esac
done

echo "🐕 Uninstalling lap-dog Security Agent..."

# Detect OS
UNAME_OS="$(uname -s)"
case "${UNAME_OS}" in
    Linux*)     OS='linux';;
    Darwin*)    OS='darwin';;
    *)          OS='unknown';;
esac

# 1. Stop and remove background services
echo "⏳ Teardown background service..."
if [ "${OS}" = "linux" ]; then
    if command -v systemctl >/dev/null 2>&1; then
        systemctl --user stop lapdog 2>/dev/null || true
        systemctl --user disable lapdog 2>/dev/null || true
        if [ -f "$HOME/.config/systemd/user/lapdog.service" ]; then
            rm -f "$HOME/.config/systemd/user/lapdog.service"
            systemctl --user daemon-reload 2>/dev/null || true
            echo "✓ Linux user systemd service removed."
        fi

        if [ "${EUID}" -eq 0 ] || command -v sudo >/dev/null 2>&1; then
            if [ -f "/etc/systemd/system/lapdog.service" ]; then
                sudo systemctl stop lapdog 2>/dev/null || true
                sudo systemctl disable lapdog 2>/dev/null || true
                sudo rm -f "/etc/systemd/system/lapdog.service"
                sudo systemctl daemon-reload 2>/dev/null || true
                echo "✓ Linux system systemd service removed."
            fi
        fi
    fi
elif [ "${OS}" = "darwin" ]; then
    PLIST="$HOME/Library/LaunchAgents/com.lapdog.agent.plist"
    if [ -f "$PLIST" ]; then
        launchctl unload "$PLIST" 2>/dev/null || true
        rm -f "$PLIST"
        echo "✓ macOS LaunchAgent removed."
    fi
fi

# 2. Locate and remove binary executable
BIN_LOCATIONS=(
    "${INSTALL_DIR}/lapdog"
    "/usr/local/bin/lapdog"
    "$HOME/.local/bin/lapdog"
)

# Also check `which lapdog` if present
if command -v lapdog >/dev/null 2>&1; then
    WHICH_LAPDOG="$(command -v lapdog)"
    BIN_LOCATIONS+=("${WHICH_LAPDOG}")
fi

REMOVED_BIN=false
for BIN in "${BIN_LOCATIONS[@]}"; do
    if [ -n "${BIN}" ] && [ -f "${BIN}" ]; then
        if [ -w "$(dirname "${BIN}")" ]; then
            rm -f "${BIN}"
            echo "✓ Removed binary at ${BIN}"
            REMOVED_BIN=true
        elif command -v sudo >/dev/null 2>&1; then
            sudo rm -f "${BIN}"
            echo "✓ Removed binary at ${BIN} (via sudo)"
            REMOVED_BIN=true
        fi
    fi
done

if [ "${REMOVED_BIN}" = false ]; then
    echo "ℹ️ No installed lapdog binary executable found."
fi

# 3. Handle Configuration & Storage Purge
CONFIG_DIR="$HOME/.lapdog"
if [ "${PURGE}" = false ] && [ "${FORCE}" = false ] && [ -t 0 ]; then
    read -p "Do you also want to remove configuration & database history (~/.lapdog)? (y/N): " RUN_PURGE
    RUN_PURGE=$(echo "${RUN_PURGE}" | tr '[:upper:]' '[:lower:]')
    if [ "${RUN_PURGE}" = "y" ] || [ "${RUN_PURGE}" = "yes" ]; then
        PURGE=true
    fi
fi

if [ "${PURGE}" = true ]; then
    if [ -d "$CONFIG_DIR" ]; then
        rm -rf "$CONFIG_DIR"
        echo "✓ Removed configuration directory at $CONFIG_DIR"
    fi
else
    echo "ℹ️ Preserving configuration at $CONFIG_DIR (Pass --purge to remove)."
fi

echo ""
echo "✅ lap-dog has been completely uninstalled!"
