#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Install MCP Memory Server on Windows
.DESCRIPTION
    Downloads and installs the MCP Memory Server for Windows
.PARAMETER Service
    Install as a Windows service
.PARAMETER InstallDir
    Installation directory (default: $env:LOCALAPPDATA\mcp-memory)
#>

param(
    [switch]$Service,
    [string]$InstallDir = "$env:LOCALAPPDATA\mcp-memory"
)

$ErrorActionPreference = "Stop"

$REPO = "coah80/mcp-memory"
$VERSION = "1.0.0"
$CONFIG_DIR = "$env:APPDATA\mcp-memory"

function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Log($message) {
    Write-ColorOutput Green "[MCP Memory] $message"
}

function Error($message) {
    Write-ColorOutput Red "[MCP Memory] $message"
    exit 1
}

function Warn($message) {
    Write-ColorOutput Yellow "[MCP Memory] $message"
}

# Detect architecture
function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { Error "Unsupported architecture: $arch" }
    }
}

# Download file
function Download-File($url, $output) {
    Log "Downloading $url..."
    try {
        Invoke-WebRequest -Uri $url -OutFile $output -UseBasicParsing
    } catch {
        Error "Failed to download: $_"
    }
}

# Install binary
function Install-Binary($arch) {
    $binaryName = "mcp-memory-windows-$arch.exe"
    $downloadUrl = "https://mcps.coah80.com/mcp-memory/$binaryName"
    $installPath = Join-Path $InstallDir "mcp-memory.exe"
    
    # Create install directory
    if (!(Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # Download binary
    Download-File $downloadUrl $installPath
    
    Log "Binary installed to $installPath"
}

# Setup configuration
function Setup-Config {
    Log "Setting up configuration..."
    
    if (!(Test-Path $CONFIG_DIR)) {
        New-Item -ItemType Directory -Path $CONFIG_DIR -Force | Out-Null
    }
    
    $configPath = Join-Path $CONFIG_DIR "config.json"
    if (!(Test-Path $configPath)) {
        $memoryDir = "$env:USERPROFILE\.mcp-memory\memories"
        $config = @{
            memory_dir = $memoryDir
            port = 8090
            host = "localhost"
        } | ConvertTo-Json
        
        Set-Content -Path $configPath -Value $config
        Log "Created default configuration at $configPath"
    } else {
        Warn "Configuration already exists at $configPath"
    }
}

# Add to PATH
function Add-ToPath {
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$InstallDir*") {
        $newPath = "$currentPath;$InstallDir"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = $newPath
        Log "Added $InstallDir to PATH"
    }
}

# Install as Windows service
function Install-Service {
    Log "Installing Windows service..."
    
    # Check if NSSM is available
    $nssmPath = Get-Command nssm -ErrorAction SilentlyContinue
    if (!$nssmPath) {
        Warn "NSSM not found. Downloading NSSM..."
        $nssmUrl = "https://nssm.cc/release/nssm-2.24.zip"
        $nssmZip = "$env:TEMP\nssm.zip"
        $nssmDir = "$env:TEMP\nssm"
        
        Download-File $nssmUrl $nssmZip
        Expand-Archive -Path $nssmZip -DestinationPath $nssmDir -Force
        
        $nssmPath = Join-Path $nssmDir "nssm-2.24\win64\nssm.exe"
    }
    
    $binaryPath = Join-Path $InstallDir "mcp-memory.exe"
    $memoryDir = "$env:USERPROFILE\.mcp-memory\memories"
    
    # Install service
    & $nssmPath install MCPMemory $binaryPath
    & $nssmPath set MCPMemory AppEnvironmentExtra "MEMORY_DIR=$memoryDir" "PORT=8090"
    & $nssmPath set MCPMemory Start SERVICE_AUTO_START
    & $nssmPath start MCPMemory
    
    Log "Service installed and started"
}

# Test installation
function Test-Installation {
    Log "Testing installation..."
    
    # Wait a moment for the service to start
    Start-Sleep -Seconds 2
    
    # Test health endpoint
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8090/health" -UseBasicParsing
        if ($response.Content -like "*ok*") {
            Log "✓ MCP Memory server is running"
        } else {
            Warn "Server may not be running properly"
        }
    } catch {
        Warn "Could not connect to server: $_"
    }
}

# Main installation
function Main {
    Log "Installing MCP Memory Server..."
    
    # Detect architecture
    $arch = Get-Architecture
    Log "Detected architecture: $arch"
    
    # Install binary
    Install-Binary $arch
    
    # Setup configuration
    Setup-Config
    
    # Add to PATH
    Add-ToPath
    
    # Install service if requested
    if ($Service) {
        Install-Service
    } else {
        Log "Skipping service installation (use -Service to install as service)"
    }
    
    # Test installation
    Test-Installation
    
    Log "Installation complete!"
    Log ""
    Log "Usage:"
    Log "  Start server: $InstallDir\mcp-memory.exe"
    Log "  Health check: Invoke-WebRequest http://localhost:8090/health"
    Log "  List memories: Invoke-WebRequest http://localhost:8090/memories"
    Log "  Get memory: Invoke-WebRequest http://localhost:8090/memories/<name>"
    Log ""
    Log "Configuration: $CONFIG_DIR\config.json"
    Log "Memory directory: $env:USERPROFILE\.mcp-memory\memories"
}

Main
