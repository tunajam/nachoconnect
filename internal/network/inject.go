package network

import (
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// Injector handles re-injecting received packets onto the local network
type Injector struct {
	handle *pcap.Handle
	iface  string
}

// NewInjector creates a new packet injector for the given interface
func NewInjector(ifaceName string) (*Injector, error) {
	handle, err := pcap.OpenLive(ifaceName, 65535, true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("failed to open interface for injection: %w", err)
	}

	return &Injector{
		handle: handle,
		iface:  ifaceName,
	}, nil
}

// Inject writes a raw Ethernet frame to the network
func (inj *Injector) Inject(frame []byte) error {
	return inj.handle.WritePacketData(frame)
}

// InjectBroadcast wraps data in a broadcast Ethernet frame and injects it
func (inj *Injector) InjectBroadcast(payload []byte, srcMAC net.HardwareAddr) error {
	// Build Ethernet frame
	eth := &layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeIPv4,
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	if err := gopacket.SerializeLayers(buf, opts, eth, gopacket.Payload(payload)); err != nil {
		return fmt.Errorf("failed to serialize frame: %w", err)
	}

	return inj.handle.WritePacketData(buf.Bytes())
}

// Close closes the injector
func (inj *Injector) Close() {
	if inj.handle != nil {
		inj.handle.Close()
	}
}
