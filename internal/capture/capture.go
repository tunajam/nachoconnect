package capture

import (
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	// Xbox system link port
	XboxPort = 3074
	// Xbox broadcast IP
	XboxBroadcastIP = "0.0.0.1"
	// Snapshot length for pcap
	snapLen = 65535
	// BPF filter for Xbox system link traffic
	bpfFilter = "udp port 3074"
)

// Capturer handles packet capture on a network interface
type Capturer struct {
	handle  *pcap.Handle
	iface   string
	packets chan []byte
	done    chan struct{}
}

// NewCapturer creates a new packet capturer for the given interface
func NewCapturer(ifaceName string) (*Capturer, error) {
	handle, err := pcap.OpenLive(ifaceName, snapLen, true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("failed to open interface %s: %w", ifaceName, err)
	}

	// Set BPF filter for Xbox traffic
	if err := handle.SetBPFFilter(bpfFilter); err != nil {
		handle.Close()
		return nil, fmt.Errorf("failed to set BPF filter: %w", err)
	}

	c := &Capturer{
		handle:  handle,
		iface:   ifaceName,
		packets: make(chan []byte, 256),
		done:    make(chan struct{}),
	}

	go c.readLoop()
	return c, nil
}

func (c *Capturer) readLoop() {
	defer close(c.packets)
	source := gopacket.NewPacketSource(c.handle, c.handle.LinkType())

	for {
		select {
		case <-c.done:
			return
		default:
			packet, err := source.NextPacket()
			if err != nil {
				continue
			}
			// Send raw bytes for tunneling
			c.packets <- packet.Data()
		}
	}
}

// Packets returns the channel of captured raw packets
func (c *Capturer) Packets() <-chan []byte {
	return c.packets
}

// Inject sends a raw Ethernet frame onto the network
func (c *Capturer) Inject(frame []byte) error {
	if c.handle == nil {
		return fmt.Errorf("capture handle not open")
	}
	return c.handle.WritePacketData(frame)
}

// Stop closes the capturer
func (c *Capturer) Stop() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	if c.handle != nil {
		c.handle.Close()
	}
}

// IsXboxPacket checks if a raw packet is Xbox system link traffic
func IsXboxPacket(data []byte) bool {
	packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.NoCopy)

	udpLayer := packet.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		return false
	}

	udp := udpLayer.(*layers.UDP)
	return udp.DstPort == XboxPort || udp.SrcPort == XboxPort
}

// ExtractMAC extracts source MAC address from raw packet
func ExtractMAC(data []byte) string {
	packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.NoCopy)
	ethLayer := packet.Layer(layers.LayerTypeEthernet)
	if ethLayer == nil {
		return ""
	}
	eth := ethLayer.(*layers.Ethernet)
	return eth.SrcMAC.String()
}

// IsBroadcast checks if packet is a broadcast frame
func IsBroadcast(data []byte) bool {
	packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.NoCopy)
	ethLayer := packet.Layer(layers.LayerTypeEthernet)
	if ethLayer == nil {
		return false
	}
	eth := ethLayer.(*layers.Ethernet)
	broadcast := net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	return eth.DstMAC.String() == broadcast.String()
}

// ListInterfaces returns available network interfaces suitable for capture
func ListInterfaces() ([]string, error) {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return nil, err
	}

	var names []string
	for _, d := range devices {
		if len(d.Addresses) > 0 {
			names = append(names, d.Name)
		}
	}
	return names, nil
}
