package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAccept_EmptyStore_CreatesEntry verifies that accepting a snapshot on a
// fresh (no existing file) store creates baselines.json with the correct entry.
func TestAccept_EmptyStore_CreatesEntry(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	f, err := mgr.Accept("snap-001", []string{"/"})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	entry, ok := f.Baselines["/"]
	if !ok {
		t.Fatal("expected baselines[\"/\"] to exist")
	}
	if entry.SnapshotID != "snap-001" {
		t.Errorf("snapshotId = %q, want %q", entry.SnapshotID, "snap-001")
	}
	if entry.AcceptedAt.IsZero() {
		t.Error("AcceptedAt should not be zero")
	}

	// Assert file now exists on disk.
	if _, err := os.Stat(filepath.Join(dir, "baselines.json")); err != nil {
		t.Errorf("baselines.json not found on disk: %v", err)
	}
}

// TestAccept_Idempotent verifies that accepting the same snapshot twice does
// not change the AcceptedAt timestamp.
func TestAccept_Idempotent(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	f1, err := mgr.Accept("snap-001", []string{"/"})
	if err != nil {
		t.Fatalf("first Accept: %v", err)
	}
	firstAcceptedAt := f1.Baselines["/"].AcceptedAt

	// Small sleep so time.Now() could differ if we did update.
	time.Sleep(5 * time.Millisecond)

	f2, err := mgr.Accept("snap-001", []string{"/"})
	if err != nil {
		t.Fatalf("second Accept: %v", err)
	}

	if f2.Baselines["/"].AcceptedAt != firstAcceptedAt {
		t.Error("AcceptedAt changed on second call — not idempotent")
	}
	if len(f2.Baselines) != 1 {
		t.Errorf("baselines map has %d entries, want 1", len(f2.Baselines))
	}
}

// TestAccept_SinglePath_PreservesOthers verifies that accepting with a specific
// path only updates that path and leaves other baseline entries unchanged.
func TestAccept_SinglePath_PreservesOthers(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Pre-populate baselines with two entries.
	initial := &BaselinesFile{
		Version: 1,
		Baselines: map[string]Entry{
			"/":          {SnapshotID: "snap-001", AcceptedAt: time.Now().UTC()},
			"/dashboard": {SnapshotID: "snap-001", AcceptedAt: time.Now().UTC()},
		},
	}
	if err := mgr.Save(initial); err != nil {
		t.Fatalf("Save initial: %v", err)
	}

	f, err := mgr.Accept("snap-002", []string{"/dashboard"})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if f.Baselines["/"].SnapshotID != "snap-001" {
		t.Errorf("/ baseline changed — expected snap-001, got %q", f.Baselines["/"].SnapshotID)
	}
	if f.Baselines["/dashboard"].SnapshotID != "snap-002" {
		t.Errorf("/dashboard baseline = %q, want snap-002", f.Baselines["/dashboard"].SnapshotID)
	}
}

// TestAccept_UpdatedTimestamp verifies that Accept sets f.Updated to
// approximately now.
func TestAccept_UpdatedTimestamp(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	before := time.Now().UTC().Add(-time.Millisecond)
	f, err := mgr.Accept("snap-001", []string{"/"})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	after := time.Now().UTC().Add(time.Millisecond)

	if f.Updated.Before(before) || f.Updated.After(after) {
		t.Errorf("Updated %v is outside [%v, %v]", f.Updated, before, after)
	}
}

// TestLoad_MissingFile_ReturnsEmpty verifies that Load on a non-existent path
// returns an empty BaselinesFile rather than an error.
func TestLoad_MissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(filepath.Join(dir, "nonexistent"))

	f, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(f.Baselines) != 0 {
		t.Errorf("expected 0 baselines, got %d", len(f.Baselines))
	}
	if f.Version != 1 {
		t.Errorf("Version = %d, want 1", f.Version)
	}
}

// TestSave_AtomicWrite verifies that no .tmp file remains after Save and that
// the final file contains valid JSON.
func TestSave_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	f := &BaselinesFile{
		Version: 1,
		Updated: time.Now().UTC(),
		Baselines: map[string]Entry{
			"/": {SnapshotID: "snap-001", AcceptedAt: time.Now().UTC()},
		},
	}
	if err := mgr.Save(f); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// .tmp file must not exist.
	tmpPath := filepath.Join(dir, "baselines.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error(".tmp file still exists after Save")
	}

	// Final file must be valid JSON.
	data, err := os.ReadFile(filepath.Join(dir, "baselines.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var check BaselinesFile
	if err := json.Unmarshal(data, &check); err != nil {
		t.Errorf("baselines.json is not valid JSON: %v", err)
	}
}
