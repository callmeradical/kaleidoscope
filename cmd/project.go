package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
)

const projectConfigFile = ".ks-project.json"

// Breakpoint is an exported struct for project-level breakpoint config.
// (The unexported breakpoint in cmd/breakpoints.go is used for the ks breakpoints command.)
type Breakpoint struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ProjectConfig is the structure stored in .ks-project.json.
type ProjectConfig struct {
	Name        string       `json:"name"`
	BaseURL     string       `json:"baseURL"`
	Paths       []string     `json:"paths"`
	Breakpoints []Breakpoint `json:"breakpoints"`
}

var defaultProjectBreakpoints = []Breakpoint{
	{"mobile", 375, 812},
	{"tablet", 768, 1024},
	{"desktop", 1280, 720},
	{"wide", 1920, 1080},
}

func readProjectConfig() (*ProjectConfig, error) {
	data, err := os.ReadFile(projectConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no .ks-project.json found in current directory")
		}
		return nil, err
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func writeProjectConfig(cfg *ProjectConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(projectConfigFile, data, 0644)
}

// RunInit creates a new .ks-project.json in the current directory.
func RunInit(args []string) {
	name := getFlagValue(args, "--name")
	baseURL := getFlagValue(args, "--base-url")
	paths := getFlagValue(args, "--paths")

	if name == "" {
		output.Fail("init", fmt.Errorf("--name is required"), "")
		os.Exit(2)
	}
	if baseURL == "" {
		output.Fail("init", fmt.Errorf("--base-url is required"), "")
		os.Exit(2)
	}
	if paths == "" {
		output.Fail("init", fmt.Errorf("--paths is required"), "")
		os.Exit(2)
	}

	if _, err := os.Stat(projectConfigFile); err == nil {
		output.Fail("init", fmt.Errorf(".ks-project.json already exists"), "")
		os.Exit(2)
	}

	parts := strings.Split(paths, ",")
	var pathList []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			pathList = append(pathList, p)
		}
	}

	cfg := ProjectConfig{
		Name:        name,
		BaseURL:     baseURL,
		Paths:       pathList,
		Breakpoints: defaultProjectBreakpoints,
	}

	if err := writeProjectConfig(&cfg); err != nil {
		output.Fail("init", err, "")
		os.Exit(2)
	}

	output.Success("init", map[string]any{
		"path":        projectConfigFile,
		"name":        cfg.Name,
		"baseURL":     cfg.BaseURL,
		"paths":       cfg.Paths,
		"breakpoints": cfg.Breakpoints,
	})
}

// RunProjectAdd appends a path to the project config.
func RunProjectAdd(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-add", fmt.Errorf("path argument required"), "")
		os.Exit(2)
	}

	cfg, err := readProjectConfig()
	if err != nil {
		output.Fail("project-add", err, "Run: ks init --name <name> --base-url <url> --paths /")
		os.Exit(2)
	}

	for _, p := range cfg.Paths {
		if p == path {
			output.Fail("project-add", fmt.Errorf("path already exists: %s", path), "")
			os.Exit(2)
		}
	}

	cfg.Paths = append(cfg.Paths, path)
	if err := writeProjectConfig(cfg); err != nil {
		output.Fail("project-add", err, "")
		os.Exit(2)
	}

	output.Success("project-add", map[string]any{
		"path":  path,
		"paths": cfg.Paths,
	})
}

// RunProjectRemove removes a path from the project config.
func RunProjectRemove(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-remove", fmt.Errorf("path argument required"), "")
		os.Exit(2)
	}

	cfg, err := readProjectConfig()
	if err != nil {
		output.Fail("project-remove", err, "Run: ks init --name <name> --base-url <url> --paths /")
		os.Exit(2)
	}

	idx := -1
	for i, p := range cfg.Paths {
		if p == path {
			idx = i
			break
		}
	}
	if idx == -1 {
		output.Fail("project-remove", fmt.Errorf("path not found: %s", path), "")
		os.Exit(2)
	}

	cfg.Paths = append(cfg.Paths[:idx], cfg.Paths[idx+1:]...)
	if err := writeProjectConfig(cfg); err != nil {
		output.Fail("project-remove", err, "")
		os.Exit(2)
	}

	output.Success("project-remove", map[string]any{
		"path":  path,
		"paths": cfg.Paths,
	})
}

// RunProjectShow outputs the full project config as structured JSON.
func RunProjectShow(args []string) {
	cfg, err := readProjectConfig()
	if err != nil {
		output.Fail("project-show", err, "Run: ks init --name <name> --base-url <url> --paths /")
		os.Exit(2)
	}
	output.Success("project-show", cfg)
}
