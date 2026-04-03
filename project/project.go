package project

import (
	"encoding/json"
	"fmt"
	"os"
)

// Filename is the name of the project config file.
const Filename = ".ks-project.json"

// Breakpoint defines a named viewport size.
type Breakpoint struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Config is the project configuration stored in .ks-project.json.
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

// Exists reports whether .ks-project.json exists in the current directory.
func Exists() bool {
	_, err := os.Stat(Filename)
	return err == nil
}

// Read reads and parses .ks-project.json from the current directory.
func Read() (*Config, error) {
	data, err := os.ReadFile(Filename)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", Filename, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", Filename, err)
	}
	return &cfg, nil
}

// Write marshals cfg as indented JSON and writes it to .ks-project.json.
func Write(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Filename, data, 0644)
}
