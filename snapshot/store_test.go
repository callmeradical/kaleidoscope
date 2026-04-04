package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeMetaJSON writes a meta.json for a snapshot under <snapshotsDir>/<id>/.
func writeMetaJSON(t *testing.T, dir string, s Snapshot) {
	t.Helper()
	snapsDir := filepath.Join(dir, "snapshots", s.ID)
	if err := os.MkdirAll(snapsDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", snapsDir, err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snapsDir, "meta.json"), data, 0644); err != nil {
		t.Fatalf("write meta.json: %v", err)
	}
}

func TestGetSnapshot_InvalidID_PathTraversal(t *testing.T) {
	tmp := t.TempDir()
	setTestStateDir(t, tmp)

	_, err := GetSnapshot("../evil")
	if err == nil {
		t.Fatal("expected error for path traversal ID")
	}
	if err.Error() != "invalid snapshot ID" {
		t.Errorf("expected 'invalid snapshot ID', got %q", err.Error())
	}
}

func TestGetSnapshot_InvalidID_WithSlash(t *testing.T) {
	tmp := t.TempDir()
	setTestStateDir(t, tmp)

	_, err := GetSnapshot("foo/bar")
	if err == nil {
		t.Fatal("expected error for ID with slash")
	}
	if err.Error() != "invalid snapshot ID" {
		t.Errorf("expected 'invalid snapshot ID', got %q", err.Error())
	}
}

func TestListSnapshots_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	setTestStateDir(t, tmp)

	snaps, err := ListSnapshots()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected empty slice, got %v", snaps)
	}
}

func TestLatestSnapshot_NoSnapshots(t *testing.T) {
	tmp := t.TempDir()
	setTestStateDir(t, tmp)

	snap, err := LatestSnapshot()
	if snap != nil {
		t.Errorf("expected nil snapshot, got %v", snap)
	}
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Error() != "no snapshots found" {
		t.Errorf("expected 'no snapshots found', got %q", err.Error())
	}
}

func TestListSnapshots_SortedByCreatedAt(t *testing.T) {
	tmp := t.TempDir()
	setTestStateDir(t, tmp)

	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	snapsIn := []Snapshot{
		{ID: "snap-b", URLPath: "/", CreatedAt: base.Add(2 * time.Hour)},
		{ID: "snap-a", URLPath: "/", CreatedAt: base},
		{ID: "snap-c", URLPath: "/", CreatedAt: base.Add(4 * time.Hour)},
	}
	for _, s := range snapsIn {
		writeMetaJSON(t, tmp, s)
	}

	snaps, err := ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots error: %v", err)
	}
	if len(snaps) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(snaps))
	}
	expectedOrder := []string{"snap-a", "snap-b", "snap-c"}
	for i, s := range snaps {
		if s.ID != expectedOrder[i] {
			t.Errorf("position %d: want %q, got %q", i, expectedOrder[i], s.ID)
		}
	}
}
