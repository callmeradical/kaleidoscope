package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNoBaseline is returned when no baseline has been set via `ks snapshot --set-baseline`.
var ErrNoBaseline = errors.New("no baseline set; run `ks snapshot --set-baseline` first")

const (
	snapshotsDirName  = "snapshots"
	kaleidoscopeDir   = ".kaleidoscope"
	snapshotFile      = "snapshot.json"
	baselinesFile     = "baselines.json"
)

// SnapshotsDir returns the path to the snapshots directory, creating it if absent.
func SnapshotsDir() (string, error) {
	dir := filepath.Join(kaleidoscopeDir, snapshotsDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create snapshots dir: %w", err)
	}
	return dir, nil
}

// ListSnapshots returns snapshot IDs sorted newest-first (lexicographic descending).
func ListSnapshots() ([]string, error) {
	dir, err := SnapshotsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read snapshots dir: %w", err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, nil
}

// LoadSnapshot reads a snapshot by ID from .kaleidoscope/snapshots/<id>/snapshot.json.
func LoadSnapshot(id string) (*Snapshot, error) {
	dir, err := SnapshotsDir()
	if err != nil {
		return nil, err
	}
	// Prevent path traversal: clean and confirm it stays within the snapshots dir
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	target := filepath.Clean(filepath.Join(dir, id, snapshotFile))
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(absTarget, absDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid snapshot id: %q", id)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("read snapshot %q: %w", id, err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse snapshot %q: %w", id, err)
	}
	return &s, nil
}

// LoadLatestSnapshot loads the most recently created snapshot.
func LoadLatestSnapshot() (*Snapshot, error) {
	ids, err := ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.New("no snapshots found; run `ks snapshot` first")
	}
	return LoadSnapshot(ids[0])
}

// baselineRecord matches the structure of baselines.json.
type baselineRecord struct {
	Baseline string `json:"baseline"`
}

// LoadBaseline reads baselines.json and loads the designated baseline snapshot.
// Returns ErrNoBaseline if the file does not exist or the baseline field is empty.
func LoadBaseline() (*Snapshot, error) {
	path := filepath.Join(kaleidoscopeDir, baselinesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoBaseline
		}
		return nil, fmt.Errorf("read baselines file: %w", err)
	}
	var rec baselineRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parse baselines file: %w", err)
	}
	if rec.Baseline == "" {
		return nil, ErrNoBaseline
	}
	return LoadSnapshot(rec.Baseline)
}
