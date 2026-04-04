package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// chdir temporarily changes the working directory for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestSnapshotPath(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	id := "20240101T120000Z-abc1234"
	got, err := SnapshotPath(id)
	if err != nil {
		t.Fatalf("SnapshotPath error: %v", err)
	}
	want := filepath.Join(tmp, ".kaleidoscope", "snapshots", id)
	if got != want {
		t.Errorf("SnapshotPath(%q) = %q; want %q", id, got, want)
	}
}

func TestURLDir(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	snapshotRoot := filepath.Join(tmp, "snap1")
	rawURL := "https://example.com/about"

	got, err := URLDir(snapshotRoot, rawURL)
	if err != nil {
		t.Fatalf("URLDir error: %v", err)
	}

	// Directory must exist.
	if _, err := os.Stat(got); os.IsNotExist(err) {
		t.Errorf("URLDir did not create directory %q", got)
	}

	// Path must end with sanitized key (no raw URL chars).
	key := URLToKey(rawURL)
	if !strings.HasSuffix(got, key) {
		t.Errorf("URLDir = %q; want suffix %q", got, key)
	}
}

func TestWriteReadManifest(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	snapshotRoot := filepath.Join(tmp, "snap-roundtrip")
	if err := os.MkdirAll(snapshotRoot, 0755); err != nil {
		t.Fatal(err)
	}

	ts := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	m := &Manifest{
		ID:         "20240101T120000Z-abc1234",
		Timestamp:  ts,
		CommitHash: "abc1234",
		URLs: []URLEntry{
			{
				URL:         "https://example.com/",
				Dir:         "example.com",
				TotalIssues: 3,
				AXNodeCount: 42,
				Breakpoints: 4,
				CapturedAt:  ts,
				Reachable:   true,
			},
		},
		Summary: Summary{
			TotalURLs:     1,
			ReachableURLs: 1,
			TotalIssues:   3,
			TotalAXNodes:  42,
		},
	}

	if err := WriteManifest(snapshotRoot, m); err != nil {
		t.Fatalf("WriteManifest error: %v", err)
	}

	// snapshot.json must exist.
	manifestPath := filepath.Join(snapshotRoot, "snapshot.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("WriteManifest did not create %q", manifestPath)
	}

	got, err := ReadManifest(snapshotRoot)
	if err != nil {
		t.Fatalf("ReadManifest error: %v", err)
	}
	if got == nil {
		t.Fatal("ReadManifest returned nil")
	}

	if got.ID != m.ID {
		t.Errorf("ID = %q; want %q", got.ID, m.ID)
	}
	if got.CommitHash != m.CommitHash {
		t.Errorf("CommitHash = %q; want %q", got.CommitHash, m.CommitHash)
	}
	if got.Summary.TotalIssues != m.Summary.TotalIssues {
		t.Errorf("Summary.TotalIssues = %d; want %d", got.Summary.TotalIssues, m.Summary.TotalIssues)
	}
	if len(got.URLs) != 1 {
		t.Errorf("len(URLs) = %d; want 1", len(got.URLs))
	}
}

func TestListSnapshotIDs_Order(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	// Create three snapshot directories in ascending order.
	snapDir := filepath.Join(tmp, ".kaleidoscope", "snapshots")
	ids := []string{
		"20240101T000000Z-aaa0001",
		"20240102T000000Z-bbb0002",
		"20240103T000000Z-ccc0003",
	}
	for _, id := range ids {
		if err := os.MkdirAll(filepath.Join(snapDir, id), 0755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ListSnapshotIDs()
	if err != nil {
		t.Fatalf("ListSnapshotIDs error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListSnapshotIDs returned %d IDs; want 3", len(got))
	}

	// Expect newest-first (descending).
	want := []string{
		"20240103T000000Z-ccc0003",
		"20240102T000000Z-bbb0002",
		"20240101T000000Z-aaa0001",
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("ListSnapshotIDs()[%d] = %q; want %q", i, got[i], w)
		}
	}
}

func TestBaselineRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	b := &BaselineManifest{
		BaselineID: "20240101T000000Z-abc1234",
		SetAt:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		CommitHash: "abc1234",
	}

	if err := WriteBaselineManifest(b); err != nil {
		t.Fatalf("WriteBaselineManifest error: %v", err)
	}

	got, err := ReadBaselineManifest()
	if err != nil {
		t.Fatalf("ReadBaselineManifest error: %v", err)
	}
	if got == nil {
		t.Fatal("ReadBaselineManifest returned nil after write")
	}
	if got.BaselineID != b.BaselineID {
		t.Errorf("BaselineID = %q; want %q", got.BaselineID, b.BaselineID)
	}
	if got.CommitHash != b.CommitHash {
		t.Errorf("CommitHash = %q; want %q", got.CommitHash, b.CommitHash)
	}
}

func TestReadBaselineManifest_Missing(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	got, err := ReadBaselineManifest()
	if err != nil {
		t.Fatalf("ReadBaselineManifest error when file missing: %v", err)
	}
	if got != nil {
		t.Errorf("ReadBaselineManifest returned non-nil when file absent: %+v", got)
	}
}
