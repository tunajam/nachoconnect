# 🧀 NachoConnect

**Xbox system link, anywhere in the world.**

A modern desktop tunneling app for original Xbox system link play over the internet. Think Discord meets XBConnect — install, plug in your Xbox, click Play.

## What It Does

NachoConnect captures Xbox system link broadcast packets on your local network, encrypts them with the Noise Protocol, and tunnels them to your friends over the internet. Their Xboxes see your game as if everyone's in the same room.

## Features

- **🎮 Lobby System** — Create/join rooms with invite codes
- **🔍 Auto-Detect Xbox** — Automatically finds your Xbox on the network
- **🔒 Encrypted Tunnels** — Noise Protocol (same crypto as WireGuard)
- **🌐 NAT Traversal** — STUN hole punching + relay fallback
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

- **Go** — Backend, networking, packet capture
- **Wails v2** — Desktop app framework (Go + Svelte)
- **gopacket** — Raw packet capture/injection via libpcap
- **flynn/noise** — Noise Protocol Framework encryption
- **Svelte** — Frontend UI

## Architecture

```
Xbox ←→ [Packet Capture (gopacket)] ←→ [Noise Tunnel (UDP)] ←→ [Peer's Capture] ←→ Xbox
                                              ↕
                                      [Lobby Server (Go)]
                                      [STUN/Relay Server]
```

## Building

### Prerequisites
- Go 1.21+
- Node.js 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- libpcap (macOS: built-in, Linux: `apt install libpcap-dev`, Windows: Npcap)

### Build
```bash
wails build
```

### Dev Mode
```bash
wails dev
```

### Server
```bash
go run ./cmd/server
```

## Protocol

NachoConnect uses a custom framing protocol over UDP:

```
[XBC Header (8B)] [Noise-encrypted Ethernet frame]
  Magic: 0x5842 ("XB")
  Type:  DATA | BROADCAST | KEEPALIVE | CONTROL
  Flags: reserved
  LobbyID: 4 bytes
```

Xbox system link operates at Layer 2 (MAC-based addressing, all consoles use IP 0.0.0.1). We capture and tunnel raw Ethernet frames — the Xbox handles all game-layer encryption internally.

## License

MIT

---

*Made with 🧀 by [TunaJam](https://tunajam.com)*
