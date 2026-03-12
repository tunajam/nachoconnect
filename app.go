package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/tunajam/nachoconnect/internal/l2tunnel"
	"github.com/tunajam/nachoconnect/internal/lobby"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct - main application controller
type App struct {
	ctx            context.Context
	cancel         context.CancelFunc
	lobbyMgr       *lobby.Manager
	mu             sync.RWMutex
	xboxFound      bool
	xboxMAC        string
	status         AppStatus
	tunnel         *l2tunnel.Tunnel
	discoverCancel context.CancelFunc
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

func NewApp() *App {
	return &App{
		lobbyMgr: lobby.NewManager(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)

	// Get local IP
	go a.detectLocalIP()
}

func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	if a.discoverCancel != nil {
		a.discoverCancel()
	}
	if a.tunnel != nil {
		a.tunnel.Stop()
	}
}

// GetStatus returns current app status
func (a *App) GetStatus() AppStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// GetInterfaces returns available network interfaces via l2tunnel list
func (a *App) GetInterfaces() []NetworkInterface {
	l2Ifaces, err := l2tunnel.List()
	if err != nil {
		// Fallback to Go net interfaces
		return a.getInterfacesFallback()
	}

	var result []NetworkInterface
	for _, iface := range l2Ifaces {
		// Get IP from Go's net package for this interface
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
	// Stop previous discovery if any
	if a.discoverCancel != nil {
		a.discoverCancel()
	}
	a.status.Interface = name
	a.mu.Unlock()

	// Start l2tunnel discover to find Xbox MACs
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
		runtime.EventsEmit(a.ctx, "status:update", a.status)
	}
}

// StartTunnel starts the l2tunnel tunnel subprocess
func (a *App) StartTunnel(iface, mac, localAddr, localPort, remoteAddr, remotePort string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop existing tunnel
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
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	a.tunnel = t
	a.status.TunnelActive = true
	a.status.Connected = true

	runtime.EventsEmit(a.ctx, "status:update", a.status)
	runtime.EventsEmit(a.ctx, "tunnel:connected", nil)

	// Monitor tunnel health
	go a.monitorTunnel(t)

	return nil
}

func (a *App) monitorTunnel(t *l2tunnel.Tunnel) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !t.IsActive() {
			a.mu.Lock()
			a.status.TunnelActive = false
			a.status.Connected = false
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "tunnel:disconnected", nil)
			runtime.EventsEmit(a.ctx, "status:update", a.status)
			return
		}
	}
}

// GetLobbies returns available lobbies
func (a *App) GetLobbies() []LobbyInfo {
	lobbies := a.lobbyMgr.ListLobbies()
	var result []LobbyInfo
	for _, l := range lobbies {
		result = append(result, lobbyToInfo(l))
	}
	if len(result) == 0 {
		result = a.getDemoLobbies()
	}
	return result
}

// CreateLobby creates a new lobby
func (a *App) CreateLobby(name, game string, maxPlayers int) (*LobbyInfo, error) {
	l, err := a.lobbyMgr.CreateLobby(name, game, maxPlayers, "NachoPlayer")
	if err != nil {
		return nil, err
	}
	info := lobbyToInfo(l)
	return &info, nil
}

// JoinLobby joins a lobby by code and starts the tunnel to the hub
func (a *App) JoinLobby(code string) (*LobbyInfo, error) {
	l, err := a.lobbyMgr.JoinLobby(code, "NachoPlayer")
	if err != nil {
		return nil, err
	}

	// Start l2tunnel to hub server when we have Xbox MAC and interface
	a.mu.RLock()
	iface := a.status.Interface
	mac := a.xboxMAC
	a.mu.RUnlock()

	if iface != "" && mac != "" {
		// TODO: Get hub address from lobby server
		go func() {
			a.StartTunnel(iface, mac, "0.0.0.0", "1337", "hub.nachoconnect.net", "1337")
		}()
	}

	info := lobbyToInfo(l)
	return &info, nil
}

// LeaveLobby leaves the current lobby
func (a *App) LeaveLobby(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lobbyMgr.LeaveLobby(id, "NachoPlayer")
	a.status.Connected = false
	a.status.TunnelActive = false
	a.status.PeerCount = 0

	if a.tunnel != nil {
		a.tunnel.Stop()
		a.tunnel = nil
	}

	runtime.EventsEmit(a.ctx, "status:update", a.status)
	return nil
}

// GetLobby returns info about a specific lobby
func (a *App) GetLobby(id string) *LobbyInfo {
	l := a.lobbyMgr.GetLobby(id)
	if l == nil {
		return nil
	}
	info := lobbyToInfo(l)
	return &info
}

// SendChat sends a chat message to the current lobby
func (a *App) SendChat(lobbyID, message string) error {
	runtime.EventsEmit(a.ctx, "chat:message", map[string]string{
		"sender":  "You",
		"message": message,
		"time":    time.Now().Format("15:04"),
	})
	return nil
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

func lobbyToInfo(l *lobby.Lobby) LobbyInfo {
	var members []PlayerInfo
	for _, m := range l.Members {
		members = append(members, PlayerInfo{
			Name:      m.Name,
			Ping:      m.Ping,
			IsHost:    m.IsHost,
			IsYou:     m.IsYou,
			Connected: true,
		})
	}
	return LobbyInfo{
		ID:         l.ID,
		Name:       l.Name,
		Game:       l.Game,
		Host:       l.Host,
		Players:    len(l.Members),
		MaxPlayers: l.MaxPlayers,
		Ping:       l.Ping,
		Region:     l.Region,
		Code:       l.Code,
		Members:    members,
	}
}

func (a *App) getDemoLobbies() []LobbyInfo {
	games := []struct {
		name, game, host, region string
		players, max, ping      int
	}{
		{"Friday Fragfest", "Halo 2", "SpartanChief", "NA-East", 4, 8, 32},
		{"Sky Pirates", "Crimson Skies", "AcePilot99", "EU-West", 2, 4, 45},
		{"Mech Madness", "MechAssault", "MechWarrior", "NA-West", 3, 4, 78},
		{"LAN Party", "Halo: CE", "RetroGamer42", "NA-East", 6, 16, 21},
		{"Spies vs Mercs", "Splinter Cell: CT", "ShadowAgent", "NA-East", 2, 8, 38},
	}

	var lobbies []LobbyInfo
	for i, g := range games {
		code := fmt.Sprintf("NACHO-%04d", rand.Intn(10000))
		lobbies = append(lobbies, LobbyInfo{
			ID:         fmt.Sprintf("demo-%d", i),
			Name:       g.name,
			Game:       g.game,
			Host:       g.host,
			Players:    g.players,
			MaxPlayers: g.max,
			Ping:       g.ping,
			Region:     g.region,
			Code:       code,
			Members: []PlayerInfo{
				{Name: g.host, Ping: g.ping, IsHost: true},
			},
		})
	}
	return lobbies
}
