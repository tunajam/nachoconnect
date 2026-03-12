package lobby

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ServerURL is the Azure lobby server base URL
var ServerURL = "https://nachoconnect-server.gentlepebble-471fc641.westus2.azurecontainerapps.io"

// Client communicates with the remote lobby server
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ServerLobby mirrors the server's lobby representation
type ServerLobby struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Game         string         `json:"game"`
	Host         string         `json:"host"`
	HostAddr     string         `json:"hostAddr,omitempty"`
	MaxPlayers   int            `json:"maxPlayers"`
	Code         string         `json:"code"`
	Region       string         `json:"region"`
	Mode         string         `json:"mode"`                   // "relay" or "direct"
	HubAddr      string         `json:"hubAddr,omitempty"`
	HubPort      int            `json:"hubPort,omitempty"`
	HostPublicIP string         `json:"hostPublicIP,omitempty"` // Direct mode: host's public IP
	HostPort     int            `json:"hostPort,omitempty"`     // Direct mode: host's UDP port
	Players      []ServerPlayer `json:"players"`
	CreatedAt    time.Time      `json:"createdAt"`
}

// ServerPlayer mirrors the server's player representation
type ServerPlayer struct {
	Name     string    `json:"name"`
	Addr     string    `json:"addr,omitempty"`
	IsHost   bool      `json:"isHost"`
	Ping     int       `json:"ping,omitempty"`
	JoinedAt time.Time `json:"joinedAt"`
}

type createLobbyReq struct {
	Name         string `json:"name"`
	Game         string `json:"game"`
	Host         string `json:"host"`
	MaxPlayers   int    `json:"maxPlayers"`
	Mode         string `json:"mode,omitempty"`         // "relay" or "direct"
	HostPublicIP string `json:"hostPublicIP,omitempty"` // Direct mode
	HostPort     int    `json:"hostPort,omitempty"`     // Direct mode
}

type joinLobbyReq struct {
	Code   string `json:"code"`
	Player string `json:"player"`
}

type leaveLobbyReq struct {
	LobbyID string `json:"lobbyId"`
	Player  string `json:"player"`
}

type pingUpdateReq struct {
	LobbyID string `json:"lobbyId"`
	Player  string `json:"player"`
	Ping    int    `json:"ping"`
}

// NewClient creates a new lobby server client
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = ServerURL
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ListLobbies fetches all lobbies from the remote server
func (c *Client) ListLobbies() ([]ServerLobby, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/lobbies")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lobbies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var lobbies []ServerLobby
	if err := json.NewDecoder(resp.Body).Decode(&lobbies); err != nil {
		return nil, fmt.Errorf("failed to decode lobbies: %w", err)
	}
	return lobbies, nil
}

// CreateLobby creates a new lobby on the remote server
func (c *Client) CreateLobby(name, game, host string, maxPlayers int) (*ServerLobby, error) {
	return c.CreateLobbyWithMode(name, game, host, maxPlayers, "relay", "", 0)
}

// CreateLobbyWithMode creates a lobby with a specific connection mode
func (c *Client) CreateLobbyWithMode(name, game, host string, maxPlayers int, mode, hostPublicIP string, hostPort int) (*ServerLobby, error) {
	if mode == "" {
		mode = "relay"
	}
	body, _ := json.Marshal(createLobbyReq{
		Name:         name,
		Game:         game,
		Host:         host,
		MaxPlayers:   maxPlayers,
		Mode:         mode,
		HostPublicIP: hostPublicIP,
		HostPort:     hostPort,
	})

	resp, err := c.httpClient.Post(c.baseURL+"/api/lobbies/create", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create lobby: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var lobby ServerLobby
	if err := json.NewDecoder(resp.Body).Decode(&lobby); err != nil {
		return nil, fmt.Errorf("failed to decode lobby: %w", err)
	}
	return &lobby, nil
}

// JoinLobby joins a lobby by code on the remote server
func (c *Client) JoinLobby(code, player string) (*ServerLobby, error) {
	body, _ := json.Marshal(joinLobbyReq{
		Code:   code,
		Player: player,
	})

	resp, err := c.httpClient.Post(c.baseURL+"/api/lobbies/join", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to join lobby: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("lobby is full")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("lobby not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var lobby ServerLobby
	if err := json.NewDecoder(resp.Body).Decode(&lobby); err != nil {
		return nil, fmt.Errorf("failed to decode lobby: %w", err)
	}
	return &lobby, nil
}

// LeaveLobby leaves a lobby on the remote server
func (c *Client) LeaveLobby(lobbyID, player string) error {
	body, _ := json.Marshal(leaveLobbyReq{
		LobbyID: lobbyID,
		Player:  player,
	})

	resp, err := c.httpClient.Post(c.baseURL+"/api/lobbies/leave", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to leave lobby: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// UpdatePing updates a player's ping on the remote server
func (c *Client) UpdatePing(lobbyID, player string, ping int) error {
	body, _ := json.Marshal(pingUpdateReq{
		LobbyID: lobbyID,
		Player:  player,
		Ping:    ping,
	})

	resp, err := c.httpClient.Post(c.baseURL+"/api/lobbies/ping", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetLobby fetches a single lobby by ID
func (c *Client) GetLobby(lobbyID string) (*ServerLobby, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/lobbies/" + lobbyID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lobby: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("lobby not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var lobby ServerLobby
	if err := json.NewDecoder(resp.Body).Decode(&lobby); err != nil {
		return nil, fmt.Errorf("failed to decode lobby: %w", err)
	}
	return &lobby, nil
}

// JoinByCode resolves an invite code and joins the lobby in one step
func (c *Client) JoinByCode(code string, gamertag string) (*ServerLobby, error) {
	return c.JoinLobby(strings.ToUpper(strings.TrimSpace(code)), gamertag)
}

// GetByCode resolves an invite code to lobby info without joining
func (c *Client) GetByCode(code string) (*ServerLobby, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/code/" + strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("lobby not found for code: %s", code)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var lobby ServerLobby
	if err := json.NewDecoder(resp.Body).Decode(&lobby); err != nil {
		return nil, fmt.Errorf("failed to decode lobby: %w", err)
	}
	return &lobby, nil
}

// Ping measures HTTP latency to the lobby server (as a proxy for real ping)
func (c *Client) Ping() (time.Duration, error) {
	start := time.Now()
	resp, err := c.httpClient.Get(c.baseURL + "/api/health")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return time.Since(start), nil
}
