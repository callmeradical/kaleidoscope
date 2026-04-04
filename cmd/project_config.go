package cmd

import (
	"encoding/json"
	"os"
)

// ProjectBreakpoint defines a named viewport size for screenshots.
type ProjectBreakpoint struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ProjectConfig holds the project configuration stored in .ks-project.json.
type ProjectConfig struct {
	Name        string              `json:"name"`
	BaseURL     string              `json:"baseUrl"`
	Paths       []string            `json:"paths"`
	Breakpoints []ProjectBreakpoint `json:"breakpoints"`
}

const projectConfigFile = ".ks-project.json"

var defaultProjectBreakpoints = []ProjectBreakpoint{
	{Name: "mobile", Width: 375, Height: 812},
	{Name: "tablet", Width: 768, Height: 1024},
	{Name: "desktop", Width: 1280, Height: 720},
	{Name: "wide", Width: 1920, Height: 1080},
}

func loadProjectConfig() (*ProjectConfig, error) {
	data, err := os.ReadFile(projectConfigFile)
	if err != nil {
		return nil, err
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveProjectConfig(cfg *ProjectConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(projectConfigFile, data, 0644)
}
