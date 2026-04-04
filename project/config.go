package project

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const ConfigFile = ".ks-project.json"

// Breakpoint represents a named viewport size.
type Breakpoint struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ProjectConfig is stored in .ks-project.json.
type ProjectConfig struct {
	Name        string       `json:"name"`
	BaseURL     string       `json:"baseURL"`
	Paths       []string     `json:"paths"`
	Breakpoints []Breakpoint `json:"breakpoints"`
}

// DefaultBreakpoints returns the four standard viewport presets.
func DefaultBreakpoints() []Breakpoint {
	return []Breakpoint{
		{Name: "mobile", Width: 375, Height: 812},
		{Name: "tablet", Width: 768, Height: 1024},
		{Name: "desktop", Width: 1280, Height: 720},
		{Name: "wide", Width: 1920, Height: 1080},
	}
}

// Load reads .ks-project.json from the current directory.
func Load() (*ProjectConfig, error) {
	path := filepath.Join(".", ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes cfg to .ks-project.json in the current directory.
func Save(cfg *ProjectConfig) error {
	path := filepath.Join(".", ConfigFile)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
