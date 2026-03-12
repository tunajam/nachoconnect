package main

import (
	"context"
	"fmt"
	"net"
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
	wsBridge       *l2tunnel.WSBridge
	discoverCancel context.CancelFunc
	currentLobby   *lobby.ServerLobby
	pingTicker     *time.Ticker
	pingCancel     context.CancelFunc
	healthCancel   context.CancelFunc
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
	Error        string `json:"error,omitempty"`
}

type LobbyInfo struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Game       string       `json:"game"`
	Host       string       `json:"host"`
	Players    int          `json:"players"`
	MaxPlayers int          `json:"maxPlayers"`
	Ping       int          `json:"ping"`
	Region     string       `json:"region"`
	Code       string       `json:"code"`
	HubAddr    string       `json:"hubAddr"`
	HubPort    int          `json:"hubPort"`
	Members    []PlayerInfo `json:"members"`
}

type PlayerInfo struct {
	Name      string `json:"name"`
	Ping      int    `json:"ping"`
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

func NewApp() *App {
	p, _ := prefs.Load()
	return &App{
		lobbyClient: lobby.NewClient(""),
		prefs:       p,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)

	// Set gamertag in status
	if a.prefs.Gamertag != "" {
		a.mu.Lock()
		a.status.Gamertag = a.prefs.Gamertag
		a.mu.Unlock()
	}

	go a.detectLocalIP()
	go a.measureServerPing()
}

func (a *App) shutdown(ctx context.Context) {
	// Graceful lobby leave
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
	if a.wsBridge != nil {
		a.wsBridge.Stop()
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
	result := perms.CheckPcapPermissions()
	return PermissionStatus{OK: result.OK, Message: result.Message}
}

// RequestPermissions prompts for elevated permissions on macOS
func (a *App) RequestPermissions() error {
	return perms.RequestElevatedPermissions(l2tunnel.BinaryPath)
}

// GetInterfaces returns available network interfaces via l2tunnel list
func (a *App) GetInterfaces() []NetworkInterface {
	l2Ifaces, err := l2tunnel.List()
	if err != nil {
		return a.getInterfacesFallback()
	}

	var result []NetworkInterface
	for _, iface := range l2Ifaces {
		ip := ""
		mac := ""
		if goIface, err := net.InterfaceByName(iface.Name); err == nil {
			mac = goIface.HardwareAddr.String()
			if addrs, err := goIface.Addrs(); err == nil {
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
						ip = ipnet.IP.String()
						break
					}
				}
			}
		}
		result = append(result, NetworkInterface{
			Name:        iface.Name,
			IP:          ip,
			MAC:         mac,
			Description: iface.Description,
		})
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

// SelectInterface sets the capture interface and starts Xbox discovery via l2tunnel
func (a *App) SelectInterface(name string) error {
	a.mu.Lock()
	if a.discoverCancel != nil {
		a.discoverCancel()
	}
	a.status.Interface = name
	a.mu.Unlock()

	// Save preference
	a.prefs.Interface = name
	_ = a.prefs.Save()

	discoverCtx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.discoverCancel = cancel
	a.mu.Unlock()

	discoveries, err := l2tunnel.Discover(discoverCtx, name)
	if err != nil {
		return fmt.Errorf("failed to start discovery on %s: %w", name, err)
	}

	go a.handleDiscoveries(discoveries)
	return nil
}

func (a *App) handleDiscoveries(ch <-chan l2tunnel.Discovery) {
	seen := make(map[string]bool)
	for d := range ch {
		if seen[d.SrcMAC] {
			continue
		}
		seen[d.SrcMAC] = true

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

			// Attempt reconnection
			if reconnectAttempts < maxReconnectAttempts {
				reconnectAttempts++
				a.mu.RLock()
				currentLobby := a.currentLobby
				iface := a.status.Interface
				mac := a.xboxMAC
				a.mu.RUnlock()

				if currentLobby != nil && iface != "" && mac != "" {
					runtime.EventsEmit(a.ctx, "tunnel:reconnecting", reconnectAttempts)
					time.Sleep(time.Duration(reconnectAttempts) * time.Second) // Exponential backoff

					err := a.StartTunnel(iface, mac, "0.0.0.0", "0",
						currentLobby.HubAddr, fmt.Sprintf("%d", currentLobby.HubPort))
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
		reconnectAttempts = 0 // Reset on successful check
	}
}

// GetLobbies returns available lobbies from the remote server
func (a *App) GetLobbies() []LobbyInfo {
	serverLobbies, err := a.lobbyClient.ListLobbies()
	if err != nil {
		// Emit error event but don't crash
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

// CreateLobby creates a new lobby on the remote server
func (a *App) CreateLobby(name, game string, maxPlayers int) (*LobbyInfo, error) {
	gamertag := a.prefs.Gamertag
	if gamertag == "" {
		gamertag = "NachoPlayer"
	}

	sl, err := a.lobbyClient.CreateLobby(name, game, gamertag, maxPlayers)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.currentLobby = sl
	a.mu.Unlock()

	// Start auto-tunnel if we have Xbox MAC and interface
	a.autoTunnel(sl)

	// Start ping measurement loop
	a.startPingLoop(sl)

	info := serverLobbyToInfo(*sl, gamertag)
	return &info, nil
}

// JoinLobby joins a lobby by code and starts the tunnel to the hub
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

	// Auto-tunnel to hub
	a.autoTunnel(sl)

	// Start ping measurement loop
	a.startPingLoop(sl)

	info := serverLobbyToInfo(*sl, gamertag)
	return &info, nil
}

// JoinLobbyByCode resolves an invite code and joins the lobby, auto-starting the tunnel
func (a *App) JoinLobbyByCode(code string) (*LobbyInfo, error) {
	return a.JoinLobby(code)
}

// autoTunnel starts l2tunnel connected to the hub via WebSocket bridge
func (a *App) autoTunnel(sl *lobby.ServerLobby) {
	a.mu.RLock()
	iface := a.status.Interface
	mac := a.xboxMAC
	a.mu.RUnlock()

	if iface == "" || mac == "" {
		runtime.EventsEmit(a.ctx, "tunnel:skipped", "No Xbox detected or interface not selected")
		return
	}

	if sl.HubAddr == "" {
		runtime.EventsEmit(a.ctx, "tunnel:skipped", "No hub relay address for this lobby")
		return
	}

	go func() {
		// Build WebSocket URL for the relay
		wsURL := fmt.Sprintf("wss://%s/relay?lobby=%s", sl.HubAddr, sl.ID)

		// Start WebSocket bridge (creates local UDP port)
		bridge, localAddr, err := l2tunnel.StartWSBridge(a.ctx, wsURL)
		if err != nil {
			runtime.EventsEmit(a.ctx, "error", fmt.Sprintf("Failed to connect to relay: %v", err))
			return
		}

		a.mu.Lock()
		a.wsBridge = bridge
		a.mu.Unlock()

		// Start l2tunnel pointed at the local bridge port
		err = a.StartTunnel(iface, mac, "0.0.0.0", "0",
			"127.0.0.1", localAddr[len("127.0.0.1:"):])
		if err != nil {
			bridge.Stop()
			runtime.EventsEmit(a.ctx, "error", fmt.Sprintf("Failed to start tunnel: %v", err))
		}
	}()
}

// LeaveLobby leaves the current lobby
func (a *App) LeaveLobby(id string) error {
	gamertag := a.prefs.Gamertag
	if gamertag == "" {
		gamertag = "NachoPlayer"
	}

	// Stop ping loop
	if a.pingCancel != nil {
		a.pingCancel()
	}

	// Leave on server
	_ = a.lobbyClient.LeaveLobby(id, gamertag)

	a.mu.Lock()
	a.currentLobby = nil
	a.status.Connected = false
	a.status.TunnelActive = false
	a.status.PeerCount = 0
	a.status.Error = ""

	if a.tunnel != nil {
		a.tunnel.Stop()
		a.tunnel = nil
	}
	if a.wsBridge != nil {
		a.wsBridge.Stop()
		a.wsBridge = nil
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

// SendChat sends a chat message to the current lobby
func (a *App) SendChat(lobbyID, message string) error {
	gamertag := a.prefs.Gamertag
	if gamertag == "" {
		gamertag = "You"
	}
	runtime.EventsEmit(a.ctx, "chat:message", map[string]string{
		"sender":  gamertag,
		"message": message,
		"time":    time.Now().Format("15:04"),
	})
	return nil
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

func (a *App) measureServerPing() {
	// Measure periodically
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// First measurement immediately
	a.doMeasurePing()

	for range ticker.C {
		a.doMeasurePing()
	}
}

func (a *App) doMeasurePing() {
	d, err := a.lobbyClient.Ping()
	if err != nil {
		return
	}
	pingMs := int(d.Milliseconds())
	a.mu.Lock()
	a.status.ServerPing = pingMs
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "ping:update", pingMs)
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

	go func() {
		ticker := time.NewTicker(7 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Measure ping
				d, err := a.lobbyClient.Ping()
				if err != nil {
					a.mu.Lock()
					a.status.Error = "Server unreachable"
					a.mu.Unlock()
					runtime.EventsEmit(a.ctx, "error", "Lost connection to lobby server")
					continue
				}

				pingMs := int(d.Milliseconds())
				a.mu.Lock()
				a.status.ServerPing = pingMs
				a.status.Error = ""
				a.mu.Unlock()

				// Update ping on server
				_ = a.lobbyClient.UpdatePing(sl.ID, gamertag, pingMs)

				// Refresh lobby data to get other players' pings
				runtime.EventsEmit(a.ctx, "ping:update", pingMs)
			}
		}
	}()
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
		ID:         sl.ID,
		Name:       sl.Name,
		Game:       sl.Game,
		Host:       sl.Host,
		Players:    len(sl.Players),
		MaxPlayers: sl.MaxPlayers,
		Ping:       0, // Will be set by client-side ping measurement
		Region:     sl.Region,
		Code:       sl.Code,
		HubAddr:    sl.HubAddr,
		HubPort:    sl.HubPort,
		Members:    members,
	}
}
