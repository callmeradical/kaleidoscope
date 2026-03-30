package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/output"
)

//go:embed skills
var skillsFS embed.FS

func RunInstallSkills(args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		output.Fail("install-skills", err, "")
		os.Exit(2)
	}

	destDir := filepath.Join(home, ".claude", "commands")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		output.Fail("install-skills", fmt.Errorf("creating commands directory: %w", err), "")
		os.Exit(2)
	}

	entries, err := skillsFS.ReadDir("skills")
	if err != nil {
		output.Fail("install-skills", err, "")
		os.Exit(2)
	}

	installed := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := skillsFS.ReadFile(filepath.Join("skills", name))
		if err != nil {
			output.Fail("install-skills", fmt.Errorf("reading embedded skill %s: %w", name, err), "")
			os.Exit(2)
		}

		// Prefix skill files with "ks-" to namespace them
		destName := "ks-" + name
		destPath := filepath.Join(destDir, destName)

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			output.Fail("install-skills", fmt.Errorf("writing %s: %w", destPath, err), "")
			os.Exit(2)
		}

		// Trim .md extension for the command name
		cmdName := destName[:len(destName)-len(filepath.Ext(destName))]
		installed = append(installed, cmdName)
	}

	output.Success("install-skills", map[string]any{
		"installed": installed,
		"directory": destDir,
		"count":     len(installed),
		"usage":     "Use /<skill-name> in Claude Code, e.g. /ks-design-review",
	})
}
