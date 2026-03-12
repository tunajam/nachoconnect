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
)

// Server is the NachoConnect lobby directory (matchmaking only, no data relay).
// All game traffic flows directly between peers via Direct Connect P2P.
type Server struct {
	mu      sync.RWMutex
	lobbies map[string]*ServerLobby
}

type ServerLobby struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Game         string         `json:"game"`
	Host         string         `json:"host"`
	HostAddr     string         `json:"hostAddr,omitempty"`
	MaxPlayers   int            `json:"maxPlayers"`
	Code         string         `json:"code"`
	Region       string         `json:"region"`
	HostPublicIP string         `json:"hostPublicIP,omitempty"`
	HostPort     int            `json:"hostPort,omitempty"`
	Players      []ServerPlayer `json:"players"`
	CreatedAt    time.Time      `json:"createdAt"`
}

type ServerPlayer struct {
	Name     string    `json:"name"`
	Addr     string    `json:"addr,omitempty"`
	IsHost   bool      `json:"isHost"`
	Ping     int       `json:"ping"`
	JoinedAt time.Time `json:"joinedAt"`
}

type CreateLobbyReq struct {
	Name         string `json:"name"`
	Game         string `json:"game"`
	Host         string `json:"host"`
	MaxPlayers   int    `json:"maxPlayers"`
	HostPublicIP string `json:"hostPublicIP,omitempty"`
	HostPort     int    `json:"hostPort,omitempty"`
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

func NewServer() *Server {
	return &Server{
		lobbies: make(map[string]*ServerLobby),
	}
}

func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

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
	go s.expiryLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/lobbies", s.handleLobbies)
	mux.HandleFunc("/api/lobbies/create", s.handleCreateLobby)
	mux.HandleFunc("/api/lobbies/join", s.handleJoinLobby)
	mux.HandleFunc("/api/lobbies/leave", s.handleLeaveLobby)
	mux.HandleFunc("/api/lobbies/ping", s.handlePingUpdate)
	mux.HandleFunc("/api/lobbies/", s.handleGetLobby)
	mux.HandleFunc("/api/code/", s.handleGetByCode)
	mux.HandleFunc("/api/join/", s.handleJoinByCode)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "0.2.0"})
	})

	handler := corsMiddleware(mux)

	port := 8420
	log.Printf("🧀 NachoConnect lobby server starting on :%d (matchmaking only, no relay)", port)
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

	lobby := &ServerLobby{
		ID:           id,
		Name:         req.Name,
		Game:         req.Game,
		Host:         req.Host,
		HostAddr:     r.RemoteAddr,
		MaxPlayers:   req.MaxPlayers,
		Code:         code,
		Region:       "Auto",
		HostPublicIP: req.HostPublicIP,
		HostPort:     req.HostPort,
		Players: []ServerPlayer{
			{Name: req.Host, Addr: r.RemoteAddr, IsHost: true, JoinedAt: time.Now()},
		},
		CreatedAt: time.Now(),
	}

	s.lobbies[id] = lobby
	log.Printf("lobby created: %s (%s) by %s [%s:%d]", lobby.Name, id, req.Host, req.HostPublicIP, req.HostPort)

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

func (s *Server) generateUniqueCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for attempt := 0; attempt < 100; attempt++ {
		b := make([]byte, 4)
		for i := range b {
			b[i] = chars[rand.Intn(len(chars))]
		}
		code := "NACHO-" + string(b)
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
