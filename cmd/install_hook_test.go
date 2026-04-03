package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeTempGitRepo creates a temp dir with .git/hooks/ and returns its path.
func makeTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755); err != nil {
		t.Fatalf("failed to create .git/hooks: %v", err)
	}
	return dir
}

// captureExit replaces osExit with a function that records the code and returns a restore func.
func captureExit(t *testing.T) (exitCode *int, restore func()) {
	t.Helper()
	code := -1
	orig := osExit
	osExit = func(c int) {
		code = c
		// Use a panic so execution stops at the os.Exit call site and tests can check exit code.
		panic("osExit called")
	}
	return &code, func() { osExit = orig }
}

// recoverExit calls f and recovers from the panic injected by captureExit.
// Returns true if osExit was called, false otherwise.
func recoverExit(f func()) (called bool) {
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok && s == "osExit called" {
				called = true
			} else {
				panic(r) // re-panic for unexpected panics
			}
		}
	}()
	f()
	return false
}

// captureStdout replaces os.Stdout with a pipe and returns a function that reads all output.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	return func() string {
		w.Close()
		os.Stdout = origStdout
		buf := make([]byte, 1<<20)
		n, _ := r.Read(buf)
		r.Close()
		return string(buf[:n])
	}
}

// TestInstallHook_HappyPath verifies the hook is written with correct content and permissions.
func TestInstallHook_HappyPath(t *testing.T) {
	dir := makeTempGitRepo(t)

	// Create .ks-project.json
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create .ks-project.json: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exitCode, restore := captureExit(t)
	defer restore()

	read := captureStdout(t)
	called := recoverExit(func() {
		RunInstallHook([]string{})
	})
	out := read()

	if called {
		t.Fatalf("RunInstallHook exited with code %d, want success; stdout: %s", *exitCode, out)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")

	// Assert hook file exists
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook file not created: %v", err)
	}

	// Assert executable permissions (0755)
	if info.Mode().Perm() != 0755 {
		t.Errorf("hook permissions = %o, want 0755", info.Mode().Perm())
	}

	// Assert content
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook file: %v", err)
	}
	cs := string(content)

	for _, want := range []string{"#!/bin/sh", "ks snapshot", "ks diff", "exit 0"} {
		if !strings.Contains(cs, want) {
			t.Errorf("hook content missing %q; got:\n%s", want, cs)
		}
	}

	// Assert JSON output contains ok:true
	if !strings.Contains(out, `"ok":true`) {
		t.Errorf("stdout missing ok:true; got: %s", out)
	}
}

// TestInstallHook_MissingProjectConfig ensures an error is returned when .ks-project.json is absent.
func TestInstallHook_MissingProjectConfig(t *testing.T) {
	dir := makeTempGitRepo(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exitCode, restore := captureExit(t)
	defer restore()

	read := captureStdout(t)
	called := recoverExit(func() {
		RunInstallHook([]string{})
	})
	out := read()

	if !called || *exitCode != 2 {
		t.Errorf("expected osExit(2); called=%v exitCode=%d; stdout: %s", called, *exitCode, out)
	}

	if !strings.Contains(out, `"ok":false`) {
		t.Errorf("stdout missing ok:false; got: %s", out)
	}

	if !strings.Contains(out, ".ks-project.json") {
		t.Errorf("error message should mention .ks-project.json; got: %s", out)
	}
}

// TestInstallHook_NotInGitRepo ensures an error is returned when .git is missing.
func TestInstallHook_NotInGitRepo(t *testing.T) {
	dir := t.TempDir() // no .git directory

	// Create .ks-project.json
	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create .ks-project.json: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exitCode, restore := captureExit(t)
	defer restore()

	read := captureStdout(t)
	called := recoverExit(func() {
		RunInstallHook([]string{})
	})
	out := read()

	if !called || *exitCode != 2 {
		t.Errorf("expected osExit(2); called=%v exitCode=%d; stdout: %s", called, *exitCode, out)
	}

	if !strings.Contains(out, `"ok":false`) {
		t.Errorf("stdout missing ok:false; got: %s", out)
	}

	if !strings.Contains(strings.ToLower(out), "git") {
		t.Errorf("error message should mention git repository; got: %s", out)
	}
}

// TestInstallHook_AlreadyExists_NoForce verifies that an existing hook is not overwritten without --force.
func TestInstallHook_AlreadyExists_NoForce(t *testing.T) {
	dir := makeTempGitRepo(t)

	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create .ks-project.json: %v", err)
	}

	const originalContent = "#!/bin/sh\necho existing hook\n"
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte(originalContent), 0755); err != nil {
		t.Fatalf("failed to write existing hook: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exitCode, restore := captureExit(t)
	defer restore()

	read := captureStdout(t)
	called := recoverExit(func() {
		RunInstallHook([]string{})
	})
	out := read()

	if !called || *exitCode != 2 {
		t.Errorf("expected osExit(2); called=%v exitCode=%d; stdout: %s", called, *exitCode, out)
	}

	if !strings.Contains(out, `"ok":false`) {
		t.Errorf("stdout missing ok:false; got: %s", out)
	}

	if !strings.Contains(out, "--force") {
		t.Errorf("hint should mention --force; got: %s", out)
	}

	// Original hook content must be unchanged
	got, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook: %v", err)
	}
	if string(got) != originalContent {
		t.Errorf("hook content was overwritten; got %q, want %q", string(got), originalContent)
	}
}

// TestInstallHook_AlreadyExists_WithForce verifies --force overwrites the existing hook.
func TestInstallHook_AlreadyExists_WithForce(t *testing.T) {
	dir := makeTempGitRepo(t)

	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create .ks-project.json: %v", err)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho old\n"), 0755); err != nil {
		t.Fatalf("failed to write existing hook: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exitCode, restore := captureExit(t)
	defer restore()

	read := captureStdout(t)
	called := recoverExit(func() {
		RunInstallHook([]string{"--force"})
	})
	out := read()

	if called {
		t.Fatalf("RunInstallHook --force exited with code %d; stdout: %s", *exitCode, out)
	}

	if !strings.Contains(out, `"ok":true`) {
		t.Errorf("stdout missing ok:true; got: %s", out)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook: %v", err)
	}
	if !strings.Contains(string(content), "ks snapshot") {
		t.Errorf("hook not overwritten with kaleidoscope script; got:\n%s", string(content))
	}
}

// TestInstallHook_FilePermissions verifies the hook is written with mode 0755.
func TestInstallHook_FilePermissions(t *testing.T) {
	dir := makeTempGitRepo(t)

	if err := os.WriteFile(filepath.Join(dir, ".ks-project.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create .ks-project.json: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exitCode, restore := captureExit(t)
	defer restore()

	read := captureStdout(t)
	called := recoverExit(func() {
		RunInstallHook([]string{})
	})
	_ = read()

	if called {
		t.Fatalf("RunInstallHook exited with code %d", *exitCode)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook not found: %v", err)
	}

	if info.Mode().Perm() != 0755 {
		t.Errorf("hook permissions = %04o, want 0755", info.Mode().Perm())
	}
}
