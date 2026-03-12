package l2tunnel

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Ping/pong protocol constants.
// Packets: "NCHO" + type(1 byte) + timestamp(8 bytes, unix nano) = 13 bytes.
var pingMagic = []byte("NCHO")

const (
	pingTypePing byte = 0x01
	pingTypePong byte = 0x02
	pingPacketLen     = 4 + 1 + 8 // magic + type + timestamp
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

		// Handle ping/pong packets — respond directly, don't broadcast
		if n == pingPacketLen && isPingPacket(buf[:n]) {
			if buf[4] == pingTypePing {
				// Respond with pong, echoing the timestamp
				pong := make([]byte, pingPacketLen)
				copy(pong, pingMagic)
				pong[4] = pingTypePong
				copy(pong[5:], buf[5:n])
				h.conn.WriteToUDP(pong, from)
			}
			// Pong packets from peers are ignored by hub
			continue
		}

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

// isPingPacket checks if a buffer starts with the NCHO magic prefix.
func isPingPacket(buf []byte) bool {
	return len(buf) >= 5 &&
		buf[0] == pingMagic[0] && buf[1] == pingMagic[1] &&
		buf[2] == pingMagic[2] && buf[3] == pingMagic[3]
}

// BuildPingPacket creates a ping packet with the current timestamp.
func BuildPingPacket() []byte {
	pkt := make([]byte, pingPacketLen)
	copy(pkt, pingMagic)
	pkt[4] = pingTypePing
	binary.BigEndian.PutUint64(pkt[5:], uint64(time.Now().UnixNano()))
	return pkt
}

// ParsePongTimestamp extracts the original send timestamp from a pong packet.
// Returns 0 if the packet is not a valid pong.
func ParsePongTimestamp(buf []byte, n int) int64 {
	if n != pingPacketLen || !isPingPacket(buf) || buf[4] != pingTypePong {
		return 0
	}
	return int64(binary.BigEndian.Uint64(buf[5:13]))
}

// Conn returns the underlying UDP connection (used by host-side ping).
func (h *Hub) Conn() *net.UDPConn {
	return h.conn
}

// PeerAddrs returns a snapshot of all current peer addresses.
func (h *Hub) PeerAddrs() []*net.UDPAddr {
	h.mu.RLock()
	defer h.mu.RUnlock()
	addrs := make([]*net.UDPAddr, 0, len(h.peers))
	for _, p := range h.peers {
		addrs = append(addrs, p.addr)
	}
	return addrs
}

// PingAllPeers sends a ping to every connected peer and waits for pongs.
// Returns a map of peer address string → RTT in milliseconds.
func (h *Hub) PingAllPeers(timeout time.Duration) map[string]int {
	peers := h.PeerAddrs()
	if len(peers) == 0 {
		return nil
	}

	pkt := BuildPingPacket()
	sentAt := time.Now()

	for _, addr := range peers {
		h.conn.WriteToUDP(pkt, addr)
	}

	results := make(map[string]int)
	remaining := make(map[string]bool)
	for _, addr := range peers {
		remaining[addr.String()] = true
	}

	buf := make([]byte, pingPacketLen)
	deadline := sentAt.Add(timeout)
	for time.Now().Before(deadline) && len(remaining) > 0 {
		h.conn.SetReadDeadline(deadline)
		n, from, err := h.conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		ts := ParsePongTimestamp(buf, n)
		if ts == 0 {
			continue
		}
		key := from.String()
		if remaining[key] {
			rtt := time.Since(time.Unix(0, ts))
			results[key] = int(rtt.Milliseconds())
			delete(remaining, key)
		}
	}
	h.conn.SetReadDeadline(time.Time{}) // clear deadline
	return results
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
