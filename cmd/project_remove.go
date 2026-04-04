package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

// RunProjectRemove implements the `ks project-remove <path>` command.
// It removes a path from the existing .ks-project.json.
func RunProjectRemove(args []string) {
	p := getArg(args)
	if p == "" {
		output.Fail("project-remove", errors.New("path argument is required"), "")
		os.Exit(2)
	}

	dir, err := os.Getwd()
	if err != nil {
		output.Fail("project-remove", fmt.Errorf("getting working directory: %w", err), "")
		os.Exit(2)
	}

	cfg, err := project.Load(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			output.Fail("project-remove", errors.New(".ks-project.json not found"), "Run: ks init")
		} else {
			output.Fail("project-remove", err, "")
		}
		os.Exit(2)
	}

	found := false
	var newPaths []string
	for _, existing := range cfg.Paths {
		if existing == p {
			found = true
		} else {
			newPaths = append(newPaths, existing)
		}
	}

	if !found {
		output.Fail("project-remove", fmt.Errorf("path not found: %s", p), "")
		os.Exit(2)
	}

	cfg.Paths = newPaths

	if err := project.Save(dir, cfg); err != nil {
		output.Fail("project-remove", fmt.Errorf("saving config: %w", err), "")
		os.Exit(2)
	}

	output.Success("project-remove", map[string]any{
		"removed":    p,
		"paths":      cfg.Paths,
		"configPath": filepath.Join(dir, project.ConfigFile),
	})
}
