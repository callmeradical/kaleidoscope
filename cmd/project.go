package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunInit(args []string) {
	name := getFlagValue(args, "--name")
	baseURL := getFlagValue(args, "--base-url")
	pathsStr := getFlagValue(args, "--paths")

	if name == "" || baseURL == "" {
		output.Fail("init", errors.New("--name and --base-url are required"), "Usage: ks init --name <name> --base-url <url> [--paths /,/dashboard]")
		os.Exit(2)
	}

	if _, err := os.Stat(snapshot.ProjectFile); err == nil {
		output.Fail("init", fmt.Errorf("%s already exists", snapshot.ProjectFile), "Use ks project-show to view the current config")
		os.Exit(2)
	}

	var paths []string
	if pathsStr != "" {
		for _, p := range strings.Split(pathsStr, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				paths = append(paths, p)
			}
		}
	}
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	p := &snapshot.ProjectConfig{
		Name:        name,
		BaseURL:     baseURL,
		Paths:       paths,
		Breakpoints: snapshot.DefaultBreakpoints,
	}
	if err := snapshot.SaveProject(p); err != nil {
		output.Fail("init", err, "")
		os.Exit(2)
	}

	output.Success("init", map[string]any{
		"file":    snapshot.ProjectFile,
		"project": p,
	})
}

func RunProjectAdd(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-add", errors.New("URL path argument required"), "Usage: ks project-add /path")
		os.Exit(2)
	}

	p, err := snapshot.LoadProject()
	if err != nil {
		output.Fail("project-add", err, "Run 'ks init' to create a project")
		os.Exit(2)
	}

	for _, existing := range p.Paths {
		if existing == path {
			output.Success("project-add", map[string]any{
				"message": "path already exists",
				"path":    path,
				"project": p,
			})
			return
		}
	}

	p.Paths = append(p.Paths, path)
	if err := snapshot.SaveProject(p); err != nil {
		output.Fail("project-add", err, "")
		os.Exit(2)
	}

	output.Success("project-add", map[string]any{
		"added":   path,
		"project": p,
	})
}

func RunProjectRemove(args []string) {
	path := getArg(args)
	if path == "" {
		output.Fail("project-remove", errors.New("URL path argument required"), "Usage: ks project-remove /path")
		os.Exit(2)
	}

	p, err := snapshot.LoadProject()
	if err != nil {
		output.Fail("project-remove", err, "Run 'ks init' to create a project")
		os.Exit(2)
	}

	var newPaths []string
	found := false
	for _, existing := range p.Paths {
		if existing == path {
			found = true
			continue
		}
		newPaths = append(newPaths, existing)
	}

	if !found {
		output.Fail("project-remove", fmt.Errorf("path %q not found in project", path), "")
		os.Exit(2)
	}

	p.Paths = newPaths
	if err := snapshot.SaveProject(p); err != nil {
		output.Fail("project-remove", err, "")
		os.Exit(2)
	}

	output.Success("project-remove", map[string]any{
		"removed": path,
		"project": p,
	})
}

func RunProjectShow(args []string) {
	p, err := snapshot.LoadProject()
	if err != nil {
		output.Fail("project-show", err, "Run 'ks init' to create a project")
		os.Exit(2)
	}
	output.Success("project-show", p)
}
