package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/callmeradical/kaleidoscope/cmd"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// captureOutput redirects os.Stdout, runs f, then returns the captured bytes.
func captureOutput(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = orig

	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	r.Close()
	return buf.String()
}

// makeSnapshotIndex writes a snapshot index.json under dir/.kaleidoscope/snapshots/.
func makeSnapshotIndex(t *testing.T, dir string, entries []snapshot.SnapshotEntry) {
	t.Helper()
	snapDir := filepath.Join(dir, ".kaleidoscope", "snapshots")
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	idx := snapshot.Index{Snapshots: entries}
	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "index.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// parseResult parses the JSON line emitted by RunAccept.
func parseResult(t *testing.T, out string) map[string]any {
	t.Helper()
	out = strings.TrimSpace(out)
	if out == "" {
		t.Fatal("no output captured from RunAccept")
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON %q: %v", out, err)
	}
	return result
}

// chdirTemp creates a temp dir, chdirs into it, and restores the original dir after test.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	return dir
}

func TestRunAccept_NoSnapshots(t *testing.T) {
	dir := chdirTemp(t)
	// create the .kaleidoscope dir but no snapshot index
	if err := os.MkdirAll(filepath.Join(dir, ".kaleidoscope"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	out := captureOutput(t, func() { cmd.RunAccept([]string{}) })
	r := parseResult(t, out)

	if ok, _ := r["ok"].(bool); ok {
		t.Errorf("expected ok=false when no snapshots exist, got ok=true")
	}
	errStr, _ := r["error"].(string)
	if !strings.Contains(strings.ToLower(errStr), "snapshot") {
		t.Errorf("expected error to mention 'snapshot', got: %q", errStr)
	}
}

func TestRunAccept_Latest(t *testing.T) {
	dir := chdirTemp(t)
	makeSnapshotIndex(t, dir, []snapshot.SnapshotEntry{
		{
			ID: "snap-001",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
			},
		},
		{
			ID: "snap-002",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
				{URL: "http://localhost/dashboard", Path: "/dashboard"},
			},
		},
	})

	out := captureOutput(t, func() { cmd.RunAccept([]string{}) })
	r := parseResult(t, out)

	if ok, _ := r["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got: %v (error: %v)", r["ok"], r["error"])
	}

	result, _ := r["result"].(map[string]any)
	snapID, _ := result["snapshot_id"].(string)
	if snapID != "snap-002" {
		t.Errorf("expected snapshot_id=snap-002, got %q", snapID)
	}

	updated, _ := result["updated"].([]any)
	if len(updated) != 2 {
		t.Errorf("expected 2 updated paths, got %d: %v", len(updated), updated)
	}
}

func TestRunAccept_ByID(t *testing.T) {
	dir := chdirTemp(t)
	makeSnapshotIndex(t, dir, []snapshot.SnapshotEntry{
		{
			ID: "snap-001",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
			},
		},
		{
			ID: "snap-002",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
			},
		},
	})

	out := captureOutput(t, func() { cmd.RunAccept([]string{"snap-001"}) })
	r := parseResult(t, out)

	if ok, _ := r["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got: %v (error: %v)", r["ok"], r["error"])
	}

	result, _ := r["result"].(map[string]any)
	snapID, _ := result["snapshot_id"].(string)
	if snapID != "snap-001" {
		t.Errorf("expected snapshot_id=snap-001, got %q", snapID)
	}
}

func TestRunAccept_ByID_NotFound(t *testing.T) {
	dir := chdirTemp(t)
	makeSnapshotIndex(t, dir, []snapshot.SnapshotEntry{
		{
			ID: "snap-001",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
			},
		},
	})

	out := captureOutput(t, func() { cmd.RunAccept([]string{"unknown-id"}) })
	r := parseResult(t, out)

	if ok, _ := r["ok"].(bool); ok {
		t.Error("expected ok=false for unknown snapshot ID")
	}
	errStr, _ := r["error"].(string)
	if !strings.Contains(strings.ToLower(errStr), "not found") {
		t.Errorf("expected error to contain 'not found', got: %q", errStr)
	}
}

func TestRunAccept_URLFilter(t *testing.T) {
	dir := chdirTemp(t)
	makeSnapshotIndex(t, dir, []snapshot.SnapshotEntry{
		{
			ID: "snap-001",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
				{URL: "http://localhost/dashboard", Path: "/dashboard"},
			},
		},
	})

	out := captureOutput(t, func() { cmd.RunAccept([]string{"--url", "/dashboard"}) })
	r := parseResult(t, out)

	if ok, _ := r["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got: %v (error: %v)", r["ok"], r["error"])
	}

	result, _ := r["result"].(map[string]any)
	updated, _ := result["updated"].([]any)
	if len(updated) != 1 {
		t.Errorf("expected 1 updated path, got %d: %v", len(updated), updated)
	}
	if len(updated) == 1 {
		if updated[0].(string) != "/dashboard" {
			t.Errorf("expected updated[0]='/dashboard', got %q", updated[0])
		}
	}

	// root path must not appear in updated or skipped
	skipped, _ := result["skipped"].([]any)
	for _, p := range append(updated, skipped...) {
		if p.(string) == "/" {
			t.Errorf("expected '/' to be absent from updated/skipped when --url /dashboard is used")
		}
	}
}

func TestRunAccept_URLFilter_NoMatch(t *testing.T) {
	dir := chdirTemp(t)
	makeSnapshotIndex(t, dir, []snapshot.SnapshotEntry{
		{
			ID: "snap-001",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
			},
		},
	})

	out := captureOutput(t, func() { cmd.RunAccept([]string{"--url", "/nonexistent"}) })
	r := parseResult(t, out)

	if ok, _ := r["ok"].(bool); ok {
		t.Error("expected ok=false when --url path not in snapshot")
	}
	errStr, _ := r["error"].(string)
	if !strings.Contains(strings.ToLower(errStr), "no url") {
		t.Errorf("expected error to contain 'no url', got: %q", errStr)
	}
}

func TestRunAccept_Idempotent(t *testing.T) {
	dir := chdirTemp(t)
	makeSnapshotIndex(t, dir, []snapshot.SnapshotEntry{
		{
			ID: "snap-001",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
				{URL: "http://localhost/about", Path: "/about"},
			},
		},
	})

	// First call: should update both
	out1 := captureOutput(t, func() { cmd.RunAccept([]string{}) })
	r1 := parseResult(t, out1)
	if ok, _ := r1["ok"].(bool); !ok {
		t.Fatalf("first call: expected ok=true, got: %v (error: %v)", r1["ok"], r1["error"])
	}

	// Second call: should skip both (idempotent)
	out2 := captureOutput(t, func() { cmd.RunAccept([]string{}) })
	r2 := parseResult(t, out2)
	if ok, _ := r2["ok"].(bool); !ok {
		t.Fatalf("second call: expected ok=true, got: %v (error: %v)", r2["ok"], r2["error"])
	}

	result2, _ := r2["result"].(map[string]any)
	updated2, _ := result2["updated"].([]any)
	skipped2, _ := result2["skipped"].([]any)
	if len(updated2) != 0 {
		t.Errorf("second call: expected 0 updated, got %d: %v", len(updated2), updated2)
	}
	if len(skipped2) != 2 {
		t.Errorf("second call: expected 2 skipped, got %d: %v", len(skipped2), skipped2)
	}
}

func TestRunAccept_InvalidID(t *testing.T) {
	dir := chdirTemp(t)
	makeSnapshotIndex(t, dir, []snapshot.SnapshotEntry{
		{
			ID: "snap-001",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
			},
		},
	})

	out := captureOutput(t, func() { cmd.RunAccept([]string{"../evil"}) })
	r := parseResult(t, out)

	if ok, _ := r["ok"].(bool); ok {
		t.Error("expected ok=false for invalid snapshot ID")
	}
	errStr, _ := r["error"].(string)
	if !strings.Contains(strings.ToLower(errStr), "invalid") {
		t.Errorf("expected error to contain 'invalid', got: %q", errStr)
	}
}

func TestRunAccept_BaselinesPersisted(t *testing.T) {
	dir := chdirTemp(t)
	makeSnapshotIndex(t, dir, []snapshot.SnapshotEntry{
		{
			ID: "snap-001",
			URLs: []snapshot.URLEntry{
				{URL: "http://localhost/", Path: "/"},
			},
		},
	})

	out := captureOutput(t, func() { cmd.RunAccept([]string{}) })
	r := parseResult(t, out)
	if ok, _ := r["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got: %v (error: %v)", r["ok"], r["error"])
	}

	// baselines.json must exist on disk
	baselinesPath := filepath.Join(dir, ".kaleidoscope", "baselines.json")
	data, err := os.ReadFile(baselinesPath)
	if err != nil {
		t.Fatalf("baselines.json not found after accept: %v", err)
	}

	var stored map[string]any
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("baselines.json is not valid JSON: %v", err)
	}
	entries, _ := stored["baselines"].([]any)
	if len(entries) != 1 {
		t.Errorf("expected 1 baseline entry on disk, got %d", len(entries))
	}
}
