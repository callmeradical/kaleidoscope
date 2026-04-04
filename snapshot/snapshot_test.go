package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// --- SnapshotDir path traversal validation ---

func TestSnapshotDir_RejectsDoubleDot(t *testing.T) {
	_, err := snapshot.SnapshotDir("../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for '..' in id, got nil")
	}
}

func TestSnapshotDir_RejectsSlash(t *testing.T) {
	_, err := snapshot.SnapshotDir("foo/bar")
	if err == nil {
		t.Fatal("expected error for '/' in id, got nil")
	}
}

func TestSnapshotDir_RejectsNullByte(t *testing.T) {
	_, err := snapshot.SnapshotDir("foo\x00bar")
	if err == nil {
		t.Fatal("expected error for null byte in id, got nil")
	}
}

func TestSnapshotDir_RejectsEmpty(t *testing.T) {
	_, err := snapshot.SnapshotDir("")
	if err == nil {
		t.Fatal("expected error for empty id, got nil")
	}
}

// --- Save + Load round-trip ---

func TestSaveLoad_RoundTrip(t *testing.T) {
	// Run inside a temp dir so .kaleidoscope/snapshots/ is isolated.
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	s := &snapshot.Snapshot{
		ID:        "20260101-120000-abc",
		CreatedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		CommitSHA: "deadbeef",
		URLs: []snapshot.URLSnapshot{
			{
				URL: "https://example.com",
				Audit: snapshot.AuditResult{
					ContrastViolations: 2,
					TouchViolations:    1,
				},
			},
		},
	}

	if err := snapshot.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := snapshot.Load(s.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("ID: got %q want %q", got.ID, s.ID)
	}
	if got.CommitSHA != s.CommitSHA {
		t.Errorf("CommitSHA: got %q want %q", got.CommitSHA, s.CommitSHA)
	}
	if len(got.URLs) != 1 {
		t.Fatalf("URLs: got %d want 1", len(got.URLs))
	}
	if got.URLs[0].Audit.ContrastViolations != 2 {
		t.Errorf("ContrastViolations: got %d want 2", got.URLs[0].Audit.ContrastViolations)
	}
}

func TestSaveLoad_FilePermissions(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	s := &snapshot.Snapshot{ID: "20260101-120000-perm"}
	if err := snapshot.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	jsonPath := filepath.Join(tmp, ".kaleidoscope", "snapshots", "20260101-120000-perm", "snapshot.json")
	info, err := os.Stat(jsonPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("snapshot.json permissions: got %o want 0600", perm)
	}
}

// --- Latest returns error when no snapshots exist ---

func TestLatest_ErrorWhenNoSnapshots(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	_, err = snapshot.Latest()
	if err == nil {
		t.Fatal("expected error from Latest() when no snapshots exist, got nil")
	}
}

// --- LoadBaselines returns empty map (not error) when file absent ---

func TestLoadBaselines_AbsentFileReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	b, err := snapshot.LoadBaselines()
	if err != nil {
		t.Fatalf("expected nil error when baselines.json absent, got: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil Baselines map, got nil")
	}
	if len(b) != 0 {
		t.Errorf("expected empty map, got %d entries", len(b))
	}
}

// --- SaveBaselines + LoadBaselines round-trip ---

func TestSaveLoadBaselines_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	b := snapshot.Baselines{
		"https://example.com":       "20260101-120000-abc",
		"https://example.com/about": "20260101-120000-abc",
	}
	if err := snapshot.SaveBaselines(b); err != nil {
		t.Fatalf("SaveBaselines: %v", err)
	}

	got, err := snapshot.LoadBaselines()
	if err != nil {
		t.Fatalf("LoadBaselines: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
	id, ok := got.BaselineFor("https://example.com")
	if !ok {
		t.Error("expected BaselineFor to find entry for https://example.com")
	}
	if id != "20260101-120000-abc" {
		t.Errorf("expected id %q, got %q", "20260101-120000-abc", id)
	}
}

func TestBaselineFor_MissingReturnsNotFound(t *testing.T) {
	b := snapshot.Baselines{}
	_, ok := b.BaselineFor("https://missing.example.com")
	if ok {
		t.Error("expected ok=false for missing URL, got true")
	}
}
