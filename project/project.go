package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// ErrNotFound is returned by Load when .ks-project.json does not exist.
var ErrNotFound = errors.New("project config not found")

// Config represents the .ks-project.json file.
type Config struct {
	Version     int      `json:"version"`
	URLs        []string `json:"urls"`
	Breakpoints []string `json:"breakpoints,omitempty"`
}

// Load reads .ks-project.json from the current working directory.
func Load() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	path := filepath.Join(cwd, ".ks-project.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("reading project config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing project config: %w", err)
	}
	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported project config version %d (expected 1)", cfg.Version)
	}
	if len(cfg.URLs) == 0 {
		return nil, fmt.Errorf("project config must contain at least one URL")
	}
	for _, u := range cfg.URLs {
		if _, err := url.Parse(u); err != nil {
			return nil, fmt.Errorf("invalid URL in project config %q: %w", u, err)
		}
	}
	return &cfg, nil
}

// Save writes cfg to .ks-project.json in the current working directory.
func Save(cfg *Config) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling project config: %w", err)
	}
	path := filepath.Join(cwd, ".ks-project.json")
	return os.WriteFile(path, data, 0644)
}
