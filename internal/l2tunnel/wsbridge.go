package l2tunnel

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

// WSBridge connects l2tunnel's UDP to a WebSocket relay server.
// l2tunnel sends/receives UDP packets to a local port.
// WSBridge listens on that local port and relays to/from the WebSocket.
type WSBridge struct {
	mu        sync.RWMutex
	wsConn    *websocket.Conn
	udpConn   *net.UDPConn
	localPort int
	cancel    context.CancelFunc
	active    bool
}

// StartWSBridge creates a local UDP listener and connects to the WebSocket relay.
// Returns the local UDP address that l2tunnel should connect to.
func StartWSBridge(ctx context.Context, wsURL string) (*WSBridge, string, error) {
	// Connect to WebSocket relay
	dialer := websocket.Dialer{}
	wsConn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to connect to relay: %w", err)
	}

	// Create local UDP listener on a random port
	udpAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		wsConn.Close()
		return nil, "", fmt.Errorf("failed to resolve UDP addr: %w", err)
	}

	udpConn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		wsConn.Close()
		return nil, "", fmt.Errorf("failed to listen UDP: %w", err)
	}

	localAddr := udpConn.LocalAddr().(*net.UDPAddr)

	ctx, cancel := context.WithCancel(ctx)
	bridge := &WSBridge{
		wsConn:    wsConn,
		udpConn:   udpConn,
		localPort: localAddr.Port,
		cancel:    cancel,
		active:    true,
	}

	// Track the l2tunnel client address (first packet sender)
	var tunnelAddr *net.UDPAddr

	// WebSocket → UDP (relay to l2tunnel)
	go func() {
		defer bridge.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, data, err := wsConn.ReadMessage()
			if err != nil {
				log.Printf("WSBridge: WebSocket read error: %v", err)
				return
			}

			if tunnelAddr != nil {
				_, err = udpConn.WriteToUDP(data, tunnelAddr)
				if err != nil {
					log.Printf("WSBridge: UDP write error: %v", err)
				}
			}
		}
	}()

	// UDP → WebSocket (l2tunnel to relay)
	go func() {
		defer bridge.Stop()
		buf := make([]byte, 65536)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			n, from, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("WSBridge: UDP read error: %v", err)
				return
			}

			// Remember the tunnel's address
			tunnelAddr = from

			err = wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				log.Printf("WSBridge: WebSocket write error: %v", err)
				return
			}
		}
	}()

	return bridge, fmt.Sprintf("127.0.0.1:%d", localAddr.Port), nil
}

// Stop closes the bridge
func (b *WSBridge) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.active {
		return
	}
	b.active = false
	if b.cancel != nil {
		b.cancel()
	}
	if b.wsConn != nil {
		b.wsConn.Close()
	}
	if b.udpConn != nil {
		b.udpConn.Close()
	}
}

// IsActive returns whether the bridge is running
func (b *WSBridge) IsActive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.active
}
