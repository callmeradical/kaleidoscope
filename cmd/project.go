package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
)

const projectConfigFile = ".ks-project.json"

// ProjectConfig is the project-level configuration stored in .ks-project.json.
type ProjectConfig struct {
	Name        string       `json:"name"`
	BaseURL     string       `json:"baseUrl"`
	Paths       []string     `json:"paths,omitempty"`
	Breakpoints []Breakpoint `json:"breakpoints"`
}

// Breakpoint defines a named viewport breakpoint.
type Breakpoint struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// DefaultBreakpoints are used when no custom breakpoints are configured.
var DefaultBreakpoints = []Breakpoint{
	{Name: "mobile", Width: 375, Height: 812},
	{Name: "tablet", Width: 768, Height: 1024},
	{Name: "desktop", Width: 1280, Height: 720},
	{Name: "wide", Width: 1920, Height: 1080},
}

func projectConfigPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	return filepath.Join(cwd, projectConfigFile), nil
}

func loadProjectConfig() (*ProjectConfig, error) {
	path, err := projectConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no project config found (run 'ks init' first): %w", err)
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid project config: %w", err)
	}
	return &cfg, nil
}

func saveProjectConfig(cfg *ProjectConfig) error {
	path, err := projectConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// RunInit creates a new .ks-project.json in the current directory.
func RunInit(args []string) {
	name := getFlagValue(args, "--name")
	baseURL := getFlagValue(args, "--base-url")
	pathsRaw := getFlagValue(args, "--paths")

	if name == "" {
		// Default to directory name
		cwd, err := os.Getwd()
		if err != nil {
			output.Fail("init", err, "Cannot determine working directory")
			os.Exit(2)
		}
		name = filepath.Base(cwd)
	}

	var paths []string
	if pathsRaw != "" {
		for _, p := range strings.Split(pathsRaw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				paths = append(paths, p)
			}
		}
	}

	cfg := &ProjectConfig{
		Name:        name,
		BaseURL:     baseURL,
		Paths:       paths,
		Breakpoints: DefaultBreakpoints,
	}

	if err := saveProjectConfig(cfg); err != nil {
		output.Fail("init", err, "Could not write project config")
		os.Exit(2)
	}

	output.Success("init", map[string]any{
		"name":        cfg.Name,
		"baseUrl":     cfg.BaseURL,
		"paths":       cfg.Paths,
		"breakpoints": cfg.Breakpoints,
		"configPath":  projectConfigFile,
	})
}

// RunProjectShow displays the current project configuration.
func RunProjectShow(args []string) {
	cfg, err := loadProjectConfig()
	if err != nil {
		output.Fail("project-show", err, "Run 'ks init' to create a project config")
		os.Exit(2)
	}

	output.Success("project-show", map[string]any{
		"name":        cfg.Name,
		"baseUrl":     cfg.BaseURL,
		"paths":       cfg.Paths,
		"breakpoints": cfg.Breakpoints,
	})
}

// RunProjectAdd adds paths to the project configuration.
func RunProjectAdd(args []string) {
	cfg, err := loadProjectConfig()
	if err != nil {
		output.Fail("project-add", err, "Run 'ks init' to create a project config")
		os.Exit(2)
	}

	newPaths := getNonFlagArgs(args)
	if len(newPaths) == 0 {
		output.Fail("project-add", fmt.Errorf("no paths provided"), "Usage: ks project-add <path> [path...]")
		os.Exit(2)
	}

	existing := make(map[string]bool)
	for _, p := range cfg.Paths {
		existing[p] = true
	}

	var added []string
	for _, p := range newPaths {
		if !existing[p] {
			cfg.Paths = append(cfg.Paths, p)
			existing[p] = true
			added = append(added, p)
		}
	}

	if err := saveProjectConfig(cfg); err != nil {
		output.Fail("project-add", err, "Could not write project config")
		os.Exit(2)
	}

	output.Success("project-add", map[string]any{
		"added": added,
		"paths": cfg.Paths,
	})
}

// RunProjectRemove removes paths from the project configuration.
func RunProjectRemove(args []string) {
	cfg, err := loadProjectConfig()
	if err != nil {
		output.Fail("project-remove", err, "Run 'ks init' to create a project config")
		os.Exit(2)
	}

	removePaths := getNonFlagArgs(args)
	if len(removePaths) == 0 {
		output.Fail("project-remove", fmt.Errorf("no paths provided"), "Usage: ks project-remove <path> [path...]")
		os.Exit(2)
	}

	removeSet := make(map[string]bool)
	for _, p := range removePaths {
		removeSet[p] = true
	}

	var kept []string
	var removed []string
	for _, p := range cfg.Paths {
		if removeSet[p] {
			removed = append(removed, p)
		} else {
			kept = append(kept, p)
		}
	}

	cfg.Paths = kept

	if err := saveProjectConfig(cfg); err != nil {
		output.Fail("project-remove", err, "Could not write project config")
		os.Exit(2)
	}

	output.Success("project-remove", map[string]any{
		"removed": removed,
		"paths":   cfg.Paths,
	})
}
