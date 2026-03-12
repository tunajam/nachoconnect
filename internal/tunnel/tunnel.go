package tunnel

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/flynn/noise"
)

const (
	// Protocol constants
	MagicBytes    = 0x5842 // "XB"
	TypeData      = 0x01
	TypeBroadcast = 0x02
	TypeKeepalive = 0x03
	TypeControl   = 0x04

	// Header size: magic(2) + type(1) + flags(1) + lobbyID(4)
	HeaderSize = 8

	// Keepalive interval
	KeepaliveInterval = 5 * time.Second

	// Max packet size
	MaxPacketSize = 65535
)

// Header is the XBC tunnel protocol header
type Header struct {
	Magic   uint16
	Type    byte
	Flags   byte
	LobbyID uint32
}

// Manager handles tunnel connections to peers
type Manager struct {
	mu       sync.RWMutex
	conn     *net.UDPConn
	peers    map[string]*Peer
	lobbyID  uint32
	active   bool
	incoming chan []byte
	outgoing chan []byte
	done     chan struct{}
	cipher   *noise.CipherState
}

// Peer represents a connected peer
type Peer struct {
	Addr      *net.UDPAddr
	Name      string
	Ping      time.Duration
	LastSeen  time.Time
	SendCS    *noise.CipherState
	RecvCS    *noise.CipherState
	Connected bool
}

// NewManager creates a new tunnel manager
func NewManager() *Manager {
	return &Manager{
		peers:    make(map[string]*Peer),
		incoming: make(chan []byte, 256),
		outgoing: make(chan []byte, 256),
		done:     make(chan struct{}),
	}
}

// Listen starts listening for incoming tunnel connections
func (m *Manager) Listen(port int) error {
	addr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	m.mu.Lock()
	m.conn = conn
	m.active = true
	m.mu.Unlock()

	go m.readLoop()
	go m.keepaliveLoop()

	return nil
}

// Connect establishes a tunnel to a peer
func (m *Manager) Connect(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve peer address: %w", err)
	}

	if m.conn == nil {
		// Create a connection if we don't have one
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
		if err != nil {
			return fmt.Errorf("failed to create UDP socket: %w", err)
		}
		m.mu.Lock()
		m.conn = conn
		m.active = true
		m.mu.Unlock()

		go m.readLoop()
		go m.keepaliveLoop()
	}

	// Perform Noise handshake
	peer, err := m.handshake(udpAddr)
	if err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	m.mu.Lock()
	m.peers[addr] = peer
	m.mu.Unlock()

	return nil
}

// Send sends a raw packet to all connected peers
func (m *Manager) Send(data []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.active || m.conn == nil {
		return
	}

	frame := m.encodeFrame(TypeData, data)

	for _, peer := range m.peers {
		if peer.Connected {
			// Encrypt if we have a cipher
			var payload []byte
			if peer.SendCS != nil {
				var err error
				payload, err = peer.SendCS.Encrypt(nil, nil, frame)
				if err != nil {
					continue
				}
			} else {
				payload = frame
			}
			m.conn.WriteToUDP(payload, peer.Addr)
		}
	}
}

// Receive returns channel of incoming decrypted packets
func (m *Manager) Receive() <-chan []byte {
	return m.incoming
}

// IsActive returns whether the tunnel is active
func (m *Manager) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// PeerCount returns number of connected peers
func (m *Manager) PeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, p := range m.peers {
		if p.Connected {
			count++
		}
	}
	return count
}

// Close shuts down the tunnel
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.active = false
	select {
	case <-m.done:
	default:
		close(m.done)
	}

	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	m.peers = make(map[string]*Peer)
}

// Internal methods

func (m *Manager) readLoop() {
	buf := make([]byte, MaxPacketSize)
	for {
		select {
		case <-m.done:
			return
		default:
		}

		n, addr, err := m.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-m.done:
				return
			default:
				continue
			}
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		// Find peer and decrypt
		m.mu.RLock()
		peer, exists := m.peers[addr.String()]
		m.mu.RUnlock()

		if exists && peer.RecvCS != nil {
			decrypted, err := peer.RecvCS.Decrypt(nil, nil, data)
			if err == nil {
				data = decrypted
			}
		}

		// Parse header
		if len(data) < HeaderSize {
			continue
		}

		header := decodeHeader(data[:HeaderSize])
		if header.Magic != MagicBytes {
			continue
		}

		payload := data[HeaderSize:]

		switch header.Type {
		case TypeData, TypeBroadcast:
			if exists {
				peer.LastSeen = time.Now()
			}
			select {
			case m.incoming <- payload:
			default:
				// Drop if channel full
			}
		case TypeKeepalive:
			if exists {
				peer.LastSeen = time.Now()
			}
		}
	}
}

func (m *Manager) keepaliveLoop() {
	ticker := time.NewTicker(KeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.mu.RLock()
			for _, peer := range m.peers {
				if peer.Connected && m.conn != nil {
					frame := m.encodeFrame(TypeKeepalive, nil)
					m.conn.WriteToUDP(frame, peer.Addr)
				}
			}
			m.mu.RUnlock()
		}
	}
}

func (m *Manager) handshake(addr *net.UDPAddr) (*Peer, error) {
	// Noise IK handshake pattern
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)
	keypair, err := cs.GenerateKeypair(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   cs,
		Pattern:        noise.HandshakeNN,
		Initiator:      true,
		StaticKeypair:  keypair,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create handshake state: %w", err)
	}

	// Send handshake message 1
	msg1, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("handshake write 1 failed: %w", err)
	}

	m.conn.WriteToUDP(msg1, addr)

	// Read response
	buf := make([]byte, MaxPacketSize)
	m.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := m.conn.ReadFromUDP(buf)
	m.conn.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, fmt.Errorf("handshake read failed: %w", err)
	}

	_, sendCS, recvCS, err := hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("handshake read 2 failed: %w", err)
	}

	return &Peer{
		Addr:      addr,
		SendCS:    sendCS,
		RecvCS:    recvCS,
		Connected: true,
		LastSeen:  time.Now(),
	}, nil
}

func (m *Manager) encodeFrame(msgType byte, payload []byte) []byte {
	frame := make([]byte, HeaderSize+len(payload))
	binary.BigEndian.PutUint16(frame[0:2], MagicBytes)
	frame[2] = msgType
	frame[3] = 0 // flags
	binary.BigEndian.PutUint32(frame[4:8], m.lobbyID)
	if len(payload) > 0 {
		copy(frame[HeaderSize:], payload)
	}
	return frame
}

func decodeHeader(data []byte) Header {
	return Header{
		Magic:   binary.BigEndian.Uint16(data[0:2]),
		Type:    data[2],
		Flags:   data[3],
		LobbyID: binary.BigEndian.Uint32(data[4:8]),
	}
}
