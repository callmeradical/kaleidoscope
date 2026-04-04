package snapshot

import (
	"os"
	"testing"
	"time"
)

func TestLoadBaselineMissing(t *testing.T) {
	setupStateDir(t)
	// No baselines.json written

	b, err := LoadBaseline()
	if err != nil {
		t.Fatalf("LoadBaseline returned error for missing file: %v", err)
	}
	if b != nil {
		t.Errorf("LoadBaseline: got %+v, want nil", b)
	}
}

func TestSaveLoadBaseline(t *testing.T) {
	stateDir := setupStateDir(t)

	now := time.Now().UTC().Truncate(time.Second)
	b := Baseline{
		SnapshotID: "20240101T120000Z-abc1234",
		SetAt:      now,
	}

	if err := SaveBaseline(&b); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(stateDir + "/baselines.json"); err != nil {
		t.Fatalf("baselines.json not found after save: %v", err)
	}

	loaded, err := LoadBaseline()
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadBaseline returned nil after save")
	}
	if loaded.SnapshotID != b.SnapshotID {
		t.Errorf("SnapshotID: got %q, want %q", loaded.SnapshotID, b.SnapshotID)
	}
	if !loaded.SetAt.Equal(b.SetAt) {
		t.Errorf("SetAt: got %v, want %v", loaded.SetAt, b.SetAt)
	}
}
