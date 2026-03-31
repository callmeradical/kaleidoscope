package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/output"
)

const hookScript = `#!/bin/sh
# Kaleidoscope pre-commit hook
# Runs snapshot + diff before each commit. Always exits 0 (advisory).

# Start browser if not running
ks status > /dev/null 2>&1 || ks start --local > /dev/null 2>&1

# Run snapshot
ks snapshot 2>/dev/null
if [ $? -ne 0 ]; then
  echo "ks: snapshot failed (URLs may be unreachable). Skipping regression check." >&2
  exit 0
fi

# Run diff and output results to stdout
ks diff
DIFF_EXIT=$?

if [ $DIFF_EXIT -ne 0 ]; then
  echo "ks: regressions detected. Review with: ks diff-report" >&2
fi

# Generate diff report
ks diff-report > /dev/null 2>&1

# Always exit 0 (advisory — agent decides whether to proceed)
exit 0
`

func RunInstallHook(args []string) {
	hookDir := filepath.Join(".git", "hooks")
	hookPath := filepath.Join(hookDir, "pre-commit")

	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		output.Fail("install-hook", fmt.Errorf("not in a git repository"), "Run from the root of a git repository")
		os.Exit(2)
	}

	if err := os.MkdirAll(hookDir, 0755); err != nil {
		output.Fail("install-hook", err, "")
		os.Exit(2)
	}

	if _, err := os.Stat(hookPath); err == nil {
		output.Fail("install-hook", fmt.Errorf("pre-commit hook already exists at %s", hookPath), "Remove the existing hook first or manually merge it")
		os.Exit(2)
	}

	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		output.Fail("install-hook", err, "")
		os.Exit(2)
	}

	output.Success("install-hook", map[string]any{
		"path":    hookPath,
		"message": "pre-commit hook installed. The hook runs ks snapshot && ks diff before each commit (advisory, always exits 0).",
	})
}
