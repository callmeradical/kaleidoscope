package cmd_test

import (
	"testing"
)

// TestRunInit_Success verifies that ks init creates a valid .ks-project.json.
func TestRunInit_Success(t *testing.T) {
	dir := t.TempDir()
	r, exitCode := runKS(t, dir,
		"init",
		"--name", "myproject",
		"--base-url", "http://localhost:3000",
		"--paths", "/,/dashboard",
	)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d (error: %s)", exitCode, r.Error)
	}
	if !r.OK {
		t.Fatalf("expected ok=true, got false (error: %s)", r.Error)
	}
	if r.Command != "init" {
		t.Errorf("command: got %q, want %q", r.Command, "init")
	}

	m := resultMap(t, r)
	if m["name"] != "myproject" {
		t.Errorf("result.name: got %v, want %q", m["name"], "myproject")
	}
	if m["baseURL"] != "http://localhost:3000" {
		t.Errorf("result.baseURL: got %v, want %q", m["baseURL"], "http://localhost:3000")
	}

	// Paths should include both / and /dashboard.
	paths, ok := m["paths"].([]interface{})
	if !ok {
		t.Fatalf("result.paths is not an array: %T", m["paths"])
	}
	if len(paths) != 2 {
		t.Errorf("result.paths length: got %d, want 2", len(paths))
	}

	// Breakpoints should default to the four standard presets.
	bps, ok := m["breakpoints"].([]interface{})
	if !ok {
		t.Fatalf("result.breakpoints is not an array: %T", m["breakpoints"])
	}
	if len(bps) != 4 {
		t.Errorf("result.breakpoints length: got %d, want 4 (default presets)", len(bps))
	}

	// configPath should be present.
	if m["configPath"] == "" || m["configPath"] == nil {
		t.Errorf("result.configPath is empty")
	}
}

// TestRunInit_AlreadyExists verifies that ks init fails if .ks-project.json already exists.
func TestRunInit_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	// First init succeeds.
	initProject(t, dir, "myproject", "http://localhost:3000", "/")

	// Second init must fail.
	r, exitCode := runKS(t, dir,
		"init",
		"--name", "myproject",
		"--base-url", "http://localhost:3000",
		"--paths", "/",
	)
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code when .ks-project.json already exists")
	}
	if r.OK {
		t.Fatal("expected ok=false when .ks-project.json already exists")
	}
}

// TestRunInit_MissingName verifies that ks init fails when --name is omitted.
func TestRunInit_MissingName(t *testing.T) {
	dir := t.TempDir()
	r, exitCode := runKS(t, dir,
		"init",
		"--base-url", "http://localhost:3000",
		"--paths", "/",
	)
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when --name is missing")
	}
	if r.OK {
		t.Fatal("expected ok=false when --name is missing")
	}
}

// TestRunInit_MissingBaseURL verifies that ks init fails when --base-url is omitted.
func TestRunInit_MissingBaseURL(t *testing.T) {
	dir := t.TempDir()
	r, exitCode := runKS(t, dir,
		"init",
		"--name", "myproject",
		"--paths", "/",
	)
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when --base-url is missing")
	}
	if r.OK {
		t.Fatal("expected ok=false when --base-url is missing")
	}
}

// TestRunInit_MissingPaths verifies that ks init fails when --paths is omitted.
func TestRunInit_MissingPaths(t *testing.T) {
	dir := t.TempDir()
	r, exitCode := runKS(t, dir,
		"init",
		"--name", "myproject",
		"--base-url", "http://localhost:3000",
	)
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when --paths is missing")
	}
	if r.OK {
		t.Fatal("expected ok=false when --paths is missing")
	}
}

// TestRunInit_InvalidPath verifies that ks init fails when a path does not start with /.
func TestRunInit_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	r, exitCode := runKS(t, dir,
		"init",
		"--name", "myproject",
		"--base-url", "http://localhost:3000",
		"--paths", "nodash",
	)
	if exitCode == 0 {
		t.Fatal("expected non-zero exit when path does not start with /")
	}
	if r.OK {
		t.Fatal("expected ok=false when path does not start with /")
	}
}
