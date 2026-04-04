package cmd

import (
	"errors"
	"os"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

func RunProjectAdd(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-add", errors.New("missing argument"), "usage: ks project-add <path>")
		os.Exit(2)
	}

	cfg, err := project.Load()
	if err != nil {
		output.Fail("project-add", err, "run 'ks init' first")
		os.Exit(2)
	}

	for _, p := range cfg.Paths {
		if p == path {
			output.Fail("project-add", errors.New("path already exists"), "")
			os.Exit(2)
		}
	}

	cfg.Paths = append(cfg.Paths, path)

	if err := project.Save(cfg); err != nil {
		output.Fail("project-add", err, "")
		os.Exit(2)
	}

	output.Success("project-add", cfg)
}
