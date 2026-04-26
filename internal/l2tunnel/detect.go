package l2tunnel

import "strings"

// XboxOUIPrefixes are known MAC OUI prefixes for original Xbox consoles.
// The Xbox NIC was manufactured by Microsoft (00:50:f2) and some later
// revisions used 00:0d:3a.
var XboxOUIPrefixes = []string{
	"00:50:f2", // Microsoft Xbox
	"00:0d:3a", // Microsoft (Xbox / Azure)
}

// IsLikelyXbox returns true if the discovery looks like an Xbox system link broadcast.
// System link traffic is always broadcast (dst ff:ff:ff:ff:ff:ff) from a known Xbox OUI.
func IsLikelyXbox(d Discovery) bool {
	src := strings.ToLower(d.SrcMAC)
	dst := strings.ToLower(d.DstMAC)

	// Must be a broadcast frame (system link always broadcasts)
	if dst != "ff:ff:ff:ff:ff:ff" {
		return false
	}

	// Check known Xbox OUI prefixes
	for _, prefix := range XboxOUIPrefixes {
		if strings.HasPrefix(src, prefix) {
			return true
		}
	}

	return false
}

// FilterResult holds the output of filtering a discovery stream.
type FilterResult struct {
	XboxMAC    string   // first Xbox MAC detected (empty if none)
	Candidates []string // non-Xbox broadcast MACs (fallback for modded NICs)
	TotalSeen  int      // total unique MACs observed
}

// FilterDiscoveries processes a slice of discoveries and returns a FilterResult.
// In production, discoveries arrive via channel; this function enables testing
// the filtering logic without subprocesses.
func FilterDiscoveries(discoveries []Discovery) FilterResult {
	seen := make(map[string]bool)
	var result FilterResult

	for _, d := range discoveries {
		if seen[d.SrcMAC] {
			continue
		}
		seen[d.SrcMAC] = true

		if IsLikelyXbox(d) {
			if result.XboxMAC == "" {
				result.XboxMAC = d.SrcMAC
			}
		} else if strings.ToLower(d.DstMAC) == "ff:ff:ff:ff:ff:ff" {
			result.Candidates = append(result.Candidates, d.SrcMAC)
		}
	}

	result.TotalSeen = len(seen)
	return result
}

// ClassifyDiscoverError categorizes an error message from the l2tunnel discover
// subprocess into a user-facing category: "permissions", "interface", or "unknown".
func ClassifyDiscoverError(errMsg string) string {
	lower := strings.ToLower(errMsg)
	if strings.Contains(lower, "permission") || strings.Contains(lower, "operation not permitted") || strings.Contains(lower, "bpf") {
		return "permissions"
	}
	if strings.Contains(lower, "no such device") || strings.Contains(lower, "no suitable device") {
		return "interface"
	}
	return "unknown"
}
