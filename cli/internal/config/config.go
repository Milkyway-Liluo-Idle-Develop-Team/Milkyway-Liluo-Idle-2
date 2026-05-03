// Package config manages CLI-local settings: server URL, saved auth token,
// and any client-side preferences. Settings are stored as JSON in the OS
// config directory (~/.config/mli-cli/config.json on Linux, etc.).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// C holds the runtime configuration. Zero value is usable: ServerURL defaults
// to localhost and Token is empty (unauthenticated).
type C struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
}

// Default returns a config with sensible development defaults.
func Default() *C {
	return &C{ServerURL: "http://localhost:8080"}
}

// Dir returns the OS-specific directory for mli-cli config files.
func Dir() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, "mli-cli")
}

// Path returns the full path to the config file.
func Path() string {
	return filepath.Join(Dir(), "config.json")
}

// Load reads the config file from disk. If it doesn't exist, returns defaults.
func Load() (*C, error) {
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c C
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.ServerURL == "" {
		c.ServerURL = Default().ServerURL
	}
	return &c, nil
}

// Save writes the config to disk, creating parent directories as needed.
func (c *C) Save() error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir config: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(Path(), data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
