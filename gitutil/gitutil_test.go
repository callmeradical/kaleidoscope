package gitutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/callmeradical/kaleidoscope/gitutil"
)

func TestShortHash_InGitRepo(t *testing.T) {
	// The workspace itself is a git repo with commits.
	hash := gitutil.ShortHash()
	if hash == "" {
		t.Error("expected non-empty short hash in a git repo, got empty string")
	}
	// Git short hash is typically 7 characters.
	if len(hash) < 7 {
		t.Errorf("expected hash of at least 7 chars, got %q (len %d)", hash, len(hash))
	}
}

func TestShortHash_NoGitAvailable(t *testing.T) {
	// Override PATH to a directory with no git binary to simulate unavailability.
	emptyBin := t.TempDir()
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", emptyBin)
	defer os.Setenv("PATH", origPath)

	// Should not panic; returns empty string gracefully.
	hash := gitutil.ShortHash()
	if hash != "" {
		t.Errorf("expected empty hash when git is unavailable, got %q", hash)
	}
}

func TestShortHash_NotAGitRepo(t *testing.T) {
	// Create a temp dir with no .git folder.
	dir := t.TempDir()

	// Change to the non-git directory.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Should not panic; returns empty string.
	hash := gitutil.ShortHash()
	if hash != "" {
		t.Errorf("expected empty hash outside git repo, got %q", hash)
	}
}

// TestShortHash_NewRepoNoCommits verifies graceful behavior when in a git repo but with no commits.
func TestShortHash_NewRepoNoCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Init a fresh repo with no commits.
	cmd := exec.Command("git", "init", filepath.Join(dir))
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hash := gitutil.ShortHash()
	if hash != "" {
		t.Errorf("expected empty hash in repo with no commits, got %q", hash)
	}
}
