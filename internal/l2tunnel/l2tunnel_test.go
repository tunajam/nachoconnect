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

func TestIsLikelyXbox(t *testing.T) {
	tests := []struct {
		name   string
		disc   Discovery
		expect bool
	}{
		{
			name:   "Xbox OUI 00:50:f2 broadcast",
			disc:   Discovery{SrcMAC: "00:50:f2:1a:2b:3c", DstMAC: "ff:ff:ff:ff:ff:ff"},
			expect: true,
		},
		{
			name:   "Xbox OUI 00:0d:3a broadcast",
			disc:   Discovery{SrcMAC: "00:0d:3a:38:ac:2e", DstMAC: "ff:ff:ff:ff:ff:ff"},
			expect: true,
		},
		{
			name:   "Xbox OUI uppercase",
			disc:   Discovery{SrcMAC: "00:50:F2:AA:BB:CC", DstMAC: "FF:FF:FF:FF:FF:FF"},
			expect: true,
		},
		{
			name:   "Xbox OUI but unicast destination — not system link",
			disc:   Discovery{SrcMAC: "00:50:f2:1a:2b:3c", DstMAC: "aa:bb:cc:dd:ee:ff"},
			expect: false,
		},
		{
			name:   "non-Xbox OUI broadcast — router ARP",
			disc:   Discovery{SrcMAC: "d4:6e:0e:11:22:33", DstMAC: "ff:ff:ff:ff:ff:ff"},
			expect: false,
		},
		{
			name:   "non-Xbox unicast",
			disc:   Discovery{SrcMAC: "aa:bb:cc:dd:ee:ff", DstMAC: "11:22:33:44:55:66"},
			expect: false,
		},
		{
			name:   "empty MACs",
			disc:   Discovery{SrcMAC: "", DstMAC: ""},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLikelyXbox(tt.disc)
			if got != tt.expect {
				t.Errorf("IsLikelyXbox(%v) = %v, want %v", tt.disc, got, tt.expect)
			}
		})
	}
}

func TestClassifyDiscoverError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{
			name:     "permission denied",
			errMsg:   "l2tunnel discover failed: Operation not permitted",
			expected: "permissions",
		},
		{
			name:     "bpf error",
			errMsg:   "l2tunnel discover failed: You don't have permission to capture on that device ((cannot open BPF device) /dev/bpf0: Permission denied)",
			expected: "permissions",
		},
		{
			name:     "no such device",
			errMsg:   "l2tunnel discover failed: No such device exists",
			expected: "interface",
		},
		{
			name:     "no suitable device",
			errMsg:   "l2tunnel discover failed: no suitable device found",
			expected: "interface",
		},
		{
			name:     "generic error",
			errMsg:   "l2tunnel discover exited unexpectedly: exit status 1",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyDiscoverError(tt.errMsg)
			if got != tt.expected {
				t.Errorf("ClassifyDiscoverError(%q) = %q, want %q", tt.errMsg, got, tt.expected)
			}
		})
	}
}

func TestFilterDiscoveries(t *testing.T) {
	// Simulate a stream of discoveries like a real network
	input := []Discovery{
		{SrcMAC: "d4:6e:0e:11:22:33", DstMAC: "ff:ff:ff:ff:ff:ff"}, // router ARP broadcast
		{SrcMAC: "aa:bb:cc:dd:ee:ff", DstMAC: "d4:6e:0e:11:22:33"}, // unicast — ignored
		{SrcMAC: "d4:6e:0e:11:22:33", DstMAC: "ff:ff:ff:ff:ff:ff"}, // router again (dedup)
		{SrcMAC: "00:50:f2:1a:2b:3c", DstMAC: "ff:ff:ff:ff:ff:ff"}, // Xbox!
		{SrcMAC: "00:50:f2:1a:2b:3c", DstMAC: "ff:ff:ff:ff:ff:ff"}, // Xbox again (dedup)
	}

	result := FilterDiscoveries(input)

	if result.XboxMAC != "00:50:f2:1a:2b:3c" {
		t.Errorf("XboxMAC = %q, want %q", result.XboxMAC, "00:50:f2:1a:2b:3c")
	}
	if result.TotalSeen != 3 { // 3 unique MACs (router, aa:bb, xbox)
		t.Errorf("TotalSeen = %d, want 3", result.TotalSeen)
	}
	if len(result.Candidates) != 1 { // router is the only non-Xbox broadcast source
		t.Errorf("Candidates = %v, want 1 candidate", result.Candidates)
	}
	if len(result.Candidates) > 0 && result.Candidates[0] != "d4:6e:0e:11:22:33" {
		t.Errorf("Candidates[0] = %q, want router MAC", result.Candidates[0])
	}
}

func TestFilterDiscoveries_NoXbox(t *testing.T) {
	input := []Discovery{
		{SrcMAC: "d4:6e:0e:11:22:33", DstMAC: "ff:ff:ff:ff:ff:ff"},
		{SrcMAC: "aa:bb:cc:dd:ee:ff", DstMAC: "ff:ff:ff:ff:ff:ff"},
	}

	result := FilterDiscoveries(input)

	if result.XboxMAC != "" {
		t.Errorf("XboxMAC should be empty, got %q", result.XboxMAC)
	}
	if len(result.Candidates) != 2 {
		t.Errorf("Candidates = %d, want 2", len(result.Candidates))
	}
}

func TestFilterDiscoveries_Empty(t *testing.T) {
	result := FilterDiscoveries(nil)

	if result.XboxMAC != "" {
		t.Errorf("XboxMAC should be empty")
	}
	if result.TotalSeen != 0 {
		t.Errorf("TotalSeen should be 0")
	}
	if len(result.Candidates) != 0 {
		t.Errorf("Candidates should be empty")
	}
}
