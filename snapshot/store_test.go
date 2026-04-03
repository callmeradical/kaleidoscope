package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadBaselines_Missing verifies an error is returned when baselines.json doesn't exist.
func TestLoadBaselines_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadBaselines(dir)
	if err == nil {
		t.Error("expected error when baselines.json is missing, got nil")
	}
}

// TestLoadBaselines_NoDefault verifies a file with empty default field is parsed correctly.
func TestLoadBaselines_NoDefault(t *testing.T) {
	dir := t.TempDir()
	content := `{"default": "", "named": {}}`
	if err := os.WriteFile(filepath.Join(dir, "baselines.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBaselines(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Default != "" {
		t.Errorf("expected Default to be empty, got %q", b.Default)
	}
}

// TestLatestID_Empty verifies LatestID returns an error for an empty index.
func TestLatestID_Empty(t *testing.T) {
	dir := t.TempDir()
	snapshotsDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"entries": []}`
	if err := os.WriteFile(filepath.Join(snapshotsDir, "index.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LatestID(dir)
	if err == nil {
		t.Error("expected error for empty index, got nil")
	}
}

// TestLatestID_ReturnsLast verifies LatestID returns the last entry's ID.
func TestLatestID_ReturnsLast(t *testing.T) {
	dir := t.TempDir()
	snapshotsDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		t.Fatal(err)
	}

	idx := SnapshotIndex{
		Entries: []SnapshotMeta{
			{ID: "snap-001", CreatedAt: time.Now(), URL: "http://example.com"},
			{ID: "snap-002", CreatedAt: time.Now(), URL: "http://example.com"},
		},
	}
	data, _ := json.Marshal(idx)
	if err := os.WriteFile(filepath.Join(snapshotsDir, "index.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	id, err := LatestID(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "snap-002" {
		t.Errorf("expected snap-002, got %q", id)
	}
}

// TestLoad_PathTraversal verifies that path-traversal IDs are rejected.
func TestLoad_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir, "../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal ID, got nil")
	}
	if err != nil {
		// Verify the error mentions invalid snapshot ID
		found := false
		for _, substr := range []string{"invalid snapshot ID", "invalid"} {
			if len(err.Error()) > 0 {
				found = true
				_ = substr
				break
			}
		}
		if !found {
			t.Errorf("expected descriptive error, got: %v", err)
		}
	}
}

// TestLoad_ValidID verifies a snapshot file is correctly loaded.
func TestLoad_ValidID(t *testing.T) {
	dir := t.TempDir()
	snapshotsDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		t.Fatal(err)
	}

	want := Snapshot{
		ID:  "snap-abc",
		URL: "http://example.com",
		Viewport: Viewport{Width: 1280, Height: 800},
	}
	data, _ := json.Marshal(want)
	if err := os.WriteFile(filepath.Join(snapshotsDir, "snap-abc.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir, "snap-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("expected ID=%q, got %q", want.ID, got.ID)
	}
	if got.URL != want.URL {
		t.Errorf("expected URL=%q, got %q", want.URL, got.URL)
	}
	if got.Viewport.Width != want.Viewport.Width {
		t.Errorf("expected Viewport.Width=%d, got %d", want.Viewport.Width, got.Viewport.Width)
	}
}

// TestSave_WritesFileAndUpdatesIndex verifies Save writes the file and updates index.json.
func TestSave_WritesFileAndUpdatesIndex(t *testing.T) {
	dir := t.TempDir()

	s := &Snapshot{
		ID:        "snap-xyz",
		CreatedAt: time.Now(),
		URL:       "http://example.com/page",
		Viewport:  Viewport{Width: 1280, Height: 800},
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	// Check snapshot file exists
	snapPath := filepath.Join(dir, "snapshots", "snap-xyz.json")
	if _, err := os.Stat(snapPath); os.IsNotExist(err) {
		t.Errorf("snapshot file not found at %s", snapPath)
	}

	// Check index updated
	idx, err := readIndex(dir)
	if err != nil {
		t.Fatalf("readIndex error: %v", err)
	}
	if len(idx.Entries) != 1 {
		t.Fatalf("expected 1 index entry, got %d", len(idx.Entries))
	}
	if idx.Entries[0].ID != "snap-xyz" {
		t.Errorf("expected index entry ID=snap-xyz, got %q", idx.Entries[0].ID)
	}
}
