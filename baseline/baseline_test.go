package baseline_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/baseline"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func TestLoad_Missing(t *testing.T) {
	dir := t.TempDir()
	b, err := baseline.Load(dir)
	if err != nil {
		t.Fatalf("expected nil error for missing baselines.json, got: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil Baselines")
	}
	if len(b.Entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(b.Entries))
	}
}

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	fixture := baseline.Baselines{
		Entries: []baseline.BaselineEntry{
			{URL: "http://localhost/", Path: "/", SnapshotID: "snap-001", AcceptedAt: "2026-01-01T00:00:00Z"},
			{URL: "http://localhost/dashboard", Path: "/dashboard", SnapshotID: "snap-001", AcceptedAt: "2026-01-01T00:00:00Z"},
		},
	}
	data, _ := json.Marshal(fixture)
	if err := os.WriteFile(filepath.Join(dir, "baselines.json"), data, 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	b, err := baseline.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(b.Entries))
	}
	if b.Entries[0].Path != "/" {
		t.Errorf("expected path '/', got %q", b.Entries[0].Path)
	}
	if b.Entries[1].SnapshotID != "snap-001" {
		t.Errorf("expected snapshot_id 'snap-001', got %q", b.Entries[1].SnapshotID)
	}
}

func TestAccept_Insert(t *testing.T) {
	b := &baseline.Baselines{}
	u := snapshot.URLEntry{URL: "http://localhost/", Path: "/"}
	changed := b.Accept(u, "snap-001")
	if !changed {
		t.Fatal("expected Accept to return true for new entry")
	}
	if len(b.Entries) != 1 {
		t.Fatalf("expected 1 entry after insert, got %d", len(b.Entries))
	}
	if b.Entries[0].SnapshotID != "snap-001" {
		t.Errorf("expected snapshot_id 'snap-001', got %q", b.Entries[0].SnapshotID)
	}
	if b.Entries[0].Path != "/" {
		t.Errorf("expected path '/', got %q", b.Entries[0].Path)
	}
	if b.Entries[0].AcceptedAt == "" {
		t.Error("expected AcceptedAt to be set")
	}
}

func TestAccept_Update(t *testing.T) {
	before := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	b := &baseline.Baselines{
		Entries: []baseline.BaselineEntry{
			{URL: "http://localhost/", Path: "/", SnapshotID: "snap-001", AcceptedAt: before},
		},
	}
	u := snapshot.URLEntry{URL: "http://localhost/", Path: "/"}
	changed := b.Accept(u, "snap-002")
	if !changed {
		t.Fatal("expected Accept to return true for updated entry")
	}
	if len(b.Entries) != 1 {
		t.Fatalf("expected 1 entry after update, got %d", len(b.Entries))
	}
	if b.Entries[0].SnapshotID != "snap-002" {
		t.Errorf("expected snapshot_id 'snap-002', got %q", b.Entries[0].SnapshotID)
	}
	if b.Entries[0].AcceptedAt == before {
		t.Error("expected AcceptedAt to be updated")
	}
}

func TestAccept_NoOp(t *testing.T) {
	acceptedAt := "2026-01-01T00:00:00Z"
	b := &baseline.Baselines{
		Entries: []baseline.BaselineEntry{
			{URL: "http://localhost/", Path: "/", SnapshotID: "snap-001", AcceptedAt: acceptedAt},
		},
	}
	u := snapshot.URLEntry{URL: "http://localhost/", Path: "/"}
	changed := b.Accept(u, "snap-001")
	if changed {
		t.Fatal("expected Accept to return false (no-op) for same snapshot_id")
	}
	if b.Entries[0].AcceptedAt != acceptedAt {
		t.Error("expected AcceptedAt to remain unchanged")
	}
}

func TestSave_Atomic(t *testing.T) {
	dir := t.TempDir()
	b := &baseline.Baselines{
		Entries: []baseline.BaselineEntry{
			{URL: "http://localhost/", Path: "/", SnapshotID: "snap-001", AcceptedAt: "2026-01-01T00:00:00Z"},
		},
	}
	if err := b.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// baselines.json must exist
	outPath := filepath.Join(dir, "baselines.json")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("baselines.json not found after Save: %v", err)
	}

	// must be valid JSON
	var loaded baseline.Baselines
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("baselines.json is not valid JSON: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected 1 entry in saved file, got %d", len(loaded.Entries))
	}

	// no temp files should remain
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "baselines.json" {
			t.Errorf("unexpected file in dir after Save: %s", e.Name())
		}
	}
}

func TestForPath_Found(t *testing.T) {
	b := &baseline.Baselines{
		Entries: []baseline.BaselineEntry{
			{URL: "http://localhost/", Path: "/", SnapshotID: "snap-001", AcceptedAt: "2026-01-01T00:00:00Z"},
			{URL: "http://localhost/dashboard", Path: "/dashboard", SnapshotID: "snap-002", AcceptedAt: "2026-01-01T00:00:00Z"},
		},
	}
	entry := b.ForPath("/dashboard")
	if entry == nil {
		t.Fatal("expected to find entry for /dashboard")
	}
	if entry.SnapshotID != "snap-002" {
		t.Errorf("expected snap-002, got %q", entry.SnapshotID)
	}
}

func TestForPath_NotFound(t *testing.T) {
	b := &baseline.Baselines{}
	entry := b.ForPath("/nonexistent")
	if entry != nil {
		t.Fatalf("expected nil for missing path, got %+v", entry)
	}
}
