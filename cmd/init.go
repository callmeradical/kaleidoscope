package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

// exit is the function used to terminate the process. Overridden in tests.
var exit = os.Exit

// RunInit handles the `ks init` command.
func RunInit(args []string) {
	name := getFlagValue(args, "--name")
	if name == "" {
		output.Fail("init", errors.New("--name is required"), "")
		exit(2)
		return
	}

	baseURL := getFlagValue(args, "--base-url")
	if baseURL == "" {
		output.Fail("init", errors.New("--base-url is required"), "")
		exit(2)
		return
	}

	pathsStr := getFlagValue(args, "--paths")
	if pathsStr == "" {
		output.Fail("init", errors.New("--paths is required"), "")
		exit(2)
		return
	}

	var paths []string
	for _, p := range strings.Split(pathsStr, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}

	if project.Exists() {
		output.Fail("init", errors.New(".ks-project.json already exists"), "")
		exit(2)
		return
	}

	cfg := project.Config{
		Name:        name,
		BaseURL:     baseURL,
		Paths:       paths,
		Breakpoints: project.DefaultBreakpoints,
	}

	if err := project.Write(&cfg); err != nil {
		output.Fail("init", err, "")
		exit(2)
		return
	}

	output.Success("init", map[string]any{
		"path":        project.Filename,
		"name":        cfg.Name,
		"baseURL":     cfg.BaseURL,
		"paths":       cfg.Paths,
		"breakpoints": cfg.Breakpoints,
	})
}
