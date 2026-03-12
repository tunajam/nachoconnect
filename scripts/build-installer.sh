#!/usr/bin/env bash
# build-installer.sh — Cross-platform build helper
# NOTE: The Windows NSIS installer must be built ON Windows (Wails doesn't cross-compile NSIS).
#
# This script documents the process and can be run on Windows via Git Bash / WSL.
#
# Prerequisites (Windows):
#   - Go 1.21+
#   - Node.js 18+
#   - Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest
#   - NSIS: choco install nsis  (or https://nsis.sourceforge.io)
#   - Npcap installer in build/bin/npcap-installer.exe (download from https://npcap.com/#download)
#   - MinGW-w64 for cross-compiling l2tunnel (x86_64-w64-mingw32-gcc)
#   - Npcap SDK in lib/l2tunnel/npcap/ (download from https://npcap.com/#download)
#
# Usage: bash scripts/build-installer.sh
# Output: dist/NachoConnect-Setup-0.1.0.exe

set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="0.1.0"
NPCAP_FILE="build/bin/npcap-installer.exe"

echo "=== NachoConnect Windows Installer Build ==="

# Check npcap
if [ ! -f "$NPCAP_FILE" ]; then
    echo "ERROR: Npcap installer not found at $NPCAP_FILE"
    echo "Download from https://npcap.com/#download and place it there."
    exit 1
fi

# Step 1: Build l2tunnel
echo "[1/3] Building l2tunnel..."
cd lib/l2tunnel
make l2tunnel.exe
cp l2tunnel.exe ../../build/bin/l2tunnel.exe
cd ../..

# Step 2: Build with Wails + NSIS
echo "[2/3] Building NachoConnect with Wails + NSIS..."
wails build -platform windows/amd64 -nsis

# Step 3: Package
echo "[3/3] Copying to dist/..."
mkdir -p dist
cp "build/bin/nachoconnect-amd64-installer.exe" "dist/NachoConnect-Setup-${VERSION}.exe"

echo ""
echo "=== Done! ==="
echo "Installer: dist/NachoConnect-Setup-${VERSION}.exe"
