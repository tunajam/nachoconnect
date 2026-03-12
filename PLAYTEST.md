# NachoConnect Playtest — Known Limitations & Assumptions

*v0.2.0 — March 12, 2026*

This is the first real-hardware playtest. Here's everything that might go wrong and what to watch for.

---

## 🔴 Likely Blockers

### 1. macOS Packet Capture Permissions
**What:** libpcap needs read access to `/dev/bpf*` devices. Without it, l2tunnel can't capture or inject Ethernet frames — nothing works.

**What you'll see:** App launches fine, interface selection works, but Xbox detection hangs forever with no traffic.

**Fix:** The app tries to prompt for admin privileges via osascript (`chmod o+r /dev/bpf*`). If that dialog doesn't appear or gets dismissed, run manually:
```bash
sudo chmod o+r /dev/bpf*
```
Note: This resets on reboot. A permanent fix would require a launch daemon or installing a helper tool.

### 2. l2tunnel Binary Not Found
**What:** The app expects `l2tunnel` bundled inside the `.dmg` at `NachoConnect.app/Contents/Resources/l2tunnel` or in the same directory as the executable.

**What you'll see:** "l2tunnel list failed" errors, no interfaces shown.

**Fix:** Verify the binary is in the app bundle. If not, download from [mborgerson/l2tunnel](https://github.com/mborgerson/l2tunnel) and place in the app's Resources folder.

### 3. Xbox Not Detected
**What:** `l2tunnel discover` listens for broadcast Ethernet frames from the Xbox. The Xbox must be actively looking for system link games (i.e., you need to be on the System Link screen in a game like Halo 2).

**What you'll see:** Setup screen stays on "Searching for Xbox..." indefinitely.

**Possible causes:**
- Xbox isn't on the System Link screen yet (it only broadcasts when searching)
- Wrong network interface selected (pick the one your Xbox is plugged into)
- Xbox is on WiFi through a bridge that strips L2 frames
- macOS permissions issue (see #1)

**Fix:** Make sure the Xbox is physically Ethernet-connected to the same switch/router as the Mac. Select the correct interface (usually `en0` for built-in Ethernet, or check `ifconfig`). Navigate to System Link in-game, then watch for detection.

---

## 🟡 Probable Issues

### 4. Interface Selection Confusion
**What:** `l2tunnel list` shows all interfaces including loopback, Thunderbolt bridges, VPNs, etc. Not obvious which one has the Xbox on it.

**What you'll see:** A long list of interfaces with cryptic names.

**Workaround:** Look for `en0` (built-in Ethernet) or a USB Ethernet adapter name. If using a USB-to-Ethernet dongle, it'll show up as something like `en5` or `en8`.

### 5. WebSocket Relay Latency
**What:** All traffic routes through the Azure hub server in West US 2. Every Ethernet frame goes: Xbox → Mac → Azure → Remote Mac → Remote Xbox. That's two internet round trips per frame.

**What you'll see:** Playable but potentially laggy, especially for fast-paced games. Halo 2 is relatively tolerant of ~50-100ms latency. Above ~150ms you'll feel it.

**What affects it:** Geographic distance to Azure West US 2 (Oregon). Two players both on the west coast = best case. Cross-country or international = noticeable lag.

### 6. Azure Container App Cold Start
**What:** The lobby server runs on Azure Container Apps with scale-to-zero. If nobody has connected in ~5 minutes, the container shuts down. Next connection takes 5-15 seconds to cold start.

**What you'll see:** First lobby list load takes a while. Might timeout on first try.

**Workaround:** Just retry. Once warm, it stays up for the session.

### 7. Tunnel Stability on Long Sessions
**What:** The WebSocket tunnel hasn't been stress-tested for long play sessions (1+ hours). The keepalive is every 15 seconds, reconnect logic exists but is unproven.

**What you'll see:** Possible mid-game disconnects, tunnel drops, or the app freezing.

**What we added:** Auto-reconnect with exponential backoff (up to 5 attempts), health monitoring every 5 seconds, interface loss detection. But none of this has been battle-tested.

### 8. macOS Firewall / Little Snitch
**What:** If you're running Little Snitch, macOS firewall, or any network filter, it may block the WebSocket connection to Azure or the local UDP traffic between l2tunnel and WSBridge.

**What you'll see:** Connection to lobby server fails, or tunnel starts but no data flows.

**Fix:** Allow NachoConnect outbound connections to `nachoconnect-server.gentlepebble-471fc641.westus2.azurecontainerapps.io` on port 443.

---

## 🟢 Might Come Up

### 9. Multiple Xboxes on Same Network
**What:** If you have 2+ Xboxes on the local network, `l2tunnel discover` will detect the first one it sees. The app currently captures one MAC address.

**Impact:** Should still work — the tunnel captures all system link traffic on the interface, not just one MAC. But untested with multiple local consoles.

### 10. USB Ethernet Adapter Disconnect
**What:** If the Ethernet adapter is unplugged mid-session, the tunnel dies.

**What you'll see:** Health loop detects it within 5 seconds, emits `tunnel:disconnected` event. UI should show disconnected state.

**Fix:** Plug it back in, restart the tunnel from the app.

### 11. Game Compatibility
**What:** NachoConnect tunnels raw Ethernet frames, which is exactly what system link uses. In theory, ANY system link game works. But each game has its own network behavior.

**Known good (from XBConnect/XLink Kai history):**
- Halo 2 ✅ (most tolerant of latency)
- Halo CE ✅
- Star Wars Battlefront 2 ✅
- Crimson Skies ✅

**Potentially problematic:**
- Games with aggressive timeout on peer discovery
- Games that use non-standard ports beyond UDP 3074

### 12. Windows Side (Npcap)
**What:** Windows build bundles Npcap installer. If Npcap isn't installed, the app should prompt. If Npcap is installed but the service isn't running, packet capture fails silently.

**Fix:** Run `sc query npcap` in PowerShell to verify. Restart service if needed: `net start npcap`.

---

## 📋 Playtest Checklist

Before starting:
- [ ] Both players have NachoConnect v0.2.0 installed
- [ ] Xbox is connected via Ethernet (not WiFi) to the router
- [ ] Mac/PC is on the same local network as their Xbox
- [ ] On Mac: run `ls -la /dev/bpf0` — if permission denied, run `sudo chmod o+r /dev/bpf*`
- [ ] On Windows: verify Npcap is installed and running
- [ ] Both players can reach the lobby server (open https://nachoconnect-server.gentlepebble-471fc641.westus2.azurecontainerapps.io/api/health in browser)

During test:
- [ ] Player 1 creates a lobby
- [ ] Player 2 joins via invite code
- [ ] Both select correct network interface
- [ ] Both navigate to System Link in-game
- [ ] Xbox detection triggers on both sides
- [ ] Tunnel starts automatically
- [ ] Xboxes see each other in system link browser
- [ ] Can start and play a game

---

## 🔧 Quick Debug Commands

```bash
# Check if l2tunnel can see interfaces
./l2tunnel list

# Check if Xbox is broadcasting (run from terminal)
sudo ./l2tunnel discover <interface_name>

# Test Azure server health
curl https://nachoconnect-server.gentlepebble-471fc641.westus2.azurecontainerapps.io/api/health

# Check macOS BPF permissions
ls -la /dev/bpf0

# Check Npcap on Windows (PowerShell)
sc query npcap
```

---

*This doc will be updated after the first playtest with real findings.*
