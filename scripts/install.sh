#!/usr/bin/env bash
set -e

export PATH="$PATH:$HOME/.local/go/bin:/usr/local/go/bin"
GO="${GO:-$(command -v go || echo "$HOME/.local/go/bin/go")}"

echo "🐕 Installing lap-dog Security Agent..."

# Build binary
"$GO" build -o /tmp/lapdog main.go

# Install binary to /usr/local/bin (or ~/.local/bin if not root)
if [ "$EUID" -eq 0 ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

cp /tmp/lapdog "$INSTALL_DIR/lapdog"
chmod +x "$INSTALL_DIR/lapdog"

echo "✅ lapdog binary installed to $INSTALL_DIR/lapdog"

# Create default config directory
CONFIG_DIR="$HOME/.lapdog"
mkdir -p "$CONFIG_DIR"

if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    cp config.example.yaml "$CONFIG_DIR/config.yaml"
    echo "📋 Created default configuration at $CONFIG_DIR/config.yaml"
fi

echo ""
echo "Next Steps:"
echo "1. Edit $CONFIG_DIR/config.yaml to add your Telegram bot token & chat ID."
echo "2. Run 'lapdog run' to start monitoring."
echo "3. Run 'lapdog service install' to run automatically on system startup."
