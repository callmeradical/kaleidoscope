package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

func RunInit(args []string) {
	if _, err := os.Stat(project.ConfigFile); err == nil {
		output.Fail("init", errors.New("project already initialised"), "delete .ks-project.json to reinitialise")
		os.Exit(2)
	}

	name := getFlagValue(args, "--name")
	if name == "" {
		output.Fail("init", errors.New("missing required flag"), "--name is required")
		os.Exit(2)
	}

	baseURL := getFlagValue(args, "--base-url")
	if baseURL == "" {
		output.Fail("init", errors.New("missing required flag"), "--base-url is required")
		os.Exit(2)
	}

	paths := []string{"/"}
	if rawPaths := getFlagValue(args, "--paths"); rawPaths != "" {
		parts := strings.Split(rawPaths, ",")
		paths = make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				paths = append(paths, p)
			}
		}
	}

	cfg := &project.ProjectConfig{
		Name:        name,
		BaseURL:     baseURL,
		Paths:       paths,
		Breakpoints: project.DefaultBreakpoints(),
	}

	if err := project.Save(cfg); err != nil {
		output.Fail("init", err, "")
		os.Exit(2)
	}

	output.Success("init", cfg)
}
