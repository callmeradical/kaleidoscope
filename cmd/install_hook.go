package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/output"
)

const hookScript = `#!/bin/sh
# Pre-commit hook installed by 'ks install-hook'
# This hook is advisory: it always exits 0 and does not block commits.

KS=$(command -v ks 2>/dev/null)
if [ -z "$KS" ]; then
  echo "ks: kaleidoscope not found in PATH — skipping pre-commit checks" >&2
  exit 0
fi

# Auto-start Chrome if not running
if ! "$KS" status 2>/dev/null | grep -q '"running":true'; then
  "$KS" start --local >/dev/null 2>&1 || true
fi

# Run snapshot; fail gracefully if URLs are unreachable
if ! "$KS" snapshot 2>/dev/null; then
  echo "ks: snapshot failed — project URLs may be unreachable. Skipping diff." >&2
  exit 0
fi

# Run diff — JSON output goes to stdout for agent consumption
"$KS" diff 2>/dev/null || true

exit 0
`

// installHookCore is the testable core of RunInstallHook.
// It takes a base directory and force flag, and returns (result, hint, err, exitCode).
func installHookCore(dir string, force bool) (map[string]any, string, error, int) {
	projectFile := filepath.Join(dir, ".ks-project.json")
	gitDir := filepath.Join(dir, ".git")
	hooksDir := filepath.Join(dir, ".git", "hooks")
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")

	if _, err := os.Stat(projectFile); err != nil {
		return nil, "", errors.New("no .ks-project.json found — run 'ks project init' first"), 2
	}

	if _, err := os.Stat(gitDir); err != nil {
		return nil, "", errors.New("not a git repository — no .git/ directory found"), 2
	}

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create hooks directory: %w", err), 2
	}

	if _, err := os.Stat(hookPath); err == nil && !force {
		return nil,
			"Run 'ks install-hook --force' to replace it, or remove .git/hooks/pre-commit manually.",
			errors.New("pre-commit hook already exists at .git/hooks/pre-commit — use --force to overwrite"),
			1
	}

	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		return nil, "", fmt.Errorf("failed to write pre-commit hook: %w", err), 2
	}

	return map[string]any{
		"path":    ".git/hooks/pre-commit",
		"message": "pre-commit hook installed successfully",
		"note":    "Hook is advisory (exits 0). Edit .git/hooks/pre-commit to customize.",
	}, "", nil, 0
}

// RunInstallHook writes a git pre-commit hook that runs snapshot and diff checks.
func RunInstallHook(args []string) {
	force := hasFlag(args, "--force")

	result, hint, err, exitCode := installHookCore(".", force)
	if err != nil {
		output.Fail("install-hook", err, hint)
		os.Exit(exitCode)
	}

	output.Success("install-hook", result)
}
