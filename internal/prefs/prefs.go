package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Preferences stores user settings locally
type Preferences struct {
	Gamertag  string `json:"gamertag"`
	Interface string `json:"interface,omitempty"`
	path      string
}

// Load reads preferences from disk, creating defaults if needed
func Load() (*Preferences, error) {
	p := &Preferences{}
	dir, err := configDir()
	if err != nil {
		return p, err
	}

	p.path = filepath.Join(dir, "preferences.json")

	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return p, err
	}

	if err := json.Unmarshal(data, p); err != nil {
		return p, err
	}
	return p, nil
}

// Save writes preferences to disk
func (p *Preferences) Save() error {
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.path, data, 0644)
}

// SetGamertag updates and saves the gamertag
func (p *Preferences) SetGamertag(tag string) error {
	p.Gamertag = tag
	return p.Save()
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "nachoconnect"), nil
}
