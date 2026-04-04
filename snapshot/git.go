package snapshot

import (
	"os/exec"
	"strings"
)

// ShortCommitHash returns the 7-char short commit hash of HEAD.
// Returns "" gracefully when not in a git repo, git is unavailable, or any other error.
func ShortCommitHash() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
