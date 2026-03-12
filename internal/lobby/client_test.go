package lobby

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// testServer is a minimal in-process lobby server for testing
type testServer struct {
	mu      sync.RWMutex
	lobbies map[string]*ServerLobby
}

func newTestServer() (*httptest.Server, *testServer) {
	ts := &testServer{lobbies: make(map[string]*ServerLobby)}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/lobbies/create", ts.handleCreate)
	mux.HandleFunc("/api/lobbies/join", ts.handleJoin)
	mux.HandleFunc("/api/lobbies/leave", ts.handleLeave)
	mux.HandleFunc("/api/lobbies/ping", ts.handlePing)
	mux.HandleFunc("/api/lobbies/", ts.handleGetLobby)
	mux.HandleFunc("/api/lobbies", ts.handleList)
	return httptest.NewServer(mux), ts
}

func (ts *testServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name, Game, Host string
		MaxPlayers       int
	}
	json.NewDecoder(r.Body).Decode(&req)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	id := randID()
	code := "NACHO-" + randID()[:4]
	l := &ServerLobby{
		ID: id, Name: req.Name, Game: req.Game, Host: req.Host,
		MaxPlayers: req.MaxPlayers, Code: code, Region: "Test",
		HostPublicIP: "", HostPort: 0,
		Players:   []ServerPlayer{{Name: req.Host, IsHost: true, JoinedAt: time.Now()}},
		CreatedAt: time.Now(),
	}
	ts.lobbies[id] = l
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(l)
}

func (ts *testServer) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req struct{ Code, Player string }
	json.NewDecoder(r.Body).Decode(&req)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, l := range ts.lobbies {
		if l.Code == req.Code {
			if len(l.Players) >= l.MaxPlayers {
				http.Error(w, "full", http.StatusConflict)
				return
			}
			l.Players = append(l.Players, ServerPlayer{Name: req.Player, JoinedAt: time.Now()})
			json.NewEncoder(w).Encode(l)
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (ts *testServer) handleLeave(w http.ResponseWriter, r *http.Request) {
	var req struct{ LobbyID, Player string }
	json.NewDecoder(r.Body).Decode(&req)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if l, ok := ts.lobbies[req.LobbyID]; ok {
		for i, p := range l.Players {
			if p.Name == req.Player {
				l.Players = append(l.Players[:i], l.Players[i+1:]...)
				break
			}
		}
		if len(l.Players) == 0 {
			delete(ts.lobbies, req.LobbyID)
		}
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (ts *testServer) handlePing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LobbyID, Player string
		Ping            int
	}
	json.NewDecoder(r.Body).Decode(&req)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if l, ok := ts.lobbies[req.LobbyID]; ok {
		for i, p := range l.Players {
			if p.Name == req.Player {
				l.Players[i].Ping = req.Ping
				break
			}
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (ts *testServer) handleGetLobby(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/lobbies/")
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if l, ok := ts.lobbies[id]; ok {
		json.NewEncoder(w).Encode(l)
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (ts *testServer) handleList(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	var list []*ServerLobby
	for _, l := range ts.lobbies {
		list = append(list, l)
	}
	if list == nil {
		list = []*ServerLobby{}
	}
	json.NewEncoder(w).Encode(list)
}

func randID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func TestClientCreateAndList(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	// Initially empty
	lobbies, err := c.ListLobbies()
	if err != nil {
		t.Fatalf("ListLobbies: %v", err)
	}
	if len(lobbies) != 0 {
		t.Errorf("expected 0, got %d", len(lobbies))
	}

	// Create
	l, err := c.CreateLobby("Halo Room", "Halo 2", "Player1", 4, "", 0)
	if err != nil {
		t.Fatalf("CreateLobby: %v", err)
	}
	if l.Name != "Halo Room" {
		t.Errorf("Name: %q", l.Name)
	}
	if l.Host != "Player1" {
		t.Errorf("Host: %q", l.Host)
	}
	if len(l.Players) != 1 {
		t.Errorf("Players: %d", len(l.Players))
	}

	// List should have one
	lobbies, _ = c.ListLobbies()
	if len(lobbies) != 1 {
		t.Errorf("expected 1, got %d", len(lobbies))
	}
}

func TestClientJoinLobby(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	l, _ := c.CreateLobby("Room", "Game", "Host", 2, "", 0)

	joined, err := c.JoinLobby(l.Code, "Guest")
	if err != nil {
		t.Fatalf("JoinLobby: %v", err)
	}
	if len(joined.Players) != 2 {
		t.Errorf("Players: %d, want 2", len(joined.Players))
	}

	// Full
	_, err = c.JoinLobby(l.Code, "Third")
	if err == nil {
		t.Error("expected full error")
	}
}

func TestClientJoinLobbyNotFound(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	_, err := c.JoinLobby("NACHO-0000", "Player")
	if err == nil {
		t.Error("expected not found error")
	}
}

func TestClientLeaveLobby(t *testing.T) {
	srv, ts := newTestServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	l, _ := c.CreateLobby("Room", "Game", "Host", 4, "", 0)
	c.JoinLobby(l.Code, "Guest")

	err := c.LeaveLobby(l.ID, "Guest")
	if err != nil {
		t.Fatalf("LeaveLobby: %v", err)
	}

	ts.mu.RLock()
	lobby := ts.lobbies[l.ID]
	ts.mu.RUnlock()
	if lobby == nil {
		t.Fatal("lobby should still exist")
	}
	if len(lobby.Players) != 1 {
		t.Errorf("Players: %d, want 1", len(lobby.Players))
	}
}

func TestClientGetLobby(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	l, _ := c.CreateLobby("Room", "Game", "Host", 4, "", 0)

	got, err := c.GetLobby(l.ID)
	if err != nil {
		t.Fatalf("GetLobby: %v", err)
	}
	if got.Name != "Room" {
		t.Errorf("Name: %q", got.Name)
	}

	// Not found
	_, err = c.GetLobby("nonexistent")
	if err == nil {
		t.Error("expected error for unknown lobby")
	}
}

func TestClientUpdatePing(t *testing.T) {
	srv, ts := newTestServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	l, _ := c.CreateLobby("Room", "Game", "Host", 4, "", 0)

	err := c.UpdatePing(l.ID, "Host", 42)
	if err != nil {
		t.Fatalf("UpdatePing: %v", err)
	}

	ts.mu.RLock()
	lobby := ts.lobbies[l.ID]
	ts.mu.RUnlock()
	if lobby.Players[0].Ping != 42 {
		t.Errorf("Ping: got %d, want 42", lobby.Players[0].Ping)
	}
}

func TestClientPing(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	d, err := c.Ping()
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
}

func TestClientServerDown(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // nothing listening

	_, err := c.ListLobbies()
	if err == nil {
		t.Error("expected error when server is down")
	}

	_, err = c.CreateLobby("x", "x", "x", 1, "", 0)
	if err == nil {
		t.Error("expected error when server is down")
	}

	_, err = c.Ping()
	if err == nil {
		t.Error("expected error when server is down")
	}
}
