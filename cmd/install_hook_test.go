package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupFakeGitRepo creates a temp directory with a .git/hooks structure.
// Returns (tempDir, cleanup func).
func setupFakeGitRepo(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755); err != nil {
		t.Fatalf("failed to create .git/hooks: %v", err)
	}
	return dir, func() {} // TempDir auto-cleaned
}

// writeProjectConfig writes a minimal .ks-project.json into dir.
func writeProjectConfig(t *testing.T, dir string) {
	t.Helper()
	f := filepath.Join(dir, ".ks-project.json")
	if err := os.WriteFile(f, []byte(`{"name":"test"}`), 0644); err != nil {
		t.Fatalf("failed to write .ks-project.json: %v", err)
	}
}

// buildBinary compiles the kaleidoscope binary into a temp dir.
// Returns the path to the binary and a cleanup function.
func buildBinary(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "ks")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/callmeradical/kaleidoscope").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}
	return bin, func() {}
}

// runKS executes the ks binary with the given args from the given working directory.
// Returns stdout+stderr combined, the exit code, and whether the process exited non-zero.
func runKS(t *testing.T, bin, workDir string, args ...string) (string, int) {
	t.Helper()
	c := exec.Command(bin, args...)
	c.Dir = workDir
	out, err := c.CombinedOutput()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}
	return string(out), exitCode
}

// ---------------------------------------------------------------------------
// Tests for: ks install-hook
// ---------------------------------------------------------------------------

// TestInstallHookNoProjectConfig verifies that running install-hook without a
// .ks-project.json in the current directory exits non-zero and emits an error.
func TestInstallHookNoProjectConfig(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()

	out, code := runKS(t, bin, dir, "install-hook")
	if code == 0 {
		t.Errorf("expected non-zero exit when .ks-project.json is missing, got 0\noutput: %s", out)
	}
	if !strings.Contains(out, "ks-project.json") {
		t.Errorf("expected output to mention .ks-project.json, got: %s", out)
	}
}

// TestInstallHookWritesExecutableScript verifies that install-hook writes an
// executable file at .git/hooks/pre-commit.
func TestInstallHookWritesExecutableScript(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()
	writeProjectConfig(t, dir)

	out, code := runKS(t, bin, dir, "install-hook")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput: %s", code, out)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("expected hook to exist at %s, got error: %v", hookPath, err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("expected hook file to be executable, mode=%v", info.Mode())
	}
}

// TestInstallHookScriptHasShebang verifies the written hook starts with #!/bin/sh.
func TestInstallHookScriptHasShebang(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()
	writeProjectConfig(t, dir)

	_, code := runKS(t, bin, dir, "install-hook")
	if code != 0 {
		t.Fatal("install-hook failed unexpectedly")
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook: %v", err)
	}
	if !strings.HasPrefix(string(data), "#!/bin/sh") {
		t.Errorf("expected hook to start with #!/bin/sh, got: %q", string(data[:min(30, len(data))]))
	}
}

// TestInstallHookScriptRunsSnapshotAndDiff verifies the hook script invokes
// both `ks snapshot` and `ks diff`.
func TestInstallHookScriptRunsSnapshotAndDiff(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()
	writeProjectConfig(t, dir)

	_, code := runKS(t, bin, dir, "install-hook")
	if code != 0 {
		t.Fatal("install-hook failed unexpectedly")
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "snapshot") {
		t.Errorf("expected hook to contain 'snapshot', got: %s", content)
	}
	if !strings.Contains(content, "diff") {
		t.Errorf("expected hook to contain 'diff', got: %s", content)
	}
}

// TestInstallHookScriptExitsZero verifies that the hook always exits 0.
func TestInstallHookScriptExitsZero(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()
	writeProjectConfig(t, dir)

	_, code := runKS(t, bin, dir, "install-hook")
	if code != 0 {
		t.Fatal("install-hook failed unexpectedly")
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook: %v", err)
	}
	// The hook script must have "exit 0" to ensure it's advisory/non-blocking.
	if !strings.Contains(string(data), "exit 0") {
		t.Errorf("expected hook to contain 'exit 0' (advisory), got: %s", string(data))
	}
}

// TestInstallHookScriptAutoStartsChrome verifies the hook tries to start Chrome.
func TestInstallHookScriptAutoStartsChrome(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()
	writeProjectConfig(t, dir)

	_, code := runKS(t, bin, dir, "install-hook")
	if code != 0 {
		t.Fatal("install-hook failed unexpectedly")
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook: %v", err)
	}
	// Hook should reference "start" to auto-start Chrome.
	if !strings.Contains(string(data), "start") {
		t.Errorf("expected hook to contain a 'start' invocation for Chrome, got: %s", string(data))
	}
}

// TestInstallHookSuccessOutputIsJSON verifies that install-hook emits JSON on success.
func TestInstallHookSuccessOutputIsJSON(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()
	writeProjectConfig(t, dir)

	out, code := runKS(t, bin, dir, "install-hook")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput: %s", code, out)
	}
	// Output must be JSON containing "ok":true and "install-hook"
	if !strings.Contains(out, `"ok":true`) {
		t.Errorf("expected JSON with ok:true, got: %s", out)
	}
	if !strings.Contains(out, "install-hook") {
		t.Errorf("expected JSON to reference install-hook command, got: %s", out)
	}
}

// TestInstallHookWarnIfHookExists verifies that when a hook already exists,
// install-hook exits non-zero (without --force) and does NOT overwrite.
func TestInstallHookWarnIfHookExists(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()
	writeProjectConfig(t, dir)

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	original := "#!/bin/sh\necho original\n"
	if err := os.WriteFile(hookPath, []byte(original), 0755); err != nil {
		t.Fatalf("failed to pre-create hook: %v", err)
	}

	out, code := runKS(t, bin, dir, "install-hook")
	if code == 0 {
		t.Errorf("expected non-zero exit when hook already exists, got 0\noutput: %s", out)
	}
	// File must not be overwritten.
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook: %v", err)
	}
	if string(data) != original {
		t.Errorf("hook was silently overwritten; expected original content, got: %s", string(data))
	}
}

// TestInstallHookForceOverwritesExisting verifies --force overwrites an existing hook.
func TestInstallHookForceOverwritesExisting(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	dir, cleanup2 := setupFakeGitRepo(t)
	defer cleanup2()
	writeProjectConfig(t, dir)

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	original := "#!/bin/sh\necho original\n"
	if err := os.WriteFile(hookPath, []byte(original), 0755); err != nil {
		t.Fatalf("failed to pre-create hook: %v", err)
	}

	_, code := runKS(t, bin, dir, "install-hook", "--force")
	if code != 0 {
		t.Fatalf("expected exit 0 with --force, got %d", code)
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook after --force: %v", err)
	}
	if string(data) == original {
		t.Errorf("expected hook to be overwritten with --force, but original content remains")
	}
}

// TestInstallHookNotInGitRepo verifies an error is returned when not in a git repo.
func TestInstallHookNotInGitRepo(t *testing.T) {
	bin, cleanup := buildBinary(t)
	defer cleanup()

	// A temp dir with no .git directory at all.
	dir := t.TempDir()
	writeProjectConfig(t, dir)

	out, code := runKS(t, bin, dir, "install-hook")
	if code == 0 {
		t.Errorf("expected non-zero exit when not in a git repo, got 0\noutput: %s", out)
	}
}

// min returns the smaller of two ints (Go 1.21+ has builtin min, but keep explicit for compat).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
