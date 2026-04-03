package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/callmeradical/kaleidoscope/project"
)

func TestRunInit_HappyPath(t *testing.T) {
	tempDir(t)

	out, code := runCapture(func() {
		RunInit([]string{"--name", "my-app", "--base-url", "http://localhost:3000", "--paths", "/,/dashboard"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); !ok {
		t.Errorf("result.ok = false, want true; output: %s", out)
	}
	if cmd, _ := result["command"].(string); cmd != "init" {
		t.Errorf("result.command = %q, want %q", cmd, "init")
	}

	// File should exist on disk
	if !project.Exists() {
		t.Error(".ks-project.json was not created")
	}

	// Verify content via project.Read
	cfg, err := project.Read()
	if err != nil {
		t.Fatalf("project.Read: %v", err)
	}
	if cfg.Name != "my-app" {
		t.Errorf("Name = %q, want %q", cfg.Name, "my-app")
	}
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "http://localhost:3000")
	}
	if len(cfg.Paths) != 2 || cfg.Paths[0] != "/" || cfg.Paths[1] != "/dashboard" {
		t.Errorf("Paths = %v, want [/ /dashboard]", cfg.Paths)
	}
}

func TestRunInit_DefaultBreakpoints(t *testing.T) {
	tempDir(t)

	out, code := runCapture(func() {
		RunInit([]string{"--name", "bp-test", "--base-url", "http://localhost", "--paths", "/"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	payload, _ := result["result"].(map[string]any)
	bps, _ := payload["breakpoints"].([]any)
	if len(bps) != 4 {
		t.Fatalf("len(breakpoints) = %d, want 4; result: %v", len(bps), payload)
	}

	// Verify via disk as well
	cfg, err := project.Read()
	if err != nil {
		t.Fatalf("project.Read: %v", err)
	}
	expected := project.DefaultBreakpoints
	if len(cfg.Breakpoints) != len(expected) {
		t.Fatalf("len(cfg.Breakpoints) = %d, want %d", len(cfg.Breakpoints), len(expected))
	}
	for i, bp := range expected {
		if cfg.Breakpoints[i] != bp {
			t.Errorf("Breakpoints[%d] = %+v, want %+v", i, cfg.Breakpoints[i], bp)
		}
	}
}

func TestRunInit_PathSplitAndTrim(t *testing.T) {
	tempDir(t)

	out, code := runCapture(func() {
		RunInit([]string{"--name", "trim-test", "--base-url", "http://localhost", "--paths", "/ , /dashboard , /settings"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	cfg, err := project.Read()
	if err != nil {
		t.Fatalf("project.Read: %v", err)
	}
	wantPaths := []string{"/", "/dashboard", "/settings"}
	if len(cfg.Paths) != len(wantPaths) {
		t.Fatalf("Paths = %v, want %v", cfg.Paths, wantPaths)
	}
	for i, p := range wantPaths {
		if cfg.Paths[i] != p {
			t.Errorf("Paths[%d] = %q, want %q", i, cfg.Paths[i], p)
		}
	}
}

func TestRunInit_MissingName(t *testing.T) {
	tempDir(t)

	out, code := runCapture(func() {
		RunInit([]string{"--base-url", "http://localhost", "--paths", "/"})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
	if errMsg, _ := result["error"].(string); !strings.Contains(errMsg, "--name") {
		t.Errorf("error message %q does not mention --name", errMsg)
	}
}

func TestRunInit_MissingBaseURL(t *testing.T) {
	tempDir(t)

	out, code := runCapture(func() {
		RunInit([]string{"--name", "test", "--paths", "/"})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
	if errMsg, _ := result["error"].(string); !strings.Contains(errMsg, "--base-url") {
		t.Errorf("error message %q does not mention --base-url", errMsg)
	}
}

func TestRunInit_MissingPaths(t *testing.T) {
	tempDir(t)

	out, code := runCapture(func() {
		RunInit([]string{"--name", "test", "--base-url", "http://localhost"})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
	if errMsg, _ := result["error"].(string); !strings.Contains(errMsg, "--paths") {
		t.Errorf("error message %q does not mention --paths", errMsg)
	}
}

func TestRunInit_AlreadyExists(t *testing.T) {
	tempDir(t)

	// Create the file first
	cfg := &project.Config{Name: "existing", BaseURL: "http://x", Paths: []string{"/"}, Breakpoints: project.DefaultBreakpoints}
	if err := project.Write(cfg); err != nil {
		t.Fatalf("setup Write: %v", err)
	}

	out, code := runCapture(func() {
		RunInit([]string{"--name", "new", "--base-url", "http://localhost", "--paths", "/"})
	})

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; output: %s", out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("result.ok = true, want false")
	}
	if errMsg, _ := result["error"].(string); !strings.Contains(errMsg, "already exists") {
		t.Errorf("error message %q does not contain 'already exists'", errMsg)
	}
}

func TestRunInit_OutputJSON(t *testing.T) {
	tempDir(t)

	out, code := runCapture(func() {
		RunInit([]string{"--name", "json-test", "--base-url", "http://localhost", "--paths", "/"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output: %s", code, out)
	}

	// Must be valid JSON
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}

	result := parseResult(t, strings.TrimSpace(out))
	payload, _ := result["result"].(map[string]any)
	if payload == nil {
		t.Fatalf("result.result is nil; output: %s", out)
	}
	if _, ok := payload["path"]; !ok {
		t.Errorf("result.result missing 'path' field; output: %s", out)
	}
	if _, ok := payload["breakpoints"]; !ok {
		t.Errorf("result.result missing 'breakpoints' field; output: %s", out)
	}
}

func TestRunInit_NoProjectFileOnError(t *testing.T) {
	tempDir(t)

	// Call init with missing --name so it errors before writing
	runCapture(func() {
		RunInit([]string{"--base-url", "http://localhost", "--paths", "/"})
	})

	// File should not have been created
	if _, err := os.Stat(project.Filename); err == nil {
		t.Error(".ks-project.json was created despite error, want it absent")
	}
}
