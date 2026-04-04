package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// captureStdout redirects os.Stdout for the duration of fn and returns the
// captured bytes. Because output.Success/Fail write to os.Stdout via fmt.Println,
// we redirect the file descriptor directly.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.Bytes()
}

// setAcceptTestStateDir sets the snapshot package's state dir override and
// restores it after the test.
func setAcceptTestStateDir(t *testing.T, dir string) {
	t.Helper()
	snapshot.SetStateDirOverride(dir)
	t.Cleanup(func() { snapshot.SetStateDirOverride("") })
}

// writeAcceptMeta writes a snapshot meta.json into the given state directory.
func writeAcceptMeta(t *testing.T, stateDir string, s snapshot.Snapshot) {
	t.Helper()
	d := filepath.Join(stateDir, "snapshots", s.ID)
	if err := os.MkdirAll(d, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(filepath.Join(d, "meta.json"), data, 0644); err != nil {
		t.Fatalf("write meta.json: %v", err)
	}
}

type acceptResult struct {
	OK     bool           `json:"ok"`
	Result acceptResultIn `json:"result"`
}
type acceptResultIn struct {
	SnapshotID string            `json:"snapshotId"`
	NoOp       bool              `json:"noOp"`
	Updated    map[string]string `json:"updated"`
	URL        string            `json:"url"`
}

func TestRunAccept_NoSnapshots_Fails(t *testing.T) {
	tmp := t.TempDir()
	setAcceptTestStateDir(t, tmp)

	var exitCalled bool
	// Patch os.Exit to avoid terminating the test process.
	origExit := osExit
	osExit = func(code int) {
		exitCalled = true
		panic("exit called") // unwind the call stack
	}
	defer func() {
		osExit = origExit
		recover() // swallow the panic from fake exit
	}()

	raw := captureStdout(t, func() {
		defer func() { recover() }()
		RunAccept([]string{})
	})

	if !exitCalled {
		t.Error("expected os.Exit to be called")
	}

	var res output.Result
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("parse output JSON: %v (raw: %s)", err, raw)
	}
	if res.OK {
		t.Error("expected ok=false")
	}
	if res.Hint == "" {
		t.Error("expected non-empty hint")
	}
}

func TestRunAccept_LatestSnapshot_AllURLs(t *testing.T) {
	tmp := t.TempDir()
	setAcceptTestStateDir(t, tmp)

	snap := snapshot.Snapshot{
		ID:        "snap-001",
		URLPath:   "/",
		CreatedAt: time.Now(),
	}
	writeAcceptMeta(t, tmp, snap)

	// Pre-populate baselines.
	if err := snapshot.WriteBaselines(snapshot.Baselines{"/": "old-id"}); err != nil {
		t.Fatalf("WriteBaselines: %v", err)
	}

	raw := captureStdout(t, func() {
		RunAccept([]string{})
	})

	var res acceptResult
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("parse output JSON: %v (raw: %s)", err, raw)
	}
	if !res.OK {
		t.Error("expected ok=true")
	}
	if res.Result.NoOp {
		t.Error("expected noOp=false")
	}
	if res.Result.Updated["/"] != "snap-001" {
		t.Errorf("/: want snap-001, got %q", res.Result.Updated["/"])
	}
}

func TestRunAccept_AlreadyBaseline_NoOp(t *testing.T) {
	tmp := t.TempDir()
	setAcceptTestStateDir(t, tmp)

	snap := snapshot.Snapshot{
		ID:        "snap-001",
		URLPath:   "/",
		CreatedAt: time.Now(),
	}
	writeAcceptMeta(t, tmp, snap)

	// Set baselines so snap-001 is already the baseline for "/".
	if err := snapshot.WriteBaselines(snapshot.Baselines{"/": "snap-001"}); err != nil {
		t.Fatalf("WriteBaselines: %v", err)
	}

	raw := captureStdout(t, func() {
		RunAccept([]string{})
	})

	var res acceptResult
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("parse output JSON: %v (raw: %s)", err, raw)
	}
	if !res.OK {
		t.Error("expected ok=true")
	}
	if !res.Result.NoOp {
		t.Error("expected noOp=true (idempotent)")
	}
}
