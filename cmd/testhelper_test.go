package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// exitError is a sentinel panic value used to intercept calls to exit() in tests.
type exitError struct{ code int }

// runCapture runs f, captures its stdout, and intercepts any exit() call.
// Returns the captured output string and the exit code (0 if exit was not called).
func runCapture(f func()) (out string, code int) {
	// Redirect stdout to a pipe first.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic("os.Pipe: " + err.Error())
	}
	os.Stdout = w

	// Override exit to panic with exitError so we can catch it.
	oldExit := exit
	exit = func(c int) { panic(exitError{c}) }

	// drainPipe closes the write end, restores stdout, and reads all output.
	drainPipe := func() string {
		w.Close()
		os.Stdout = oldStdout
		exit = oldExit
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		r.Close()
		return buf.String()
	}

	// Catch any exitError panic.
	func() {
		defer func() {
			if rv := recover(); rv != nil {
				if e, ok := rv.(exitError); ok {
					code = e.code
				} else {
					// Re-panic unexpected panics after draining so we don't leak.
					out = drainPipe()
					panic(rv)
				}
			}
		}()
		f()
	}()

	out = drainPipe()
	return out, code
}

// parseResult parses a JSON output.Result from s.
func parseResult(t *testing.T, s string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		t.Fatalf("parseResult: invalid JSON %q: %v", s, err)
	}
	return result
}

// chdir changes into dir and restores the original on t.Cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// tempDir creates a temp directory, chdirs into it, and cleans up on t.Cleanup.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ks-cmd-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	chdir(t, dir)
	return dir
}
