#!/usr/bin/env bash
set -e

REPO="${LAPDOG_REPO:-mochiron-desu/lap-dog}"
VERSION="${LAPDOG_VERSION:-latest}"

echo "🐕 Installing lap-dog Security Agent..."

# Detect OS
UNAME_OS="$(uname -s)"
case "${UNAME_OS}" in
    Linux*)     OS='linux';;
    Darwin*)    OS='darwin';;
    *)          echo "❌ Unsupported operating system: ${UNAME_OS}"; exit 1;;
esac

# Detect Architecture
UNAME_ARCH="$(uname -m)"
case "${UNAME_ARCH}" in
    x86_64|amd64)   ARCH='amd64';;
    aarch64|arm64)  ARCH='arm64';;
    *)              echo "❌ Unsupported architecture: ${UNAME_ARCH}"; exit 1;;
esac

BINARY_NAME="lapdog-${OS}-${ARCH}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

DOWNLOAD_SUCCESS=false

# Attempt prebuilt binary download from GitHub Releases
echo "🔍 Detecting platform: ${OS}/${ARCH}"
if command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1; then
    if [ "${VERSION}" = "latest" ]; then
        RELEASE_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
        CHECKSUM_URL="https://github.com/${REPO}/releases/latest/download/checksums.txt"
    else
        RELEASE_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"
        CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
    fi

    echo "📦 Downloading prebuilt binary: ${BINARY_NAME}..."

    FETCH_CMD=""
    if command -v curl >/dev/null 2>&1; then
        FETCH_CMD="curl -fsSL"
    else
        FETCH_CMD="wget -qO-"
    fi

    if ${FETCH_CMD} "${RELEASE_URL}" > "${TMP_DIR}/lapdog" 2>/dev/null; then
        DOWNLOAD_SUCCESS=true
        echo "✓ Binary downloaded successfully."

        # Attempt SHA256 Checksum Verification
        if ${FETCH_CMD} "${CHECKSUM_URL}" > "${TMP_DIR}/checksums.txt" 2>/dev/null; then
            EXPECTED_SHA=$(grep "${BINARY_NAME}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
            if [ -n "${EXPECTED_SHA}" ]; then
                if command -v sha256sum >/dev/null 2>&1; then
                    ACTUAL_SHA=$(sha256sum "${TMP_DIR}/lapdog" | awk '{print $1}')
                elif command -v shasum >/dev/null 2>&1; then
                    ACTUAL_SHA=$(shasum -a 256 "${TMP_DIR}/lapdog" | awk '{print $1}')
                else
                    ACTUAL_SHA=""
                fi

                if [ -n "${ACTUAL_SHA}" ]; then
                    if [ "${EXPECTED_SHA}" = "${ACTUAL_SHA}" ]; then
                        echo "🔒 Checksum verified: SHA256 matches (${EXPECTED_SHA:0:12}...)"
                    else
                        echo "⚠️ SHA256 mismatch! Download may be corrupted."
                        exit 1
                    fi
                fi
            fi
        fi
    fi
fi

# Fallback to local compilation if download failed or skipped
if [ "${DOWNLOAD_SUCCESS}" = false ]; then
    echo "⚠️ Prebuilt release binary not found or network restricted. Attempting local Go compilation..."
    export PATH="$PATH:$HOME/.local/go/bin:/usr/local/go/bin"
    GO="${GO:-$(command -v go || echo "$HOME/.local/go/bin/go")}"

    if command -v "$GO" >/dev/null 2>&1 || [ -x "$GO" ]; then
        if [ -f "main.go" ]; then
            "$GO" build -o "${TMP_DIR}/lapdog" main.go
            echo "✓ Local build complete."
        else
            echo "❌ Error: Could not download prebuilt binary and 'main.go' was not found in current directory."
            echo "   Please check your network connection or specify LAPDOG_REPO/LAPDOG_VERSION."
            exit 1
        fi
    else
        echo "❌ Go compiler not found. Please install Go or check release repository accessibility."
        exit 1
    fi
fi

# Determine destination directory
if [ -n "${INSTALL_DIR}" ]; then
    TARGET_DIR="${INSTALL_DIR}"
elif [ "${EUID}" -eq 0 ]; then
    TARGET_DIR="/usr/local/bin"
else
    TARGET_DIR="$HOME/.local/bin"
fi

mkdir -p "${TARGET_DIR}"
cp "${TMP_DIR}/lapdog" "${TARGET_DIR}/lapdog"
chmod +x "${TARGET_DIR}/lapdog"

echo "✅ lapdog binary installed to ${TARGET_DIR}/lapdog"

# Ensure target directory is in PATH
if [[ ":$PATH:" != *":${TARGET_DIR}:"* ]]; then
    echo "💡 Note: Add ${TARGET_DIR} to your PATH if not already present:"
    echo "   export PATH=\"\$PATH:${TARGET_DIR}\""
fi

# Create default configuration directory if needed
CONFIG_DIR="$HOME/.lapdog"
mkdir -p "$CONFIG_DIR"

if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    if [ -f "config.example.yaml" ]; then
        cp config.example.yaml "$CONFIG_DIR/config.yaml"
    else
        cat << 'EOF' > "$CONFIG_DIR/config.yaml"
telegram_bot_token: ""
telegram_chat_id: 0
poll_interval_seconds: 3
away_mode: true
storage_path: "~/.lapdog/lapdog_store.json"
log_level: "info"
failed_login_threshold: 3
failed_login_window_seconds: 120
rate_limit_per_min: 5
mock_mode: false
EOF
    fi
    echo "📋 Created default configuration at $CONFIG_DIR/config.yaml"
fi

echo ""
# Launch interactive setup wizard if running interactively
if [ -t 0 ]; then
    read -p "Would you like to run interactive setup now? (Y/n): " RUN_SETUP
    RUN_SETUP=$(echo "${RUN_SETUP}" | tr '[:upper:]' '[:lower:]')
    if [ "${RUN_SETUP}" != "n" ] && [ "${RUN_SETUP}" != "no" ]; then
        "${TARGET_DIR}/lapdog" setup
        exit 0
    fi
fi

echo "Next Steps:"
echo "1. Run '${TARGET_DIR}/lapdog setup' for interactive setup."
echo "2. Or edit $CONFIG_DIR/config.yaml and run '${TARGET_DIR}/lapdog run'."
