package snapshot_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// writeSnapshot writes a snapshot JSON file to dir/snapshots/<id>/snapshot.json.
func writeSnapshot(t *testing.T, dir string, s *snapshot.Snapshot) {
	t.Helper()
	snapshotDir := filepath.Join(dir, "snapshots", s.ID)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "snapshot.json"), data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoad_ValidSnapshot(t *testing.T) {
	dir := t.TempDir()
	want := &snapshot.Snapshot{
		ID:        "snap-001",
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		CommitSHA: "abc123",
	}
	writeSnapshot(t, dir, want)

	got, err := snapshot.Load(dir, "snap-001")
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if got.CommitSHA != want.CommitSHA {
		t.Errorf("CommitSHA: got %q, want %q", got.CommitSHA, want.CommitSHA)
	}
}

func TestLoad_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := snapshot.Load(dir, "../etc")
	if err == nil {
		t.Fatal("expected error for path traversal ID, got nil")
	}
}

func TestLatest_ReturnsNewest(t *testing.T) {
	dir := t.TempDir()

	older := &snapshot.Snapshot{
		ID:        "snap-old",
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	newer := &snapshot.Snapshot{
		ID:        "snap-new",
		CreatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	writeSnapshot(t, dir, older)
	writeSnapshot(t, dir, newer)

	got, err := snapshot.Latest(dir)
	if err != nil {
		t.Fatalf("Latest returned unexpected error: %v", err)
	}
	if got.ID != newer.ID {
		t.Errorf("Latest returned %q, want %q", got.ID, newer.ID)
	}
}

func TestLoadBaseline_MissingDefaultKey(t *testing.T) {
	dir := t.TempDir()

	// Write baselines.json with no "default" key.
	data, _ := json.Marshal(map[string]string{"other": "snap-001"})
	if err := os.WriteFile(filepath.Join(dir, "baselines.json"), data, 0644); err != nil {
		t.Fatalf("writing baselines.json: %v", err)
	}

	_, err := snapshot.LoadBaseline(dir)
	if err == nil {
		t.Fatal("expected error when default key missing, got nil")
	}
}

func TestLoadBaseline_MissingFile(t *testing.T) {
	dir := t.TempDir()
	// No baselines.json written.
	_, err := snapshot.LoadBaseline(dir)
	if err == nil {
		t.Fatal("expected error when baselines.json missing, got nil")
	}
}
