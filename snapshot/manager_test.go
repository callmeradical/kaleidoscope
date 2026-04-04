package snapshot_test

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// chdir temporarily changes the working directory to dir and restores it on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// setupKaleidoscope creates a .kaleidoscope/ directory in dir so browser.StateDir()
// returns the local path.
func setupKaleidoscope(t *testing.T, dir string) string {
	t.Helper()
	ksDir := filepath.Join(dir, ".kaleidoscope")
	if err := os.MkdirAll(ksDir, 0755); err != nil {
		t.Fatalf("mkdirall .kaleidoscope: %v", err)
	}
	return ksDir
}

// ---------------------------------------------------------------------------
// TestURLSlug
// ---------------------------------------------------------------------------

func TestURLSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSub  string // slug must contain this substring
		notEmpty bool   // slug must be non-empty
	}{
		{
			name:    "root path",
			input:   "https://example.com/",
			wantSub: "example.com",
		},
		{
			name:    "subpath",
			input:   "https://example.com/about",
			wantSub: "about",
		},
		{
			name:    "query strings excluded",
			input:   "https://example.com/page?foo=bar",
			wantSub: "page",
		},
		{
			name:    "unicode replaced with underscore",
			input:   "https://example.com/héllo",
			wantSub: "example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			slug := snapshot.URLSlug(tc.input)
			if slug == "" {
				t.Errorf("expected non-empty slug for %q, got empty", tc.input)
				return
			}
			if tc.wantSub != "" && !strings.Contains(slug, tc.wantSub) {
				t.Errorf("expected slug to contain %q, got %q", tc.wantSub, slug)
			}
			// Query strings should NOT appear in slug
			if strings.Contains(slug, "?") || strings.Contains(slug, "=") {
				t.Errorf("slug should not contain query string, got %q", slug)
			}
		})
	}
}

func TestURLSlug_MaxLength(t *testing.T) {
	// Construct a URL with a very long path (>200 chars).
	longPath := strings.Repeat("a", 250)
	input := "https://example.com/" + longPath
	slug := snapshot.URLSlug(input)
	if len(slug) > 200 {
		t.Errorf("expected slug length <= 200, got %d", len(slug))
	}
}

// ---------------------------------------------------------------------------
// TestNewID
// ---------------------------------------------------------------------------

func TestNewID_WithHash(t *testing.T) {
	hash := "abc1234"
	before := time.Now().Unix()
	id := snapshot.NewID(hash)
	after := time.Now().Unix()

	// Format: "<unix_seconds>-<hash>"
	if !strings.HasSuffix(id, "-"+hash) {
		t.Errorf("expected ID to end with -%s, got %q", hash, id)
	}
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		t.Fatalf("expected ID with dash separator, got %q", id)
	}
	epoch, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		t.Fatalf("expected numeric epoch prefix in %q: %v", id, err)
	}
	if epoch < before || epoch > after {
		t.Errorf("epoch %d out of range [%d, %d]", epoch, before, after)
	}
}

func TestNewID_WithoutHash(t *testing.T) {
	before := time.Now().Unix()
	id := snapshot.NewID("")
	after := time.Now().Unix()

	// Format: "<unix_seconds>" only — no dash
	if strings.Contains(id, "-") {
		t.Errorf("expected no dash in timestamp-only ID, got %q", id)
	}
	epoch, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		t.Fatalf("expected numeric epoch in %q: %v", id, err)
	}
	if epoch < before || epoch > after {
		t.Errorf("epoch %d out of range [%d, %d]", epoch, before, after)
	}
}

// ---------------------------------------------------------------------------
// TestListIDs
// ---------------------------------------------------------------------------

func TestListIDs_NewestFirst(t *testing.T) {
	dir := t.TempDir()
	setupKaleidoscope(t, dir)
	chdir(t, dir)

	// Create fake snapshot directories with known epoch-based names (out of order).
	ids := []string{
		"1700000003-abc0003",
		"1700000001-abc0001",
		"1700000005-abc0005",
		"1700000002-abc0002",
		"1700000004-abc0004",
	}
	snapshotsDir := filepath.Join(dir, ".kaleidoscope", "snapshots")
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if err := os.Mkdir(filepath.Join(snapshotsDir, id), 0755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := snapshot.ListIDs()
	if err != nil {
		t.Fatalf("ListIDs() error: %v", err)
	}
	if len(got) != len(ids) {
		t.Fatalf("expected %d IDs, got %d: %v", len(ids), len(got), got)
	}

	// Verify descending order.
	expected := make([]string, len(ids))
	copy(expected, ids)
	sort.Sort(sort.Reverse(sort.StringSlice(expected)))
	for i, want := range expected {
		if got[i] != want {
			t.Errorf("ListIDs()[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestListIDs_Empty(t *testing.T) {
	dir := t.TempDir()
	setupKaleidoscope(t, dir)
	// Create snapshots dir but leave it empty.
	if err := os.MkdirAll(filepath.Join(dir, ".kaleidoscope", "snapshots"), 0755); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	got, err := snapshot.ListIDs()
	if err != nil {
		t.Fatalf("expected no error for empty snapshots dir, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 IDs, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// TestWriteReadManifest
// ---------------------------------------------------------------------------

func TestWriteReadManifest(t *testing.T) {
	dir := t.TempDir()
	setupKaleidoscope(t, dir)
	chdir(t, dir)

	id := "1700000001-abc0001"
	ts := time.Now().UTC().Truncate(time.Second)
	m := &snapshot.Manifest{
		ID:          id,
		Timestamp:   ts,
		CommitHash:  "abc0001",
		ProjectURLs: []string{"http://localhost:3000/", "http://localhost:3000/about"},
		ProjectName: "testapp",
		BaseURL:     "http://localhost:3000",
		URLSummaries: []snapshot.URLSummary{
			{
				URL:                "http://localhost:3000/",
				Slug:               "localhost_3000_",
				ContrastViolations: 2,
				TouchViolations:    1,
				TypographyWarnings: 3,
				AXActiveNodes:      42,
				AXTotalNodes:       50,
			},
		},
	}

	if err := snapshot.WriteManifest(id, m); err != nil {
		t.Fatalf("WriteManifest error: %v", err)
	}

	got, err := snapshot.ReadManifest(id)
	if err != nil {
		t.Fatalf("ReadManifest error: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID: got %q, want %q", got.ID, m.ID)
	}
	if !got.Timestamp.Equal(m.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, m.Timestamp)
	}
	if got.CommitHash != m.CommitHash {
		t.Errorf("CommitHash: got %q, want %q", got.CommitHash, m.CommitHash)
	}
	if len(got.ProjectURLs) != len(m.ProjectURLs) {
		t.Errorf("ProjectURLs len: got %d, want %d", len(got.ProjectURLs), len(m.ProjectURLs))
	}
	if len(got.URLSummaries) != 1 {
		t.Errorf("URLSummaries len: got %d, want 1", len(got.URLSummaries))
	} else {
		s := got.URLSummaries[0]
		if s.ContrastViolations != 2 {
			t.Errorf("ContrastViolations: got %d, want 2", s.ContrastViolations)
		}
		if s.AXActiveNodes != 42 {
			t.Errorf("AXActiveNodes: got %d, want 42", s.AXActiveNodes)
		}
	}
}

// ---------------------------------------------------------------------------
// TestReadBaselines
// ---------------------------------------------------------------------------

func TestReadBaselinesNoFile(t *testing.T) {
	dir := t.TempDir()
	setupKaleidoscope(t, dir)
	chdir(t, dir)

	// No baselines.json exists; should return nil, nil.
	b, err := snapshot.ReadBaselines()
	if err != nil {
		t.Fatalf("expected nil error when baselines.json absent, got %v", err)
	}
	if b != nil {
		t.Errorf("expected nil BaselinesFile when absent, got %+v", b)
	}
}

func TestWriteReadBaselines(t *testing.T) {
	dir := t.TempDir()
	setupKaleidoscope(t, dir)
	chdir(t, dir)

	want := &snapshot.BaselinesFile{
		DefaultBaseline: "1700000001-abc0001",
		URLBaselines: map[string]string{
			"localhost_3000_": "1700000001-abc0001",
		},
	}

	if err := snapshot.WriteBaselines(want); err != nil {
		t.Fatalf("WriteBaselines error: %v", err)
	}

	got, err := snapshot.ReadBaselines()
	if err != nil {
		t.Fatalf("ReadBaselines error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil BaselinesFile, got nil")
	}
	if got.DefaultBaseline != want.DefaultBaseline {
		t.Errorf("DefaultBaseline: got %q, want %q", got.DefaultBaseline, want.DefaultBaseline)
	}
	if len(got.URLBaselines) != len(want.URLBaselines) {
		t.Errorf("URLBaselines len: got %d, want %d", len(got.URLBaselines), len(want.URLBaselines))
	}
}
