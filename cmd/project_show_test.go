package cmd_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/callmeradical/kaleidoscope/cmd"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
)

// TestRunProjectShow_Helper is the subprocess entry point.
func TestRunProjectShow_Helper(t *testing.T) {
	if os.Getenv("KS_TEST_HELPER") != t.Name() {
		t.Skip("subprocess helper – not for direct invocation")
	}
	dir := os.Getenv("KS_TEST_WORKDIR")
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	cmd.RunProjectShow(nil)
}

func runProjectShow(t *testing.T, dir string) (int, []byte) {
	t.Helper()
	c := exec.Command(os.Args[0], "-test.run=^TestRunProjectShow_Helper$", "-test.v")
	c.Env = append(os.Environ(),
		"KS_TEST_HELPER=TestRunProjectShow_Helper",
		"KS_TEST_WORKDIR="+dir,
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

// TestRunProjectShow_OutputsConfig verifies project-show emits full config JSON via output.Result.
func TestRunProjectShow_OutputsConfig(t *testing.T) {
	dir := t.TempDir()
	want := &project.ProjectConfig{
		Name: "showtest", BaseURL: "http://show.example.com",
		Paths: []string{"/", "/about"}, Breakpoints: project.DefaultBreakpoints(),
	}
	raw, _ := json.MarshalIndent(want, "", "  ")
	os.WriteFile(filepath.Join(dir, project.ConfigFile), raw, 0644)

	code, out := runProjectShow(t, dir)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; output: %s", code, out)
	}

	// Find the JSON line in the output (subprocess outputs go test header lines too).
	var result output.Result
	for _, line := range splitLines(out) {
		if len(line) > 0 && line[0] == '{' {
			if err := json.Unmarshal([]byte(line), &result); err == nil {
				break
			}
		}
	}

	if !result.OK {
		t.Errorf("output.Result.OK: got false, want true; error=%q", result.Error)
	}
	if result.Command != "project-show" {
		t.Errorf("output.Result.Command: got %q, want %q", result.Command, "project-show")
	}
	if result.Result == nil {
		t.Fatal("output.Result.Result is nil")
	}

	// Verify the config fields are present in the result.
	resultBytes, _ := json.Marshal(result.Result)
	var gotCfg project.ProjectConfig
	json.Unmarshal(resultBytes, &gotCfg)

	if gotCfg.Name != want.Name {
		t.Errorf("config Name: got %q, want %q", gotCfg.Name, want.Name)
	}
	if gotCfg.BaseURL != want.BaseURL {
		t.Errorf("config BaseURL: got %q, want %q", gotCfg.BaseURL, want.BaseURL)
	}
}

// TestRunProjectShow_NoConfigFails verifies project-show exits 2 when config is missing.
func TestRunProjectShow_NoConfigFails(t *testing.T) {
	dir := t.TempDir()

	code, _ := runProjectShow(t, dir)
	if code != 2 {
		t.Errorf("expected exit code 2 when config missing, got %d", code)
	}
}

// splitLines splits bytes into non-empty string lines.
func splitLines(b []byte) []string {
	var lines []string
	start := 0
	for i, c := range b {
		if c == '\n' {
			line := string(b[start:i])
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(b) {
		line := string(b[start:])
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

