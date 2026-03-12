package l2tunnel

import (
	"testing"
)

func TestParseListOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []Interface
	}{
		{
			name:   "empty",
			input:  "",
			expect: nil,
		},
		{
			name: "single device",
			input: `device 0: en0
- description: Wi-Fi
- flags: PCAP_IF_UP PCAP_IF_RUNNING`,
			expect: []Interface{
				{Index: 0, Name: "en0", Description: "Wi-Fi", Flags: []string{"PCAP_IF_UP", "PCAP_IF_RUNNING"}},
			},
		},
		{
			name: "multiple devices",
			input: `device 0: en0
- description: Wi-Fi
- flags: PCAP_IF_UP PCAP_IF_RUNNING
device 1: lo0
- description: Loopback
- flags: PCAP_IF_LOOPBACK PCAP_IF_UP PCAP_IF_RUNNING
device 2: en1
- flags: PCAP_IF_UP`,
			expect: []Interface{
				{Index: 0, Name: "en0", Description: "Wi-Fi", Flags: []string{"PCAP_IF_UP", "PCAP_IF_RUNNING"}},
				{Index: 1, Name: "lo0", Description: "Loopback", Flags: []string{"PCAP_IF_LOOPBACK", "PCAP_IF_UP", "PCAP_IF_RUNNING"}},
				{Index: 2, Name: "en1", Description: "", Flags: []string{"PCAP_IF_UP"}},
			},
		},
		{
			name: "no description",
			input: `device 5: utun0
- flags: PCAP_IF_UP PCAP_IF_RUNNING`,
			expect: []Interface{
				{Index: 5, Name: "utun0", Flags: []string{"PCAP_IF_UP", "PCAP_IF_RUNNING"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListOutput(tt.input)
			if len(got) != len(tt.expect) {
				t.Fatalf("len: got %d, want %d", len(got), len(tt.expect))
			}
			for i, iface := range got {
				exp := tt.expect[i]
				if iface.Index != exp.Index {
					t.Errorf("[%d] Index: got %d, want %d", i, iface.Index, exp.Index)
				}
				if iface.Name != exp.Name {
					t.Errorf("[%d] Name: got %q, want %q", i, iface.Name, exp.Name)
				}
				if iface.Description != exp.Description {
					t.Errorf("[%d] Description: got %q, want %q", i, iface.Description, exp.Description)
				}
				if len(iface.Flags) != len(exp.Flags) {
					t.Errorf("[%d] Flags len: got %d, want %d", i, len(iface.Flags), len(exp.Flags))
				} else {
					for j, f := range iface.Flags {
						if f != exp.Flags[j] {
							t.Errorf("[%d] Flag[%d]: got %q, want %q", i, j, f, exp.Flags[j])
						}
					}
				}
			}
		})
	}
}

func TestTunnelConfigFields(t *testing.T) {
	cfg := TunnelConfig{
		Interface:  "en0",
		FilterMode: "-s",
		MAC:        "00:0d:3a:38:ac:2e",
		LocalAddr:  "0.0.0.0",
		LocalPort:  "0",
		RemoteAddr: "127.0.0.1",
		RemotePort: "1337",
	}

	if cfg.Interface != "en0" {
		t.Errorf("Interface: got %q", cfg.Interface)
	}
	if cfg.FilterMode != "-s" {
		t.Errorf("FilterMode: got %q", cfg.FilterMode)
	}
	if cfg.MAC != "00:0d:3a:38:ac:2e" {
		t.Errorf("MAC: got %q", cfg.MAC)
	}
}

func TestBinaryPathInit(t *testing.T) {
	// BinaryPath should have some value after init
	if BinaryPath == "" {
		t.Error("BinaryPath should not be empty")
	}
}
