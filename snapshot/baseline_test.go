package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// setupKaleidoscopeDir creates a project-local .kaleidoscope dir in a temp root
// and changes CWD to that root so browser.StateDir() returns the local dir.
func setupKaleidoscopeDir(t *testing.T) string {
	t.Helper()
	tmpRoot := t.TempDir()
	ksDir := filepath.Join(tmpRoot, ".kaleidoscope")
	if err := os.MkdirAll(ksDir, 0755); err != nil {
		t.Fatalf("mkdir .kaleidoscope: %v", err)
	}
	t.Chdir(tmpRoot)
	return ksDir
}

// TestReadBaseline_NoFile asserts (nil, nil) when baselines.json does not exist.
func TestReadBaseline_NoFile(t *testing.T) {
	setupKaleidoscopeDir(t)

	b, err := snapshot.ReadBaseline()
	if err != nil {
		t.Fatalf("ReadBaseline returned unexpected error: %v", err)
	}
	if b != nil {
		t.Errorf("ReadBaseline with no file should return nil, got %+v", b)
	}
}

// TestWriteReadBaseline_RoundTrip asserts full field preservation across write/read.
func TestWriteReadBaseline_RoundTrip(t *testing.T) {
	setupKaleidoscopeDir(t)

	want := &snapshot.Baseline{
		SnapshotID: "1712345678901-abc1234",
		PromotedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		PromotedBy: "auto",
	}

	if err := snapshot.WriteBaseline(want); err != nil {
		t.Fatalf("WriteBaseline error: %v", err)
	}

	got, err := snapshot.ReadBaseline()
	if err != nil {
		t.Fatalf("ReadBaseline error: %v", err)
	}
	if got == nil {
		t.Fatal("ReadBaseline returned nil after write")
	}
	if got.SnapshotID != want.SnapshotID {
		t.Errorf("SnapshotID: got %q, want %q", got.SnapshotID, want.SnapshotID)
	}
	if !got.PromotedAt.Equal(want.PromotedAt) {
		t.Errorf("PromotedAt: got %v, want %v", got.PromotedAt, want.PromotedAt)
	}
	if got.PromotedBy != want.PromotedBy {
		t.Errorf("PromotedBy: got %q, want %q", got.PromotedBy, want.PromotedBy)
	}
}

// TestEnsureBaseline_FirstRun asserts that the first call returns (true, nil)
// and creates baselines.json with PromotedBy "auto".
func TestEnsureBaseline_FirstRun(t *testing.T) {
	setupKaleidoscopeDir(t)

	promoted, err := snapshot.EnsureBaseline("snap-001")
	if err != nil {
		t.Fatalf("EnsureBaseline error: %v", err)
	}
	if !promoted {
		t.Error("EnsureBaseline on first run should return true (promoted), got false")
	}

	b, err := snapshot.ReadBaseline()
	if err != nil {
		t.Fatalf("ReadBaseline after EnsureBaseline: %v", err)
	}
	if b == nil {
		t.Fatal("baselines.json should have been created")
	}
	if b.SnapshotID != "snap-001" {
		t.Errorf("SnapshotID: got %q, want %q", b.SnapshotID, "snap-001")
	}
	if b.PromotedBy != "auto" {
		t.Errorf("PromotedBy: got %q, want %q", b.PromotedBy, "auto")
	}
}

// TestEnsureBaseline_Idempotent asserts that a second call with a different ID
// returns (false, nil) and does NOT overwrite the first baseline.
func TestEnsureBaseline_Idempotent(t *testing.T) {
	setupKaleidoscopeDir(t)

	// First promotion
	if _, err := snapshot.EnsureBaseline("snap-001"); err != nil {
		t.Fatalf("first EnsureBaseline: %v", err)
	}

	// Second promotion with a different ID — should be a no-op
	promoted, err := snapshot.EnsureBaseline("snap-002")
	if err != nil {
		t.Fatalf("second EnsureBaseline: %v", err)
	}
	if promoted {
		t.Error("second EnsureBaseline should return false (not promoted), got true")
	}

	// Baseline must still point to snap-001
	b, err := snapshot.ReadBaseline()
	if err != nil {
		t.Fatalf("ReadBaseline: %v", err)
	}
	if b == nil {
		t.Fatal("baseline should still exist")
	}
	if b.SnapshotID != "snap-001" {
		t.Errorf("SnapshotID after second EnsureBaseline: got %q, want %q", b.SnapshotID, "snap-001")
	}
}
