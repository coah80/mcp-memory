#!/bin/bash

set -e

REPO="coah80/mcp-memory"
INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/mcp-memory"
SERVICE_NAME="mcp-memory"
MEMORY_DIR="$HOME/.mcp-memory/memories"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[MCP Memory]${NC} $1"
}

error() {
    echo -e "${RED}[MCP Memory]${NC} $1"
    exit 1
}

warn() {
    echo -e "${YELLOW}[MCP Memory]${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac
    
    case $OS in
        linux|darwin)
            ;;
        *)
            error "Unsupported OS: $OS"
            ;;
    esac
    
    echo "${OS}-${ARCH}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install binary
install_binary() {
    local platform=$1
    local binary_name="mcp-memory-${platform}"
    local download_url="https://mcps.coah80.com/mcp-memory/${binary_name}"
    
    log "Downloading MCP Memory server..."
    
    # Create install directory
    mkdir -p "$INSTALL_DIR"
    
    # Download binary
    if command_exists curl; then
        curl -sL "$download_url" -o "$INSTALL_DIR/mcp-memory"
    elif command_exists wget; then
        wget -q "$download_url" -O "$INSTALL_DIR/mcp-memory"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
    
    # Make executable
    chmod +x "$INSTALL_DIR/mcp-memory"
    
    log "Binary installed to $INSTALL_DIR/mcp-memory"
}

# Setup configuration
setup_config() {
    log "Setting up configuration..."
    
    mkdir -p "$CONFIG_DIR"
    
    # Create default config if it doesn't exist
    if [ ! -f "$CONFIG_DIR/config.json" ]; then
        cat > "$CONFIG_DIR/config.json" << EOF
{
    "memory_dir": "$MEMORY_DIR",
    "port": 8090,
    "host": "localhost"
}
EOF
        log "Created default configuration at $CONFIG_DIR/config.json"
    else
        warn "Configuration already exists at $CONFIG_DIR/config.json"
    fi
    
    # Ensure memory directory exists
    mkdir -p "$MEMORY_DIR"
}

# Install systemd service (Linux only)
install_service() {
    if [ "$OS" != "linux" ]; then
        return
    fi
    
    if ! command_exists systemctl; then
        warn "systemctl not found, skipping service installation"
        return
    fi
    
    log "Installing systemd service..."
    
    sudo tee /etc/systemd/system/${SERVICE_NAME}.service > /dev/null << EOF
[Unit]
Description=MCP Memory Server
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=$INSTALL_DIR/mcp-memory
Restart=on-failure
RestartSec=5
Environment=MEMORY_DIR=$HOME/.mcp-memory/memories
Environment=PORT=8090

[Install]
WantedBy=multi-user.target
EOF
    
    sudo systemctl daemon-reload
    sudo systemctl enable ${SERVICE_NAME}
    sudo systemctl start ${SERVICE_NAME}
    
    log "Service installed and started"
}

# Install launchd service (macOS only)
install_launchd() {
    if [ "$OS" != "darwin" ]; then
        return
    fi
    
    log "Installing launchd service..."
    
    local plist_path="$HOME/Library/LaunchAgents/com.coah80.mcp-memory.plist"
    
    cat > "$plist_path" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.coah80.mcp-memory</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/mcp-memory</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>EnvironmentVariables</key>
    <dict>
        <key>MEMORY_DIR</key>
        <string>$HOME/.mcp-memory/memories</string>
        <key>PORT</key>
        <string>8090</string>
    </dict>
</dict>
</plist>
EOF
    
    launchctl load "$plist_path"
    launchctl start com.coah80.mcp-memory
    
    log "Launchd service installed and started"
}

# Add to PATH
add_to_path() {
    local shell_rc=""
    
    case $SHELL in
        */bash)
            shell_rc="$HOME/.bashrc"
            ;;
        */zsh)
            shell_rc="$HOME/.zshrc"
            ;;
        */fish)
            shell_rc="$HOME/.config/fish/config.fish"
            ;;
        *)
            warn "Unknown shell: $SHELL"
            return
            ;;
    esac
    
    if [ -n "$shell_rc" ]; then
        if ! grep -q "$INSTALL_DIR" "$shell_rc" 2>/dev/null; then
            echo "" >> "$shell_rc"
            echo "# MCP Memory" >> "$shell_rc"
            echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$shell_rc"
            log "Added $INSTALL_DIR to PATH in $shell_rc"
            warn "Please run 'source $shell_rc' or restart your shell"
        fi
    fi
}

# Test installation
test_installation() {
    log "Testing installation..."
    
    # Wait a moment for the service to start
    sleep 2
    
    # Test health endpoint
    if command_exists curl; then
        if curl -s http://localhost:8090/health | grep -q "ok"; then
            log "✓ MCP Memory server is running"
        else
            warn "Server may not be running. Check with: systemctl status $SERVICE_NAME (Linux) or launchctl list | grep mcp-memory (macOS)"
        fi
    fi
}

# Main installation
main() {
    log "Installing MCP Memory Server..."
    
    # Detect platform
    PLATFORM=$(detect_platform)
    log "Detected platform: $PLATFORM"
    
    # Install binary
    install_binary "$PLATFORM"
    
    # Setup configuration
    setup_config
    
    # Add to PATH
    add_to_path
    
    # Install service
    if [ "$1" = "--service" ]; then
        if [ "$OS" = "linux" ]; then
            install_service
        elif [ "$OS" = "darwin" ]; then
            install_launchd
        fi
    else
        log "Skipping service installation (use --service to install as service)"
    fi
    
    # Test installation
    test_installation
    
    log "Installation complete!"
    log ""
    log "Usage:"
    log "  Start server: $INSTALL_DIR/mcp-memory"
    log "  Health check: curl http://localhost:8090/health"
    log "  List memories: curl http://localhost:8090/memories"
    log "  Get memory: curl http://localhost:8090/memories/<name>"
    log ""
    log "Configuration: $CONFIG_DIR/config.json"
    log "Memory directory: $HOME/.config/opencode/memory"
}

main "$@"
