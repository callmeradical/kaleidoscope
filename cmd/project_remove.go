package cmd

import (
	"errors"
	"os"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

func RunProjectRemove(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-remove", errors.New("missing argument"), "usage: ks project-remove <path>")
		os.Exit(2)
	}

	cfg, err := project.Load()
	if err != nil {
		output.Fail("project-remove", err, "run 'ks init' first")
		os.Exit(2)
	}

	found := false
	filtered := cfg.Paths[:0]
	for _, p := range cfg.Paths {
		if p == path {
			found = true
		} else {
			filtered = append(filtered, p)
		}
	}

	if !found {
		output.Fail("project-remove", errors.New("path not found"), "")
		os.Exit(2)
	}

	cfg.Paths = filtered

	if err := project.Save(cfg); err != nil {
		output.Fail("project-remove", err, "")
		os.Exit(2)
	}

	output.Success("project-remove", cfg)
}
