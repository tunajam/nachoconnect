package nat

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"time"
)

const (
	// STUN message types
	BindingRequest  = 0x0001
	BindingResponse = 0x0101

	// STUN attributes
	AttrMappedAddress    = 0x0001
	AttrXorMappedAddress = 0x0020

	// STUN magic cookie
	MagicCookie = 0x2112A442

	// Default STUN servers
	DefaultSTUNServer = "stun.l.google.com:19302"
)

// STUNResult contains the result of a STUN query
type STUNResult struct {
	PublicIP   string
	PublicPort int
	NATType    string
	Latency    time.Duration
}

// DiscoverPublicAddress uses STUN to discover the public IP and port mapping
func DiscoverPublicAddress(stunServer string) (*STUNResult, error) {
	if stunServer == "" {
		stunServer = DefaultSTUNServer
	}

	addr, err := net.ResolveUDPAddr("udp4", stunServer)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve STUN server: %w", err)
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to STUN server: %w", err)
	}
	defer conn.Close()

	// Build STUN binding request
	txID := make([]byte, 12)
	rand.Read(txID)

	request := make([]byte, 20)
	binary.BigEndian.PutUint16(request[0:2], BindingRequest)
	binary.BigEndian.PutUint16(request[2:4], 0) // message length
	binary.BigEndian.PutUint32(request[4:8], MagicCookie)
	copy(request[8:20], txID)

	start := time.Now()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write(request); err != nil {
		return nil, fmt.Errorf("failed to send STUN request: %w", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read STUN response: %w", err)
	}

	latency := time.Since(start)

	if n < 20 {
		return nil, fmt.Errorf("STUN response too short")
	}

	// Parse response
	msgType := binary.BigEndian.Uint16(buf[0:2])
	if msgType != BindingResponse {
		return nil, fmt.Errorf("unexpected STUN response type: 0x%04x", msgType)
	}

	msgLen := binary.BigEndian.Uint16(buf[2:4])
	if int(msgLen)+20 > n {
		return nil, fmt.Errorf("STUN response length mismatch")
	}

	// Parse attributes
	offset := 20
	var publicIP string
	var publicPort int

	for offset < 20+int(msgLen) {
		if offset+4 > n {
			break
		}
		attrType := binary.BigEndian.Uint16(buf[offset : offset+2])
		attrLen := binary.BigEndian.Uint16(buf[offset+2 : offset+4])
		offset += 4

		if offset+int(attrLen) > n {
			break
		}

		switch attrType {
		case AttrXorMappedAddress:
			if attrLen >= 8 {
				port := binary.BigEndian.Uint16(buf[offset+2:offset+4]) ^ uint16(MagicCookie>>16)
				ip := make(net.IP, 4)
				binary.BigEndian.PutUint32(ip, binary.BigEndian.Uint32(buf[offset+4:offset+8])^MagicCookie)
				publicIP = ip.String()
				publicPort = int(port)
			}
		case AttrMappedAddress:
			if publicIP == "" && attrLen >= 8 {
				publicPort = int(binary.BigEndian.Uint16(buf[offset+2 : offset+4]))
				publicIP = fmt.Sprintf("%d.%d.%d.%d", buf[offset+4], buf[offset+5], buf[offset+6], buf[offset+7])
			}
		}

		// Align to 4-byte boundary
		offset += int(attrLen)
		if offset%4 != 0 {
			offset += 4 - (offset % 4)
		}
	}

	if publicIP == "" {
		return nil, fmt.Errorf("no mapped address in STUN response")
	}

	return &STUNResult{
		PublicIP:   publicIP,
		PublicPort: publicPort,
		NATType:    "unknown", // Would need multiple STUN queries to determine
		Latency:    latency,
	}, nil
}

// HolePunch attempts UDP hole punching between two peers
func HolePunch(localConn *net.UDPConn, remoteAddr *net.UDPAddr, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	localConn.SetDeadline(deadline)
	defer localConn.SetDeadline(time.Time{})

	// Send periodic packets to punch through NAT
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	punch := []byte("NACHO-PUNCH")
	buf := make([]byte, 64)

	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("hole punch timeout after %v", timeout)
			}
			localConn.WriteToUDP(punch, remoteAddr)
		default:
			localConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			n, addr, err := localConn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			if n > 0 && addr.IP.Equal(remoteAddr.IP) {
				return nil // Hole punched!
			}
		}
	}
}
