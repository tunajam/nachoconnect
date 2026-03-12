package l2tunnel

import (
	"fmt"
	"log"
	"net"
	"time"
)

// UPnPResult holds the result of a UPnP port mapping attempt.
type UPnPResult struct {
	Success    bool   `json:"success"`
	ExternalIP string `json:"externalIP,omitempty"`
	Port       int    `json:"port"`
	Message    string `json:"message"`
}

// TryUPnPForward attempts to forward a UDP port via UPnP/NAT-PMP.
// This is a best-effort operation — many routers don't support UPnP.
func TryUPnPForward(port int) UPnPResult {
	ip, err := discoverGatewayUPnP(port)
	if err != nil {
		log.Printf("UPnP: not available — %v", err)
		return UPnPResult{
			Success: false,
			Port:    port,
			Message: fmt.Sprintf("UPnP not available: %v. Manual port forward required.", err),
		}
	}

	return UPnPResult{
		Success:    true,
		ExternalIP: ip,
		Port:       port,
		Message:    fmt.Sprintf("UPnP: port %d forwarded successfully", port),
	}
}

// RemoveUPnPForward removes a UPnP port mapping if one was created.
func RemoveUPnPForward(port int) {
	log.Printf("UPnP: removing port mapping for %d (if any)", port)
}

func discoverGatewayUPnP(port int) (string, error) {
	ssdpAddr := "239.255.255.250:1900"
	msg := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n" +
		"\r\n"

	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		return "", err
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	_, err = conn.WriteToUDP([]byte(msg), addr)
	if err != nil {
		return "", fmt.Errorf("SSDP send failed: %w", err)
	}

	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return "", fmt.Errorf("no UPnP gateway found (timeout)")
	}

	_ = n
	// Gateway responded — full SOAP AddPortMapping not yet implemented.
	// TODO: Parse SSDP response Location header, fetch device XML, call AddPortMapping
	return "", fmt.Errorf("UPnP gateway detected but SOAP port mapping not yet implemented")
}
