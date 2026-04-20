package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helpers ---

// setupTestRepo creates a temp directory with a .git/hooks/ structure.
// If withProject is true, it also creates a .ks-project.json file.
// Returns the temp dir path and a cleanup function.
func setupTestRepo(t *testing.T, withProject bool) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if withProject {
		if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(`{"urls":["http://localhost:3000"]}`), 0644); err != nil {
			t.Fatalf("WriteFile .ks-project.json: %v", err)
		}
	}
	return dir, func() {} // t.TempDir cleans up automatically
}

// captureRun runs fn, capturing stdout output, the exit code, and whether
// exitFn was called. It always restores os.Stdout and exitFn afterward.
type runResult struct {
	stdout string
	code   int
	exited bool
}

func captureRun(fn func()) (res runResult) {
	// Capture stdout.
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	// Override exitFn.
	origExit := exitFn
	exitFn = func(code int) {
		res.code = code
		res.exited = true
		// Close the write end so the reader gets EOF.
		w.Close()
		os.Stdout = origStdout
		exitFn = origExit
		panic("captureRun-exit")
	}

	defer func() {
		// Restore regardless of panic.
		if !res.exited {
			w.Close()
			os.Stdout = origStdout
		}
		exitFn = origExit
		if r2 := recover(); r2 != nil && r2 != "captureRun-exit" {
			panic(r2) // re-panic if it's not our sentinel
		}
		out, _ := io.ReadAll(r)
		res.stdout += string(out)
	}()

	fn()
	return
}

// changeDir changes the working directory to dir and restores it on cleanup.
func changeDir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Logf("warning: could not restore working directory: %v", err)
		}
	})
}

// parseJSON unmarshals a JSON line from stdout into a map.
func parseJSON(t *testing.T, output string) map[string]interface{} {
	t.Helper()
	output = strings.TrimSpace(output)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(output), &m); err != nil {
		t.Fatalf("parseJSON: %v\nraw output: %q", err, output)
	}
	return m
}

// --- findGitRoot unit tests ---

// TestFindGitRoot_FromRoot verifies findGitRoot returns the repo root when called
// from the repo root itself.
func TestFindGitRoot_FromRoot(t *testing.T) {
	dir, _ := setupTestRepo(t, false)
	got, err := findGitRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

// TestFindGitRoot_FromSubdir verifies findGitRoot walks up from a subdirectory.
func TestFindGitRoot_FromSubdir(t *testing.T) {
	dir, _ := setupTestRepo(t, false)
	subdir := filepath.Join(dir, "src", "components")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	got, err := findGitRoot(subdir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

// TestFindGitRoot_NoGitDir verifies findGitRoot returns an error for non-git dirs.
func TestFindGitRoot_NoGitDir(t *testing.T) {
	// Use a directory with no .git ancestor (a plain temp dir not created by setupTestRepo).
	dir := t.TempDir()
	_, err := findGitRoot(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error should mention 'not a git repository', got: %v", err)
	}
}

// --- hookScript content tests ---

// TestHookScript_Shebang verifies the hook script starts with #!/bin/sh.
func TestHookScript_Shebang(t *testing.T) {
	if !strings.HasPrefix(hookScript, "#!/bin/sh") {
		t.Errorf("hookScript should start with '#!/bin/sh', got prefix: %q", hookScript[:20])
	}
}

// TestHookScript_ContainsSnapshot verifies the hook script invokes ks snapshot.
func TestHookScript_ContainsSnapshot(t *testing.T) {
	if !strings.Contains(hookScript, "ks snapshot") {
		t.Error("hookScript should contain 'ks snapshot'")
	}
}

// TestHookScript_ContainsDiff verifies the hook script invokes ks diff.
func TestHookScript_ContainsDiff(t *testing.T) {
	if !strings.Contains(hookScript, "ks diff") {
		t.Error("hookScript should contain 'ks diff'")
	}
}

// TestHookScript_ExitZero verifies the hook script ends with exit 0 (non-blocking).
func TestHookScript_ExitZero(t *testing.T) {
	if !strings.Contains(hookScript, "exit 0") {
		t.Error("hookScript should contain 'exit 0' to be advisory/non-blocking")
	}
}

// TestHookScript_AutoStartChrome verifies the hook attempts to start Chrome.
func TestHookScript_AutoStartChrome(t *testing.T) {
	if !strings.Contains(hookScript, "ks start") {
		t.Error("hookScript should contain 'ks start' for auto-start Chrome")
	}
}

// --- RunInstallHook tests ---

// TestRunInstallHook_NoProjectConfig verifies that running without .ks-project.json
// emits ok:false and exits with code 2.
func TestRunInstallHook_NoProjectConfig(t *testing.T) {
	dir, _ := setupTestRepo(t, false) // no .ks-project.json
	changeDir(t, dir)

	res := captureRun(func() {
		RunInstallHook([]string{})
	})

	if !res.exited {
		t.Fatal("expected exitFn to be called")
	}
	if res.code != 2 {
		t.Errorf("expected exit code 2, got %d", res.code)
	}
	m := parseJSON(t, res.stdout)
	if m["ok"] != false {
		t.Errorf("expected ok:false, got %v", m["ok"])
	}
	if !strings.Contains(strings.ToLower(fmt.Sprint(m["error"])), "ks-project.json") &&
		!strings.Contains(strings.ToLower(fmt.Sprint(m["hint"])), "ks init") {
		t.Errorf("error/hint should mention missing config or ks init, got error=%v hint=%v", m["error"], m["hint"])
	}
}

// TestRunInstallHook_NotAGitRepo verifies that running outside a git repo
// emits ok:false and exits with code 2.
func TestRunInstallHook_NotAGitRepo(t *testing.T) {
	// Create a plain dir with .ks-project.json but no .git/.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	changeDir(t, dir)

	res := captureRun(func() {
		RunInstallHook([]string{})
	})

	if !res.exited {
		t.Fatal("expected exitFn to be called")
	}
	if res.code != 2 {
		t.Errorf("expected exit code 2, got %d", res.code)
	}
	m := parseJSON(t, res.stdout)
	if m["ok"] != false {
		t.Errorf("expected ok:false, got %v", m["ok"])
	}
	if !strings.Contains(strings.ToLower(fmt.Sprint(m["error"])), "git") {
		t.Errorf("error should mention 'git', got: %v", m["error"])
	}
}

// TestRunInstallHook_FirstInstall verifies that the hook file is created with
// ok:true and overwrite:false on a fresh install.
func TestRunInstallHook_FirstInstall(t *testing.T) {
	dir, _ := setupTestRepo(t, true)
	changeDir(t, dir)

	res := captureRun(func() {
		RunInstallHook([]string{})
	})

	if res.exited {
		t.Fatalf("exitFn should not be called on success, got exit code %d\nstdout: %s", res.code, res.stdout)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Errorf("hook file not created at %s", hookPath)
	}

	m := parseJSON(t, res.stdout)
	if m["ok"] != true {
		t.Errorf("expected ok:true, got %v", m["ok"])
	}
	result, _ := m["result"].(map[string]interface{})
	if result == nil {
		t.Fatal("expected result object in output")
	}
	if result["overwrite"] != false {
		t.Errorf("expected overwrite:false, got %v", result["overwrite"])
	}
}

// TestRunInstallHook_ExistingHookNoForce verifies that an existing hook is NOT
// overwritten and ok:false is returned when --force is absent.
func TestRunInstallHook_ExistingHookNoForce(t *testing.T) {
	dir, _ := setupTestRepo(t, true)
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	original := "#!/bin/sh\necho original\n"
	if err := os.WriteFile(hookPath, []byte(original), 0755); err != nil {
		t.Fatalf("pre-create hook: %v", err)
	}
	changeDir(t, dir)

	res := captureRun(func() {
		RunInstallHook([]string{})
	})

	if !res.exited {
		t.Fatal("expected exitFn to be called when hook exists and no --force")
	}
	if res.code != 2 {
		t.Errorf("expected exit code 2, got %d", res.code)
	}
	m := parseJSON(t, res.stdout)
	if m["ok"] != false {
		t.Errorf("expected ok:false, got %v", m["ok"])
	}
	if !strings.Contains(fmt.Sprint(m["hint"]), "--force") {
		t.Errorf("hint should mention '--force', got: %v", m["hint"])
	}

	// Original content must be preserved.
	got, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != original {
		t.Errorf("hook content was changed; want %q, got %q", original, string(got))
	}
}

// TestRunInstallHook_ExistingHookWithForce verifies that --force overwrites the
// existing hook and returns ok:true with overwrite:true.
func TestRunInstallHook_ExistingHookWithForce(t *testing.T) {
	dir, _ := setupTestRepo(t, true)
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho original\n"), 0755); err != nil {
		t.Fatalf("pre-create hook: %v", err)
	}
	changeDir(t, dir)

	res := captureRun(func() {
		RunInstallHook([]string{"--force"})
	})

	if res.exited {
		t.Fatalf("exitFn should not be called on success with --force, exit code %d\nstdout: %s", res.code, res.stdout)
	}

	m := parseJSON(t, res.stdout)
	if m["ok"] != true {
		t.Errorf("expected ok:true, got %v", m["ok"])
	}
	result, _ := m["result"].(map[string]interface{})
	if result == nil {
		t.Fatal("expected result object")
	}
	if result["overwrite"] != true {
		t.Errorf("expected overwrite:true, got %v", result["overwrite"])
	}

	// Hook content should now be the kaleidoscope script.
	got, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != hookScript {
		t.Error("hook file content does not match hookScript constant after --force overwrite")
	}
}

// TestRunInstallHook_ExecutablePermissions verifies the written hook has execute bits set.
func TestRunInstallHook_ExecutablePermissions(t *testing.T) {
	dir, _ := setupTestRepo(t, true)
	changeDir(t, dir)

	res := captureRun(func() {
		RunInstallHook([]string{})
	})
	if res.exited {
		t.Fatalf("unexpected exit %d: %s", res.code, res.stdout)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("hook file is not executable: mode %v", info.Mode())
	}
}

// TestRunInstallHook_HookContent verifies the written hook contains required elements.
func TestRunInstallHook_HookContent(t *testing.T) {
	dir, _ := setupTestRepo(t, true)
	changeDir(t, dir)

	res := captureRun(func() {
		RunInstallHook([]string{})
	})
	if res.exited {
		t.Fatalf("unexpected exit %d: %s", res.code, res.stdout)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	s := string(content)

	checks := []struct {
		name string
		want string
	}{
		{"shebang", "#!/bin/sh"},
		{"ks snapshot", "ks snapshot"},
		{"ks diff", "ks diff"},
		{"exit 0", "exit 0"},
		{"ks start (auto-start Chrome)", "ks start"},
	}
	for _, c := range checks {
		if !strings.Contains(s, c.want) {
			t.Errorf("hook content missing %s (%q)", c.name, c.want)
		}
	}
}

// TestRunInstallHook_HookPathInOutput verifies the output result includes the hookPath.
func TestRunInstallHook_HookPathInOutput(t *testing.T) {
	dir, _ := setupTestRepo(t, true)
	changeDir(t, dir)

	res := captureRun(func() {
		RunInstallHook([]string{})
	})
	if res.exited {
		t.Fatalf("unexpected exit: %s", res.stdout)
	}

	m := parseJSON(t, res.stdout)
	result, _ := m["result"].(map[string]interface{})
	if result == nil {
		t.Fatal("expected result object")
	}
	hookPath, _ := result["hookPath"].(string)
	if hookPath == "" {
		t.Error("result.hookPath should be non-empty")
	}
	if !strings.Contains(hookPath, filepath.Join(".git", "hooks", "pre-commit")) {
		t.Errorf("hookPath %q should contain .git/hooks/pre-commit", hookPath)
	}
}
