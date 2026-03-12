package l2tunnel

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWSBridge_ConnectAndRelay(t *testing.T) {
	// Set up a mock WebSocket relay server
	var serverConn *websocket.Conn
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	received := make(chan []byte, 10)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		serverConn = conn
		defer conn.Close()

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			received <- data
			// Echo back
			conn.WriteMessage(websocket.BinaryMessage, data)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/relay"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bridge, localAddr, err := StartWSBridge(ctx, wsURL)
	if err != nil {
		t.Fatalf("StartWSBridge: %v", err)
	}
	defer bridge.Stop()

	if !bridge.IsActive() {
		t.Error("bridge should be active after start")
	}

	// Parse the local address
	if !strings.HasPrefix(localAddr, "127.0.0.1:") {
		t.Fatalf("unexpected local addr: %s", localAddr)
	}

	// Connect a "l2tunnel" UDP client to the bridge
	udpAddr, err := net.ResolveUDPAddr("udp4", localAddr)
	if err != nil {
		t.Fatal(err)
	}

	clientConn, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	// Send a packet from "l2tunnel" → bridge → WebSocket server
	testData := []byte("hello-from-tunnel")
	_, err = clientConn.Write(testData)
	if err != nil {
		t.Fatal(err)
	}

	// Server should receive it
	select {
	case data := <-received:
		if string(data) != "hello-from-tunnel" {
			t.Errorf("server received %q, want %q", string(data), "hello-from-tunnel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to receive data")
	}

	// Server echoes back, l2tunnel client should receive it
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(buf[:n]) != "hello-from-tunnel" {
		t.Errorf("client received %q, want echo", string(buf[:n]))
	}

	_ = serverConn // suppress unused warning
}

func TestWSBridge_StopIdempotent(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/relay"
	ctx := context.Background()

	bridge, _, err := StartWSBridge(ctx, wsURL)
	if err != nil {
		t.Fatal(err)
	}

	bridge.Stop()
	if bridge.IsActive() {
		t.Error("should be inactive after stop")
	}

	// Second stop should not panic
	bridge.Stop()
}

func TestWSBridge_InvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, err := StartWSBridge(ctx, "ws://127.0.0.1:1/nonexistent")
	if err == nil {
		t.Error("expected error connecting to invalid URL")
	}
}
