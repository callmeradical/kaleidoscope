package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/callmeradical/kaleidoscope/output"
)

// setupTempDir creates a temp dir, chdirs into it, and returns a cleanup func
// that restores the original working directory.
func setupTempDir(t *testing.T) func() {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	return func() {
		if err := os.Chdir(orig); err != nil {
			t.Logf("failed to restore working directory: %v", err)
		}
	}
}

// captureStdout redirects os.Stdout to a pipe, calls fn, and returns what was written.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	orig := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// writeProjectFile writes a minimal valid project config to disk for test setup.
func writeProjectFile(t *testing.T, cfg *ProjectConfig) {
	t.Helper()
	if err := writeProjectConfig(cfg); err != nil {
		t.Fatalf("writeProjectConfig: %v", err)
	}
}

// minimalConfig returns a basic ProjectConfig for test setup.
func minimalConfig(paths []string) *ProjectConfig {
	return &ProjectConfig{
		Name:        "test-app",
		BaseURL:     "https://example.com",
		Paths:       paths,
		Breakpoints: defaultProjectBreakpoints,
	}
}

// ---- RunInit tests ----

func TestRunInit_Success(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	captureStdout(func() {
		RunInit([]string{"--name", "my-app", "--base-url", "https://example.com", "--paths", "/,/dashboard"})
	})

	data, err := os.ReadFile(projectConfigFile)
	if err != nil {
		t.Fatalf("expected .ks-project.json to be created: %v", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.Name != "my-app" {
		t.Errorf("name: got %q, want %q", cfg.Name, "my-app")
	}
	if cfg.BaseURL != "https://example.com" {
		t.Errorf("baseURL: got %q, want %q", cfg.BaseURL, "https://example.com")
	}
	if len(cfg.Paths) != 2 || cfg.Paths[0] != "/" || cfg.Paths[1] != "/dashboard" {
		t.Errorf("paths: got %v, want [/ /dashboard]", cfg.Paths)
	}
}

func TestRunInit_DefaultBreakpoints(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	captureStdout(func() {
		RunInit([]string{"--name", "my-app", "--base-url", "https://example.com", "--paths", "/"})
	})

	data, err := os.ReadFile(projectConfigFile)
	if err != nil {
		t.Fatalf("expected .ks-project.json: %v", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	expected := []struct {
		Name   string
		Width  int
		Height int
	}{
		{"mobile", 375, 812},
		{"tablet", 768, 1024},
		{"desktop", 1280, 720},
		{"wide", 1920, 1080},
	}

	if len(cfg.Breakpoints) != len(expected) {
		t.Fatalf("breakpoints count: got %d, want %d", len(cfg.Breakpoints), len(expected))
	}
	for i, want := range expected {
		got := cfg.Breakpoints[i]
		if got.Name != want.Name || got.Width != want.Width || got.Height != want.Height {
			t.Errorf("breakpoint[%d]: got {%s,%d,%d}, want {%s,%d,%d}",
				i, got.Name, got.Width, got.Height, want.Name, want.Width, want.Height)
		}
	}
}

func TestRunInit_PathsTrimmed(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	captureStdout(func() {
		RunInit([]string{"--name", "my-app", "--base-url", "https://example.com", "--paths", "/ , /about , /contact"})
	})

	data, err := os.ReadFile(projectConfigFile)
	if err != nil {
		t.Fatalf("expected .ks-project.json: %v", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(cfg.Paths) != 3 || cfg.Paths[0] != "/" || cfg.Paths[1] != "/about" || cfg.Paths[2] != "/contact" {
		t.Errorf("paths not trimmed correctly: %v", cfg.Paths)
	}
}

func TestRunInit_OutputJSON(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	out := captureStdout(func() {
		RunInit([]string{"--name", "my-app", "--base-url", "https://example.com", "--paths", "/"})
	})

	var result output.Result
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("unmarshal output JSON: %v\noutput was: %s", err, out)
	}
	if !result.OK {
		t.Errorf("expected ok=true, got %v", result.OK)
	}
	if result.Command != "init" {
		t.Errorf("command: got %q, want %q", result.Command, "init")
	}
}

// TestRunInit_ErrorIfExists verifies that RunInit exits with code 2 when .ks-project.json already exists.
func TestRunInit_ErrorIfExists(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		// Running as subprocess: set up a fresh temp dir
		dir := os.TempDir()
		tmp, _ := os.MkdirTemp(dir, "ks-test-*")
		defer os.RemoveAll(tmp)
		os.Chdir(tmp)
		// Create file first
		os.WriteFile(filepath.Join(tmp, projectConfigFile), []byte("{}"), 0644)
		RunInit([]string{"--name", "x", "--base-url", "http://x.com", "--paths", "/"})
		os.Exit(0) // should not reach here
	}

	cmd := exec.Command(os.Args[0], fmt.Sprintf("-test.run=^%s$", t.Name()))
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	out, err := cmd.Output()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected non-zero exit, got nil error; output: %s", out)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code: got %d, want 2", exitErr.ExitCode())
	}

	var result output.Result
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if result.OK {
		t.Error("expected ok=false")
	}
	if !strings.Contains(result.Error, "already exists") {
		t.Errorf("expected 'already exists' in error, got: %s", result.Error)
	}
}

// ---- RunProjectAdd tests ----

func TestRunProjectAdd_Success(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	writeProjectFile(t, minimalConfig([]string{"/"}))

	captureStdout(func() {
		RunProjectAdd([]string{"/settings"})
	})

	cfg, err := readProjectConfig()
	if err != nil {
		t.Fatalf("readProjectConfig: %v", err)
	}

	if len(cfg.Paths) != 2 || cfg.Paths[0] != "/" || cfg.Paths[1] != "/settings" {
		t.Errorf("paths: got %v, want [/ /settings]", cfg.Paths)
	}
}

func TestRunProjectAdd_OutputJSON(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	writeProjectFile(t, minimalConfig([]string{"/"}))

	out := captureStdout(func() {
		RunProjectAdd([]string{"/settings"})
	})

	var result output.Result
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if !result.OK {
		t.Error("expected ok=true")
	}
	if result.Command != "project-add" {
		t.Errorf("command: got %q, want %q", result.Command, "project-add")
	}
}

// TestRunProjectAdd_DuplicateError verifies exit code 2 when path already exists.
func TestRunProjectAdd_DuplicateError(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		dir := os.TempDir()
		tmp, _ := os.MkdirTemp(dir, "ks-test-*")
		defer os.RemoveAll(tmp)
		os.Chdir(tmp)
		cfg := minimalConfig([]string{"/", "/settings"})
		writeProjectConfig(cfg)
		RunProjectAdd([]string{"/settings"})
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], fmt.Sprintf("-test.run=^%s$", t.Name()))
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	out, err := cmd.Output()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected non-zero exit; output: %s", out)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code: got %d, want 2", exitErr.ExitCode())
	}

	var result output.Result
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if result.OK {
		t.Error("expected ok=false")
	}
	if !strings.Contains(result.Error, "already exists") {
		t.Errorf("expected 'already exists' in error, got: %s", result.Error)
	}
}

// ---- RunProjectRemove tests ----

func TestRunProjectRemove_Success(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	writeProjectFile(t, minimalConfig([]string{"/", "/settings"}))

	captureStdout(func() {
		RunProjectRemove([]string{"/settings"})
	})

	cfg, err := readProjectConfig()
	if err != nil {
		t.Fatalf("readProjectConfig: %v", err)
	}

	if len(cfg.Paths) != 1 || cfg.Paths[0] != "/" {
		t.Errorf("paths: got %v, want [/]", cfg.Paths)
	}
}

func TestRunProjectRemove_OutputJSON(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	writeProjectFile(t, minimalConfig([]string{"/", "/settings"}))

	out := captureStdout(func() {
		RunProjectRemove([]string{"/settings"})
	})

	var result output.Result
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if !result.OK {
		t.Error("expected ok=true")
	}
	if result.Command != "project-remove" {
		t.Errorf("command: got %q, want %q", result.Command, "project-remove")
	}
}

func TestRunProjectRemove_PreservesOrder(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	writeProjectFile(t, minimalConfig([]string{"/", "/about", "/settings", "/contact"}))

	captureStdout(func() {
		RunProjectRemove([]string{"/settings"})
	})

	cfg, err := readProjectConfig()
	if err != nil {
		t.Fatalf("readProjectConfig: %v", err)
	}

	want := []string{"/", "/about", "/contact"}
	if len(cfg.Paths) != len(want) {
		t.Fatalf("paths length: got %d, want %d; paths: %v", len(cfg.Paths), len(want), cfg.Paths)
	}
	for i, w := range want {
		if cfg.Paths[i] != w {
			t.Errorf("paths[%d]: got %q, want %q", i, cfg.Paths[i], w)
		}
	}
}

// TestRunProjectRemove_NotFoundError verifies exit code 2 when path doesn't exist.
func TestRunProjectRemove_NotFoundError(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		dir := os.TempDir()
		tmp, _ := os.MkdirTemp(dir, "ks-test-*")
		defer os.RemoveAll(tmp)
		os.Chdir(tmp)
		cfg := minimalConfig([]string{"/"})
		writeProjectConfig(cfg)
		RunProjectRemove([]string{"/settings"})
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], fmt.Sprintf("-test.run=^%s$", t.Name()))
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	out, err := cmd.Output()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected non-zero exit; output: %s", out)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code: got %d, want 2", exitErr.ExitCode())
	}

	var result output.Result
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if result.OK {
		t.Error("expected ok=false")
	}
	if !strings.Contains(result.Error, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", result.Error)
	}
}

// ---- RunProjectShow tests ----

func TestRunProjectShow_Success(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	writeProjectFile(t, minimalConfig([]string{"/", "/dashboard"}))

	out := captureStdout(func() {
		RunProjectShow(nil)
	})

	var result output.Result
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if !result.OK {
		t.Errorf("expected ok=true, got ok=%v, error=%q", result.OK, result.Error)
	}
	if result.Command != "project-show" {
		t.Errorf("command: got %q, want %q", result.Command, "project-show")
	}

	// Decode result payload into ProjectConfig
	payload, err := json.Marshal(result.Result)
	if err != nil {
		t.Fatalf("re-marshal result: %v", err)
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("unmarshal cfg from result: %v", err)
	}
	if cfg.Name != "test-app" {
		t.Errorf("name: got %q, want %q", cfg.Name, "test-app")
	}
	if cfg.BaseURL != "https://example.com" {
		t.Errorf("baseURL: got %q, want %q", cfg.BaseURL, "https://example.com")
	}
	if len(cfg.Paths) != 2 || cfg.Paths[0] != "/" || cfg.Paths[1] != "/dashboard" {
		t.Errorf("paths: got %v, want [/ /dashboard]", cfg.Paths)
	}
}

// TestRunProjectShow_MissingFileError verifies exit code 2 when no config file exists.
func TestRunProjectShow_MissingFileError(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		dir := os.TempDir()
		tmp, _ := os.MkdirTemp(dir, "ks-test-*")
		defer os.RemoveAll(tmp)
		os.Chdir(tmp)
		RunProjectShow(nil)
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], fmt.Sprintf("-test.run=^%s$", t.Name()))
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	out, err := cmd.Output()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected non-zero exit; output: %s", out)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code: got %d, want 2", exitErr.ExitCode())
	}

	var result output.Result
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out)
	}
	if result.OK {
		t.Error("expected ok=false")
	}
	if !strings.Contains(result.Hint, "ks init") {
		t.Errorf("expected 'ks init' in hint, got: %q", result.Hint)
	}
}
