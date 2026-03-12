package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Server is the NachoConnect lobby/matchmaking server
type Server struct {
	mu      sync.RWMutex
	lobbies map[string]*ServerLobby
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
	Players    []ServerPlayer `json:"players"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type ServerPlayer struct {
	Name     string `json:"name"`
	Addr     string `json:"addr,omitempty"`
	IsHost   bool   `json:"isHost"`
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

func NewServer() *Server {
	return &Server{
		lobbies: make(map[string]*ServerLobby),
	}
}

func main() {
	s := NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/lobbies", s.handleLobbies)
	mux.HandleFunc("/api/lobbies/create", s.handleCreateLobby)
	mux.HandleFunc("/api/lobbies/join", s.handleJoinLobby)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "0.1.0"})
	})

	// CORS middleware
	handler := corsMiddleware(mux)

	port := 8420
	log.Printf("🧀 NachoConnect server starting on :%d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handler); err != nil {
		log.Fatal(err)
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
	code := fmt.Sprintf("NACHO-%04d", rand.Intn(10000))

	lobby := &ServerLobby{
		ID:         id,
		Name:       req.Name,
		Game:       req.Game,
		Host:       req.Host,
		HostAddr:   r.RemoteAddr,
		MaxPlayers: req.MaxPlayers,
		Code:       code,
		Region:     "Auto",
		Players: []ServerPlayer{
			{Name: req.Host, Addr: r.RemoteAddr, IsHost: true, JoinedAt: time.Now()},
		},
		CreatedAt: time.Now(),
	}

	s.lobbies[id] = lobby

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

	for _, l := range s.lobbies {
		if l.Code == req.Code {
			if len(l.Players) >= l.MaxPlayers {
				http.Error(w, "lobby full", http.StatusConflict)
				return
			}
			l.Players = append(l.Players, ServerPlayer{
				Name:     req.Player,
				Addr:     r.RemoteAddr,
				IsHost:   false,
				JoinedAt: time.Now(),
			})

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(l)
			return
		}
	}

	http.Error(w, "lobby not found", http.StatusNotFound)
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
