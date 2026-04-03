package cmd

import (
	"errors"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

// RunProjectAdd handles `ks project-add <path>`.
func RunProjectAdd(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-add", errors.New("path argument is required"), "Usage: ks project-add <path>")
		exit(2)
		return
	}

	cfg, err := project.Read()
	if err != nil {
		output.Fail("project-add", err, "Run: ks init --name <name> --base-url <url> --paths <paths>")
		exit(2)
		return
	}

	for _, existing := range cfg.Paths {
		if existing == path {
			output.Fail("project-add", errors.New("path already exists: "+path), "")
			exit(2)
			return
		}
	}

	cfg.Paths = append(cfg.Paths, path)

	if err := project.Write(cfg); err != nil {
		output.Fail("project-add", err, "")
		exit(2)
		return
	}

	output.Success("project-add", map[string]any{
		"path":  path,
		"paths": cfg.Paths,
	})
}

// RunProjectRemove handles `ks project-remove <path>`.
func RunProjectRemove(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-remove", errors.New("path argument is required"), "Usage: ks project-remove <path>")
		exit(2)
		return
	}

	cfg, err := project.Read()
	if err != nil {
		output.Fail("project-remove", err, "Run: ks init --name <name> --base-url <url> --paths <paths>")
		exit(2)
		return
	}

	found := false
	newPaths := cfg.Paths[:0:0]
	for _, p := range cfg.Paths {
		if p == path {
			found = true
		} else {
			newPaths = append(newPaths, p)
		}
	}

	if !found {
		output.Fail("project-remove", errors.New("path not found: "+path), "")
		exit(2)
		return
	}

	cfg.Paths = newPaths

	if err := project.Write(cfg); err != nil {
		output.Fail("project-remove", err, "")
		exit(2)
		return
	}

	output.Success("project-remove", map[string]any{
		"removed": path,
		"paths":   cfg.Paths,
	})
}

// RunProjectShow handles `ks project-show`.
func RunProjectShow(args []string) {
	cfg, err := project.Read()
	if err != nil {
		output.Fail("project-show", err, "Run: ks init --name <name> --base-url <url> --paths <paths>")
		exit(2)
		return
	}

	output.Success("project-show", cfg)
}
