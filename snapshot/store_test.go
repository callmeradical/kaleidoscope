package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, ".kaleidoscope")
	return NewStore(root)
}

func sampleResults() []PathResult {
	return []PathResult{
		{
			Path: "/",
			Screenshots: map[string][]byte{
				"mobile":  []byte("PNG-mobile"),
				"tablet":  []byte("PNG-tablet"),
				"desktop": []byte("PNG-desktop"),
				"wide":    []byte("PNG-wide"),
			},
			Audit:  map[string]any{"totalIssues": 3},
			AxTree: map[string]any{"nodeCount": 42},
		},
		{
			Path: "/about",
			Screenshots: map[string][]byte{
				"mobile":  []byte("PNG-mobile-about"),
				"tablet":  []byte("PNG-tablet-about"),
				"desktop": []byte("PNG-desktop-about"),
				"wide":    []byte("PNG-wide-about"),
			},
			Audit:  map[string]any{"totalIssues": 1},
			AxTree: map[string]any{"nodeCount": 20},
		},
	}
}

func TestGenerateID_WithCommit(t *testing.T) {
	ts := time.Date(2026, 4, 18, 15, 30, 0, 0, time.UTC)
	id := GenerateID(ts, "abc1234")
	want := "20260418T153000Z-abc1234"
	if id != want {
		t.Errorf("got %q, want %q", id, want)
	}
}

func TestGenerateID_NoCommit(t *testing.T) {
	ts := time.Date(2026, 4, 18, 15, 30, 0, 0, time.UTC)
	id := GenerateID(ts, "")
	want := "20260418T153000Z"
	if id != want {
		t.Errorf("got %q, want %q", id, want)
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"/", "_root"},
		{"", "_root"},
		{"/about", "about"},
		{"/docs/api", "docs_api"},
		{"/a/b/c", "a_b_c"},
	}
	for _, tc := range tests {
		got := sanitizePath(tc.input)
		if got != tc.want {
			t.Errorf("sanitizePath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCreate_DirectoryStructure(t *testing.T) {
	store := tempStore(t)
	ts := time.Date(2026, 4, 18, 15, 0, 0, 0, time.UTC)
	projectCfg := map[string]string{"name": "test"}

	m, err := store.Create(ts, "abc1234", projectCfg, sampleResults())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Check snapshot ID
	if m.ID != "20260418T150000Z-abc1234" {
		t.Errorf("unexpected ID: %s", m.ID)
	}

	// Check snapshot directory exists
	snapDir := filepath.Join(store.Root, "snapshots", m.ID)
	if _, err := os.Stat(snapDir); err != nil {
		t.Fatalf("snapshot dir missing: %v", err)
	}

	// Check snapshot.json
	manifestData, err := os.ReadFile(filepath.Join(snapDir, "snapshot.json"))
	if err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("invalid manifest: %v", err)
	}
	if manifest.CommitHash != "abc1234" {
		t.Errorf("commit hash: got %q, want %q", manifest.CommitHash, "abc1234")
	}

	// Check _root path directory
	rootDir := filepath.Join(snapDir, "_root")
	for _, bp := range []string{"mobile", "tablet", "desktop", "wide"} {
		png := filepath.Join(rootDir, bp+".png")
		if _, err := os.Stat(png); err != nil {
			t.Errorf("missing screenshot %s: %v", png, err)
		}
	}
	if _, err := os.Stat(filepath.Join(rootDir, "audit.json")); err != nil {
		t.Error("missing _root/audit.json")
	}
	if _, err := os.Stat(filepath.Join(rootDir, "ax-tree.json")); err != nil {
		t.Error("missing _root/ax-tree.json")
	}

	// Check /about path directory
	aboutDir := filepath.Join(snapDir, "about")
	if _, err := os.Stat(filepath.Join(aboutDir, "mobile.png")); err != nil {
		t.Error("missing about/mobile.png")
	}
	if _, err := os.Stat(filepath.Join(aboutDir, "audit.json")); err != nil {
		t.Error("missing about/audit.json")
	}
	if _, err := os.Stat(filepath.Join(aboutDir, "ax-tree.json")); err != nil {
		t.Error("missing about/ax-tree.json")
	}
}

func TestCreate_ManifestContents(t *testing.T) {
	store := tempStore(t)
	ts := time.Date(2026, 4, 18, 15, 0, 0, 0, time.UTC)
	projectCfg := map[string]any{"name": "myproj", "baseUrl": "http://localhost:3000"}

	m, err := store.Create(ts, "def5678", projectCfg, sampleResults())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if m.Stats.PathCount != 2 {
		t.Errorf("PathCount: got %d, want 2", m.Stats.PathCount)
	}
	if m.Stats.BreakpointCount != 4 {
		t.Errorf("BreakpointCount: got %d, want 4", m.Stats.BreakpointCount)
	}
	if m.Stats.ScreenshotCount != 8 {
		t.Errorf("ScreenshotCount: got %d, want 8", m.Stats.ScreenshotCount)
	}
	if m.Stats.AuditCount != 2 {
		t.Errorf("AuditCount: got %d, want 2", m.Stats.AuditCount)
	}
	if m.Stats.AxTreeCount != 2 {
		t.Errorf("AxTreeCount: got %d, want 2", m.Stats.AxTreeCount)
	}
	if len(m.Paths) != 2 {
		t.Errorf("Paths count: got %d, want 2", len(m.Paths))
	}
}

func TestList_ReverseChronological(t *testing.T) {
	store := tempStore(t)
	projectCfg := map[string]string{"name": "test"}
	results := sampleResults()

	ts1 := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	ts3 := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)

	// Create in non-chronological order
	store.Create(ts2, "bbb", projectCfg, results)
	store.Create(ts1, "aaa", projectCfg, results)
	store.Create(ts3, "ccc", projectCfg, results)

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(list))
	}
	// Newest first
	if list[0].CommitHash != "ccc" {
		t.Errorf("first should be ccc, got %s", list[0].CommitHash)
	}
	if list[1].CommitHash != "bbb" {
		t.Errorf("second should be bbb, got %s", list[1].CommitHash)
	}
	if list[2].CommitHash != "aaa" {
		t.Errorf("third should be aaa, got %s", list[2].CommitHash)
	}
}

func TestList_Empty(t *testing.T) {
	store := tempStore(t)
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestGet(t *testing.T) {
	store := tempStore(t)
	ts := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	m, err := store.Create(ts, "xyz", map[string]string{"name": "test"}, sampleResults())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(m.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != m.ID {
		t.Errorf("ID mismatch: got %q, want %q", got.ID, m.ID)
	}
	if got.CommitHash != "xyz" {
		t.Errorf("CommitHash: got %q, want %q", got.CommitHash, "xyz")
	}
}

func TestGet_NotFound(t *testing.T) {
	store := tempStore(t)
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestBaseline_AutoPromotion(t *testing.T) {
	store := tempStore(t)

	// No baseline initially
	if store.HasBaseline() {
		t.Error("expected no baseline initially")
	}

	// Promote
	if err := store.PromoteBaseline("snap-001"); err != nil {
		t.Fatalf("PromoteBaseline: %v", err)
	}

	if !store.HasBaseline() {
		t.Error("expected baseline after promotion")
	}

	bf, err := store.LoadBaseline()
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if bf.Current != "snap-001" {
		t.Errorf("baseline: got %q, want %q", bf.Current, "snap-001")
	}
}

func TestBaseline_Update(t *testing.T) {
	store := tempStore(t)

	store.PromoteBaseline("snap-001")
	store.PromoteBaseline("snap-002")

	bf, err := store.LoadBaseline()
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if bf.Current != "snap-002" {
		t.Errorf("baseline: got %q, want %q", bf.Current, "snap-002")
	}
}

func TestCreate_NoCommitHash(t *testing.T) {
	store := tempStore(t)
	ts := time.Date(2026, 4, 18, 15, 0, 0, 0, time.UTC)

	m, err := store.Create(ts, "", map[string]string{"name": "test"}, sampleResults())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if m.ID != "20260418T150000Z" {
		t.Errorf("ID without commit: got %q, want %q", m.ID, "20260418T150000Z")
	}
	if m.CommitHash != "" {
		t.Errorf("CommitHash should be empty, got %q", m.CommitHash)
	}
}
