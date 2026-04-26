package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/tunajam/nachoconnect/internal/l2tunnel"
	"github.com/tunajam/nachoconnect/internal/lobby"
	"github.com/tunajam/nachoconnect/internal/perms"
	"github.com/tunajam/nachoconnect/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct - main application controller
type App struct {
	ctx            context.Context
	cancel         context.CancelFunc
	lobbyClient    *lobby.Client
	prefs          *prefs.Preferences
	mu             sync.RWMutex
	xboxFound      bool
	xboxMAC        string
	status         AppStatus
	tunnel         *l2tunnel.Tunnel
	hub            *l2tunnel.Hub
	upnpPort       int // port we UPnP-forwarded (0 = none)
	discoverCancel context.CancelFunc
	currentLobby   *lobby.ServerLobby
	pingTicker     *time.Ticker
	pingCancel     context.CancelFunc
	healthCancel   context.CancelFunc
	peerPings      map[string]int // addr string → RTT ms (host-side per-peer pings)
	pingConn       *net.UDPConn   // peer-side UDP socket for ping
}

type AppStatus struct {
	XboxDetected bool   `json:"xboxDetected"`
	XboxMAC      string `json:"xboxMAC"`
	TunnelActive bool   `json:"tunnelActive"`
	Connected    bool   `json:"connected"`
	PeerCount    int    `json:"peerCount"`
	LocalIP      string `json:"localIP"`
	PublicIP     string `json:"publicIP"`
	Interface    string `json:"interface"`
	Gamertag     string `json:"gamertag"`
	ServerPing   int    `json:"serverPing"`
	PeerPing     int    `json:"peerPing"`
	Error        string `json:"error,omitempty"`
}

type LobbyInfo struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Game         string       `json:"game"`
	Host         string       `json:"host"`
	Players      int          `json:"players"`
	MaxPlayers   int          `json:"maxPlayers"`
	Ping         int          `json:"ping"`
	Region       string       `json:"region"`
	Code         string       `json:"code"`
	HostPublicIP string       `json:"hostPublicIP"`
	HostPort     int          `json:"hostPort"`
	Members      []PlayerInfo `json:"members"`
}

type PlayerInfo struct {
	Name      string `json:"name"`
	Ping      int    `json:"ping"`
	P2PPing   int    `json:"p2pPing"`
	IsHost    bool   `json:"isHost"`
	IsYou     bool   `json:"isYou"`
	Connected bool   `json:"connected"`
}

type NetworkInterface struct {
	Name        string `json:"name"`
	IP          string `json:"ip"`
	MAC         string `json:"mac"`
	Description string `json:"description,omitempty"`
}

type PermissionStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// PortForwardInfo provides instructions for manual port forwarding
type PortForwardInfo struct {
	PublicIP   string `json:"publicIP"`
	LocalIP    string `json:"localIP"`
	Port       int    `json:"port"`
	GatewayIP  string `json:"gatewayIP"`
	UPnPStatus string `json:"upnpStatus"` // "success", "failed", "untried"
}

func NewApp() *App {
	p, _ := prefs.Load()
	return &App{
		lobbyClient: lobby.NewClient(""),
		prefs:       p,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)

	if a.prefs.Gamertag != "" {
		a.mu.Lock()
		a.status.Gamertag = a.prefs.Gamertag
		a.mu.Unlock()
	}

	go a.detectLocalIP()
	// Server ping disabled — replaced by P2P ping when in a lobby
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.RLock()
	currentLobby := a.currentLobby
	gamertag := a.prefs.Gamertag
	a.mu.RUnlock()

	if currentLobby != nil && gamertag != "" {
		_ = a.lobbyClient.LeaveLobby(currentLobby.ID, gamertag)
	}

	if a.cancel != nil {
		a.cancel()
	}
	if a.discoverCancel != nil {
		a.discoverCancel()
	}
	if a.pingCancel != nil {
		a.pingCancel()
	}
	if a.healthCancel != nil {
		a.healthCancel()
	}
	if a.tunnel != nil {
		a.tunnel.Stop()
	}
	if a.hub != nil {
		a.hub.Stop()
	}
	if a.upnpPort > 0 {
		l2tunnel.RemoveUPnPForward(a.upnpPort)
	}
}

// GetStatus returns current app status
func (a *App) GetStatus() AppStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// GetGamertag returns the stored gamertag
func (a *App) GetGamertag() string {
	return a.prefs.Gamertag
}

// SetGamertag saves the user's gamertag
func (a *App) SetGamertag(tag string) error {
	if tag == "" {
		return fmt.Errorf("gamertag cannot be empty")
	}
	if err := a.prefs.SetGamertag(tag); err != nil {
		return err
	}
	a.mu.Lock()
	a.status.Gamertag = tag
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())
	return nil
}

// CheckPermissions checks if pcap permissions are available
func (a *App) CheckPermissions() PermissionStatus {
	// If BPF setup was already done, do a quick check
	if perms.IsSetupDone() {
		result := perms.CheckPcapPermissions()
		if result.OK {
			return PermissionStatus{OK: true, Message: result.Message}
		}
		// Setup was done but permissions lost (e.g. OS update) — need re-setup
	}
	result := perms.CheckPcapPermissions()
	return PermissionStatus{OK: result.OK, Message: result.Message}
}

// RequestPermissions installs permissions for packet capture.
// macOS: Creates access_bpf group, adds user, installs LaunchDaemon. Prompts for admin password once.
// Windows: Runs bundled Npcap installer silently.
func (a *App) RequestPermissions() error {
	if !perms.IsNpcapInstalled() {
		if err := perms.InstallNpcap(); err != nil {
			return err
		}
	}
	return perms.RequestElevatedPermissions(l2tunnel.BinaryPath)
}

// IsBPFSetupDone returns whether the one-time BPF setup has been completed
func (a *App) IsBPFSetupDone() bool {
	return perms.IsSetupDone()
}

// friendlyInterfaceName returns a human-readable label for macOS interface names.
// On non-macOS platforms or if a description from l2tunnel is already available, returns that instead.
func friendlyInterfaceName(name, l2Description string, goIface *net.Interface) string {
	// If l2tunnel already provided a non-empty description (common on Windows/Npcap), prefer it
	if l2Description != "" {
		return l2Description
	}

	// Only apply friendly names on macOS
	if goruntime.GOOS != "darwin" {
		return ""
	}

	// Skip known virtual/system interfaces
	for _, prefix := range []string{"utun", "awdl", "llw"} {
		if strings.HasPrefix(name, prefix) {
			return "" // caller should skip these
		}
	}

	if name == "bridge0" {
		return "Thunderbolt Bridge"
	}

	// en* interfaces — distinguish Wi-Fi vs Ethernet using interface flags
	if strings.HasPrefix(name, "en") {
		if goIface != nil && goIface.Flags&net.FlagBroadcast != 0 {
			// On macOS, en0 is typically Wi-Fi on laptops, but could be Ethernet on desktops.
			// Wi-Fi interfaces support multicast; we use a heuristic: en0 = Wi-Fi unless MTU suggests otherwise
			if name == "en0" {
				// Most Macs: en0 is Wi-Fi. Mac Pro/Mac mini with no Wi-Fi: en0 is Ethernet.
				// MTU 1500 is standard for both, so check if there's a wireless hint in the name
				return "Wi-Fi"
			}
			// en1-en9: typically Ethernet adapters, USB Ethernet dongles, or Thunderbolt Ethernet
			return "Ethernet"
		}
		return "Ethernet"
	}

	return ""
}

// GetInterfaces returns available network interfaces via l2tunnel list
func (a *App) GetInterfaces() []NetworkInterface {
	l2Ifaces, err := l2tunnel.List()
	if err != nil {
		return a.getInterfacesFallback()
	}

	var result []NetworkInterface
	for _, iface := range l2Ifaces {
		// Skip known virtual interfaces on macOS
		if goruntime.GOOS == "darwin" {
			skip := false
			for _, prefix := range []string{"utun", "awdl", "llw"} {
				if strings.HasPrefix(iface.Name, prefix) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}

		ip := ""
		mac := ""
		var goIface *net.Interface
		if gi, err := net.InterfaceByName(iface.Name); err == nil {
			goIface = gi
			// Skip loopback and down interfaces
			if goIface.Flags&net.FlagLoopback != 0 || goIface.Flags&net.FlagUp == 0 {
				continue
			}
			mac = goIface.HardwareAddr.String()
			// Skip interfaces with no MAC (virtual/loopback)
			if mac == "" {
				continue
			}
			if addrs, err := goIface.Addrs(); err == nil {
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
						ip = ipnet.IP.String()
						break
					}
				}
			}
		} else {
			// Can't resolve interface details — skip
			continue
		}

		friendly := friendlyInterfaceName(iface.Name, iface.Description, goIface)
		desc := friendly
		if desc == "" {
			desc = iface.Description
		}

		result = append(result, NetworkInterface{
			Name:        iface.Name,
			IP:          ip,
			MAC:         mac,
			Description: desc,
		})
	}
	// Sort: interfaces with an IP first, then by name (Ethernet-like names first)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			// Prioritize interfaces with IPs
			if result[j].IP != "" && result[i].IP == "" {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func (a *App) getInterfacesFallback() []NetworkInterface {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var result []NetworkInterface
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		ip := ""
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				break
			}
		}
		if ip == "" {
			continue
		}
		result = append(result, NetworkInterface{
			Name: iface.Name,
			IP:   ip,
			MAC:  iface.HardwareAddr.String(),
		})
	}
	return result
}

// SelectInterface sets the capture interface and starts Xbox discovery
func (a *App) SelectInterface(name string) error {
	a.mu.Lock()
	if a.discoverCancel != nil {
		a.discoverCancel()
	}
	a.status.Interface = name
	a.mu.Unlock()

	a.prefs.Interface = name
	_ = a.prefs.Save()

	discoverCtx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.discoverCancel = cancel
	a.mu.Unlock()

	discoveries, errCh, err := l2tunnel.Discover(discoverCtx, name)
	if err != nil {
		return fmt.Errorf("failed to start discovery on %s: %w", name, err)
	}

	go a.handleDiscoveries(discoveries, errCh)
	return nil
}

func (a *App) handleDiscoveries(ch <-chan l2tunnel.Discovery, errCh <-chan error) {
	seen := make(map[string]bool)
	var candidates []string // non-Xbox broadcast sources, in case OUI check misses

	for d := range ch {
		if seen[d.SrcMAC] {
			continue
		}
		seen[d.SrcMAC] = true

		if l2tunnel.IsLikelyXbox(d) {
			a.mu.Lock()
			if !a.xboxFound {
				a.xboxFound = true
				a.xboxMAC = d.SrcMAC
				a.status.XboxDetected = true
				a.status.XboxMAC = d.SrcMAC
				runtime.EventsEmit(a.ctx, "xbox:detected", d.SrcMAC)
			}
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())
		} else if strings.ToLower(d.DstMAC) == "ff:ff:ff:ff:ff:ff" {
			candidates = append(candidates, d.SrcMAC)
		}

		// Emit scan activity so UI knows the capture is working
		runtime.EventsEmit(a.ctx, "discover:activity", len(seen))
	}

	// If no Xbox OUI matched but we saw broadcast candidates, offer them
	a.mu.RLock()
	found := a.xboxFound
	a.mu.RUnlock()
	if !found && len(candidates) > 0 {
		runtime.EventsEmit(a.ctx, "discover:candidates", candidates)
	}

	// Discovery channel closed — check if the subprocess reported an error
	if err, ok := <-errCh; ok && err != nil {
		category := l2tunnel.ClassifyDiscoverError(err.Error())
		runtime.EventsEmit(a.ctx, "discover:error", category, err.Error())
	}
}

// StartTunnel starts the l2tunnel tunnel subprocess
func (a *App) StartTunnel(iface, mac, localAddr, localPort, remoteAddr, remotePort string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.tunnel != nil {
		a.tunnel.Stop()
	}

	cfg := l2tunnel.TunnelConfig{
		Interface:  iface,
		FilterMode: "-s",
		MAC:        mac,
		LocalAddr:  localAddr,
		LocalPort:  localPort,
		RemoteAddr: remoteAddr,
		RemotePort: remotePort,
	}

	t, err := l2tunnel.StartTunnel(cfg)
	if err != nil {
		a.status.Error = fmt.Sprintf("Tunnel failed: %v", err)
		runtime.EventsEmit(a.ctx, "status:update", a.status)
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	a.tunnel = t
	a.status.TunnelActive = true
	a.status.Connected = true
	a.status.Error = ""

	runtime.EventsEmit(a.ctx, "status:update", a.status)
	runtime.EventsEmit(a.ctx, "tunnel:connected", nil)

	go a.monitorTunnel(t)
	return nil
}

func (a *App) monitorTunnel(t *l2tunnel.Tunnel) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	reconnectAttempts := 0
	maxReconnectAttempts := 3

	for range ticker.C {
		if !t.IsActive() {
			a.mu.Lock()
			a.status.TunnelActive = false
			a.status.Connected = false
			a.mu.Unlock()

			if reconnectAttempts < maxReconnectAttempts {
				reconnectAttempts++
				a.mu.RLock()
				currentLobby := a.currentLobby
				iface := a.status.Interface
				mac := a.xboxMAC
				hub := a.hub
				a.mu.RUnlock()

				if currentLobby != nil && iface != "" && mac != "" {
					runtime.EventsEmit(a.ctx, "tunnel:reconnecting", reconnectAttempts)
					time.Sleep(time.Duration(reconnectAttempts) * time.Second)

					// Reconnect: if we're the host, point at local hub; else point at host IP
					var remoteAddr string
					var remotePort string
					if hub != nil {
						remoteAddr = "127.0.0.1"
						remotePort = fmt.Sprintf("%d", hub.Port())
					} else if currentLobby.HostPublicIP != "" {
						remoteAddr = currentLobby.HostPublicIP
						remotePort = fmt.Sprintf("%d", currentLobby.HostPort)
					} else {
						break
					}

					err := a.StartTunnel(iface, mac, "0.0.0.0", "0", remoteAddr, remotePort)
					if err == nil {
						reconnectAttempts = 0
						continue
					}
				}
			}

			runtime.EventsEmit(a.ctx, "tunnel:disconnected", nil)
			a.mu.Lock()
			a.status.Error = "Tunnel disconnected"
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())
			return
		}
		reconnectAttempts = 0
	}
}

// GetLobbies returns available lobbies from the remote server
func (a *App) GetLobbies() []LobbyInfo {
	serverLobbies, err := a.lobbyClient.ListLobbies()
	if err != nil {
		runtime.EventsEmit(a.ctx, "error", fmt.Sprintf("Failed to fetch lobbies: %v", err))
		return nil
	}

	gamertag := a.prefs.Gamertag
	var result []LobbyInfo
	for _, sl := range serverLobbies {
		result = append(result, serverLobbyToInfo(sl, gamertag))
	}
	return result
}

// CreateLobby creates a lobby and starts hosting via Direct Connect P2P.
// Starts a local UDP hub, detects public IP, tries UPnP, registers with lobby server.
func (a *App) CreateLobby(name, game string, maxPlayers, port int) (*LobbyInfo, error) {
	gamertag := a.prefs.Gamertag
	if gamertag == "" {
		gamertag = "NachoPlayer"
	}
	if port <= 0 {
		port = 9999
	}

	// Start local UDP hub
	hub, err := l2tunnel.StartHub(port)
	if err != nil {
		return nil, fmt.Errorf("failed to start hub on port %d: %w", port, err)
	}
	a.mu.Lock()
	if a.hub != nil {
		a.hub.Stop()
	}
	a.hub = hub
	a.mu.Unlock()
	hostPort := hub.Port()

	// Try UPnP auto-forward
	upnpResult := l2tunnel.TryUPnPForward(hostPort)
	if upnpResult.Success {
		a.mu.Lock()
		a.upnpPort = hostPort
		a.mu.Unlock()
	}

	// Detect public IP
	hostPublicIP, _ := l2tunnel.DetectPublicIP()
	a.mu.Lock()
	a.status.PublicIP = hostPublicIP
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())

	// Register lobby on server
	sl, err := a.lobbyClient.CreateLobby(name, game, gamertag, maxPlayers, hostPublicIP, hostPort)
	if err != nil {
		a.mu.Lock()
		a.hub.Stop()
		a.hub = nil
		a.mu.Unlock()
		return nil, err
	}

	a.mu.Lock()
	a.currentLobby = sl
	a.mu.Unlock()

	// Connect l2tunnel to local hub
	a.autoTunnelHost()

	a.startPingLoop(sl)

	info := serverLobbyToInfo(*sl, gamertag)
	return &info, nil
}

// GetPortForwardInfo returns info needed for manual port forwarding
func (a *App) GetPortForwardInfo(port int) PortForwardInfo {
	publicIP, _ := l2tunnel.DetectPublicIP()
	localIP := a.getLocalIP()
	gatewayIP := a.detectGateway()

	a.mu.Lock()
	a.status.PublicIP = publicIP
	a.mu.Unlock()

	return PortForwardInfo{
		PublicIP:   publicIP,
		LocalIP:    localIP,
		Port:       port,
		GatewayIP:  gatewayIP,
		UPnPStatus: "untried",
	}
}

// DetectPublicIP returns the host's public IP address
func (a *App) DetectPublicIP() string {
	ip, err := l2tunnel.DetectPublicIP()
	if err != nil {
		return ""
	}
	a.mu.Lock()
	a.status.PublicIP = ip
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())
	return ip
}

// TryUPnP attempts UPnP port forwarding and returns the result
func (a *App) TryUPnP(port int) map[string]interface{} {
	result := l2tunnel.TryUPnPForward(port)
	return map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
		"port":    result.Port,
	}
}

// JoinLobby joins a lobby by code and connects directly to the host via P2P.
func (a *App) JoinLobby(code string) (*LobbyInfo, error) {
	gamertag := a.prefs.Gamertag
	if gamertag == "" {
		gamertag = "NachoPlayer"
	}

	sl, err := a.lobbyClient.JoinLobby(code, gamertag)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.currentLobby = sl
	a.mu.Unlock()

	// Connect directly to host
	if sl.HostPublicIP != "" && sl.HostPort > 0 {
		a.autoTunnelPeer(sl)
	} else {
		runtime.EventsEmit(a.ctx, "error", "Host has not published their connection info yet")
	}

	a.startPingLoop(sl)

	info := serverLobbyToInfo(*sl, gamertag)
	return &info, nil
}

// JoinLobbyByCode is an alias for JoinLobby
func (a *App) JoinLobbyByCode(code string) (*LobbyInfo, error) {
	return a.JoinLobby(code)
}

// autoTunnelHost connects l2tunnel to the local hub (host side)
func (a *App) autoTunnelHost() {
	a.mu.RLock()
	iface := a.status.Interface
	mac := a.xboxMAC
	hub := a.hub
	a.mu.RUnlock()

	if iface == "" || mac == "" {
		runtime.EventsEmit(a.ctx, "tunnel:skipped", "No Xbox detected or interface not selected")
		return
	}
	if hub == nil {
		runtime.EventsEmit(a.ctx, "tunnel:skipped", "Hub not running")
		return
	}

	go func() {
		err := a.StartTunnel(iface, mac, "0.0.0.0", "0",
			"127.0.0.1", fmt.Sprintf("%d", hub.Port()))
		if err != nil {
			a.mu.Lock()
			a.status.Error = fmt.Sprintf("Failed to start tunnel: %v", err)
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "error", a.status.Error)
			runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())
		}
		a.startHealthLoop(iface)
	}()
}

// autoTunnelPeer connects l2tunnel directly to host's public IP:port (peer side)
func (a *App) autoTunnelPeer(sl *lobby.ServerLobby) {
	a.mu.RLock()
	iface := a.status.Interface
	mac := a.xboxMAC
	a.mu.RUnlock()

	if iface == "" || mac == "" {
		runtime.EventsEmit(a.ctx, "tunnel:skipped", "No Xbox detected or interface not selected")
		return
	}

	go func() {
		err := a.StartTunnel(iface, mac, "0.0.0.0", "0",
			sl.HostPublicIP, fmt.Sprintf("%d", sl.HostPort))
		if err != nil {
			a.mu.Lock()
			a.status.Error = fmt.Sprintf("Failed to connect to host: %v", err)
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "error", a.status.Error)
			runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())
			return
		}
		a.startHealthLoop(iface)
	}()
}

// startHealthLoop monitors interface availability
func (a *App) startHealthLoop(ifaceName string) {
	if a.healthCancel != nil {
		a.healthCancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.healthCancel = cancel

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := net.InterfaceByName(ifaceName)
				if err != nil {
					a.mu.Lock()
					a.status.Error = fmt.Sprintf("Interface %s disappeared (adapter unplugged?)", ifaceName)
					a.status.TunnelActive = false
					a.status.Connected = false
					a.mu.Unlock()
					runtime.EventsEmit(a.ctx, "error", a.status.Error)
					runtime.EventsEmit(a.ctx, "tunnel:disconnected", nil)
					runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())
					return
				}
			}
		}
	}()
}

// LeaveLobby leaves the current lobby
func (a *App) LeaveLobby(id string) error {
	gamertag := a.prefs.Gamertag
	if gamertag == "" {
		gamertag = "NachoPlayer"
	}

	if a.pingCancel != nil {
		a.pingCancel()
	}

	_ = a.lobbyClient.LeaveLobby(id, gamertag)

	a.mu.Lock()
	a.currentLobby = nil
	a.peerPings = nil
	a.status.PeerPing = 0
	a.status.Connected = false
	a.status.TunnelActive = false
	a.status.PeerCount = 0
	a.status.Error = ""

	if a.tunnel != nil {
		a.tunnel.Stop()
		a.tunnel = nil
	}
	if a.hub != nil {
		a.hub.Stop()
		a.hub = nil
	}
	if a.upnpPort > 0 {
		l2tunnel.RemoveUPnPForward(a.upnpPort)
		a.upnpPort = 0
	}
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "status:update", a.GetStatus())
	return nil
}

// GetLobby returns info about a specific lobby (refreshed from server)
func (a *App) GetLobby(id string) *LobbyInfo {
	sl, err := a.lobbyClient.GetLobby(id)
	if err != nil {
		return nil
	}
	info := serverLobbyToInfo(*sl, a.prefs.Gamertag)
	return &info
}

// RefreshLobby refreshes the current lobby data from the server
func (a *App) RefreshLobby() *LobbyInfo {
	a.mu.RLock()
	current := a.currentLobby
	a.mu.RUnlock()
	if current == nil {
		return nil
	}
	return a.GetLobby(current.ID)
}

// GetServerPing returns the current measured ping to the server
func (a *App) GetServerPing() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status.ServerPing
}

// Internal methods

func (a *App) detectLocalIP() {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	a.mu.Lock()
	a.status.LocalIP = localAddr.IP.String()
	a.mu.Unlock()
}

func (a *App) getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func (a *App) detectGateway() string {
	// Common gateway IPs — try to find the one that responds
	// This is a best-effort heuristic
	candidates := []string{"192.168.1.1", "192.168.0.1", "10.0.0.1", "172.16.0.1"}
	localIP := a.getLocalIP()
	if localIP != "" {
		// Guess gateway from local IP (replace last octet with .1)
		parts := net.ParseIP(localIP).To4()
		if parts != nil {
			guess := fmt.Sprintf("%d.%d.%d.1", parts[0], parts[1], parts[2])
			// Put our guess first
			candidates = append([]string{guess}, candidates...)
		}
	}

	for _, gw := range candidates {
		conn, err := net.DialTimeout("tcp", gw+":80", 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return gw
		}
		// Also try HTTPS
		conn, err = net.DialTimeout("tcp", gw+":443", 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return gw
		}
	}
	// Return best guess even if we can't reach it
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func (a *App) startPingLoop(sl *lobby.ServerLobby) {
	if a.pingCancel != nil {
		a.pingCancel()
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.pingCancel = cancel

	gamertag := a.prefs.Gamertag
	if gamertag == "" {
		gamertag = "NachoPlayer"
	}

	a.mu.RLock()
	isHost := a.hub != nil
	a.mu.RUnlock()

	if isHost {
		go a.pingLoopHost(ctx, sl, gamertag)
	} else {
		go a.pingLoopPeer(ctx, sl, gamertag)
	}
}

// pingLoopHost — host pings all connected peers via the hub
func (a *App) pingLoopHost(ctx context.Context, sl *lobby.ServerLobby, gamertag string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			hub := a.hub
			a.mu.RUnlock()
			if hub == nil {
				continue
			}

			results := hub.PingAllPeers(2 * time.Second)

			a.mu.Lock()
			a.peerPings = results
			// Host's own ping is ~0
			a.status.PeerPing = 0
			a.status.ServerPing = 0
			a.mu.Unlock()

			// Emit per-peer pings for UI
			runtime.EventsEmit(a.ctx, "ping:p2p", map[string]interface{}{
				"self":  0,
				"peers": results,
			})

			// Update lobby server with our ping (0 for host)
			_ = a.lobbyClient.UpdatePing(sl.ID, gamertag, 0)
		}
	}
}

// pingLoopPeer — peer sends UDP pings to the host and measures RTT
func (a *App) pingLoopPeer(ctx context.Context, sl *lobby.ServerLobby, gamertag string) {
	if sl.HostPublicIP == "" || sl.HostPort <= 0 {
		return
	}

	hostAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", sl.HostPublicIP, sl.HostPort))
	if err != nil {
		return
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return
	}
	a.mu.Lock()
	a.pingConn = conn
	a.mu.Unlock()

	defer func() {
		conn.Close()
		a.mu.Lock()
		a.pingConn = nil
		a.mu.Unlock()
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pkt := l2tunnel.BuildPingPacket()
			sendTime := time.Now()
			_, err := conn.WriteToUDP(pkt, hostAddr)
			if err != nil {
				continue
			}

			// Wait for pong
			buf := make([]byte, 13)
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			ts := l2tunnel.ParsePongTimestamp(buf, n)
			if ts == 0 {
				continue
			}

			// Verify timestamp matches what we sent
			sentTs := int64(binary.BigEndian.Uint64(pkt[5:13]))
			if ts != sentTs {
				continue
			}

			rtt := int(time.Since(sendTime).Milliseconds())

			a.mu.Lock()
			a.status.PeerPing = rtt
			a.status.ServerPing = rtt // Keep ServerPing updated for backward compat
			a.mu.Unlock()

			runtime.EventsEmit(a.ctx, "ping:p2p", map[string]interface{}{
				"self": rtt,
			})
			runtime.EventsEmit(a.ctx, "ping:update", rtt)

			_ = a.lobbyClient.UpdatePing(sl.ID, gamertag, rtt)
		}
	}
}

// GetPeerPings returns the host's measured RTT to each peer (host-only).
func (a *App) GetPeerPings() map[string]int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.peerPings == nil {
		return map[string]int{}
	}
	result := make(map[string]int, len(a.peerPings))
	for k, v := range a.peerPings {
		result[k] = v
	}
	return result
}

func serverLobbyToInfo(sl lobby.ServerLobby, myGamertag string) LobbyInfo {
	var members []PlayerInfo
	for _, p := range sl.Players {
		members = append(members, PlayerInfo{
			Name:      p.Name,
			Ping:      p.Ping,
			IsHost:    p.IsHost,
			IsYou:     p.Name == myGamertag,
			Connected: true,
		})
	}
	return LobbyInfo{
		ID:           sl.ID,
		Name:         sl.Name,
		Game:         sl.Game,
		Host:         sl.Host,
		Players:      len(sl.Players),
		MaxPlayers:   sl.MaxPlayers,
		Ping:         0,
		Region:       sl.Region,
		Code:         sl.Code,
		HostPublicIP: sl.HostPublicIP,
		HostPort:     sl.HostPort,
		Members:      members,
	}
}
