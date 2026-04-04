package cmd_test

import (
	"testing"
)

// TestRunProjectAdd_Success verifies that ks project-add appends a path and outputs ok=true.
func TestRunProjectAdd_Success(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir, "myproject", "http://localhost:3000", "/,/dashboard")

	r, exitCode := runKS(t, dir, "project-add", "/settings")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d (error: %s)", exitCode, r.Error)
	}
	if !r.OK {
		t.Fatalf("expected ok=true, got false (error: %s)", r.Error)
	}
	if r.Command != "project-add" {
		t.Errorf("command: got %q, want %q", r.Command, "project-add")
	}

	m := resultMap(t, r)
	if m["added"] != "/settings" {
		t.Errorf("result.added: got %v, want %q", m["added"], "/settings")
	}

	paths, ok := m["paths"].([]interface{})
	if !ok {
		t.Fatalf("result.paths is not an array: %T", m["paths"])
	}
	if len(paths) != 3 {
		t.Errorf("result.paths length: got %d, want 3", len(paths))
	}

	// Verify /settings is in the list.
	found := false
	for _, p := range paths {
		if p == "/settings" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("result.paths does not contain /settings: %v", paths)
	}
}

// TestRunProjectAdd_Duplicate verifies that adding an existing path returns an error.
func TestRunProjectAdd_Duplicate(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir, "myproject", "http://localhost:3000", "/,/dashboard")

	r, exitCode := runKS(t, dir, "project-add", "/dashboard")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when adding a duplicate path")
	}
	if r.OK {
		t.Fatal("expected ok=false when adding a duplicate path")
	}
}

// TestRunProjectAdd_NoConfig verifies that the command fails when no .ks-project.json exists.
func TestRunProjectAdd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	r, exitCode := runKS(t, dir, "project-add", "/settings")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when .ks-project.json not found")
	}
	if r.OK {
		t.Fatal("expected ok=false when .ks-project.json not found")
	}
}

// TestRunProjectAdd_NoArg verifies that the command fails when no path argument is provided.
func TestRunProjectAdd_NoArg(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir, "myproject", "http://localhost:3000", "/")

	r, exitCode := runKS(t, dir, "project-add")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when no path argument is provided")
	}
	if r.OK {
		t.Fatal("expected ok=false when no path argument is provided")
	}
}
