package cmd

import (
	"errors"
	"os"

	"github.com/callmeradical/kaleidoscope/output"
)

// hookScript is the pre-commit hook written to .git/hooks/pre-commit.
const hookScript = `#!/bin/sh
# kaleidoscope pre-commit hook — auto-installed by ks install-hook
# Advisory only: always exits 0 so the commit is never blocked.

# Verify ks is available.
if ! command -v ks >/dev/null 2>&1; then
  printf '{"ok":false,"command":"pre-commit","error":"ks not found in PATH","hint":"Install kaleidoscope and ensure ks is in your PATH"}\n'
  exit 0
fi

# Require project config.
if [ ! -f ".ks-project.json" ]; then
  printf '{"ok":false,"command":"pre-commit","error":"no .ks-project.json found","hint":"Run from the project root"}\n'
  exit 0
fi

# Auto-start Chrome via ks start if not already running.
ks status 2>/dev/null | grep -q '"running":true' || ks start 2>/dev/null

# Take snapshot before diff.
ks snapshot 2>/dev/null

# Output structured diff JSON to stdout for agent consumption.
ks diff 2>/dev/null

exit 0
`

// RunInstallHook installs a git pre-commit hook for regression checks.
func RunInstallHook(args []string) {
	// 1. Check .ks-project.json exists.
	if _, err := os.Stat(".ks-project.json"); err != nil {
		output.Fail("install-hook",
			errors.New("no .ks-project.json found in current directory"),
			"Run from the project root, or create a .ks-project.json first")
		os.Exit(2)
	}

	// 2. Check .git/ directory exists.
	info, err := os.Stat(".git")
	if err != nil || !info.IsDir() {
		output.Fail("install-hook",
			errors.New("no .git/ directory found"),
			"Run from the root of a git repository")
		os.Exit(2)
	}

	// 3. Check for an existing hook — do not silently overwrite.
	if _, err := os.Stat(".git/hooks/pre-commit"); err == nil {
		output.Fail("install-hook",
			errors.New("pre-commit hook already exists at .git/hooks/pre-commit"),
			"Remove the existing hook manually and re-run install-hook to replace it")
		os.Exit(2)
	}

	// 4. Ensure .git/hooks/ directory exists.
	if err := os.MkdirAll(".git/hooks", 0755); err != nil {
		output.Fail("install-hook", err, "")
		os.Exit(2)
	}

	// 5. Write the hook script.
	if err := os.WriteFile(".git/hooks/pre-commit", []byte(hookScript), 0644); err != nil {
		output.Fail("install-hook", err, "")
		os.Exit(2)
	}

	// 6. Set executable permission (overrides restrictive umask).
	if err := os.Chmod(".git/hooks/pre-commit", 0755); err != nil {
		output.Fail("install-hook", err, "")
		os.Exit(2)
	}

	// 7. Report success.
	output.Success("install-hook", map[string]any{
		"hookPath": ".git/hooks/pre-commit",
		"mode":     "0755",
		"advisory": true,
	})
}
