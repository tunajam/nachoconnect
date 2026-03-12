package l2tunnel

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Hub listens on a single UDP port and broadcasts frames between all connected peers.
// Used for Direct Connect mode where the host runs the hub locally.
type Hub struct {
	mu       sync.RWMutex
	conn     *net.UDPConn
	peers    map[string]*hubPeer
	port     int
	active   bool
	stopCh   chan struct{}
	timeout  time.Duration
}

type hubPeer struct {
	addr     *net.UDPAddr
	lastSeen time.Time
}

const (
	defaultPeerTimeout = 60 * time.Second
	hubBufSize         = 65536
)

// StartHub starts a UDP hub on the given port. Port 0 picks a random port.
func StartHub(port int) (*Hub, error) {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, fmt.Errorf("resolve addr: %w", err)
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}

	actualPort := conn.LocalAddr().(*net.UDPAddr).Port

	h := &Hub{
		conn:    conn,
		peers:   make(map[string]*hubPeer),
		port:    actualPort,
		active:  true,
		stopCh:  make(chan struct{}),
		timeout: defaultPeerTimeout,
	}

	go h.readLoop()
	go h.expireLoop()

	log.Printf("Hub listening on UDP :%d", actualPort)
	return h, nil
}

// Port returns the port the hub is listening on.
func (h *Hub) Port() int {
	return h.port
}

// IsActive returns whether the hub is running.
func (h *Hub) IsActive() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.active
}

// PeerCount returns current number of tracked peers.
func (h *Hub) PeerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.peers)
}

// Stop shuts down the hub.
func (h *Hub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.active {
		return
	}
	h.active = false
	close(h.stopCh)
	h.conn.Close()
}

func (h *Hub) readLoop() {
	buf := make([]byte, hubBufSize)
	for {
		n, from, err := h.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-h.stopCh:
				return
			default:
				log.Printf("Hub read error: %v", err)
				continue
			}
		}

		key := from.String()

		h.mu.Lock()
		p, exists := h.peers[key]
		if !exists {
			p = &hubPeer{addr: from}
			h.peers[key] = p
			log.Printf("Hub: new peer %s (total: %d)", key, len(h.peers))
		}
		p.lastSeen = time.Now()
		h.mu.Unlock()

		// Broadcast to all other peers
		data := make([]byte, n)
		copy(data, buf[:n])

		h.mu.RLock()
		for peerKey, peer := range h.peers {
			if peerKey != key {
				h.conn.WriteToUDP(data, peer.addr)
			}
		}
		h.mu.RUnlock()
	}
}

func (h *Hub) expireLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.mu.Lock()
			now := time.Now()
			for key, peer := range h.peers {
				if now.Sub(peer.lastSeen) > h.timeout {
					log.Printf("Hub: peer expired %s", key)
					delete(h.peers, key)
				}
			}
			h.mu.Unlock()
		}
	}
}
