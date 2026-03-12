# 🧀 NachoConnect

**Xbox system link, anywhere in the world.**

A modern desktop tunneling app for original Xbox system link play over the internet. Think Discord meets XBConnect — install, plug in your Xbox, click Play.

## What It Does

NachoConnect wraps [l2tunnel](https://github.com/mborgerson/l2tunnel) — a battle-tested C utility for Layer 2 traffic tunneling — with a slick desktop UI. It captures Xbox system link broadcast packets on your local network and tunnels them via UDP to a relay hub. Their Xboxes see your game as if everyone's in the same room.

## Features

- **🎮 Lobby System** — Create/join rooms with invite codes
- **🔍 Auto-Detect Xbox** — Automatically finds your Xbox on the network via l2tunnel discover
- **🌐 UDP Relay** — Hub server forwards packets between all peers
- **📊 Latency Display** — See ping to every player
- **💬 In-Lobby Chat** — Coordinate with your team
- **🎨 Modern UI** — Dark theme, gaming aesthetic

## Supported Games

All Xbox system link games work, including:
- Halo: CE & Halo 2
- MechAssault 1 & 2
- Crimson Skies
- Star Wars: Battlefront I/II
- Splinter Cell: Pandora Tomorrow / Chaos Theory
- Counter-Strike, TimeSplitters, and 80+ more

## Tech Stack

- **Go** — Backend, app bindings, hub relay server
- **Wails v2** — Desktop app framework (Go + Svelte)
- **[l2tunnel](https://github.com/mborgerson/l2tunnel)** — C utility for L2 packet capture & tunneling (libpcap)
- **Svelte** — Frontend UI

## Architecture

```
Xbox ←→ [l2tunnel (libpcap)] ←→ [UDP] ←→ [Hub Relay (Go)] ←→ [UDP] ←→ [l2tunnel] ←→ Xbox
                                                ↕
                                        [Lobby Server (Go)]
```

The Go app wraps l2tunnel as a subprocess:
- `l2tunnel list` — enumerate network interfaces
- `l2tunnel discover <iface>` — find Xbox MAC addresses on the wire
- `l2tunnel tunnel <iface> -s <mac> <local> <port> <remote> <port>` — tunnel L2 traffic

## Installation

### Windows
1. Download `NachoConnect-Setup-0.1.0.exe` from [Releases](https://github.com/tunajam/nachoconnect/releases)
2. Run the installer — it bundles l2tunnel and the Npcap driver
3. Launch NachoConnect from Start Menu or Desktop
4. Plug in your Xbox and play!

> The installer requires admin privileges to install the Npcap packet capture driver.

### macOS
1. Download `NachoConnect-0.1.0.dmg` from [Releases](https://github.com/tunajam/nachoconnect/releases)
2. Open the DMG and drag NachoConnect to Applications
3. Launch and allow network permissions when prompted

> macOS uses the built-in libpcap — no extra drivers needed.

## Building from Source

### Prerequisites
- Go 1.21+
- Node.js 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- C compiler (Xcode CLI tools on macOS, mingw on Windows)
- libpcap (macOS: built-in, Linux: `apt install libpcap-dev`, Windows: Npcap SDK)

### Build l2tunnel
```bash
# Initialize submodule
git submodule update --init

# Build l2tunnel binary
bash scripts/build-l2tunnel.sh
```

### Dev Mode
```bash
wails dev
```

### Build Release
```bash
wails build
```

### Hub Server
```bash
# Run the UDP relay hub (Go rewrite of l2tunnel's hub.py)
go run ./cmd/hub --port 1337

# With peer timeout (seconds)
go run ./cmd/hub --port 1337 --timeout 60
```

### Lobby Server
```bash
go run ./cmd/server
```

## How It Works

Xbox system link operates at Layer 2 (MAC-based addressing, all consoles use IP 0.0.0.1). l2tunnel captures raw Ethernet frames from the Xbox's network interface and forwards them over UDP to a central hub. The hub relays packets to all other connected peers, who inject them back onto their local networks. To each Xbox, it looks like all consoles are on the same LAN.

## Credits

- **[l2tunnel](https://github.com/mborgerson/l2tunnel)** by Matt Borgerson — the networking engine that makes this possible
- **[Wails](https://wails.io)** — Go + web frontend desktop apps

## License

MIT

---

*Made with 🧀 by [TunaJam](https://tunajam.com)*
