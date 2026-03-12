package l2tunnel

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestHubStartStop(t *testing.T) {
	hub, err := StartHub(0)
	if err != nil {
		t.Fatalf("StartHub: %v", err)
	}
	defer hub.Stop()

	if hub.Port() == 0 {
		t.Fatal("expected non-zero port")
	}
	if !hub.IsActive() {
		t.Fatal("expected hub to be active")
	}
	if hub.PeerCount() != 0 {
		t.Fatalf("expected 0 peers, got %d", hub.PeerCount())
	}

	hub.Stop()
	if hub.IsActive() {
		t.Fatal("expected hub to be inactive after Stop")
	}
}

func TestHubBroadcast(t *testing.T) {
	hub, err := StartHub(0)
	if err != nil {
		t.Fatalf("StartHub: %v", err)
	}
	defer hub.Stop()

	addr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:"+itoa(hub.Port()))

	// Create two "peers"
	peer1, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer1.Close()

	peer2, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer2.Close()

	// peer1 sends a packet to hub — this registers peer1
	peer1.WriteToUDP([]byte("hello from peer1"), addr)
	time.Sleep(50 * time.Millisecond)

	// peer2 sends a packet — this registers peer2 and broadcasts to peer1
	peer2.WriteToUDP([]byte("hello from peer2"), addr)

	// peer1 should receive peer2's message
	buf := make([]byte, 1024)
	peer1.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := peer1.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("peer1 read: %v", err)
	}
	if string(buf[:n]) != "hello from peer2" {
		t.Fatalf("unexpected message: %q", string(buf[:n]))
	}

	// Now peer1 sends again — peer2 should receive it
	peer1.WriteToUDP([]byte("reply from peer1"), addr)
	peer2.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err = peer2.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("peer2 read: %v", err)
	}
	if string(buf[:n]) != "reply from peer1" {
		t.Fatalf("unexpected message: %q", string(buf[:n]))
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
