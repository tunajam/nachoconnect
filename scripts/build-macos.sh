#!/usr/bin/env bash
# build-macos.sh — Build NachoConnect .dmg for macOS
#
# Prerequisites:
#   - Go 1.21+, Node.js 18+, Wails CLI
#   - create-dmg: brew install create-dmg
#   - libpcap (included with macOS)
#   - C compiler (Xcode CLT)
#
# Usage: bash scripts/build-macos.sh
# Output: dist/NachoConnect-0.1.0.dmg

set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="0.1.0"
APP_NAME="NachoConnect"

echo "=== NachoConnect macOS Build ==="

# Step 1: Build l2tunnel
echo "[1/3] Building l2tunnel..."
cd lib/l2tunnel
make clean
make l2tunnel
cd ../..

# Step 2: Build Wails app
echo "[2/3] Building with Wails..."
wails build -platform darwin/universal

APP_PATH="build/bin/${APP_NAME}.app"

if [ ! -d "$APP_PATH" ]; then
    echo "ERROR: $APP_PATH not found. Wails build may have failed."
    exit 1
fi

# Bundle l2tunnel inside the .app
echo "    Bundling l2tunnel binary..."
cp lib/l2tunnel/l2tunnel "$APP_PATH/Contents/MacOS/l2tunnel"

# Step 3: Create DMG
echo "[3/3] Creating DMG..."
mkdir -p dist

DMG_PATH="dist/${APP_NAME}-${VERSION}.dmg"

# Remove old DMG if exists
rm -f "$DMG_PATH"

if command -v create-dmg &>/dev/null; then
    create-dmg \
        --volname "${APP_NAME}" \
        --volicon "build/appicon.png" \
        --window-pos 200 120 \
        --window-size 600 400 \
        --icon-size 100 \
        --icon "${APP_NAME}.app" 175 190 \
        --hide-extension "${APP_NAME}.app" \
        --app-drop-link 425 190 \
        "$DMG_PATH" \
        "$APP_PATH"
else
    echo "create-dmg not found, using hdiutil..."
    STAGING=$(mktemp -d)
    cp -R "$APP_PATH" "$STAGING/"
    ln -s /Applications "$STAGING/Applications"
    hdiutil create -volname "$APP_NAME" -srcfolder "$STAGING" -ov -format UDZO "$DMG_PATH"
    rm -rf "$STAGING"
fi

echo ""
echo "=== Done! ==="
echo "DMG: $DMG_PATH"
