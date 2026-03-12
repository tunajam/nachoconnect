package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateUniqueCode(t *testing.T) {
	s := NewServer()
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code := s.generateUniqueCode()
		if !strings.HasPrefix(code, "NACHO-") {
			t.Errorf("code %q does not start with NACHO-", code)
		}
		if len(code) != 10 { // NACHO- + 4 chars
			t.Errorf("code %q has unexpected length %d", code, len(code))
		}
		if codes[code] {
			t.Errorf("duplicate code generated: %s", code)
		}
		codes[code] = true
	}
}

func TestCodeCaseInsensitivity(t *testing.T) {
	s := NewServer()

	// Create a lobby
	body := `{"name":"Test","game":"Halo 2","host":"Player1","maxPlayers":4}`
	req := httptest.NewRequest("POST", "/api/lobbies/create", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleCreateLobby(w, req)

	if w.Code != 200 {
		t.Fatalf("create failed: %d", w.Code)
	}

	var lobby ServerLobby
	json.NewDecoder(w.Body).Decode(&lobby)
	code := lobby.Code

	// Try resolving with lowercase
	lowerCode := strings.ToLower(code)
	req2 := httptest.NewRequest("GET", "/api/code/"+lowerCode, nil)
	w2 := httptest.NewRecorder()
	s.handleGetByCode(w2, req2)

	if w2.Code != 200 {
		t.Errorf("lowercase code lookup failed: %d (code=%s)", w2.Code, lowerCode)
	}

	// Try with mixed case
	mixedCode := strings.ToLower(code[:3]) + strings.ToUpper(code[3:])
	req3 := httptest.NewRequest("GET", "/api/code/"+mixedCode, nil)
	w3 := httptest.NewRecorder()
	s.handleGetByCode(w3, req3)

	if w3.Code != 200 {
		t.Errorf("mixed case code lookup failed: %d (code=%s)", w3.Code, mixedCode)
	}
}

func TestCodeResolution(t *testing.T) {
	s := NewServer()

	// Invalid code
	req := httptest.NewRequest("GET", "/api/code/NACHO-ZZZZ", nil)
	w := httptest.NewRecorder()
	s.handleGetByCode(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid code, got %d", w.Code)
	}

	// Create lobby then resolve
	body := `{"name":"Test","game":"Halo 2","host":"Host","maxPlayers":4}`
	reqC := httptest.NewRequest("POST", "/api/lobbies/create", strings.NewReader(body))
	wC := httptest.NewRecorder()
	s.handleCreateLobby(wC, reqC)

	var lobby ServerLobby
	json.NewDecoder(wC.Body).Decode(&lobby)

	req2 := httptest.NewRequest("GET", "/api/code/"+lobby.Code, nil)
	w2 := httptest.NewRecorder()
	s.handleGetByCode(w2, req2)
	if w2.Code != 200 {
		t.Errorf("valid code lookup failed: %d", w2.Code)
	}

	var resolved ServerLobby
	json.NewDecoder(w2.Body).Decode(&resolved)
	if resolved.ID != lobby.ID {
		t.Errorf("resolved lobby ID mismatch: %s != %s", resolved.ID, lobby.ID)
	}
}

func TestJoinByCodeEndpoint(t *testing.T) {
	s := NewServer()

	// Create lobby
	body := `{"name":"Test","game":"Halo 2","host":"Host","maxPlayers":4}`
	reqC := httptest.NewRequest("POST", "/api/lobbies/create", strings.NewReader(body))
	wC := httptest.NewRecorder()
	s.handleCreateLobby(wC, reqC)

	var lobby ServerLobby
	json.NewDecoder(wC.Body).Decode(&lobby)

	// Join via GET endpoint
	req := httptest.NewRequest("GET", "/api/join/"+strings.ToLower(lobby.Code)+"?player=Joiner", nil)
	w := httptest.NewRecorder()
	s.handleJoinByCode(w, req)

	if w.Code != 200 {
		t.Fatalf("join by code failed: %d", w.Code)
	}

	var joined ServerLobby
	json.NewDecoder(w.Body).Decode(&joined)
	if len(joined.Players) != 2 {
		t.Errorf("expected 2 players, got %d", len(joined.Players))
	}
}
