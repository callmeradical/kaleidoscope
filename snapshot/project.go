package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
)

// Breakpoint defines a viewport size for capturing.
type Breakpoint struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// DefaultBreakpoints are the four standard presets.
var DefaultBreakpoints = []Breakpoint{
	{"mobile", 375, 812},
	{"tablet", 768, 1024},
	{"desktop", 1280, 720},
	{"wide", 1920, 1080},
}

// ProjectConfig is the schema for .ks-project.json.
type ProjectConfig struct {
	Name        string       `json:"name"`
	BaseURL     string       `json:"baseURL"`
	Paths       []string     `json:"paths"`
	Breakpoints []Breakpoint `json:"breakpoints"`
}

const ProjectFile = ".ks-project.json"

// LoadProject reads .ks-project.json from the current directory.
func LoadProject() (*ProjectConfig, error) {
	data, err := os.ReadFile(ProjectFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ProjectFile, err)
	}
	var p ProjectConfig
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ProjectFile, err)
	}
	return &p, nil
}

// SaveProject writes p to .ks-project.json in the current directory.
func SaveProject(p *ProjectConfig) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ProjectFile, data, 0644)
}
