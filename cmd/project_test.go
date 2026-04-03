package cmd

import (
	"strings"
	"testing"

	"github.com/callmeradical/kaleidoscope/project"
)

// seedProject writes a minimal .ks-project.json with the given paths.
func seedProject(t *testing.T, paths []string) {
	t.Helper()
	cfg := &project.Config{
		Name:        "test-app",
		BaseURL:     "http://localhost:3000",
		Paths:       paths,
		Breakpoints: project.DefaultBreakpoints,
	}
	if err := project.Write(cfg); err != nil {
		t.Fatalf("seedProject Write: %v", err)
	}
}

// ---- RunProjectAdd ----

func TestRunProjectAdd_HappyPath(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/"})

	out, code := runCapture(func() {
		RunProjectAdd([]string{"/settings"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); !ok {
		t.Errorf("result.ok = false, want true; output: %s", out)
	}
	if cmd, _ := result["command"].(string); cmd != "project-add" {
		t.Errorf("result.command = %q, want %q", cmd, "project-add")
	}

	// Verify config was updated on disk
	cfg, err := project.Read()
	if err != nil {
		t.Fatalf("project.Read: %v", err)
	}
	found := false
	for _, p := range cfg.Paths {
		if p == "/settings" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Paths = %v, want /settings to be present", cfg.Paths)
	}
}

func TestRunProjectAdd_OutputContainsPaths(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/"})

	out, code := runCapture(func() {
		RunProjectAdd([]string{"/about"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	payload, _ := result["result"].(map[string]any)
	if payload == nil {
		t.Fatalf("result.result is nil; output: %s", out)
	}
	paths, _ := payload["paths"].([]any)
	if len(paths) != 2 {
		t.Errorf("len(result.result.paths) = %d, want 2; result: %v", len(paths), payload)
	}
}

func TestRunProjectAdd_DuplicatePath(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/", "/settings"})

	out, code := runCapture(func() {
		RunProjectAdd([]string{"/settings"})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
	if errMsg, _ := result["error"].(string); !strings.Contains(errMsg, "already exists") {
		t.Errorf("error %q does not contain 'already exists'", errMsg)
	}
}

func TestRunProjectAdd_MissingArgument(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/"})

	out, code := runCapture(func() {
		RunProjectAdd([]string{})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
}

func TestRunProjectAdd_MissingProjectFile(t *testing.T) {
	tempDir(t)
	// No .ks-project.json created

	out, code := runCapture(func() {
		RunProjectAdd([]string{"/settings"})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
	if hint, _ := result["hint"].(string); !strings.Contains(hint, "ks init") {
		t.Errorf("hint %q does not mention 'ks init'", hint)
	}
}

// ---- RunProjectRemove ----

func TestRunProjectRemove_HappyPath(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/", "/settings", "/dashboard"})

	out, code := runCapture(func() {
		RunProjectRemove([]string{"/settings"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); !ok {
		t.Errorf("result.ok = false, want true; output: %s", out)
	}
	if cmd, _ := result["command"].(string); cmd != "project-remove" {
		t.Errorf("result.command = %q, want %q", cmd, "project-remove")
	}

	// Verify config was updated on disk
	cfg, err := project.Read()
	if err != nil {
		t.Fatalf("project.Read: %v", err)
	}
	for _, p := range cfg.Paths {
		if p == "/settings" {
			t.Errorf("/settings still present in Paths = %v", cfg.Paths)
		}
	}
	if len(cfg.Paths) != 2 {
		t.Errorf("len(Paths) = %d, want 2; Paths = %v", len(cfg.Paths), cfg.Paths)
	}
}

func TestRunProjectRemove_OutputContainsRemoved(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/", "/settings"})

	out, code := runCapture(func() {
		RunProjectRemove([]string{"/settings"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	payload, _ := result["result"].(map[string]any)
	if payload == nil {
		t.Fatalf("result.result is nil; output: %s", out)
	}
	if removed, _ := payload["removed"].(string); removed != "/settings" {
		t.Errorf("result.result.removed = %q, want %q", removed, "/settings")
	}
	paths, _ := payload["paths"].([]any)
	if len(paths) != 1 {
		t.Errorf("len(result.result.paths) = %d, want 1", len(paths))
	}
}

func TestRunProjectRemove_NonExistentPath(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/", "/dashboard"})

	out, code := runCapture(func() {
		RunProjectRemove([]string{"/nonexistent"})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
	if errMsg, _ := result["error"].(string); !strings.Contains(errMsg, "path not found") {
		t.Errorf("error %q does not contain 'path not found'", errMsg)
	}
}

func TestRunProjectRemove_MissingArgument(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/"})

	out, code := runCapture(func() {
		RunProjectRemove([]string{})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
}

func TestRunProjectRemove_MissingProjectFile(t *testing.T) {
	tempDir(t)
	// No .ks-project.json

	out, code := runCapture(func() {
		RunProjectRemove([]string{"/settings"})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
}

// ---- RunProjectShow ----

func TestRunProjectShow_HappyPath(t *testing.T) {
	tempDir(t)
	seedProject(t, []string{"/", "/dashboard"})

	out, code := runCapture(func() {
		RunProjectShow([]string{})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); !ok {
		t.Errorf("result.ok = false, want true; output: %s", out)
	}
	if cmd, _ := result["command"].(string); cmd != "project-show" {
		t.Errorf("result.command = %q, want %q", cmd, "project-show")
	}

	payload, _ := result["result"].(map[string]any)
	if payload == nil {
		t.Fatalf("result.result is nil; output: %s", out)
	}
	if name, _ := payload["name"].(string); name != "test-app" {
		t.Errorf("result.result.name = %q, want %q", name, "test-app")
	}
	if baseURL, _ := payload["baseURL"].(string); baseURL != "http://localhost:3000" {
		t.Errorf("result.result.baseURL = %q, want %q", baseURL, "http://localhost:3000")
	}
	paths, _ := payload["paths"].([]any)
	if len(paths) != 2 {
		t.Errorf("len(result.result.paths) = %d, want 2", len(paths))
	}
	bps, _ := payload["breakpoints"].([]any)
	if len(bps) != 4 {
		t.Errorf("len(result.result.breakpoints) = %d, want 4", len(bps))
	}
}

func TestRunProjectShow_MissingProjectFile(t *testing.T) {
	tempDir(t)
	// No .ks-project.json

	out, code := runCapture(func() {
		RunProjectShow([]string{})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
	if hint, _ := result["hint"].(string); !strings.Contains(hint, "ks init") {
		t.Errorf("hint %q does not mention 'ks init'", hint)
	}
}

func TestRunProjectShow_FullConfigInResult(t *testing.T) {
	tempDir(t)
	// Seed with all fields
	cfg := &project.Config{
		Name:        "full-app",
		BaseURL:     "http://example.com",
		Paths:       []string{"/", "/a", "/b"},
		Breakpoints: project.DefaultBreakpoints,
	}
	if err := project.Write(cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	out, code := runCapture(func() {
		RunProjectShow([]string{})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	payload, _ := result["result"].(map[string]any)
	if payload == nil {
		t.Fatalf("result.result is nil")
	}
	// All four Config fields must be present
	for _, key := range []string{"name", "baseURL", "paths", "breakpoints"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("result.result missing field %q", key)
		}
	}
	paths, _ := payload["paths"].([]any)
	if len(paths) != 3 {
		t.Errorf("len(paths) = %d, want 3", len(paths))
	}
}
