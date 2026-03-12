package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Hub is a UDP relay that forwards packets between peers (Go rewrite of l2tunnel's hub.py)
// When a peer sends a packet, it's forwarded to all other known peers.
// Peers are registered on first packet. Optionally expire after inactivity.

type Peer struct {
	Addr     *net.UDPAddr
	LastSeen time.Time
}

type Hub struct {
	mu      sync.RWMutex
	peers   map[string]*Peer
	timeout time.Duration
}

func NewHub(timeout time.Duration) *Hub {
	return &Hub{
		peers:   make(map[string]*Peer),
		timeout: timeout,
	}
}

func (h *Hub) Run(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	conn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer conn.Close()

	log.Printf("🧀 NachoConnect Hub listening on %s", addr)

	// Expiry goroutine
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

		key := from.String()

		// Register or update peer
		h.mu.Lock()
		peer, exists := h.peers[key]
		if !exists {
			peer = &Peer{Addr: from}
			h.peers[key] = peer
			log.Printf("peer connected: %s (total: %d)", key, len(h.peers))
		}
		peer.LastSeen = time.Now()
		h.mu.Unlock()

		// Forward to all other peers
		data := make([]byte, n)
		copy(data, buf[:n])

		h.mu.RLock()
		for peerKey, p := range h.peers {
			if peerKey != key {
				conn.WriteToUDP(data, p.Addr)
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
				delete(h.peers, key)
			}
		}
		h.mu.Unlock()
	}
}

func main() {
	host := flag.String("host", "0.0.0.0", "bind address")
	port := flag.Int("port", 1337, "UDP port")
	timeout := flag.Int("timeout", 0, "peer inactivity timeout in seconds (0 = no timeout)")
	flag.Parse()

	var t time.Duration
	if *timeout > 0 {
		t = time.Duration(*timeout) * time.Second
	}

	hub := NewHub(t)
	addr := fmt.Sprintf("%s:%d", *host, *port)
	if err := hub.Run(addr); err != nil {
		log.Fatal(err)
	}
}
