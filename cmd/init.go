package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

// RunInit implements the `ks init` command.
// It creates a new .ks-project.json in the current directory.
func RunInit(args []string) {
	name := getFlagValue(args, "--name")
	baseURL := getFlagValue(args, "--base-url")
	paths := getFlagValue(args, "--paths")

	if name == "" {
		output.Fail("init", errors.New("--name is required"), "")
		os.Exit(2)
	}
	if baseURL == "" {
		output.Fail("init", errors.New("--base-url is required"), "")
		os.Exit(2)
	}
	if paths == "" {
		output.Fail("init", errors.New("--paths is required"), "")
		os.Exit(2)
	}

	dir, err := os.Getwd()
	if err != nil {
		output.Fail("init", fmt.Errorf("getting working directory: %w", err), "")
		os.Exit(2)
	}

	// Fail if config already exists.
	_, loadErr := project.Load(dir)
	if loadErr == nil {
		output.Fail("init", errors.New(".ks-project.json already exists"), "Delete .ks-project.json first, or use ks project-add to modify it.")
		os.Exit(2)
	} else if !errors.Is(loadErr, fs.ErrNotExist) {
		output.Fail("init", loadErr, "")
		os.Exit(2)
	}

	// Parse and validate paths.
	rawPaths := strings.Split(paths, ",")
	var parsedPaths []string
	for _, p := range rawPaths {
		p = strings.TrimSpace(p)
		if !strings.HasPrefix(p, "/") {
			output.Fail("init", fmt.Errorf("path must start with /: %s", p), "")
			os.Exit(2)
		}
		parsedPaths = append(parsedPaths, p)
	}

	cfg := project.Config{
		Name:        name,
		BaseURL:     baseURL,
		Paths:       parsedPaths,
		Breakpoints: project.DefaultBreakpoints,
	}

	if err := project.Save(dir, &cfg); err != nil {
		output.Fail("init", fmt.Errorf("saving config: %w", err), "")
		os.Exit(2)
	}

	output.Success("init", map[string]any{
		"name":        cfg.Name,
		"baseURL":     cfg.BaseURL,
		"paths":       cfg.Paths,
		"breakpoints": cfg.Breakpoints,
		"configPath":  filepath.Join(dir, project.ConfigFile),
	})
}
