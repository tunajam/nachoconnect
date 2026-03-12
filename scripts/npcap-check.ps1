# npcap-check.ps1 — Check if Npcap is installed; prompt to install if not
# Called by NachoConnect on first launch (Windows only)

$NpcapPath = "$env:SystemRoot\System32\Npcap"
$NpcapService = Get-Service -Name "npcap" -ErrorAction SilentlyContinue

if ((Test-Path "$NpcapPath\NPFInstall.exe") -or $NpcapService) {
    Write-Host "Npcap is installed."
    exit 0
}

Write-Host "Npcap is NOT installed. NachoConnect requires Npcap for Xbox detection." -ForegroundColor Yellow

# Check if bundled installer exists next to this script or in app directory
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$AppDir = Split-Path -Parent $ScriptDir
$BundledInstaller = @(
    "$AppDir\npcap-installer.exe",
    "$AppDir\redist\npcap-installer.exe",
    "$ScriptDir\npcap-installer.exe"
) | Where-Object { Test-Path $_ } | Select-Object -First 1

if ($BundledInstaller) {
    Write-Host "Found bundled Npcap installer: $BundledInstaller"
    $answer = Read-Host "Install Npcap now? (Y/n)"
    if ($answer -eq '' -or $answer -match '^[Yy]') {
        Write-Host "Installing Npcap..."
        Start-Process -FilePath $BundledInstaller -ArgumentList "/winpcap_mode=yes" -Wait -Verb RunAs
        Write-Host "Npcap installation complete."
        exit 0
    }
} else {
    Write-Host ""
    Write-Host "Please download and install Npcap from: https://npcap.com/dist/npcap-1.80.exe" -ForegroundColor Cyan
    Write-Host "After installing, restart NachoConnect."
    Start-Process "https://npcap.com/#download"
}

exit 1
