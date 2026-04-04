package snapshot

import (
	"os"
	"testing"
	"time"
)

// setupStateDir creates a .kaleidoscope/ directory in tmpDir and chdirs to it,
// so browser.StateDir() resolves to the temp location.
func setupStateDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	stateDir := tmpDir + "/.kaleidoscope"
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("creating state dir: %v", err)
	}
	return stateDir
}

func TestSaveLoad(t *testing.T) {
	setupStateDir(t)

	now := time.Now().UTC().Truncate(time.Second)
	m := Manifest{
		ID:         "20240101T120000Z-abc1234",
		Timestamp:  now,
		CommitHash: "abc1234",
		ProjectConfig: ProjectConfig{
			Name: "test-project",
			URLs: []string{"http://localhost:3000", "http://localhost:3000/about"},
		},
		URLs: []URLEntry{
			{
				URL:         "http://localhost:3000",
				Dir:         "localhost-3000",
				Breakpoints: []string{"mobile-375x812.png", "tablet-768x1024.png"},
				AuditSummary: AuditSummary{
					TotalIssues:        3,
					ContrastViolations: 1,
					TouchViolations:    2,
				},
				AxNodeCount: 42,
			},
		},
	}

	if err := Save(&m); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(m.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ID != m.ID {
		t.Errorf("ID: got %q, want %q", loaded.ID, m.ID)
	}
	if !loaded.Timestamp.Equal(m.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", loaded.Timestamp, m.Timestamp)
	}
	if loaded.CommitHash != m.CommitHash {
		t.Errorf("CommitHash: got %q, want %q", loaded.CommitHash, m.CommitHash)
	}
	if loaded.ProjectConfig.Name != m.ProjectConfig.Name {
		t.Errorf("ProjectConfig.Name: got %q, want %q", loaded.ProjectConfig.Name, m.ProjectConfig.Name)
	}
	if len(loaded.URLs) != len(m.URLs) {
		t.Errorf("URLs count: got %d, want %d", len(loaded.URLs), len(m.URLs))
	}
	if len(loaded.URLs) > 0 {
		if loaded.URLs[0].AxNodeCount != m.URLs[0].AxNodeCount {
			t.Errorf("AxNodeCount: got %d, want %d", loaded.URLs[0].AxNodeCount, m.URLs[0].AxNodeCount)
		}
	}
}

func TestListSortOrder(t *testing.T) {
	setupStateDir(t)

	older := Manifest{
		ID:        "20240101T100000Z",
		Timestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		ProjectConfig: ProjectConfig{Name: "p", URLs: []string{"http://localhost"}},
	}
	newer := Manifest{
		ID:        "20240101T120000Z",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		ProjectConfig: ProjectConfig{Name: "p", URLs: []string{"http://localhost"}},
	}

	if err := Save(&older); err != nil {
		t.Fatalf("Save older: %v", err)
	}
	if err := Save(&newer); err != nil {
		t.Fatalf("Save newer: %v", err)
	}

	list, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List count: got %d, want 2", len(list))
	}
	if !list[0].Timestamp.After(list[1].Timestamp) {
		t.Errorf("List not sorted: first=%v, second=%v", list[0].Timestamp, list[1].Timestamp)
	}
	if list[0].ID != newer.ID {
		t.Errorf("First item: got %q, want %q", list[0].ID, newer.ID)
	}
}

func TestListEmptyDir(t *testing.T) {
	setupStateDir(t)
	// snapshots/ does not exist yet

	list, err := List()
	if err != nil {
		t.Fatalf("List returned error for missing dir: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List count: got %d, want 0", len(list))
	}
}
