package perms

import (
	"runtime"
	"strings"
	"testing"
)

func TestCheckPcapPermissions(t *testing.T) {
	result := CheckPcapPermissions()
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
		if !result.OK {
			t.Error("should be OK on linux")
		}
	}
}

func TestIsNpcapInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows-only test")
	}
	if !IsNpcapInstalled() {
		t.Error("should return true on non-windows")
	}
}

func TestBuildInstallScript(t *testing.T) {
	script := buildInstallScript("testuser")

	// Should contain key operations
	if !strings.Contains(script, "dseditgroup -o create") {
		t.Error("should create group")
	}
	if !strings.Contains(script, "dseditgroup -o edit -a testuser") {
		t.Error("should add user to group")
	}
	if !strings.Contains(script, launchDaemonPath) {
		t.Error("should reference launch daemon path")
	}
	if !strings.Contains(script, "launchctl bootstrap") {
		t.Error("should load launch daemon")
	}
	if !strings.Contains(script, chmodBPFScript) {
		t.Error("should run chmod script immediately")
	}
}

func TestEmbeddedFiles(t *testing.T) {
	if chmodBPFScriptContent == "" {
		t.Error("chmod-bpf.sh should be embedded")
	}
	if !strings.Contains(chmodBPFScriptContent, "access_bpf") {
		t.Error("chmod script should reference access_bpf group")
	}

	if launchDaemonContent == "" {
		t.Error("plist should be embedded")
	}
	if !strings.Contains(launchDaemonContent, "com.nachoconnect.chmod-bpf") {
		t.Error("plist should have correct label")
	}
}

func TestIsSetupDone(t *testing.T) {
	if runtime.GOOS != "darwin" {
		if !IsSetupDone() {
			t.Error("non-darwin should always return true")
		}
	}
	// On darwin, depends on whether flag file exists — just verify it doesn't crash
	_ = IsSetupDone()
}

func TestFlagFilePath(t *testing.T) {
	path := flagFilePath()
	if path == "" {
		t.Error("flag file path should not be empty")
	}
	if !strings.Contains(path, installedFlagFile) {
		t.Errorf("path should contain flag file name, got %q", path)
	}
}
