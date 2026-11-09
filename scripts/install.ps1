# lap-dog Security Agent Windows PowerShell Installer
# Usage: irm https://raw.githubusercontent.com/mochiron-desu/lap-dog/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = if ($env:LAPDOG_REPO) { $env:LAPDOG_REPO } else { "mochiron-desu/lap-dog" }
$Version = if ($env:LAPDOG_VERSION) { $env:LAPDOG_VERSION } else { "latest" }

Write-Host "🐕 Installing lap-dog Security Agent..." -ForegroundColor Cyan

# Detect Architecture
$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$BinaryName = "lapdog-windows-$Arch.exe"

$InstallDir = "$env:LOCALAPPDATA\lapdog\bin"
$ExePath = Join-Path $InstallDir "lapdog.exe"

# Determine Download URL
if ($Version -eq "latest") {
    $DownloadUrl = "https://github.com/$Repo/releases/latest/download/$BinaryName"
    $ChecksumUrl = "https://github.com/$Repo/releases/latest/download/checksums.txt"
} else {
    $DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$BinaryName"
    $ChecksumUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"
}

if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

Write-Host "📦 Downloading prebuilt binary for Windows/$Arch..." -ForegroundColor Yellow
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ExePath -UseBasicParsing
    Write-Host "✓ Binary downloaded to $ExePath" -ForegroundColor Green
} catch {
    Write-Host "❌ Failed to download prebuilt release binary from GitHub: $_" -ForegroundColor Red
    Write-Host "Please check repository accessibility ($Repo) or specify LAPDOG_VERSION." -ForegroundColor Red
    exit 1
}

# Verify Checksum if available
try {
    $TempChecksum = Join-Path $env:TEMP "lapdog_checksums.txt"
    Invoke-WebRequest -Uri $ChecksumUrl -OutFile $TempChecksum -UseBasicParsing
    if (Test-Path $TempChecksum) {
        $ChecksumLines = Get-Content $TempChecksum
        $MatchingLine = $ChecksumLines | Where-Object { $_ -match $BinaryName }
        if ($MatchingLine) {
            $ExpectedHash = ($MatchingLine -split '\s+')[0]
            $ActualHash = (Get-FileHash -Path $ExePath -Algorithm SHA256).Hash.ToLower()
            if ($ExpectedHash.ToLower() -eq $ActualHash) {
                Write-Host "🔒 SHA256 Checksum Verified successfully!" -ForegroundColor Green
            } else {
                Write-Host "⚠️ Checksum mismatch! Downloaded binary may be corrupted." -ForegroundColor Yellow
            }
        }
    }
} catch {
    # Non-fatal checksum download failure
}

# Add InstallDir to User PATH environment variable if absent
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = "$UserPath;$InstallDir"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "💡 Added $InstallDir to User PATH." -ForegroundColor Yellow
}

# Ensure Config Directory
$ConfigDir = "$env:USERPROFILE\.lapdog"
$ConfigFile = Join-Path $ConfigDir "config.yaml"
if (!(Test-Path $ConfigDir)) {
    New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
}

if (!(Test-Path $ConfigFile)) {
    $DefaultConfig = @"
telegram_bot_token: ""
telegram_chat_id: 0
poll_interval_seconds: 3
away_mode: true
storage_path: "$($ConfigDir.Replace('\', '/'))/lapdog_store.json"
log_level: "info"
failed_login_threshold: 3
failed_login_window_seconds: 120
rate_limit_per_min: 5
mock_mode: false
"@
    Set-Content -Path $ConfigFile -Value $DefaultConfig -Encoding UTF8
    Write-Host "📋 Created default configuration at $ConfigFile" -ForegroundColor Green
}

Write-Host ""
Write-Host "✅ lap-dog installed successfully!" -ForegroundColor Green
Write-Host "Next Steps:" -ForegroundColor Cyan
Write-Host "1. Run 'lapdog setup' in PowerShell for interactive configuration & service setup."
Write-Host "2. Or configure $ConfigFile and run 'lapdog service install' for auto-start at login."
