package snapshot_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// ---------------------------------------------------------------------------
// URLDir
// ---------------------------------------------------------------------------

func TestURLDir_Root(t *testing.T) {
	got := snapshot.URLDir("/")
	if got != "root" {
		t.Errorf("URLDir(%q) = %q, want %q", "/", got, "root")
	}
}

func TestURLDir_SingleSegment(t *testing.T) {
	got := snapshot.URLDir("/about")
	if got != "about" {
		t.Errorf("URLDir(%q) = %q, want %q", "/about", got, "about")
	}
}

func TestURLDir_MultiSegment(t *testing.T) {
	got := snapshot.URLDir("/products/items")
	if got != "products_items" {
		t.Errorf("URLDir(%q) = %q, want %q", "/products/items", got, "products_items")
	}
}

func TestURLDir_StripQueryString(t *testing.T) {
	got := snapshot.URLDir("/page?q=1")
	if got != "page" {
		t.Errorf("URLDir(%q) = %q, want %q", "/page?q=1", got, "page")
	}
}

func TestURLDir_SpecialChars(t *testing.T) {
	got := snapshot.URLDir("/my page! @#")
	if got == "" {
		t.Fatal("URLDir returned empty string for a non-empty path")
	}
	// No leading or trailing underscores; non-alnum → '_'
	if strings.HasPrefix(got, "_") || strings.HasSuffix(got, "_") {
		t.Errorf("URLDir result has leading/trailing underscore: %q", got)
	}
	// Must not contain any chars outside [a-zA-Z0-9_-]
	for _, c := range got {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			t.Errorf("URLDir result contains invalid char %q in %q", c, got)
		}
	}
}

// ---------------------------------------------------------------------------
// GenerateID
// ---------------------------------------------------------------------------

func TestGenerateID_Format(t *testing.T) {
	id := snapshot.GenerateID()
	if id == "" {
		t.Fatal("GenerateID returned empty string")
	}
	parts := strings.SplitN(id, "-", 2)
	// Timestamp prefix must be a numeric string of at least 13 digits (unix ms)
	tsPart := parts[0]
	if len(tsPart) < 13 {
		t.Errorf("timestamp prefix too short: %q (len %d, want ≥13)", tsPart, len(tsPart))
	}
	if _, err := strconv.ParseInt(tsPart, 10, 64); err != nil {
		t.Errorf("timestamp prefix is not numeric: %q", tsPart)
	}
	// If a commit suffix is present it must be alphanumeric
	if len(parts) == 2 {
		hash := parts[1]
		for _, c := range hash {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				t.Errorf("commit hash contains invalid char %q in %q", c, hash)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// WriteManifest / ReadManifest round-trip
// ---------------------------------------------------------------------------

func TestWriteReadManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &snapshot.Manifest{
		ID:         "1234567890123-abc1234",
		CommitHash: "abc1234",
		Timestamp:  time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		ProjectConfig: snapshot.ProjectConfig{
			Name:        "test-proj",
			URLs:        []string{"http://localhost:3000"},
			Breakpoints: []string{"mobile", "desktop"},
		},
		URLs: []snapshot.URLEntry{
			{URL: "http://localhost:3000", Dir: "root", AxNodeCount: 42},
		},
	}

	if err := snapshot.WriteManifest(dir, want); err != nil {
		t.Fatalf("WriteManifest error: %v", err)
	}

	got, err := snapshot.ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if got.CommitHash != want.CommitHash {
		t.Errorf("CommitHash: got %q, want %q", got.CommitHash, want.CommitHash)
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, want.Timestamp)
	}
	if got.ProjectConfig.Name != want.ProjectConfig.Name {
		t.Errorf("ProjectConfig.Name: got %q, want %q", got.ProjectConfig.Name, want.ProjectConfig.Name)
	}
	if len(got.URLs) != len(want.URLs) {
		t.Errorf("URLs length: got %d, want %d", len(got.URLs), len(want.URLs))
	}
}

// ---------------------------------------------------------------------------
// List — sort order
// ---------------------------------------------------------------------------

func TestList_SortDescending(t *testing.T) {
	// Set up a temp dir as the project-local .kaleidoscope to ensure
	// SnapshotsDir uses our temp location.
	tmpRoot := t.TempDir()
	ksDir := filepath.Join(tmpRoot, ".kaleidoscope")
	if err := os.MkdirAll(ksDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpRoot)

	older := &snapshot.Manifest{
		ID:        "older",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	newer := &snapshot.Manifest{
		ID:        "newer",
		Timestamp: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
	}

	// Write the manifests into snapshot subdirs
	ssDir := filepath.Join(ksDir, "snapshots")
	for _, m := range []*snapshot.Manifest{older, newer} {
		subDir := filepath.Join(ssDir, m.ID)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := snapshot.WriteManifest(subDir, m); err != nil {
			t.Fatalf("WriteManifest(%s): %v", m.ID, err)
		}
	}

	got, err := snapshot.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d manifests, want 2", len(got))
	}
	if got[0].ID != "newer" {
		t.Errorf("List[0].ID = %q, want %q (should be most recent first)", got[0].ID, "newer")
	}
	if got[1].ID != "older" {
		t.Errorf("List[1].ID = %q, want %q", got[1].ID, "older")
	}
}

func TestList_EmptyDirectory(t *testing.T) {
	tmpRoot := t.TempDir()
	ksDir := filepath.Join(tmpRoot, ".kaleidoscope")
	if err := os.MkdirAll(ksDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpRoot)

	got, err := snapshot.List()
	if err != nil {
		t.Fatalf("List on empty dir returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List on empty dir returned %d items, want 0", len(got))
	}
}
