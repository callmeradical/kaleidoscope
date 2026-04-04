package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

// setTestStateDir redirects the state directory to dir for the duration of the
// test and restores the original value via t.Cleanup.
func setTestStateDir(t *testing.T, dir string) {
	t.Helper()
	orig := stateDirOverride
	stateDirOverride = dir
	t.Cleanup(func() { stateDirOverride = orig })
}

func TestReadBaselines_FileNotExist(t *testing.T) {
	tmp := t.TempDir()
	setTestStateDir(t, filepath.Join(tmp, "nonexistent"))

	b, err := ReadBaselines()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(b) != 0 {
		t.Fatalf("expected empty map, got %v", b)
	}
}

func TestWriteAndReadBaselines_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	setTestStateDir(t, tmp)

	want := Baselines{"/": "snap-001", "/dashboard": "snap-002"}
	if err := WriteBaselines(want); err != nil {
		t.Fatalf("WriteBaselines error: %v", err)
	}

	got, err := ReadBaselines()
	if err != nil {
		t.Fatalf("ReadBaselines error: %v", err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %q: want %q, got %q", k, v, got[k])
		}
	}
	if len(got) != len(want) {
		t.Errorf("length mismatch: want %d, got %d", len(want), len(got))
	}

	// Verify file mode.
	path := filepath.Join(tmp, "baselines.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat baselines.json: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("expected mode 0644, got %v", info.Mode().Perm())
	}
}

func TestAcceptSnapshot_AllURLs(t *testing.T) {
	current := Baselines{"/": "old-id", "/dashboard": "old-id"}
	snap := &Snapshot{ID: "new-id", URLPath: "/"}

	updated, wasNoOp, err := AcceptSnapshot(current, snap, "*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wasNoOp {
		t.Error("expected wasNoOp=false")
	}
	for k, v := range updated {
		if v != "new-id" {
			t.Errorf("key %q: want %q, got %q", k, "new-id", v)
		}
	}
}

func TestAcceptSnapshot_SpecificURL(t *testing.T) {
	current := Baselines{"/": "old-id", "/dashboard": "old-id"}
	snap := &Snapshot{ID: "new-id", URLPath: "/dashboard"}

	updated, wasNoOp, err := AcceptSnapshot(current, snap, "/dashboard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wasNoOp {
		t.Error("expected wasNoOp=false")
	}
	if updated["/dashboard"] != "new-id" {
		t.Errorf("/dashboard: want new-id, got %q", updated["/dashboard"])
	}
	if updated["/"] != "old-id" {
		t.Errorf("/: want old-id, got %q", updated["/"])
	}
}

func TestAcceptSnapshot_Idempotent(t *testing.T) {
	current := Baselines{"/": "same-id"}
	snap := &Snapshot{ID: "same-id", URLPath: "/"}

	_, wasNoOp, err := AcceptSnapshot(current, snap, "*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wasNoOp {
		t.Error("expected wasNoOp=true for idempotent accept")
	}
}

func TestAcceptSnapshot_EmptyCurrentBaselines(t *testing.T) {
	current := Baselines{}
	snap := &Snapshot{ID: "snap-001", URLPath: "/new"}

	updated, wasNoOp, err := AcceptSnapshot(current, snap, "*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wasNoOp {
		t.Error("expected wasNoOp=false")
	}
	if updated["/new"] != "snap-001" {
		t.Errorf("/new: want snap-001, got %q", updated["/new"])
	}
}

func TestAcceptSnapshot_NoURLFlag_UsesSnapURLPath(t *testing.T) {
	current := Baselines{}
	snap := &Snapshot{ID: "snap-001", URLPath: "/settings"}

	updated, _, err := AcceptSnapshot(current, snap, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated["/settings"] != "snap-001" {
		t.Errorf("/settings: want snap-001, got %q", updated["/settings"])
	}
}
