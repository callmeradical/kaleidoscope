package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
)

// RunInit implements the `ks init` command.
func RunInit(args []string) {
	name := getFlagValue(args, "--name")
	baseURL := getFlagValue(args, "--base-url")
	paths := getFlagValue(args, "--paths")

	if name == "" {
		output.Fail("init", errors.New("missing --name"), "")
		os.Exit(2)
	}
	if baseURL == "" {
		output.Fail("init", errors.New("missing --base-url"), "")
		os.Exit(2)
	}

	if _, err := os.Stat(projectConfigFile); err == nil {
		output.Fail("init", errors.New("project already initialized"), "")
		os.Exit(2)
	}

	var parsedPaths []string
	if paths != "" {
		parsedPaths = strings.Split(paths, ",")
	} else {
		parsedPaths = []string{"/"}
	}

	cfg := ProjectConfig{
		Name:        name,
		BaseURL:     baseURL,
		Paths:       parsedPaths,
		Breakpoints: defaultProjectBreakpoints,
	}

	if err := saveProjectConfig(&cfg); err != nil {
		output.Fail("init", err, "")
		os.Exit(2)
	}

	output.Success("init", cfg)
}

// RunProjectAdd implements the `ks project-add` command.
func RunProjectAdd(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-add", errors.New("missing path argument"), "")
		os.Exit(2)
	}

	cfg, err := loadProjectConfig()
	if err != nil {
		output.Fail("project-add", errors.New("project not initialized"), "run ks init first")
		os.Exit(2)
	}

	for _, p := range cfg.Paths {
		if p == path {
			output.Fail("project-add", errors.New("path already exists"), "")
			os.Exit(2)
		}
	}

	cfg.Paths = append(cfg.Paths, path)

	if err := saveProjectConfig(cfg); err != nil {
		output.Fail("project-add", err, "")
		os.Exit(2)
	}

	output.Success("project-add", map[string]any{"path": path, "paths": cfg.Paths})
}

// RunProjectRemove implements the `ks project-remove` command.
func RunProjectRemove(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-remove", errors.New("missing path argument"), "")
		os.Exit(2)
	}

	cfg, err := loadProjectConfig()
	if err != nil {
		output.Fail("project-remove", errors.New("project not initialized"), "run ks init first")
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
		output.Fail("project-remove", errors.New("path not found"), "")
		os.Exit(2)
	}

	cfg.Paths = append(cfg.Paths[:idx], cfg.Paths[idx+1:]...)

	if err := saveProjectConfig(cfg); err != nil {
		output.Fail("project-remove", err, "")
		os.Exit(2)
	}

	output.Success("project-remove", map[string]any{"path": path, "paths": cfg.Paths})
}

// RunProjectShow implements the `ks project-show` command.
func RunProjectShow(args []string) {
	cfg, err := loadProjectConfig()
	if err != nil {
		output.Fail("project-show", errors.New("project not initialized"), "run ks init first")
		os.Exit(2)
	}

	output.Success("project-show", cfg)
}
