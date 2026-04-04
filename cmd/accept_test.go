package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/baseline"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// captureOutput redirects os.Stdout while f runs and returns what was written.
func captureOutput(f func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// parseResult unmarshals the JSON line written by output.Success / output.Fail.
func parseResult(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &m); err != nil {
		t.Fatalf("parse output JSON: %v\nraw: %s", err, raw)
	}
	return m
}

// makeSnapshot is a test helper that writes a snapshot to the store.
func makeSnapshot(t *testing.T, storeDir string, snap *snapshot.Snapshot) {
	t.Helper()
	store, err := snapshot.OpenStore(storeDir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Save(snap); err != nil {
		t.Fatalf("Save snapshot: %v", err)
	}
}

// TestAcceptCmd_NoSnapshots checks that an empty store produces an error result.
func TestAcceptCmd_NoSnapshots(t *testing.T) {
	ksDir := t.TempDir()

	var code int
	out := captureOutput(func() {
		code = acceptCmd([]string{}, ksDir)
	})

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	result := parseResult(t, out)
	if result["ok"] != false {
		t.Errorf("ok = %v, want false", result["ok"])
	}
	errMsg, _ := result["error"].(string)
	if !strings.Contains(errMsg, "no snapshots found") {
		t.Errorf("error %q does not contain 'no snapshots found'", errMsg)
	}
	hint, _ := result["hint"].(string)
	if !strings.Contains(hint, "ks snapshot") {
		t.Errorf("hint %q does not reference 'ks snapshot'", hint)
	}
}

// TestAcceptCmd_LatestSnapshot checks that calling with no args accepts the
// latest snapshot and writes all URL paths to baselines.json.
func TestAcceptCmd_LatestSnapshot(t *testing.T) {
	ksDir := t.TempDir()
	storeDir := filepath.Join(ksDir, "snapshots")

	snap := &snapshot.Snapshot{
		ID:        "20260404T120000Z",
		CreatedAt: time.Now().UTC(),
		URLs: []snapshot.SnapshotURL{
			{URL: "http://localhost/", Path: "/"},
			{URL: "http://localhost/dashboard", Path: "/dashboard"},
		},
	}
	makeSnapshot(t, storeDir, snap)

	var code int
	out := captureOutput(func() {
		code = acceptCmd([]string{}, ksDir)
	})

	if code != 0 {
		t.Errorf("exit code = %d, want 0\noutput: %s", code, out)
	}
	result := parseResult(t, out)
	if result["ok"] != true {
		t.Errorf("ok = %v, want true", result["ok"])
	}

	res, _ := result["result"].(map[string]any)
	if res == nil {
		t.Fatal("result field missing")
	}
	if res["snapshotId"] != snap.ID {
		t.Errorf("snapshotId = %v, want %v", res["snapshotId"], snap.ID)
	}
	paths, _ := res["paths"].([]any)
	if len(paths) != 2 {
		t.Errorf("paths len = %d, want 2", len(paths))
	}

	// baselines.json should exist and have both paths.
	mgr := baseline.NewManager(ksDir)
	f, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load baselines: %v", err)
	}
	if f.Baselines["/"].SnapshotID != snap.ID {
		t.Errorf("baselines[\"/\"] = %q, want %q", f.Baselines["/"].SnapshotID, snap.ID)
	}
	if f.Baselines["/dashboard"].SnapshotID != snap.ID {
		t.Errorf("baselines[\"/dashboard\"] = %q, want %q", f.Baselines["/dashboard"].SnapshotID, snap.ID)
	}
}

// TestAcceptCmd_SpecificSnapshotID checks that an explicit snapshot ID is
// accepted instead of the latest.
func TestAcceptCmd_SpecificSnapshotID(t *testing.T) {
	ksDir := t.TempDir()
	storeDir := filepath.Join(ksDir, "snapshots")

	older := &snapshot.Snapshot{
		ID:        "snap-old",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
		URLs:      []snapshot.SnapshotURL{{URL: "http://localhost/", Path: "/"}},
	}
	newer := &snapshot.Snapshot{
		ID:        "snap-new",
		CreatedAt: time.Now().UTC(),
		URLs:      []snapshot.SnapshotURL{{URL: "http://localhost/", Path: "/"}},
	}
	makeSnapshot(t, storeDir, older)
	makeSnapshot(t, storeDir, newer)

	var code int
	out := captureOutput(func() {
		code = acceptCmd([]string{"snap-old"}, ksDir)
	})

	if code != 0 {
		t.Errorf("exit code = %d, want 0\noutput: %s", code, out)
	}
	result := parseResult(t, out)
	res, _ := result["result"].(map[string]any)
	if res["snapshotId"] != "snap-old" {
		t.Errorf("snapshotId = %v, want snap-old", res["snapshotId"])
	}
}

// TestAcceptCmd_UnknownSnapshotID checks that referencing a non-existent
// snapshot ID returns an error.
func TestAcceptCmd_UnknownSnapshotID(t *testing.T) {
	ksDir := t.TempDir()
	storeDir := filepath.Join(ksDir, "snapshots")
	makeSnapshot(t, storeDir, &snapshot.Snapshot{
		ID:        "snap-001",
		CreatedAt: time.Now().UTC(),
		URLs:      []snapshot.SnapshotURL{{URL: "http://localhost/", Path: "/"}},
	})

	var code int
	out := captureOutput(func() {
		code = acceptCmd([]string{"nonexistent-id"}, ksDir)
	})

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	result := parseResult(t, out)
	if result["ok"] != false {
		t.Errorf("ok = %v, want false", result["ok"])
	}
}

// TestAcceptCmd_URLPathNotInSnapshot checks that --url with a path not present
// in the snapshot returns an error.
func TestAcceptCmd_URLPathNotInSnapshot(t *testing.T) {
	ksDir := t.TempDir()
	storeDir := filepath.Join(ksDir, "snapshots")
	makeSnapshot(t, storeDir, &snapshot.Snapshot{
		ID:        "snap-001",
		CreatedAt: time.Now().UTC(),
		URLs:      []snapshot.SnapshotURL{{URL: "http://localhost/", Path: "/"}},
	})

	var code int
	out := captureOutput(func() {
		code = acceptCmd([]string{"--url", "/missing"}, ksDir)
	})

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	result := parseResult(t, out)
	if result["ok"] != false {
		t.Errorf("ok = %v, want false", result["ok"])
	}
	errMsg, _ := result["error"].(string)
	if !strings.Contains(errMsg, "snapshot does not contain URL path") {
		t.Errorf("error %q does not contain expected message", errMsg)
	}
}

// TestAcceptCmd_URLFilter_PreservesOtherPaths checks that --url updates only
// the specified path and leaves other baseline entries unchanged.
func TestAcceptCmd_URLFilter_PreservesOtherPaths(t *testing.T) {
	ksDir := t.TempDir()
	storeDir := filepath.Join(ksDir, "snapshots")

	// Pre-populate baselines with an existing "/" entry.
	mgr := baseline.NewManager(ksDir)
	_, err := mgr.Accept("snap-old", []string{"/"})
	if err != nil {
		t.Fatalf("pre-populate baseline: %v", err)
	}

	// Snapshot has both "/" and "/dashboard".
	makeSnapshot(t, storeDir, &snapshot.Snapshot{
		ID:        "snap-new",
		CreatedAt: time.Now().UTC(),
		URLs: []snapshot.SnapshotURL{
			{URL: "http://localhost/", Path: "/"},
			{URL: "http://localhost/dashboard", Path: "/dashboard"},
		},
	})

	var code int
	out := captureOutput(func() {
		code = acceptCmd([]string{"--url", "/dashboard"}, ksDir)
	})

	if code != 0 {
		t.Errorf("exit code = %d, want 0\noutput: %s", code, out)
	}

	f, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load baselines: %v", err)
	}
	if f.Baselines["/"].SnapshotID != "snap-old" {
		t.Errorf("/ baseline = %q, want snap-old (should be unchanged)", f.Baselines["/"].SnapshotID)
	}
	if f.Baselines["/dashboard"].SnapshotID != "snap-new" {
		t.Errorf("/dashboard baseline = %q, want snap-new", f.Baselines["/dashboard"].SnapshotID)
	}
}
