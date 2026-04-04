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

// TestRunProjectRemove_Helper is the subprocess entry point.
func TestRunProjectRemove_Helper(t *testing.T) {
	if os.Getenv("KS_TEST_HELPER") != t.Name() {
		t.Skip("subprocess helper – not for direct invocation")
	}
	dir := os.Getenv("KS_TEST_WORKDIR")
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var args []string
	json.Unmarshal([]byte(os.Getenv("KS_TEST_ARGS")), &args)
	cmd.RunProjectRemove(args)
}

func runProjectRemove(t *testing.T, dir string, args []string) (int, []byte) {
	t.Helper()
	rawArgs, _ := json.Marshal(args)
	c := exec.Command(os.Args[0], "-test.run=^TestRunProjectRemove_Helper$", "-test.v")
	c.Env = append(os.Environ(),
		"KS_TEST_HELPER=TestRunProjectRemove_Helper",
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

// TestRunProjectRemove_RemovesPath verifies that project-remove removes the path and persists config.
func TestRunProjectRemove_RemovesPath(t *testing.T) {
	dir := t.TempDir()
	seedConfig(t, dir, []string{"/", "/settings"})

	code, _ := runProjectRemove(t, dir, []string{"/settings"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, project.ConfigFile))
	var cfg project.ProjectConfig
	json.Unmarshal(data, &cfg)

	if len(cfg.Paths) != 1 {
		t.Fatalf("Paths len: got %d, want 1", len(cfg.Paths))
	}
	if cfg.Paths[0] != "/" {
		t.Errorf("Paths[0]: got %q, want /", cfg.Paths[0])
	}
}

// TestRunProjectRemove_NonExistentPathFails verifies that removing a missing path exits 2.
func TestRunProjectRemove_NonExistentPathFails(t *testing.T) {
	dir := t.TempDir()
	seedConfig(t, dir, []string{"/"})

	code, _ := runProjectRemove(t, dir, []string{"/missing"})
	if code != 2 {
		t.Errorf("expected exit code 2 for missing path, got %d", code)
	}
}

// TestRunProjectRemove_NoConfigFails verifies that project-remove exits 2 when config is missing.
func TestRunProjectRemove_NoConfigFails(t *testing.T) {
	dir := t.TempDir()

	code, _ := runProjectRemove(t, dir, []string{"/settings"})
	if code != 2 {
		t.Errorf("expected exit code 2 when config missing, got %d", code)
	}
}
