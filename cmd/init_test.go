package cmd_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/callmeradical/kaleidoscope/cmd"
	"github.com/callmeradical/kaleidoscope/project"
)

// TestRunInit_Helper is the subprocess entry point. It must not be called directly.
func TestRunInit_Helper(t *testing.T) {
	if os.Getenv("KS_TEST_HELPER") != t.Name() {
		t.Skip("subprocess helper – not for direct invocation")
	}
	dir := os.Getenv("KS_TEST_WORKDIR")
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var args []string
	json.Unmarshal([]byte(os.Getenv("KS_TEST_ARGS")), &args)
	cmd.RunInit(args)
}

// runInit invokes RunInit in a subprocess and returns the exit code and stdout.
func runInit(t *testing.T, dir string, args []string) (int, []byte) {
	t.Helper()
	rawArgs, _ := json.Marshal(args)
	c := exec.Command(os.Args[0], "-test.run=^TestRunInit_Helper$", "-test.v")
	c.Env = append(os.Environ(),
		"KS_TEST_HELPER=TestRunInit_Helper",
		"KS_TEST_WORKDIR="+dir,
		"KS_TEST_ARGS="+string(rawArgs),
	)
	out, _ := c.Output()
	code := 0
	if c.ProcessState != nil && !c.ProcessState.Success() {
		if ec := c.ProcessState.ExitCode(); ec != -1 {
			code = ec
		} else {
			code = 1
		}
	}
	return code, out
}

// TestRunInit_CreatesValidConfig verifies RunInit creates .ks-project.json with correct fields.
func TestRunInit_CreatesValidConfig(t *testing.T) {
	dir := t.TempDir()
	args := []string{"--name", "myapp", "--base-url", "http://localhost:3000"}

	code, _ := runInit(t, dir, args)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	data, err := os.ReadFile(filepath.Join(dir, project.ConfigFile))
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	var cfg project.ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON in config: %v", err)
	}
	if cfg.Name != "myapp" {
		t.Errorf("Name: got %q, want %q", cfg.Name, "myapp")
	}
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("BaseURL: got %q, want %q", cfg.BaseURL, "http://localhost:3000")
	}
	if len(cfg.Paths) != 1 || cfg.Paths[0] != "/" {
		t.Errorf("Paths: got %v, want [/]", cfg.Paths)
	}
	if len(cfg.Breakpoints) != 4 {
		t.Errorf("Breakpoints: got %d, want 4", len(cfg.Breakpoints))
	}
}

// TestRunInit_FailsIfConfigExists verifies RunInit exits 2 and does not overwrite when config exists.
func TestRunInit_FailsIfConfigExists(t *testing.T) {
	dir := t.TempDir()

	existing := &project.ProjectConfig{
		Name: "original", BaseURL: "http://orig.com",
		Paths: []string{"/"}, Breakpoints: project.DefaultBreakpoints(),
	}
	raw, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(dir, project.ConfigFile), raw, 0644)

	args := []string{"--name", "new", "--base-url", "http://new.com"}
	code, _ := runInit(t, dir, args)
	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}

	// File must not be overwritten.
	raw2, _ := os.ReadFile(filepath.Join(dir, project.ConfigFile))
	var cfg project.ProjectConfig
	json.Unmarshal(raw2, &cfg)
	if cfg.Name != "original" {
		t.Errorf("config was overwritten; Name: got %q, want %q", cfg.Name, "original")
	}
}

// TestRunInit_WithPaths verifies --paths splits on comma into a Paths slice.
func TestRunInit_WithPaths(t *testing.T) {
	dir := t.TempDir()
	args := []string{"--name", "p", "--base-url", "http://localhost", "--paths", "/,/dashboard"}

	code, _ := runInit(t, dir, args)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, project.ConfigFile))
	var cfg project.ProjectConfig
	json.Unmarshal(data, &cfg)

	if len(cfg.Paths) != 2 {
		t.Fatalf("Paths len: got %d, want 2", len(cfg.Paths))
	}
	if cfg.Paths[0] != "/" || cfg.Paths[1] != "/dashboard" {
		t.Errorf("Paths: got %v, want [/ /dashboard]", cfg.Paths)
	}
}
