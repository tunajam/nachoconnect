package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Server is the NachoConnect lobby/matchmaking server with integrated WebSocket relay
type Server struct {
	mu          sync.RWMutex
	lobbies     map[string]*ServerLobby
	nextHubPort int
	relayMu     sync.RWMutex
	relayPeers  map[string]*RelayPeer
}

type ServerLobby struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Game       string         `json:"game"`
	Host       string         `json:"host"`
	HostAddr   string         `json:"hostAddr,omitempty"`
	MaxPlayers int            `json:"maxPlayers"`
	Code       string         `json:"code"`
	Region     string         `json:"region"`
	HubAddr    string         `json:"hubAddr"`
	HubPort    int            `json:"hubPort"`
	Players    []ServerPlayer `json:"players"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type ServerPlayer struct {
	Name     string    `json:"name"`
	Addr     string    `json:"addr,omitempty"`
	IsHost   bool      `json:"isHost"`
	Ping     int       `json:"ping"`
	JoinedAt time.Time `json:"joinedAt"`
}

type CreateLobbyReq struct {
	Name       string `json:"name"`
	Game       string `json:"game"`
	Host       string `json:"host"`
	MaxPlayers int    `json:"maxPlayers"`
}

type JoinLobbyReq struct {
	Code   string `json:"code"`
	Player string `json:"player"`
}

type LeaveLobbyReq struct {
	LobbyID string `json:"lobbyId"`
	Player  string `json:"player"`
}

type PingUpdateReq struct {
	LobbyID string `json:"lobbyId"`
	Player  string `json:"player"`
	Ping    int    `json:"ping"`
}

// RelayPeer is a WebSocket peer in the relay
type RelayPeer struct {
	Conn     *websocket.Conn
	LobbyID  string
	LastSeen time.Time
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewServer() *Server {
	return &Server{
		lobbies:     make(map[string]*ServerLobby),
		nextHubPort: 1338,
		relayPeers:  make(map[string]*RelayPeer),
	}
}

// normalizeCode uppercases and trims an invite code for case-insensitive matching
func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// findLobbyByCode finds a lobby by its invite code (case-insensitive)
func (s *Server) findLobbyByCode(code string) *ServerLobby {
	norm := normalizeCode(code)
	for _, l := range s.lobbies {
		if l.Code == norm {
			return l
		}
	}
	return nil
}

func main() {
	s := NewServer()

	// Start lobby expiry goroutine (clean up lobbies with no players after 5 minutes)
	go s.expiryLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/lobbies", s.handleLobbies)
	mux.HandleFunc("/api/lobbies/create", s.handleCreateLobby)
	mux.HandleFunc("/api/lobbies/join", s.handleJoinLobby)
	mux.HandleFunc("/api/lobbies/leave", s.handleLeaveLobby)
	mux.HandleFunc("/api/lobbies/ping", s.handlePingUpdate)
	mux.HandleFunc("/api/lobbies/", s.handleGetLobby) // /api/lobbies/<id>
	mux.HandleFunc("/api/code/", s.handleGetByCode)   // /api/code/<code>
	mux.HandleFunc("/api/join/", s.handleJoinByCode)   // /api/join/<code>?player=X
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "0.1.0"})
	})
	mux.HandleFunc("/relay", s.handleRelay)

	// CORS middleware
	handler := corsMiddleware(mux)

	port := 8420
	log.Printf("🧀 NachoConnect server starting on :%d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handler); err != nil {
		log.Fatal(err)
	}
}

func (s *Server) expiryLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, l := range s.lobbies {
			if len(l.Players) == 0 && now.Sub(l.CreatedAt) > 5*time.Minute {
				log.Printf("expiring empty lobby: %s (%s)", l.Name, id)
				delete(s.lobbies, id)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) handleLobbies(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var lobbies []*ServerLobby
	for _, l := range s.lobbies {
		lobbies = append(lobbies, l)
	}
	if lobbies == nil {
		lobbies = []*ServerLobby{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lobbies)
}

func (s *Server) handleCreateLobby(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateLobbyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("lobby-%s", randString(8))
	code := s.generateUniqueCode()

	// Assign a hub port for this lobby
	// In production, all lobbies share the same hub on port 1337 using lobby ID routing
	// For now, assign sequential ports (the hub server will be updated to support multi-lobby)
	hubPort := 1337 // Use single hub port — hub handles all lobbies
	hubAddr := "nachoconnect-server.gentlepebble-471fc641.westus2.azurecontainerapps.io"

	lobby := &ServerLobby{
		ID:         id,
		Name:       req.Name,
		Game:       req.Game,
		Host:       req.Host,
		HostAddr:   r.RemoteAddr,
		MaxPlayers: req.MaxPlayers,
		Code:       code,
		Region:     "Auto",
		HubAddr:    hubAddr,
		HubPort:    hubPort,
		Players: []ServerPlayer{
			{Name: req.Host, Addr: r.RemoteAddr, IsHost: true, JoinedAt: time.Now()},
		},
		CreatedAt: time.Now(),
	}

	s.lobbies[id] = lobby
	log.Printf("lobby created: %s (%s) by %s", lobby.Name, id, req.Host)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lobby)
}

func (s *Server) handleJoinLobby(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JoinLobbyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	l := s.findLobbyByCode(req.Code)
	if l == nil {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}
	if len(l.Players) >= l.MaxPlayers {
		http.Error(w, "lobby full", http.StatusConflict)
		return
	}
	// Check if player already in lobby
	for _, p := range l.Players {
		if p.Name == req.Player {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(l)
			return
		}
	}
	l.Players = append(l.Players, ServerPlayer{
		Name:     req.Player,
		Addr:     r.RemoteAddr,
		IsHost:   false,
		JoinedAt: time.Now(),
	})
	log.Printf("player %s joined lobby %s (%s)", req.Player, l.Name, l.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(l)
}

func (s *Server) handleLeaveLobby(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LeaveLobbyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lobbies[req.LobbyID]
	if !exists {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}

	for i, p := range l.Players {
		if p.Name == req.Player {
			l.Players = append(l.Players[:i], l.Players[i+1:]...)
			log.Printf("player %s left lobby %s (%s)", req.Player, l.Name, l.ID)
			break
		}
	}

	// If host left, promote next player or delete lobby
	if len(l.Players) == 0 {
		delete(s.lobbies, req.LobbyID)
		log.Printf("lobby deleted (empty): %s", req.LobbyID)
	} else if req.Player == l.Host {
		l.Players[0].IsHost = true
		l.Host = l.Players[0].Name
		log.Printf("new host for %s: %s", l.Name, l.Host)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handlePingUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PingUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lobbies[req.LobbyID]
	if !exists {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}

	for i, p := range l.Players {
		if p.Name == req.Player {
			l.Players[i].Ping = req.Ping
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleGetLobby(w http.ResponseWriter, r *http.Request) {
	// Extract lobby ID from path: /api/lobbies/<id>
	path := strings.TrimPrefix(r.URL.Path, "/api/lobbies/")
	if path == "" || path == r.URL.Path {
		http.Error(w, "lobby ID required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	l, exists := s.lobbies[path]
	if !exists {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(l)
}

func (s *Server) handleRelay(w http.ResponseWriter, r *http.Request) {
	lobbyID := r.URL.Query().Get("lobby")
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	key := conn.RemoteAddr().String()

	s.relayMu.Lock()
	s.relayPeers[key] = &RelayPeer{
		Conn:     conn,
		LobbyID:  lobbyID,
		LastSeen: time.Now(),
	}
	count := len(s.relayPeers)
	s.relayMu.Unlock()

	log.Printf("relay peer connected: %s lobby=%s (total: %d)", key, lobbyID, count)

	defer func() {
		s.relayMu.Lock()
		delete(s.relayPeers, key)
		s.relayMu.Unlock()
		conn.Close()
		log.Printf("relay peer disconnected: %s", key)
	}()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if msgType != websocket.BinaryMessage {
			continue
		}

		s.relayMu.Lock()
		if p, ok := s.relayPeers[key]; ok {
			p.LastSeen = time.Now()
		}
		s.relayMu.Unlock()

		// Forward to all other peers in the same lobby
		s.relayMu.RLock()
		for peerKey, p := range s.relayPeers {
			if peerKey == key {
				continue
			}
			if lobbyID != "" && p.LobbyID != "" && p.LobbyID != lobbyID {
				continue
			}
			_ = p.Conn.WriteMessage(websocket.BinaryMessage, data)
		}
		s.relayMu.RUnlock()
	}
}

// handleGetByCode returns lobby info for a given invite code: GET /api/code/<code>
func (s *Server) handleGetByCode(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/api/code/")
	if code == "" {
		http.Error(w, "code required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	l := s.findLobbyByCode(code)
	if l == nil {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(l)
}

// handleJoinByCode joins a lobby by invite code via GET: GET /api/join/<code>?player=<name>
func (s *Server) handleJoinByCode(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/api/join/")
	player := r.URL.Query().Get("player")
	if code == "" || player == "" {
		http.Error(w, "code and player query param required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	l := s.findLobbyByCode(code)
	if l == nil {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}
	if len(l.Players) >= l.MaxPlayers {
		http.Error(w, "lobby full", http.StatusConflict)
		return
	}
	for _, p := range l.Players {
		if p.Name == player {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(l)
			return
		}
	}
	l.Players = append(l.Players, ServerPlayer{
		Name:     player,
		Addr:     r.RemoteAddr,
		IsHost:   false,
		JoinedAt: time.Now(),
	})
	log.Printf("player %s joined lobby %s via code %s", player, l.Name, code)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(l)
}

// generateUniqueCode creates a NACHO-XXXX code with 4 alphanumeric chars, ensuring uniqueness
func (s *Server) generateUniqueCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1 to avoid confusion
	for attempt := 0; attempt < 100; attempt++ {
		b := make([]byte, 4)
		for i := range b {
			b[i] = chars[rand.Intn(len(chars))]
		}
		code := "NACHO-" + string(b)
		// Check uniqueness
		found := false
		for _, l := range s.lobbies {
			if l.Code == code {
				found = true
				break
			}
		}
		if !found {
			return code
		}
	}
	// Fallback: use longer code
	b := make([]byte, 6)
	const chars2 = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for i := range b {
		b[i] = chars2[rand.Intn(len(chars2))]
	}
	return "NACHO-" + string(b)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func randString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
