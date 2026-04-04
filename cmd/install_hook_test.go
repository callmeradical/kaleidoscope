package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTempDir creates a temp dir and changes the working directory to it.
// Restores the original working directory on test cleanup.
func setupTempDir(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
	return dir
}

// runInstallHookSubprocess runs RunInstallHook in a subprocess and returns
// the combined output and exit code. Tests that expect os.Exit must use this.
func runInstallHookSubprocess(t *testing.T, dir string) (output string, exitCode int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestInstallHookSubprocess", "-test.v")
	cmd.Env = append(os.Environ(), "KS_TEST_INSTALL_HOOK=1", "KS_TEST_HOOK_DIR="+dir)
	out, err := cmd.CombinedOutput()
	output = string(out)
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return output, exitCode
}

// TestInstallHookSubprocess is the subprocess entry point.
// It is invoked by runInstallHookSubprocess and should not be run directly.
func TestInstallHookSubprocess(t *testing.T) {
	if os.Getenv("KS_TEST_INSTALL_HOOK") != "1" {
		t.Skip("subprocess only")
	}
	dir := os.Getenv("KS_TEST_HOOK_DIR")
	if dir == "" {
		t.Fatal("KS_TEST_HOOK_DIR not set")
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	RunInstallHook([]string{})
}

// TestRunInstallHook_MissingProjectConfig asserts that RunInstallHook exits 2
// and outputs a JSON error when .ks-project.json is absent.
func TestRunInstallHook_MissingProjectConfig(t *testing.T) {
	dir := t.TempDir()
	// No .ks-project.json created.

	out, code := runInstallHookSubprocess(t, dir)
	if code != 2 {
		t.Errorf("expected exit code 2, got %d\noutput: %s", code, out)
	}

	wantMsg := "no .ks-project.json found in current directory"
	if !strings.Contains(out, wantMsg) {
		t.Errorf("expected output to contain %q\ngot: %s", wantMsg, out)
	}

	// Validate JSON shape.
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	// Extract first JSON line from output.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			if err := json.Unmarshal([]byte(line), &result); err == nil {
				break
			}
		}
	}
	if result.OK {
		t.Error("expected ok=false in JSON output")
	}
}

// TestRunInstallHook_NotGitRepo asserts that RunInstallHook exits 2
// and outputs a JSON error when .git/ is absent.
func TestRunInstallHook_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	// Create .ks-project.json but no .git/.
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, code := runInstallHookSubprocess(t, dir)
	if code != 2 {
		t.Errorf("expected exit code 2, got %d\noutput: %s", code, out)
	}

	wantMsg := "no .git/ directory found"
	if !strings.Contains(out, wantMsg) {
		t.Errorf("expected output to contain %q\ngot: %s", wantMsg, out)
	}
}

// TestRunInstallHook_HookAlreadyExists asserts that RunInstallHook exits 2,
// outputs a JSON warning, and does NOT overwrite the existing hook.
func TestRunInstallHook_HookAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	gitDir := filepath.Join(dir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hookPath := filepath.Join(hooksDir, "pre-commit")
	originalContent := "#!/bin/sh\necho original\n"
	if err := os.WriteFile(hookPath, []byte(originalContent), 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	out, code := runInstallHookSubprocess(t, dir)
	if code != 2 {
		t.Errorf("expected exit code 2, got %d\noutput: %s", code, out)
	}

	wantMsg := "pre-commit hook already exists at .git/hooks/pre-commit"
	if !strings.Contains(out, wantMsg) {
		t.Errorf("expected output to contain %q\ngot: %s", wantMsg, out)
	}

	// The existing hook must not be overwritten.
	got, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	if string(got) != originalContent {
		t.Errorf("hook was overwritten; expected %q, got %q", originalContent, string(got))
	}
}

// TestRunInstallHook_SuccessCleanInstall asserts that RunInstallHook creates
// an executable pre-commit hook script with required content.
func TestRunInstallHook_SuccessCleanInstall(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	out, code := runInstallHookSubprocess(t, dir)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d\noutput: %s", code, out)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook file not created: %v", err)
	}

	// Mode must be 0755.
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected mode 0755, got %v", info.Mode().Perm())
	}

	// Content must include required lines.
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	for _, want := range []string{"#!/bin/sh", "ks snapshot", "ks diff", "exit 0"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("hook script missing %q\ncontent:\n%s", want, string(content))
		}
	}

	// JSON output must include ok:true and hookPath.
	if !strings.Contains(out, `"ok":true`) {
		t.Errorf("expected ok:true in output\ngot: %s", out)
	}
	if !strings.Contains(out, "hookPath") {
		t.Errorf("expected hookPath in output\ngot: %s", out)
	}
}

// TestRunInstallHook_HooksDirCreatedIfAbsent asserts that RunInstallHook
// creates .git/hooks/ if it does not already exist.
func TestRunInstallHook_HooksDirCreatedIfAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	// Create .git but NOT .git/hooks.
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	_, code := runInstallHookSubprocess(t, dir)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	hooksDir := filepath.Join(dir, ".git", "hooks")
	info, err := os.Stat(hooksDir)
	if err != nil {
		t.Fatalf(".git/hooks/ not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".git/hooks/ is not a directory")
	}
}

// TestRunInstallHook_HookAutoStartsChrome asserts that the hook script
// contains logic to auto-start Chrome via `ks start` if not already running.
func TestRunInstallHook_HookAutoStartsChrome(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	_, code := runInstallHookSubprocess(t, dir)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}

	if !strings.Contains(string(content), "ks start") {
		t.Errorf("hook script should contain 'ks start' for auto-starting Chrome\ncontent:\n%s", string(content))
	}
}

// TestRunInstallHook_AdvisoryExitZero asserts the hook script always exits 0.
func TestRunInstallHook_AdvisoryExitZero(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	_, code := runInstallHookSubprocess(t, dir)
	if code != 0 {
		t.Errorf("install-hook should exit 0 on success, got %d", code)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}

	// The generated hook script itself must end with exit 0.
	if !strings.Contains(string(content), "exit 0") {
		t.Errorf("hook script must contain 'exit 0' (advisory/non-blocking)\ncontent:\n%s", string(content))
	}
}
