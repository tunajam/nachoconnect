package lobby

import (
	"testing"
)

func TestManagerCreateAndGet(t *testing.T) {
	m := NewManager()
	l, err := m.CreateLobby("Test Room", "Halo 2", 4, "Host1")
	if err != nil {
		t.Fatalf("CreateLobby: %v", err)
	}

	if l.Name != "Test Room" {
		t.Errorf("Name: got %q", l.Name)
	}
	if l.Game != "Halo 2" {
		t.Errorf("Game: got %q", l.Game)
	}
	if l.Host != "Host1" {
		t.Errorf("Host: got %q", l.Host)
	}
	if l.MaxPlayers != 4 {
		t.Errorf("MaxPlayers: got %d", l.MaxPlayers)
	}
	if len(l.Members) != 1 {
		t.Fatalf("Members: got %d", len(l.Members))
	}
	if !l.Members[0].IsHost {
		t.Error("first member should be host")
	}

	// Get by ID
	got := m.GetLobby(l.ID)
	if got == nil || got.ID != l.ID {
		t.Error("GetLobby should find the lobby")
	}

	// Get non-existent
	if m.GetLobby("nope") != nil {
		t.Error("should return nil for unknown ID")
	}
}

func TestManagerJoinLobby(t *testing.T) {
	m := NewManager()
	l, _ := m.CreateLobby("Room", "Game", 2, "Host")

	joined, err := m.JoinLobby(l.Code, "Player2")
	if err != nil {
		t.Fatalf("JoinLobby: %v", err)
	}
	if len(joined.Members) != 2 {
		t.Errorf("Members: got %d, want 2", len(joined.Members))
	}

	// Full lobby
	_, err = m.JoinLobby(l.Code, "Player3")
	if err == nil {
		t.Error("expected error joining full lobby")
	}
}

func TestManagerJoinLobbyNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.JoinLobby("NACHO-9999", "Player")
	if err == nil {
		t.Error("expected error for unknown code")
	}
}

func TestManagerLeaveLobby(t *testing.T) {
	m := NewManager()
	l, _ := m.CreateLobby("Room", "Game", 4, "Host")
	m.JoinLobby(l.Code, "Player2")

	m.LeaveLobby(l.ID, "Player2")
	got := m.GetLobby(l.ID)
	if got == nil {
		t.Fatal("lobby should still exist")
	}
	if len(got.Members) != 1 {
		t.Errorf("Members: got %d, want 1", len(got.Members))
	}

	// Leave last player → lobby deleted
	m.LeaveLobby(l.ID, "Host")
	if m.GetLobby(l.ID) != nil {
		t.Error("lobby should be deleted when empty")
	}
}

func TestManagerLeaveNonExistent(t *testing.T) {
	m := NewManager()
	// Should not panic
	m.LeaveLobby("nope", "Player")
}

func TestManagerListLobbies(t *testing.T) {
	m := NewManager()
	if lobbies := m.ListLobbies(); len(lobbies) != 0 {
		t.Errorf("expected 0 lobbies, got %d", len(lobbies))
	}

	m.CreateLobby("A", "Game", 4, "P1")
	m.CreateLobby("B", "Game", 4, "P2")

	lobbies := m.ListLobbies()
	if len(lobbies) != 2 {
		t.Errorf("expected 2 lobbies, got %d", len(lobbies))
	}
}

func TestGenerateCode(t *testing.T) {
	code := generateCode()
	if len(code) < 6 {
		t.Errorf("code too short: %q", code)
	}
	if code[:6] != "NACHO-" {
		t.Errorf("code should start with NACHO-, got %q", code)
	}
}
