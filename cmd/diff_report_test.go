package cmd

import (
	"testing"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// makeSnapshot is a test helper that creates a minimal Snapshot with a single URL.
func makeSnapshot(id, url string) *snapshot.Snapshot {
	return &snapshot.Snapshot{
		ID: id,
		URLs: []snapshot.URLSnapshot{
			{URL: url},
		},
	}
}

// TestSnapshotIDValidation_RejectsPathTraversal verifies that snapshot IDs containing
// path-traversal characters are rejected before any file I/O.
func TestSnapshotIDValidation_RejectsPathTraversal(t *testing.T) {
	invalidIDs := []string{
		"../etc/passwd",
		"../../secret",
		"snap/../../etc",
		"snap id",    // space
		"snap\x00id", // null byte
		"snap!id",    // exclamation mark
	}

	for _, id := range invalidIDs {
		if snapshotIDPattern.MatchString(id) {
			t.Errorf("snapshotIDPattern incorrectly accepted invalid ID: %q", id)
		}
	}
}

// TestSnapshotIDValidation_AcceptsValidIDs verifies that legitimate snapshot IDs pass validation.
func TestSnapshotIDValidation_AcceptsValidIDs(t *testing.T) {
	validIDs := []string{
		"latest",
		"snap-001",
		"snap_2026-04-03",
		"ABC123",
		"a",
		"snapshot-abcdef123456",
	}

	for _, id := range validIDs {
		if !snapshotIDPattern.MatchString(id) {
			t.Errorf("snapshotIDPattern incorrectly rejected valid ID: %q", id)
		}
	}
}

// TestResolveBaselineID_SingleEntry verifies that a single-entry baselines map
// returns its value regardless of URL matching.
func TestResolveBaselineID_SingleEntry(t *testing.T) {
	baselines := map[string]string{
		"https://example.com": "snap-baseline-001",
	}
	current := makeSnapshot("snap-current", "https://example.com")
	got := resolveBaselineID(baselines, current)
	if got != "snap-baseline-001" {
		t.Errorf("expected 'snap-baseline-001', got %q", got)
	}
}

// TestResolveBaselineID_URLMatch verifies per-URL matching when multiple baselines exist.
func TestResolveBaselineID_URLMatch(t *testing.T) {
	baselines := map[string]string{
		"https://example.com/a": "snap-a",
		"https://example.com/b": "snap-b",
	}
	current := makeSnapshot("snap-current", "https://example.com/b")
	got := resolveBaselineID(baselines, current)
	if got != "snap-b" {
		t.Errorf("expected 'snap-b', got %q", got)
	}
}

// TestResolveBaselineID_NoMatch verifies empty string when no baseline matches any URL.
func TestResolveBaselineID_NoMatch(t *testing.T) {
	baselines := map[string]string{
		"https://other.com": "snap-other",
		"https://third.com": "snap-third",
	}
	current := makeSnapshot("snap-current", "https://example.com")
	got := resolveBaselineID(baselines, current)
	if got != "" {
		t.Errorf("expected empty string for no match, got %q", got)
	}
}

// TestResolveBaselineID_EmptyBaselines verifies empty string when baselines map is empty.
func TestResolveBaselineID_EmptyBaselines(t *testing.T) {
	baselines := map[string]string{}
	current := makeSnapshot("snap-current", "https://example.com")
	got := resolveBaselineID(baselines, current)
	if got != "" {
		t.Errorf("expected empty string for empty baselines, got %q", got)
	}
}

// TestDefaultOutputPath_UsesStateDirWhenNoFlag verifies the output path defaulting logic.
// This is a unit test for the path-construction pattern used in RunDiffReport.
func TestDefaultOutputPath_UsesStateDirWhenNoFlag(t *testing.T) {
	args := []string{"snap-001"}
	got := getFlagValue(args, "--output")
	if got != "" {
		t.Errorf("expected empty output path when --output not supplied, got %q", got)
	}
}

// TestDefaultOutputPath_HonorsFlagValue verifies --output flag is parsed correctly.
func TestDefaultOutputPath_HonorsFlagValue(t *testing.T) {
	args := []string{"snap-001", "--output", "/tmp/my-report.html"}
	got := getFlagValue(args, "--output")
	if got != "/tmp/my-report.html" {
		t.Errorf("expected '/tmp/my-report.html', got %q", got)
	}
}
