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

// TestRunProjectAdd_Helper is the subprocess entry point.
func TestRunProjectAdd_Helper(t *testing.T) {
	if os.Getenv("KS_TEST_HELPER") != t.Name() {
		t.Skip("subprocess helper – not for direct invocation")
	}
	dir := os.Getenv("KS_TEST_WORKDIR")
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var args []string
	json.Unmarshal([]byte(os.Getenv("KS_TEST_ARGS")), &args)
	cmd.RunProjectAdd(args)
}

func runProjectAdd(t *testing.T, dir string, args []string) (int, []byte) {
	t.Helper()
	rawArgs, _ := json.Marshal(args)
	c := exec.Command(os.Args[0], "-test.run=^TestRunProjectAdd_Helper$", "-test.v")
	c.Env = append(os.Environ(),
		"KS_TEST_HELPER=TestRunProjectAdd_Helper",
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

// seedConfig writes a basic project config to dir.
func seedConfig(t *testing.T, dir string, paths []string) {
	t.Helper()
	cfg := &project.ProjectConfig{
		Name: "test", BaseURL: "http://localhost",
		Paths: paths, Breakpoints: project.DefaultBreakpoints(),
	}
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, project.ConfigFile), raw, 0644); err != nil {
		t.Fatal(err)
	}
}

// TestRunProjectAdd_AppendsPath verifies that project-add appends path and persists config.
func TestRunProjectAdd_AppendsPath(t *testing.T) {
	dir := t.TempDir()
	seedConfig(t, dir, []string{"/"})

	code, _ := runProjectAdd(t, dir, []string{"/settings"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, project.ConfigFile))
	var cfg project.ProjectConfig
	json.Unmarshal(data, &cfg)

	if len(cfg.Paths) != 2 {
		t.Fatalf("Paths len: got %d, want 2", len(cfg.Paths))
	}
	if cfg.Paths[1] != "/settings" {
		t.Errorf("Paths[1]: got %q, want /settings", cfg.Paths[1])
	}
}

// TestRunProjectAdd_DuplicatePathFails verifies that adding an existing path exits 2.
func TestRunProjectAdd_DuplicatePathFails(t *testing.T) {
	dir := t.TempDir()
	seedConfig(t, dir, []string{"/", "/settings"})

	code, _ := runProjectAdd(t, dir, []string{"/settings"})
	if code != 2 {
		t.Errorf("expected exit code 2 for duplicate path, got %d", code)
	}
}

// TestRunProjectAdd_NoConfigFails verifies that project-add exits 2 when config is missing.
func TestRunProjectAdd_NoConfigFails(t *testing.T) {
	dir := t.TempDir()

	code, _ := runProjectAdd(t, dir, []string{"/settings"})
	if code != 2 {
		t.Errorf("expected exit code 2 when config missing, got %d", code)
	}
}
