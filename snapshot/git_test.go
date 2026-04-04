package snapshot

import (
	"os"
	"testing"
)

// TestShortCommitHash_InRepo verifies that ShortCommitHash returns a non-empty
// 7-character hash when called from within a git repository.
func TestShortCommitHash_InRepo(t *testing.T) {
	// /workspace is a git repo (confirmed by the presence of .git)
	got := ShortCommitHash()
	if got == "" {
		t.Fatal("ShortCommitHash() returned empty string inside a git repo; want 7-char hash")
	}
	if len(got) != 7 {
		t.Errorf("ShortCommitHash() = %q (len %d); want 7-char hash", got, len(got))
	}
}

// TestShortCommitHash_NotRepo verifies that ShortCommitHash returns "" when
// called from a directory that is not a git repository.
func TestShortCommitHash_NotRepo(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})

	got := ShortCommitHash()
	if got != "" {
		t.Errorf("ShortCommitHash() = %q outside git repo; want empty string", got)
	}
}
