package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

// RunProjectShow implements the `ks project-show` command.
// It outputs the full project config as JSON.
func RunProjectShow(args []string) {
	dir, err := os.Getwd()
	if err != nil {
		output.Fail("project-show", fmt.Errorf("getting working directory: %w", err), "")
		os.Exit(2)
	}

	cfg, err := project.Load(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			output.Fail("project-show", errors.New(".ks-project.json not found"), "Run: ks init")
		} else {
			output.Fail("project-show", err, "")
		}
		os.Exit(2)
	}

	output.Success("project-show", cfg)
}
