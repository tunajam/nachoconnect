package lobby

import "testing"

func TestPingQuality(t *testing.T) {
	tests := []struct {
		ping int
		want string
	}{
		{0, "green"},
		{1, "green"},
		{25, "green"},
		{49, "green"},
		{50, "yellow"},
		{75, "yellow"},
		{100, "yellow"},
		{101, "red"},
		{200, "red"},
		{999, "red"},
	}

	for _, tt := range tests {
		got := PingQuality(tt.ping)
		if got != tt.want {
			t.Errorf("PingQuality(%d) = %q, want %q", tt.ping, got, tt.want)
		}
	}
}
