package project_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/callmeradical/kaleidoscope/project"
)

// chdir changes the working directory for the duration of the test,
// restoring it on cleanup.
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

// TestLoad_NotFound verifies Load returns ErrNotFound when .ks-project.json is absent.
func TestLoad_NotFound(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, err := project.Load()
	if !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

// TestLoad_InvalidVersion verifies Load returns an error for version != 1.
func TestLoad_InvalidVersion(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := map[string]any{
		"version": 2,
		"urls":    []string{"http://localhost:3000"},
	}
	writeJSON(t, filepath.Join(dir, ".ks-project.json"), cfg)

	_, err := project.Load()
	if err == nil {
		t.Fatal("expected error for version 2, got nil")
	}
}

// TestLoad_EmptyURLs verifies Load returns an error when urls is empty.
func TestLoad_EmptyURLs(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := map[string]any{
		"version": 1,
		"urls":    []string{},
	}
	writeJSON(t, filepath.Join(dir, ".ks-project.json"), cfg)

	_, err := project.Load()
	if err == nil {
		t.Fatal("expected error for empty urls, got nil")
	}
}

// TestLoad_MalformedURL verifies Load returns an error for an unparseable URL.
// Note: url.Parse is quite permissive; we test with a structurally invalid value.
func TestLoad_MalformedURL(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// url.Parse returns an error on URLs with invalid characters like control chars.
	cfg := map[string]any{
		"version": 1,
		"urls":    []string{"http://localhost:3000", "://\x00bad"},
	}
	writeJSON(t, filepath.Join(dir, ".ks-project.json"), cfg)

	_, err := project.Load()
	if err == nil {
		t.Fatal("expected error for malformed URL, got nil")
	}
}

// TestLoad_HappyPath verifies Load returns a valid Config for a well-formed file.
func TestLoad_HappyPath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := map[string]any{
		"version": 1,
		"urls":    []string{"http://localhost:3000", "http://localhost:3000/about"},
	}
	writeJSON(t, filepath.Join(dir, ".ks-project.json"), cfg)

	got, err := project.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("version: got %d, want 1", got.Version)
	}
	if len(got.URLs) != 2 {
		t.Errorf("urls count: got %d, want 2", len(got.URLs))
	}
}

// TestSaveLoad_RoundTrip verifies that Save followed by Load returns identical data.
func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	orig := &project.Config{
		Version: 1,
		URLs:    []string{"http://localhost:3000", "http://localhost:3000/contact"},
	}
	if err := project.Save(orig); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := project.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != orig.Version {
		t.Errorf("version: got %d, want %d", got.Version, orig.Version)
	}
	if len(got.URLs) != len(orig.URLs) {
		t.Fatalf("urls count: got %d, want %d", len(got.URLs), len(orig.URLs))
	}
	for i, u := range orig.URLs {
		if got.URLs[i] != u {
			t.Errorf("urls[%d]: got %q, want %q", i, got.URLs[i], u)
		}
	}
}

// writeJSON marshals v and writes it to path.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
