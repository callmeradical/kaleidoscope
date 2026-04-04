package cmd_test

import (
	"testing"
)

// TestRunProjectRemove_Success verifies that ks project-remove removes a path and outputs ok=true.
func TestRunProjectRemove_Success(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir, "myproject", "http://localhost:3000", "/,/dashboard,/settings")

	r, exitCode := runKS(t, dir, "project-remove", "/settings")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d (error: %s)", exitCode, r.Error)
	}
	if !r.OK {
		t.Fatalf("expected ok=true, got false (error: %s)", r.Error)
	}
	if r.Command != "project-remove" {
		t.Errorf("command: got %q, want %q", r.Command, "project-remove")
	}

	m := resultMap(t, r)
	if m["removed"] != "/settings" {
		t.Errorf("result.removed: got %v, want %q", m["removed"], "/settings")
	}

	paths, ok := m["paths"].([]interface{})
	if !ok {
		t.Fatalf("result.paths is not an array: %T", m["paths"])
	}
	if len(paths) != 2 {
		t.Errorf("result.paths length: got %d, want 2 (after removal)", len(paths))
	}

	// Verify /settings is no longer in the list.
	for _, p := range paths {
		if p == "/settings" {
			t.Errorf("result.paths still contains /settings after removal: %v", paths)
		}
	}
}

// TestRunProjectRemove_NotFound verifies that removing a non-existent path returns an error.
func TestRunProjectRemove_NotFound(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir, "myproject", "http://localhost:3000", "/,/dashboard")

	r, exitCode := runKS(t, dir, "project-remove", "/nonexistent")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when path is not in the config")
	}
	if r.OK {
		t.Fatal("expected ok=false when path is not in the config")
	}
}

// TestRunProjectRemove_NoConfig verifies that the command fails when no .ks-project.json exists.
func TestRunProjectRemove_NoConfig(t *testing.T) {
	dir := t.TempDir()
	r, exitCode := runKS(t, dir, "project-remove", "/settings")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when .ks-project.json not found")
	}
	if r.OK {
		t.Fatal("expected ok=false when .ks-project.json not found")
	}
}

// TestRunProjectRemove_NoArg verifies that the command fails when no path argument is provided.
func TestRunProjectRemove_NoArg(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir, "myproject", "http://localhost:3000", "/")

	r, exitCode := runKS(t, dir, "project-remove")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when no path argument is provided")
	}
	if r.OK {
		t.Fatal("expected ok=false when no path argument is provided")
	}
}
