# build-installer.ps1 — Build NachoConnect Windows installer
# Run on Windows with Wails, Go, NSIS, MinGW-w64, and npm installed
#
# Usage: .\scripts\build-installer.ps1
# Output: dist\NachoConnect-Setup-0.1.0.exe

$ErrorActionPreference = "Stop"
$Version = "0.1.0"
$NpcapVersion = "1.80"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Set-Location $ProjectRoot

Write-Host "=== NachoConnect Installer Build ===" -ForegroundColor Cyan

# Step 1: Check Npcap installer
$NpcapDest = "build\bin\npcap-installer.exe"
if (!(Test-Path $NpcapDest)) {
    Write-Host "ERROR: Npcap installer not found at $NpcapDest" -ForegroundColor Red
    Write-Host "Download from https://npcap.com/#download and place it there." -ForegroundColor Red
    exit 1
} else {
    Write-Host "[1/4] Npcap installer found" -ForegroundColor Green
}

# Step 2: Build l2tunnel
Write-Host "[2/4] Building l2tunnel.exe..." -ForegroundColor Yellow
Push-Location "lib\l2tunnel"
make l2tunnel.exe
Copy-Item "l2tunnel.exe" "..\..\build\bin\l2tunnel.exe" -Force
Pop-Location

# Step 3: Build with Wails (includes NSIS)
Write-Host "[3/4] Building NachoConnect with Wails + NSIS..." -ForegroundColor Yellow
wails build -platform windows/amd64 -nsis

# Step 4: Copy to dist
Write-Host "[4/4] Packaging..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path "dist" | Out-Null
$InstallerSrc = "build\bin\nachoconnect-amd64-installer.exe"
$InstallerDst = "dist\NachoConnect-Setup-${Version}.exe"
Copy-Item $InstallerSrc $InstallerDst -Force

Write-Host ""
Write-Host "=== Done! ===" -ForegroundColor Green
Write-Host "Installer: $InstallerDst"
