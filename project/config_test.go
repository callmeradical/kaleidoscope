package project_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/callmeradical/kaleidoscope/project"
)

// chdir temporarily changes the working directory to dir, restoring on test cleanup.
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

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	content := `{"name":"myapp","baseURL":"http://localhost:3000","urls":["http://localhost:3000/","http://localhost:3000/about"]}`
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	cfg, err := project.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Name != "myapp" {
		t.Errorf("expected name %q, got %q", "myapp", cfg.Name)
	}
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("expected baseURL %q, got %q", "http://localhost:3000", cfg.BaseURL)
	}
	if len(cfg.URLs) != 2 {
		t.Errorf("expected 2 URLs, got %d", len(cfg.URLs))
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, err := project.Load()
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	// Error message should hint at 'ks init'
	if !strings.Contains(err.Error(), "init") {
		t.Errorf("expected error to contain 'init', got: %s", err.Error())
	}
}

func TestLoad_EmptyURLs(t *testing.T) {
	dir := t.TempDir()
	content := `{"name":"myapp","baseURL":"http://localhost:3000","urls":[]}`
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	_, err := project.Load()
	if err == nil {
		t.Fatal("expected error for empty URLs, got nil")
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	content := `{invalid json`
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	_, err := project.Load()
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
