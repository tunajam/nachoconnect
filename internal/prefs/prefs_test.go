package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Load from non-existent dir should return empty prefs
	dir := t.TempDir()
	origConfigDir := configDirOverride
	configDirOverride = dir
	defer func() { configDirOverride = origConfigDir }()

	p, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Gamertag != "" {
		t.Errorf("default Gamertag should be empty, got %q", p.Gamertag)
	}
	if p.Interface != "" {
		t.Errorf("default Interface should be empty, got %q", p.Interface)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	origConfigDir := configDirOverride
	configDirOverride = dir
	defer func() { configDirOverride = origConfigDir }()

	p, _ := Load()
	p.Gamertag = "TestPlayer"
	p.Interface = "en0"
	if err := p.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(filepath.Join(dir, "preferences.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var loaded Preferences
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if loaded.Gamertag != "TestPlayer" {
		t.Errorf("Gamertag: got %q, want %q", loaded.Gamertag, "TestPlayer")
	}

	// Load again
	p2, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p2.Gamertag != "TestPlayer" {
		t.Errorf("reloaded Gamertag: got %q", p2.Gamertag)
	}
}

func TestSetGamertag(t *testing.T) {
	dir := t.TempDir()
	origConfigDir := configDirOverride
	configDirOverride = dir
	defer func() { configDirOverride = origConfigDir }()

	p, _ := Load()
	if err := p.SetGamertag("NachoKing"); err != nil {
		t.Fatalf("SetGamertag: %v", err)
	}

	p2, _ := Load()
	if p2.Gamertag != "NachoKing" {
		t.Errorf("got %q, want NachoKing", p2.Gamertag)
	}
}

func TestLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	origConfigDir := configDirOverride
	configDirOverride = dir
	defer func() { configDirOverride = origConfigDir }()

	// Write corrupt JSON
	os.WriteFile(filepath.Join(dir, "preferences.json"), []byte("{broken"), 0644)

	_, err := Load()
	if err == nil {
		t.Error("expected error loading corrupt JSON")
	}
}
