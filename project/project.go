package project

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ConfigFile is the name of the project config file.
const ConfigFile = ".ks-project.json"

// Breakpoint represents a named viewport size.
type Breakpoint struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Config holds the kaleidoscope project configuration.
type Config struct {
	Name        string       `json:"name"`
	BaseURL     string       `json:"baseURL"`
	Paths       []string     `json:"paths"`
	Breakpoints []Breakpoint `json:"breakpoints"`
}

// DefaultBreakpoints are the four standard viewport presets.
var DefaultBreakpoints = []Breakpoint{
	{Name: "mobile", Width: 375, Height: 812},
	{Name: "tablet", Width: 768, Height: 1024},
	{Name: "desktop", Width: 1280, Height: 720},
	{Name: "wide", Width: 1920, Height: 1080},
}

// Load reads .ks-project.json from dir.
// Returns fs.ErrNotExist (via errors.Is) if the file does not exist.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save atomically writes cfg as JSON to .ks-project.json in dir.
func Save(dir string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, ConfigFile+".tmp")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, ConfigFile))
}

// Validate returns an error if cfg is missing required fields.
func Validate(cfg *Config) error {
	if cfg.Name == "" {
		return errors.New("name is required")
	}
	if cfg.BaseURL == "" {
		return errors.New("baseURL is required")
	}
	if len(cfg.Paths) == 0 {
		return errors.New("paths must not be empty")
	}
	return nil
}
