# lap-dog Security Agent Windows PowerShell Uninstaller
# Usage: irm https://raw.githubusercontent.com/mochiron-desu/lap-dog/main/scripts/uninstall.ps1 | iex

param (
    [Switch]$Purge,
    [Switch]$Force
)

$ErrorActionPreference = "Continue"

Write-Host "🐕 Uninstalling lap-dog Security Agent..." -ForegroundColor Cyan

# 1. Stop and remove Windows Service
Write-Host "⏳ Stopping and deleting Windows service..." -ForegroundColor Yellow
try {
    sc.exe stop lapdog 2>$null | Out-Null
    sc.exe delete lapdog 2>$null | Out-Null
    Write-Host "✓ Windows service 'lapdog' removed." -ForegroundColor Green
} catch {
    # Ignore if service wasn't installed
}

# 2. Locate and remove binary executable
$InstallDir = "$env:LOCALAPPDATA\lapdog\bin"
$ExePath = Join-Path $InstallDir "lapdog.exe"

if (Test-Path $ExePath) {
    try {
        Remove-Item -Path $ExePath -Force
        Write-Host "✓ Removed binary executable at $ExePath" -ForegroundColor Green
    } catch {
        Write-Host "⚠️ Could not immediately delete binary file $ExePath (may be locked by running process)." -ForegroundColor Yellow
    }
}

if ((Test-Path $InstallDir) -and (Get-ChildItem $InstallDir).Count -eq 0) {
    Remove-Item -Path $InstallDir -Force -Recurse 2>$null
}

# 3. Clean up User PATH environment variable
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -like "*$InstallDir*") {
    $NewPath = ($UserPath -split ';' | Where-Object { $_ -ne $InstallDir }) -join ';'
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = ($env:Path -split ';' | Where-Object { $_ -ne $InstallDir }) -join ';'
    Write-Host "✓ Removed $InstallDir from User PATH." -ForegroundColor Green
}

# 4. Handle Configuration & Storage Purge
$ConfigDir = "$env:USERPROFILE\.lapdog"
$ShouldPurge = $Purge.IsPresent

if (-not $Purge.IsPresent -and -not $Force.IsPresent -and [Environment]::UserInteractive) {
    $Response = Read-Host "Do you also want to remove configuration & database history ($ConfigDir)? (y/N)"
    if ($Response -eq 'y' -or $Response -eq 'yes') {
        $ShouldPurge = $true
    }
}

if ($ShouldPurge -and (Test-Path $ConfigDir)) {
    Remove-Item -Path $ConfigDir -Recurse -Force
    Write-Host "✓ Removed configuration directory at $ConfigDir" -ForegroundColor Green
} else {
    if (Test-Path $ConfigDir) {
        Write-Host "ℹ️ Preserving configuration at $ConfigDir (Pass -Purge switch to remove)." -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "✅ lap-dog has been uninstalled successfully!" -ForegroundColor Green
