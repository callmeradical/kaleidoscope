package project_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/callmeradical/kaleidoscope/project"
)

func writeProjectConfig(t *testing.T, dir string, cfg project.ProjectConfig) string {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(dir, ".ks-project.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestLoadConfig_Valid asserts that a valid JSON file is fully parsed.
func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	want := project.ProjectConfig{
		Name:        "my-project",
		URLs:        []string{"http://localhost:3000", "http://localhost:3000/about"},
		Breakpoints: []string{"mobile", "tablet", "desktop", "wide"},
	}
	path := writeProjectConfig(t, dir, want)

	got, err := project.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("LoadConfig returned nil config")
	}
	if got.Name != want.Name {
		t.Errorf("Name: got %q, want %q", got.Name, want.Name)
	}
	if len(got.URLs) != len(want.URLs) {
		t.Errorf("URLs length: got %d, want %d", len(got.URLs), len(want.URLs))
	}
	for i, u := range want.URLs {
		if i < len(got.URLs) && got.URLs[i] != u {
			t.Errorf("URLs[%d]: got %q, want %q", i, got.URLs[i], u)
		}
	}
	if len(got.Breakpoints) != len(want.Breakpoints) {
		t.Errorf("Breakpoints length: got %d, want %d", len(got.Breakpoints), len(want.Breakpoints))
	}
}

// TestLoadConfig_MissingFile asserts that a missing file returns an error.
func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := project.LoadConfig("/nonexistent/path/.ks-project.json")
	if err == nil {
		t.Fatal("LoadConfig with missing file should return an error, got nil")
	}
}

// TestFindConfig_InCWD asserts that FindConfig finds the config when present in CWD.
func TestFindConfig_InCWD(t *testing.T) {
	dir := t.TempDir()
	cfg := project.ProjectConfig{
		Name: "test-project",
		URLs: []string{"http://localhost:8080"},
	}
	writeProjectConfig(t, dir, cfg)
	t.Chdir(dir)

	got, err := project.FindConfig()
	if err != nil {
		t.Fatalf("FindConfig returned unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("FindConfig returned nil, expected non-nil config")
	}
	if got.Name != cfg.Name {
		t.Errorf("Name: got %q, want %q", got.Name, cfg.Name)
	}
}

// TestFindConfig_NotFound asserts that FindConfig returns (nil, nil) when no config exists.
func TestFindConfig_NotFound(t *testing.T) {
	// Use a temp dir with no .ks-project.json and no .git directory
	dir := t.TempDir()
	t.Chdir(dir)

	got, err := project.FindConfig()
	if err != nil {
		t.Fatalf("FindConfig returned unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("FindConfig should return nil when no config found, got %+v", got)
	}
}
