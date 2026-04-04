package gitutil

import (
	"os/exec"
	"strings"
)

// ShortHash returns the short (7-char) git commit hash of HEAD, or an empty
// string if git is unavailable or the working directory is not a git repo.
func ShortHash() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
