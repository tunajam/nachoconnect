# build-installer.ps1 — Build NachoConnect Windows installer
# Run on Windows with Wails, Go, NSIS, and npm installed
#
# Usage: .\scripts\build-installer.ps1
# Output: build\bin\nachoconnect-amd64-installer.exe → dist\NachoConnect-Setup-0.1.0.exe

$ErrorActionPreference = "Stop"
$Version = "0.1.0"
$NpcapVersion = "1.80"
$NpcapUrl = "https://npcap.com/dist/npcap-${NpcapVersion}-oem.exe"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Set-Location $ProjectRoot

Write-Host "=== NachoConnect Installer Build ===" -ForegroundColor Cyan

# Step 1: Download Npcap installer if not cached
$NpcapDest = "build\bin\npcap-installer.exe"
if (!(Test-Path $NpcapDest)) {
    Write-Host "[1/3] Downloading Npcap ${NpcapVersion}..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Force -Path "build\bin" | Out-Null
    # NOTE: Npcap OEM requires a license. For redistribution you need an OEM license from npcap.com.
    # For personal/testing use, download the free installer manually from https://npcap.com/#download
    # and place it at build\bin\npcap-installer.exe
    Write-Host "  !! Npcap OEM requires a license for redistribution." -ForegroundColor Red
    Write-Host "  !! Download the free installer from https://npcap.com/#download" -ForegroundColor Red
    Write-Host "  !! and save it as: $NpcapDest" -ForegroundColor Red
    if (!(Test-Path $NpcapDest)) {
        Write-Host "  Attempting download (may fail without OEM access)..."
        try {
            Invoke-WebRequest -Uri $NpcapUrl -OutFile $NpcapDest
        } catch {
            Write-Host "  Download failed. Please manually place npcap installer at $NpcapDest" -ForegroundColor Red
            exit 1
        }
    }
} else {
    Write-Host "[1/3] Npcap installer already cached" -ForegroundColor Green
}

# Step 2: Build with Wails (includes NSIS)
Write-Host "[2/3] Building NachoConnect with Wails + NSIS..." -ForegroundColor Yellow
wails build -platform windows/amd64 -nsis

# Step 3: Copy to dist
Write-Host "[3/3] Packaging..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path "dist" | Out-Null
$InstallerSrc = "build\bin\nachoconnect-amd64-installer.exe"
$InstallerDst = "dist\NachoConnect-Setup-${Version}.exe"
Copy-Item $InstallerSrc $InstallerDst -Force

Write-Host ""
Write-Host "=== Done! ===" -ForegroundColor Green
Write-Host "Installer: $InstallerDst"
