package lobby

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Lobby represents a game lobby
type Lobby struct {
	ID         string
	Name       string
	Game       string
	Host       string
	MaxPlayers int
	Ping       int
	Region     string
	Code       string
	Members    []Member
	CreatedAt  time.Time
}

// Member represents a lobby member
type Member struct {
	Name   string
	Ping   int
	IsHost bool
	IsYou  bool
}

// Manager handles lobby operations (local for MVP, server-backed later)
type Manager struct {
	mu      sync.RWMutex
	lobbies map[string]*Lobby
}

// NewManager creates a new lobby manager
func NewManager() *Manager {
	return &Manager{
		lobbies: make(map[string]*Lobby),
	}
}

// CreateLobby creates a new lobby
func (m *Manager) CreateLobby(name, game string, maxPlayers int, playerName string) (*Lobby, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	code := generateCode()

	lobby := &Lobby{
		ID:         id,
		Name:       name,
		Game:       game,
		Host:       playerName,
		MaxPlayers: maxPlayers,
		Region:     "Local",
		Code:       code,
		Members: []Member{
			{Name: playerName, Ping: 0, IsHost: true, IsYou: true},
		},
		CreatedAt: time.Now(),
	}

	m.lobbies[id] = lobby
	return lobby, nil
}

// JoinLobby joins a lobby by code
func (m *Manager) JoinLobby(code, playerName string) (*Lobby, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find lobby by code
	for _, l := range m.lobbies {
		if l.Code == code {
			if len(l.Members) >= l.MaxPlayers {
				return nil, fmt.Errorf("lobby is full")
			}
			l.Members = append(l.Members, Member{
				Name:   playerName,
				Ping:   0,
				IsHost: false,
				IsYou:  true,
			})
			return l, nil
		}
	}

	return nil, fmt.Errorf("lobby not found with code: %s", code)
}

// LeaveLobby removes a player from a lobby
func (m *Manager) LeaveLobby(id, playerName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, exists := m.lobbies[id]
	if !exists {
		return
	}

	for i, member := range l.Members {
		if member.Name == playerName {
			l.Members = append(l.Members[:i], l.Members[i+1:]...)
			break
		}
	}

	// Delete lobby if empty
	if len(l.Members) == 0 {
		delete(m.lobbies, id)
	}
}

// GetLobby returns a lobby by ID
func (m *Manager) GetLobby(id string) *Lobby {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lobbies[id]
}

// ListLobbies returns all lobbies
func (m *Manager) ListLobbies() []*Lobby {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Lobby
	for _, l := range m.lobbies {
		result = append(result, l)
	}
	return result
}

func generateID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 12)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func generateCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return "NACHO-" + string(b)
}
