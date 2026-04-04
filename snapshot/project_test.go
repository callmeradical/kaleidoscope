package snapshot

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLoadProjectConfigMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	// No .ks-project.json

	_, err := LoadProjectConfig()
	if err == nil {
		t.Fatal("expected error for missing .ks-project.json, got nil")
	}
	if !strings.Contains(err.Error(), ".ks-project.json") {
		t.Errorf("error message lacks actionable guidance: %v", err)
	}
}

func TestLoadProjectConfigValid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	cfg := ProjectConfig{
		Name: "my-project",
		URLs: []string{"http://localhost:3000", "http://localhost:3000/about"},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(".ks-project.json", data, 0644); err != nil {
		t.Fatalf("writing .ks-project.json: %v", err)
	}

	loaded, err := LoadProjectConfig()
	if err != nil {
		t.Fatalf("LoadProjectConfig: %v", err)
	}
	if loaded.Name != cfg.Name {
		t.Errorf("Name: got %q, want %q", loaded.Name, cfg.Name)
	}
	if len(loaded.URLs) != len(cfg.URLs) {
		t.Errorf("URLs count: got %d, want %d", len(loaded.URLs), len(cfg.URLs))
	}
}

func TestLoadProjectConfigEmptyURLs(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	cfg := ProjectConfig{Name: "no-urls"}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(".ks-project.json", data, 0644); err != nil {
		t.Fatalf("writing .ks-project.json: %v", err)
	}

	_, err := LoadProjectConfig()
	if err == nil {
		t.Fatal("expected error for empty URLs, got nil")
	}
	if !strings.Contains(err.Error(), "no URLs") {
		t.Errorf("unexpected error message: %v", err)
	}
}
