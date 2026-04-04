package cmd_test

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ksBin is the path to the compiled ks binary, built once in TestMain.
var ksBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "ks-test-bin-*")
	if err != nil {
		log.Fatalf("testmain: MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmp)

	ksBin = filepath.Join(tmp, "ks")
	buildCmd := exec.Command("go", "build", "-o", ksBin, "github.com/callmeradical/kaleidoscope")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		log.Fatalf("testmain: build failed: %v\n%s", err, out)
	}

	os.Exit(m.Run())
}

// ksResult is the standard JSON envelope returned by all ks commands.
type ksResult struct {
	OK      bool            `json:"ok"`
	Command string          `json:"command"`
	Result  json.RawMessage `json:"result"`
	Error   string          `json:"error"`
	Hint    string          `json:"hint"`
}

// runKS executes the ks binary with args in dir.
// Returns parsed ksResult and process exit code.
// The binary always writes JSON to stdout; stdout is captured even on non-zero exit.
func runKS(t *testing.T, dir string, args ...string) (ksResult, int) {
	t.Helper()
	cmd := exec.Command(ksBin, args...)
	cmd.Dir = dir

	out, err := cmd.Output()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("runKS(%v): unexpected error: %v", args, err)
		}
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return ksResult{}, exitCode
	}

	var r ksResult
	if jsonErr := json.Unmarshal([]byte(raw), &r); jsonErr != nil {
		t.Logf("runKS(%v): raw stdout: %q", args, raw)
		t.Fatalf("runKS(%v): JSON parse error: %v", args, jsonErr)
	}
	return r, exitCode
}

// resultMap unmarshals the Result field of a ksResult into a map.
func resultMap(t *testing.T, r ksResult) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(r.Result, &m); err != nil {
		t.Fatalf("resultMap: %v (raw: %s)", err, r.Result)
	}
	return m
}

// initProject bootstraps a .ks-project.json in dir via ks init.
// Fails the test if init does not succeed.
func initProject(t *testing.T, dir, name, baseURL, paths string) {
	t.Helper()
	r, exitCode := runKS(t, dir, "init", "--name", name, "--base-url", baseURL, "--paths", paths)
	if exitCode != 0 || !r.OK {
		t.Fatalf("initProject: exit=%d ok=%v error=%q", exitCode, r.OK, r.Error)
	}
}
