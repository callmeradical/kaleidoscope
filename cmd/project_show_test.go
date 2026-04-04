package cmd_test

import (
	"encoding/json"
	"testing"
)

// TestRunProjectShow_Success verifies that ks project-show outputs the full config.
func TestRunProjectShow_Success(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir, "myproject", "http://localhost:3000", "/,/dashboard")

	r, exitCode := runKS(t, dir, "project-show")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d (error: %s)", exitCode, r.Error)
	}
	if !r.OK {
		t.Fatalf("expected ok=true, got false (error: %s)", r.Error)
	}
	if r.Command != "project-show" {
		t.Errorf("command: got %q, want %q", r.Command, "project-show")
	}

	// Result should be the full config with name, baseURL, paths, breakpoints.
	var cfg struct {
		Name        string            `json:"name"`
		BaseURL     string            `json:"baseURL"`
		Paths       []string          `json:"paths"`
		Breakpoints []json.RawMessage `json:"breakpoints"`
	}
	if err := json.Unmarshal(r.Result, &cfg); err != nil {
		t.Fatalf("result is not a valid Config: %v (raw: %s)", err, r.Result)
	}

	if cfg.Name != "myproject" {
		t.Errorf("result.name: got %q, want %q", cfg.Name, "myproject")
	}
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("result.baseURL: got %q, want %q", cfg.BaseURL, "http://localhost:3000")
	}
	if len(cfg.Paths) != 2 {
		t.Errorf("result.paths length: got %d, want 2", len(cfg.Paths))
	}
	// Breakpoints should include the four defaults.
	if len(cfg.Breakpoints) != 4 {
		t.Errorf("result.breakpoints length: got %d, want 4", len(cfg.Breakpoints))
	}
}

// TestRunProjectShow_NoConfig verifies that ks project-show fails when no .ks-project.json exists.
func TestRunProjectShow_NoConfig(t *testing.T) {
	dir := t.TempDir()
	r, exitCode := runKS(t, dir, "project-show")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when .ks-project.json not found")
	}
	if r.OK {
		t.Fatal("expected ok=false when .ks-project.json not found")
	}
}
