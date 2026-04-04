package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/output"
)

const hookScript = `#!/bin/sh
# Kaleidoscope pre-commit hook
# Installed by: ks install-hook
# Advisory only — always exits 0

KS=$(command -v ks 2>/dev/null)
if [ -z "$KS" ]; then
  echo '{"ok":false,"command":"pre-commit","error":"ks not found in PATH","hint":"install kaleidoscope and ensure it is in PATH"}' >&2
  exit 0
fi

# Auto-start Chrome if not running
if ! "$KS" status >/dev/null 2>&1; then
  "$KS" start --local >/dev/null 2>&1 || true
fi

# Run snapshot; on failure, emit warning and skip diff
SNAP=$("$KS" snapshot 2>&1)
SNAP_OK=$(echo "$SNAP" | grep -c '"ok":true' || true)

if [ "$SNAP_OK" -eq 0 ]; then
  echo "$SNAP"
  echo '{"ok":false,"command":"pre-commit","error":"snapshot failed — skipping diff","hint":"check that project URLs are reachable"}' >&2
  exit 0
fi

# Run diff and emit to stdout for agent consumption
"$KS" diff

exit 0
`

// RunInstallHook installs a git pre-commit hook that runs ks snapshot and ks diff.
func RunInstallHook(args []string) {
	force := hasFlag(args, "--force")

	// Step B: validate .ks-project.json exists in CWD
	cwd, err := os.Getwd()
	if err != nil {
		output.Fail("install-hook", err, "")
		os.Exit(2)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".ks-project.json")); err != nil {
		output.Fail("install-hook", fmt.Errorf("no .ks-project.json found in current directory"), "run 'ks project init' to create a project config")
		os.Exit(2)
	}

	// Step C: locate git root
	gitRoot, err := findGitRoot()
	if err != nil {
		output.Fail("install-hook", err, "")
		os.Exit(2)
	}

	// Step D: compute hook path
	hookPath := filepath.Join(gitRoot, ".git", "hooks", "pre-commit")

	// Step E: check for existing hook
	overwritten := false
	if _, err := os.Stat(hookPath); err == nil {
		if !force {
			output.Fail("install-hook", fmt.Errorf("pre-commit hook already exists at %s", hookPath), "re-run with --force to overwrite")
			os.Exit(2)
		}
		overwritten = true
	}

	// Step F: ensure hooks directory exists
	if err := os.MkdirAll(filepath.Join(gitRoot, ".git", "hooks"), 0755); err != nil {
		output.Fail("install-hook", fmt.Errorf("failed to create hooks directory: %w", err), "")
		os.Exit(2)
	}

	// Step G: write hook script
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		output.Fail("install-hook", fmt.Errorf("failed to write hook: %w", err), "")
		os.Exit(2)
	}

	// Step H: set executable bit explicitly
	if err := os.Chmod(hookPath, 0755); err != nil {
		output.Fail("install-hook", fmt.Errorf("failed to set executable bit: %w", err), "")
		os.Exit(2)
	}

	// Step I: output success
	output.Success("install-hook", map[string]interface{}{
		"path":        hookPath,
		"overwritten": overwritten,
	})
}
