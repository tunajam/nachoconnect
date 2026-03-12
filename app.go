package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/tunajam/nachoconnect/internal/capture"
	"github.com/tunajam/nachoconnect/internal/lobby"
	"github.com/tunajam/nachoconnect/internal/tunnel"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct - main application controller
type App struct {
	ctx       context.Context
	cancel    context.CancelFunc
	capturer  *capture.Capturer
	tunnel    *tunnel.Manager
	lobbyMgr  *lobby.Manager
	mu        sync.RWMutex
	xboxFound bool
	xboxMAC   string
	status    AppStatus
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
	Name string `json:"name"`
	IP   string `json:"ip"`
	MAC  string `json:"mac"`
}

func NewApp() *App {
	return &App{
		lobbyMgr: lobby.NewManager(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.tunnel = tunnel.NewManager()

	// Start Xbox detection in background
	go a.detectXbox()

	// Get local IP
	go a.detectLocalIP()
}

func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	if a.capturer != nil {
		a.capturer.Stop()
	}
	if a.tunnel != nil {
		a.tunnel.Close()
	}
}

// GetStatus returns current app status
func (a *App) GetStatus() AppStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// GetInterfaces returns available network interfaces
func (a *App) GetInterfaces() []NetworkInterface {
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

// SelectInterface sets the capture interface
func (a *App) SelectInterface(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.capturer != nil {
		a.capturer.Stop()
	}

	cap, err := capture.NewCapturer(name)
	if err != nil {
		return fmt.Errorf("failed to open interface %s: %w", name, err)
	}
	a.capturer = cap
	a.status.Interface = name

	// Start capture loop
	go a.captureLoop()

	return nil
}

// GetLobbies returns available lobbies
func (a *App) GetLobbies() []LobbyInfo {
	lobbies := a.lobbyMgr.ListLobbies()
	var result []LobbyInfo
	for _, l := range lobbies {
		result = append(result, lobbyToInfo(l))
	}
	// If empty, return demo lobbies for UI testing
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

// JoinLobby joins a lobby by code
func (a *App) JoinLobby(code string) (*LobbyInfo, error) {
	l, err := a.lobbyMgr.JoinLobby(code, "NachoPlayer")
	if err != nil {
		return nil, err
	}

	// Start tunnel to lobby host
	if a.tunnel != nil {
		go a.startTunnel(l)
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
		a.tunnel.Close()
		a.tunnel = tunnel.NewManager()
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

func (a *App) detectXbox() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			// In a real implementation, we'd sniff for Xbox broadcast packets
			// For now, simulate detection based on network scan
			a.mu.Lock()
			if !a.xboxFound {
				// Check for Xbox-like devices (OUI 00:50:F2 = Microsoft Xbox)
				// This is a simplified check - real implementation would use pcap
				a.status.XboxDetected = false
			}
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "status:update", a.status)
		}
	}
}

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

func (a *App) captureLoop() {
	if a.capturer == nil {
		return
	}

	packets := a.capturer.Packets()
	for {
		select {
		case <-a.ctx.Done():
			return
		case pkt, ok := <-packets:
			if !ok {
				return
			}
			// Check if this is an Xbox system link packet
			if capture.IsXboxPacket(pkt) {
				a.mu.Lock()
				if !a.xboxFound {
					a.xboxFound = true
					a.xboxMAC = capture.ExtractMAC(pkt)
					a.status.XboxDetected = true
					a.status.XboxMAC = a.xboxMAC
					runtime.EventsEmit(a.ctx, "xbox:detected", a.xboxMAC)
				}
				a.mu.Unlock()

				// Forward to tunnel if active
				if a.tunnel != nil && a.tunnel.IsActive() {
					a.tunnel.Send(pkt)
				}
			}
		}
	}
}

func (a *App) startTunnel(l *lobby.Lobby) {
	a.mu.Lock()
	a.status.TunnelActive = true
	a.status.Connected = true
	a.status.PeerCount = len(l.Members)
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "status:update", a.status)
	runtime.EventsEmit(a.ctx, "tunnel:connected", nil)

	// In a real implementation, establish Noise Protocol tunnel here
	// For MVP, we simulate the connection
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
