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

func TestHubPingPong(t *testing.T) {
	hub, err := StartHub(0)
	if err != nil {
		t.Fatalf("StartHub: %v", err)
	}
	defer hub.Stop()

	addr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:"+itoa(hub.Port()))

	peer, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	// Send a ping packet
	pkt := BuildPingPacket()
	peer.WriteToUDP(pkt, addr)

	// Should get pong back
	buf := make([]byte, 64)
	peer.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := peer.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}

	ts := ParsePongTimestamp(buf, n)
	if ts == 0 {
		t.Fatal("expected valid pong timestamp")
	}

	// Verify the timestamp matches what we sent
	sentTs := ParsePongTimestamp(pkt, len(pkt))
	// pkt is a ping, not pong, so ParsePongTimestamp returns 0 for ping type
	// Instead check raw bytes
	if n != pingPacketLen {
		t.Fatalf("expected %d bytes, got %d", pingPacketLen, n)
	}
	if buf[4] != pingTypePong {
		t.Fatalf("expected pong type 0x02, got 0x%02x", buf[4])
	}
	// Timestamp should be echoed back
	if string(buf[5:13]) != string(pkt[5:13]) {
		t.Fatal("pong timestamp doesn't match ping timestamp")
	}

	_ = sentTs
}

func TestHubPingNotBroadcast(t *testing.T) {
	hub, err := StartHub(0)
	if err != nil {
		t.Fatalf("StartHub: %v", err)
	}
	defer hub.Stop()

	addr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:"+itoa(hub.Port()))

	// Register peer1 by sending a normal packet
	peer1, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer1.Close()
	peer1.WriteToUDP([]byte("register peer1"), addr)
	time.Sleep(50 * time.Millisecond)

	// peer2 sends a ping — peer1 should NOT receive it
	peer2, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer2.Close()
	peer2.WriteToUDP(BuildPingPacket(), addr)

	// peer1 should NOT receive anything
	buf := make([]byte, 64)
	peer1.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = peer1.ReadFromUDP(buf)
	if err == nil {
		t.Fatal("ping packet was broadcast to other peers — should not be")
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
