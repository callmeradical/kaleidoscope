package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupInstallHookDir creates a temp dir with the given files/dirs.
// Pass paths to create as files (empty) or dirs (ending in "/").
func setupInstallHookDir(t *testing.T, entries ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, e := range entries {
		if strings.HasSuffix(e, "/") {
			if err := os.MkdirAll(filepath.Join(dir, e), 0755); err != nil {
				t.Fatalf("setup: mkdir %s: %v", e, err)
			}
		} else {
			path := filepath.Join(dir, e)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatalf("setup: mkdir parent of %s: %v", e, err)
			}
			if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
				t.Fatalf("setup: write %s: %v", e, err)
			}
		}
	}
	return dir
}

// TestInstallHook_NoProjectConfig verifies that missing .ks-project.json returns an error.
func TestInstallHook_NoProjectConfig(t *testing.T) {
	dir := t.TempDir() // no .ks-project.json, no .git

	_, _, err, exitCode := installHookCore(dir, false)
	if err == nil {
		t.Fatal("expected error when .ks-project.json is missing, got nil")
	}
	if !strings.Contains(err.Error(), ".ks-project.json") {
		t.Errorf("error message should mention .ks-project.json, got: %s", err.Error())
	}
	if exitCode != 2 {
		t.Errorf("expected exit code 2, got %d", exitCode)
	}
}

// TestInstallHook_NoGitRepo verifies that missing .git directory returns an error.
func TestInstallHook_NoGitRepo(t *testing.T) {
	dir := setupInstallHookDir(t, ".ks-project.json") // has config but no .git

	_, _, err, exitCode := installHookCore(dir, false)
	if err == nil {
		t.Fatal("expected error when .git directory is missing, got nil")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error message should mention git repository, got: %s", err.Error())
	}
	if exitCode != 2 {
		t.Errorf("expected exit code 2, got %d", exitCode)
	}
}

// TestInstallHook_SuccessfulInstall verifies that the hook is written correctly.
func TestInstallHook_SuccessfulInstall(t *testing.T) {
	dir := setupInstallHookDir(t, ".ks-project.json", ".git/hooks/")

	result, hint, err, exitCode := installHookCore(dir, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hint != "" {
		t.Errorf("expected no hint, got: %s", hint)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")

	// Hook file should exist
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("pre-commit hook file not created: %v", err)
	}

	// Hook file should be executable (mode 0755)
	mode := info.Mode()
	if mode&0111 == 0 {
		t.Errorf("pre-commit hook should be executable, got mode %o", mode)
	}

	// Hook content should start with #!/bin/sh
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook file: %v", err)
	}
	if !strings.HasPrefix(string(data), "#!/bin/sh") {
		t.Errorf("hook should start with #!/bin/sh, got: %q", string(data[:min(20, len(data))]))
	}

	// Result should have path and message
	if result == nil {
		t.Fatal("expected result map, got nil")
	}
	if result["path"] != ".git/hooks/pre-commit" {
		t.Errorf("result.path should be .git/hooks/pre-commit, got: %v", result["path"])
	}
	if result["message"] == "" {
		t.Error("result.message should not be empty")
	}
}

// TestInstallHook_HooksDirCreatedAutomatically verifies MkdirAll creates hooks/ if missing.
func TestInstallHook_HooksDirCreatedAutomatically(t *testing.T) {
	// Only create .git/ (no hooks/ subdirectory)
	dir := setupInstallHookDir(t, ".ks-project.json", ".git/")

	_, _, err, _ := installHookCore(dir, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); err != nil {
		t.Errorf("pre-commit hook should be created even when hooks/ dir is missing: %v", err)
	}
}

// TestInstallHook_ExistingHookNoForce verifies that existing hook is not overwritten without --force.
func TestInstallHook_ExistingHookNoForce(t *testing.T) {
	dir := setupInstallHookDir(t, ".ks-project.json", ".git/hooks/")

	sentinel := "sentinel content do not overwrite"
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte(sentinel), 0755); err != nil {
		t.Fatalf("writing sentinel hook: %v", err)
	}

	_, hint, err, exitCode := installHookCore(dir, false)
	if err == nil {
		t.Fatal("expected error when hook exists and --force is not set, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force, got: %s", err.Error())
	}
	if !strings.Contains(hint, "--force") {
		t.Errorf("hint should mention --force, got: %s", hint)
	}
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	// File should NOT be overwritten
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook file: %v", err)
	}
	if string(data) != sentinel {
		t.Errorf("existing hook should not be overwritten; got %q", string(data))
	}
}

// TestInstallHook_ExistingHookWithForce verifies that --force overwrites the existing hook.
func TestInstallHook_ExistingHookWithForce(t *testing.T) {
	dir := setupInstallHookDir(t, ".ks-project.json", ".git/hooks/")

	sentinel := "sentinel content to be replaced"
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte(sentinel), 0755); err != nil {
		t.Fatalf("writing sentinel hook: %v", err)
	}

	result, _, err, exitCode := installHookCore(dir, true)
	if err != nil {
		t.Fatalf("expected no error with --force, got: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if result == nil {
		t.Fatal("expected result map, got nil")
	}

	// File should be replaced with the new hook script
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook file: %v", err)
	}
	if string(data) == sentinel {
		t.Error("hook file should have been replaced, but still contains sentinel")
	}
	if !strings.HasPrefix(string(data), "#!/bin/sh") {
		t.Errorf("new hook should start with #!/bin/sh")
	}
}

// TestInstallHook_ScriptSyntax verifies the hook script is valid POSIX sh.
func TestInstallHook_ScriptSyntax(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found in PATH, skipping syntax check")
	}

	// Write hook script to a temp file and check syntax with sh -n
	tmp, err := os.CreateTemp("", "ks-pre-commit-*")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(hookScript); err != nil {
		t.Fatalf("writing hook script: %v", err)
	}
	tmp.Close()

	out, err := exec.Command(sh, "-n", tmp.Name()).CombinedOutput()
	if err != nil {
		t.Errorf("hook script has syntax errors: %v\n%s", err, out)
	}
}

// TestInstallHook_ScriptExitsZero verifies the hook script exits 0 when ks is not in PATH.
func TestInstallHook_ScriptExitsZero(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found in PATH, skipping exit-code test")
	}

	// Write hook script to a temp file and execute with empty PATH
	tmp, err := os.CreateTemp("", "ks-pre-commit-*")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(hookScript); err != nil {
		t.Fatalf("writing hook script: %v", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		t.Fatalf("chmod hook script: %v", err)
	}
	tmp.Close()

	cmd := exec.Command(sh, tmp.Name())
	cmd.Env = []string{"PATH="} // ks not in PATH → script should still exit 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Errorf("hook script exited with code %d (not 0); should always be advisory", exitErr.ExitCode())
		} else {
			t.Errorf("running hook script: %v", err)
		}
	}
}

// TestInstallHook_ScriptEndsWithExit0 verifies the hook script constant ends with exit 0.
func TestInstallHook_ScriptEndsWithExit0(t *testing.T) {
	trimmed := strings.TrimRight(hookScript, "\n\r\t ")
	if !strings.HasSuffix(trimmed, "exit 0") {
		t.Errorf("hook script should end with 'exit 0', got suffix: %q",
			trimmed[max(0, len(trimmed)-20):])
	}
}

// TestInstallHook_HasForceFlag verifies --force flag parsing via RunInstallHook path.
func TestInstallHook_ParseForceFlag(t *testing.T) {
	tests := []struct {
		args  []string
		force bool
	}{
		{[]string{}, false},
		{[]string{"--force"}, true},
		{[]string{"--force", "--other"}, true},
		{[]string{"--other"}, false},
	}
	for _, tc := range tests {
		got := hasFlag(tc.args, "--force")
		if got != tc.force {
			t.Errorf("hasFlag(%v, --force) = %v, want %v", tc.args, got, tc.force)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
