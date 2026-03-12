#!/bin/bash
# Build l2tunnel from source
# macOS: just needs libpcap (pre-installed)
# Linux: needs libpcap-dev
# Windows: needs mingw + npcap SDK

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
L2TUNNEL_DIR="$PROJECT_DIR/lib/l2tunnel"

if [ ! -d "$L2TUNNEL_DIR" ]; then
    echo "Error: l2tunnel submodule not found at $L2TUNNEL_DIR"
    echo "Run: git submodule update --init"
    exit 1
fi

cd "$L2TUNNEL_DIR"

case "$(uname -s)" in
    Darwin|Linux)
        echo "Building l2tunnel for $(uname -s)..."
        make
        echo "Built: $L2TUNNEL_DIR/l2tunnel"
        
        if [ "$(uname -s)" = "Darwin" ]; then
            echo ""
            echo "NOTE: l2tunnel needs root for raw network access."
            echo "Run: sudo chown root $L2TUNNEL_DIR/l2tunnel && sudo chmod u+s $L2TUNNEL_DIR/l2tunnel"
        fi
        ;;
    MINGW*|MSYS*|CYGWIN*)
        echo "Building l2tunnel for Windows..."
        if [ ! -d "npcap" ]; then
            echo "Downloading Npcap SDK..."
            curl -L -o npcap-sdk.zip https://nmap.org/npcap/dist/npcap-sdk-1.04.zip
            unzip -d npcap npcap-sdk.zip
            rm npcap-sdk.zip
        fi
        make l2tunnel.exe
        echo "Built: $L2TUNNEL_DIR/l2tunnel.exe"
        echo ""
        echo "NOTE: Install Npcap from https://nmap.org/npcap/ in WinPcap-compatible mode."
        ;;
    *)
        echo "Unsupported platform: $(uname -s)"
        exit 1
        ;;
esac
