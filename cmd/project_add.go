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

// RunProjectAdd implements the `ks project-add <path>` command.
// It appends a path to the existing .ks-project.json.
func RunProjectAdd(args []string) {
	p := getArg(args)
	if p == "" {
		output.Fail("project-add", errors.New("path argument is required"), "")
		os.Exit(2)
	}
	if !strings.HasPrefix(p, "/") {
		output.Fail("project-add", fmt.Errorf("path must start with /: %s", p), "")
		os.Exit(2)
	}

	dir, err := os.Getwd()
	if err != nil {
		output.Fail("project-add", fmt.Errorf("getting working directory: %w", err), "")
		os.Exit(2)
	}

	cfg, err := project.Load(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			output.Fail("project-add", errors.New(".ks-project.json not found"), "Run: ks init")
		} else {
			output.Fail("project-add", err, "")
		}
		os.Exit(2)
	}

	for _, existing := range cfg.Paths {
		if existing == p {
			output.Fail("project-add", fmt.Errorf("path already exists: %s", p), "")
			os.Exit(2)
		}
	}

	cfg.Paths = append(cfg.Paths, p)

	if err := project.Save(dir, cfg); err != nil {
		output.Fail("project-add", fmt.Errorf("saving config: %w", err), "")
		os.Exit(2)
	}

	output.Success("project-add", map[string]any{
		"added":      p,
		"paths":      cfg.Paths,
		"configPath": filepath.Join(dir, project.ConfigFile),
	})
}
