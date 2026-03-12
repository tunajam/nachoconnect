package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub is a relay that forwards packets between peers.
// Supports both UDP (direct) and WebSocket (NAT-friendly) modes.
// When a peer sends a packet, it's forwarded to all other known peers.

type Peer struct {
	Addr     *net.UDPAddr
	WS       *websocket.Conn
	LastSeen time.Time
	LobbyID  string
}

type Hub struct {
	mu      sync.RWMutex
	peers   map[string]*Peer
	timeout time.Duration
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewHub(timeout time.Duration) *Hub {
	return &Hub{
		peers:   make(map[string]*Peer),
		timeout: timeout,
	}
}

// RunUDP starts the UDP relay (for direct connections)
func (h *Hub) RunUDP(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	conn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer conn.Close()

	log.Printf("🧀 NachoConnect Hub (UDP) listening on %s", addr)

	if h.timeout > 0 {
		go h.expireLoop()
	}

	buf := make([]byte, 65536)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}

		key := "udp:" + from.String()

		h.mu.Lock()
		peer, exists := h.peers[key]
		if !exists {
			peer = &Peer{Addr: from}
			h.peers[key] = peer
			log.Printf("UDP peer connected: %s (total: %d)", key, len(h.peers))
		}
		peer.LastSeen = time.Now()
		h.mu.Unlock()

		data := make([]byte, n)
		copy(data, buf[:n])

		h.mu.RLock()
		for peerKey, p := range h.peers {
			if peerKey != key {
				if p.Addr != nil {
					conn.WriteToUDP(data, p.Addr)
				}
				if p.WS != nil {
					p.WS.WriteMessage(websocket.BinaryMessage, data)
				}
			}
		}
		h.mu.RUnlock()
	}
}

// HandleWebSocket handles a WebSocket connection for relay
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	lobbyID := r.URL.Query().Get("lobby")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	key := "ws:" + conn.RemoteAddr().String()

	h.mu.Lock()
	h.peers[key] = &Peer{
		WS:       conn,
		LastSeen: time.Now(),
		LobbyID:  lobbyID,
	}
	peerCount := len(h.peers)
	h.mu.Unlock()

	log.Printf("WebSocket peer connected: %s lobby=%s (total: %d)", key, lobbyID, peerCount)

	defer func() {
		h.mu.Lock()
		delete(h.peers, key)
		h.mu.Unlock()
		conn.Close()
		log.Printf("WebSocket peer disconnected: %s", key)
	}()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if msgType != websocket.BinaryMessage {
			continue
		}

		h.mu.Lock()
		if p, ok := h.peers[key]; ok {
			p.LastSeen = time.Now()
		}
		h.mu.Unlock()

		// Forward to all other peers in the same lobby (or all peers if no lobby filtering)
		h.mu.RLock()
		for peerKey, p := range h.peers {
			if peerKey == key {
				continue
			}
			// If lobby filtering is active, only relay to same lobby
			if lobbyID != "" && p.LobbyID != "" && p.LobbyID != lobbyID {
				continue
			}
			if p.WS != nil {
				p.WS.WriteMessage(websocket.BinaryMessage, data)
			}
			if p.Addr != nil {
				// Can't send UDP from here without the UDP conn
			}
		}
		h.mu.RUnlock()
	}
}

func (h *Hub) expireLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.mu.Lock()
		now := time.Now()
		for key, peer := range h.peers {
			if now.Sub(peer.LastSeen) > h.timeout {
				log.Printf("peer expired: %s", key)
				if peer.WS != nil {
					peer.WS.Close()
				}
				delete(h.peers, key)
			}
		}
		h.mu.Unlock()
	}
}

func main() {
	host := flag.String("host", "0.0.0.0", "bind address")
	port := flag.Int("port", 1337, "UDP port")
	wsPort := flag.Int("ws-port", 1338, "WebSocket port (0 to disable)")
	timeout := flag.Int("timeout", 60, "peer inactivity timeout in seconds (0 = no timeout)")
	flag.Parse()

	var t time.Duration
	if *timeout > 0 {
		t = time.Duration(*timeout) * time.Second
	}

	hub := NewHub(t)

	// Start WebSocket server
	if *wsPort > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/relay", hub.HandleWebSocket)
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"status":"ok","peers":%d}`, len(hub.peers))
		})

		go func() {
			wsAddr := fmt.Sprintf("%s:%d", *host, *wsPort)
			log.Printf("🧀 NachoConnect Hub (WebSocket) listening on %s", wsAddr)
			if err := http.ListenAndServe(wsAddr, mux); err != nil {
				log.Printf("WebSocket server error: %v", err)
			}
		}()
	}

	// Start UDP relay
	udpAddr := fmt.Sprintf("%s:%d", *host, *port)
	if err := hub.Run(udpAddr); err != nil {
		log.Fatal(err)
	}
}

// Run starts the UDP relay (alias for RunUDP)
func (h *Hub) Run(addr string) error {
	return h.RunUDP(addr)
}
