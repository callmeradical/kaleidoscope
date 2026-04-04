package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindGitRootFindsGitDir verifies findGitRoot() locates the repo root from
// any subdirectory below it.
func TestFindGitRootFindsGitDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("setup subdirs: %v", err)
	}

	// Change CWD to the deeply nested subdir for this test.
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(sub); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, err := findGitRoot()
	if err != nil {
		t.Fatalf("findGitRoot() returned error: %v", err)
	}
	// Resolve symlinks for comparison (macOS /tmp is /private/tmp).
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Errorf("findGitRoot() = %q, want %q", gotResolved, wantResolved)
	}
}

// TestFindGitRootAtRoot verifies findGitRoot() works when CWD is the git root itself.
func TestFindGitRootAtRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, err := findGitRoot()
	if err != nil {
		t.Fatalf("findGitRoot() returned error: %v", err)
	}
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Errorf("findGitRoot() = %q, want %q", gotResolved, wantResolved)
	}
}

// TestFindGitRootNoRepo verifies findGitRoot() returns an error when there is
// no .git directory in the hierarchy.
func TestFindGitRootNoRepo(t *testing.T) {
	dir := t.TempDir()

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err := findGitRoot()
	if err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("expected error message to mention 'git', got: %v", err)
	}
}
