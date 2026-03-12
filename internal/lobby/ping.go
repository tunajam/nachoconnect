package lobby

// PingQuality returns a color classification for a ping value.
//
//	green:  <50ms (great)
//	yellow: 50-100ms (okay)
//	red:    >100ms (poor)
func PingQuality(pingMs int) string {
	switch {
	case pingMs < 50:
		return "green"
	case pingMs <= 100:
		return "yellow"
	default:
		return "red"
	}
}
