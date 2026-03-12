package l2tunnel

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

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
	wsURL     string
	ctx       context.Context
	onError   func(error)
}

const (
	wsKeepaliveInterval = 15 * time.Second
	wsConnectTimeout    = 10 * time.Second
	maxReconnectDelay   = 30 * time.Second
)

// StartWSBridge creates a local UDP listener and connects to the WebSocket relay.
// Returns the local UDP address that l2tunnel should connect to.
func StartWSBridge(ctx context.Context, wsURL string) (*WSBridge, string, error) {
	return StartWSBridgeWithCallback(ctx, wsURL, nil)
}

// StartWSBridgeWithCallback is like StartWSBridge but accepts an error callback
// that fires on connection loss (before reconnect attempts).
func StartWSBridgeWithCallback(ctx context.Context, wsURL string, onError func(error)) (*WSBridge, string, error) {
	// Connect to WebSocket relay with retry
	wsConn, err := dialWSWithRetry(ctx, wsURL, 3)
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
		wsURL:     wsURL,
		ctx:       ctx,
		onError:   onError,
	}

	// Track the l2tunnel client address (first packet sender)
	var tunnelAddr *net.UDPAddr
	var tunnelMu sync.RWMutex

	// Keepalive: send WebSocket ping frames periodically
	go func() {
		ticker := time.NewTicker(wsKeepaliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				bridge.mu.RLock()
				ws := bridge.wsConn
				bridge.mu.RUnlock()
				if ws != nil {
					if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
						log.Printf("WSBridge: keepalive ping failed: %v", err)
					}
				}
			}
		}
	}()

	// WebSocket → UDP (relay to l2tunnel)
	go func() {
		for {
			select {
			case <-ctx.Done():
				bridge.Stop()
				return
			default:
			}

			bridge.mu.RLock()
			ws := bridge.wsConn
			bridge.mu.RUnlock()

			_, data, err := ws.ReadMessage()
			if err != nil {
				log.Printf("WSBridge: WebSocket read error: %v", err)
				if bridge.onError != nil {
					bridge.onError(err)
				}
				// Try to reconnect
				if !bridge.reconnect() {
					bridge.Stop()
					return
				}
				continue
			}

			tunnelMu.RLock()
			ta := tunnelAddr
			tunnelMu.RUnlock()
			if ta != nil {
				_, err = udpConn.WriteToUDP(data, ta)
				if err != nil {
					log.Printf("WSBridge: UDP write error: %v", err)
				}
			}
		}
	}()

	// UDP → WebSocket (l2tunnel to relay)
	go func() {
		buf := make([]byte, 65536)
		for {
			select {
			case <-ctx.Done():
				bridge.Stop()
				return
			default:
			}

			n, from, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("WSBridge: UDP read error: %v", err)
				bridge.Stop()
				return
			}

			// Remember the tunnel's address
			tunnelMu.Lock()
			tunnelAddr = from
			tunnelMu.Unlock()

			bridge.mu.RLock()
			ws := bridge.wsConn
			bridge.mu.RUnlock()

			err = ws.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				log.Printf("WSBridge: WebSocket write error: %v", err)
				if bridge.onError != nil {
					bridge.onError(err)
				}
				if !bridge.reconnect() {
					bridge.Stop()
					return
				}
			}
		}
	}()

	return bridge, fmt.Sprintf("127.0.0.1:%d", localAddr.Port), nil
}

// reconnect attempts to re-establish the WebSocket connection with exponential backoff.
func (b *WSBridge) reconnect() bool {
	b.mu.Lock()
	if !b.active {
		b.mu.Unlock()
		return false
	}
	// Close old connection
	if b.wsConn != nil {
		b.wsConn.Close()
	}
	b.mu.Unlock()

	wsConn, err := dialWSWithRetry(b.ctx, b.wsURL, 5)
	if err != nil {
		log.Printf("WSBridge: reconnect failed after retries: %v", err)
		return false
	}

	b.mu.Lock()
	b.wsConn = wsConn
	b.mu.Unlock()
	log.Printf("WSBridge: reconnected to relay")
	return true
}

// dialWSWithRetry dials a WebSocket URL with exponential backoff.
func dialWSWithRetry(ctx context.Context, wsURL string, maxAttempts int) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: wsConnectTimeout,
	}
	delay := 1 * time.Second
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
			if delay > maxReconnectDelay {
				delay = maxReconnectDelay
			}
		}
		conn, _, err := dialer.DialContext(ctx, wsURL, nil)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		log.Printf("WSBridge: connect attempt %d/%d failed: %v", i+1, maxAttempts, err)
	}
	return nil, lastErr
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
