package perms

import (
	"runtime"
	"testing"
)

func TestCheckPcapPermissions(t *testing.T) {
	result := CheckPcapPermissions()
	// On macOS CI/test environments, BPF may or may not be readable
	// Just verify the function returns a valid result
	if result.Message == "" {
		t.Error("Message should not be empty")
	}

	switch runtime.GOOS {
	case "darwin":
		// Should return either OK or permission denied message
		if !result.OK && result.Message == "" {
			t.Error("should have a message when not OK")
		}
	case "linux":
		// Should pass through as not required
		if !result.OK {
			t.Error("should be OK on linux")
		}
	}
}

func TestIsNpcapInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows-only test")
	}
	// On non-windows, should return true
	if !IsNpcapInstalled() {
		t.Error("should return true on non-windows")
	}
}
