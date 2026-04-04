package cmd

import (
	"os"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

func RunProjectShow(args []string) {
	cfg, err := project.Load()
	if err != nil {
		output.Fail("project-show", err, "run 'ks init' first")
		os.Exit(2)
	}

	output.Success("project-show", cfg)
}
