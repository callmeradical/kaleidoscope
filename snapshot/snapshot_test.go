package snapshot_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/project"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// chdir changes the working directory for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// ---------------------------------------------------------------------------
// Slug tests (via exported UniqueSlug helper)
// ---------------------------------------------------------------------------

func TestUniqueSlug_StandardURL(t *testing.T) {
	seen := map[string]int{}
	slug := snapshot.UniqueSlug("http://localhost:3000", seen)
	want := "localhost-3000"
	if slug != want {
		t.Errorf("slugify(http://localhost:3000) = %q, want %q", slug, want)
	}
}

func TestUniqueSlug_SubPath(t *testing.T) {
	seen := map[string]int{}
	slug := snapshot.UniqueSlug("http://localhost:3000/about/team", seen)
	want := "localhost-3000-about-team"
	if slug != want {
		t.Errorf("slugify(http://localhost:3000/about/team) = %q, want %q", slug, want)
	}
}

func TestUniqueSlug_Truncation(t *testing.T) {
	// Build a URL whose slug would exceed 80 chars.
	longPath := "http://localhost:3000/" + repeat("a", 100)
	seen := map[string]int{}
	slug := snapshot.UniqueSlug(longPath, seen)
	if len(slug) > 80 {
		t.Errorf("slug length %d exceeds 80", len(slug))
	}
}

func TestUniqueSlug_CollisionResolution(t *testing.T) {
	seen := map[string]int{}
	s1 := snapshot.UniqueSlug("http://localhost:3000", seen)
	s2 := snapshot.UniqueSlug("http://localhost:3000", seen)
	s3 := snapshot.UniqueSlug("http://localhost:3000", seen)

	if s1 == s2 {
		t.Errorf("s1 and s2 should differ: both %q", s1)
	}
	if s1 == s3 || s2 == s3 {
		t.Errorf("s3 %q should differ from s1 %q and s2 %q", s3, s1, s2)
	}
}

// ---------------------------------------------------------------------------
// NewID format test
// ---------------------------------------------------------------------------

func TestNewID_Format(t *testing.T) {
	id := snapshot.NewID()
	// Matches <timestamp> with optional -<hash> suffix.
	re := regexp.MustCompile(`^\d{8}T\d{6}Z(-[a-f0-9]+)?$`)
	if !re.MatchString(id) {
		t.Errorf("NewID() = %q, does not match expected pattern", id)
	}
}

// ---------------------------------------------------------------------------
// SnapshotRoot tests
// ---------------------------------------------------------------------------

func TestSnapshotRoot_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	root, err := snapshot.SnapshotRoot()
	if err != nil {
		t.Fatalf("SnapshotRoot: %v", err)
	}
	want := filepath.Join(dir, ".kaleidoscope", "snapshots")
	if root != want {
		t.Errorf("SnapshotRoot() = %q, want %q", root, want)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat %s: %v", root, err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", root)
	}
}

// ---------------------------------------------------------------------------
// Store test
// ---------------------------------------------------------------------------

func TestStore_WritesManifest(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	m := &snapshot.Manifest{
		ID:        "20240101T120000Z",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Project:   project.Config{Version: 1, URLs: []string{"http://localhost:3000"}},
		URLs: []snapshot.URLEntry{
			{URL: "http://localhost:3000", Slug: "localhost-3000"},
		},
	}
	if err := snapshot.Store(m); err != nil {
		t.Fatalf("Store: %v", err)
	}

	root, _ := snapshot.SnapshotRoot()
	manifestPath := filepath.Join(root, m.ID, "snapshot.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading snapshot.json: %v", err)
	}

	var got snapshot.Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal snapshot.json: %v", err)
	}
	if got.ID != m.ID {
		t.Errorf("ID: got %q, want %q", got.ID, m.ID)
	}
	if len(got.URLs) != 1 {
		t.Errorf("URLs count: got %d, want 1", len(got.URLs))
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestList_SortedDescending(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Write two fake snapshots with different timestamps.
	earlier := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	later := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)

	writeManifest(t, dir, &snapshot.Manifest{
		ID:        "20240101T100000Z",
		Timestamp: earlier,
		Project:   project.Config{Version: 1, URLs: []string{"http://localhost:3000"}},
	})
	writeManifest(t, dir, &snapshot.Manifest{
		ID:        "20240102T100000Z",
		Timestamp: later,
		Project:   project.Config{Version: 1, URLs: []string{"http://localhost:3000"}},
	})

	entries, err := snapshot.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("List returned %d entries, want 2", len(entries))
	}
	if !entries[0].Timestamp.After(entries[1].Timestamp) {
		t.Errorf("List not sorted descending: [0]=%v [1]=%v", entries[0].Timestamp, entries[1].Timestamp)
	}
}

// ---------------------------------------------------------------------------
// Baselines tests
// ---------------------------------------------------------------------------

func TestLoadBaselines_AbsentReturnsNil(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	bl, err := snapshot.LoadBaselines()
	if err != nil {
		t.Fatalf("LoadBaselines: %v", err)
	}
	if bl != nil {
		t.Errorf("expected nil baselines when file absent, got %+v", bl)
	}
}

func TestSaveLoadBaselines_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	orig := &snapshot.Baselines{SnapshotID: "20240101T120000Z-abc1234"}
	if err := snapshot.SaveBaselines(orig); err != nil {
		t.Fatalf("SaveBaselines: %v", err)
	}

	got, err := snapshot.LoadBaselines()
	if err != nil {
		t.Fatalf("LoadBaselines: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil baselines after save")
	}
	if got.SnapshotID != orig.SnapshotID {
		t.Errorf("SnapshotID: got %q, want %q", got.SnapshotID, orig.SnapshotID)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeManifest(t *testing.T, baseDir string, m *snapshot.Manifest) {
	t.Helper()
	snapshotsDir := filepath.Join(baseDir, ".kaleidoscope", "snapshots", m.ID)
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", snapshotsDir, err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotsDir, "snapshot.json"), data, 0644); err != nil {
		t.Fatalf("write snapshot.json: %v", err)
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
