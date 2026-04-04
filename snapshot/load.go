package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// LoadByID loads a snapshot from .kaleidoscope/snapshots/<id>/snapshot.json.
func LoadByID(id string) (*Snapshot, error) {
	p, err := snapshotPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(p, "snapshot.json"))
	if err != nil {
		return nil, fmt.Errorf("snapshot %q not found: %w", id, err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("snapshot %q: invalid JSON: %w", id, err)
	}
	return &s, nil
}

// LoadLatest returns the most recent snapshot by lexicographic directory name.
func LoadLatest() (*Snapshot, error) {
	dir := SnapshotsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("no snapshots found (directory unreadable): %w", err)
	}
	// Collect directory names and sort descending.
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no snapshots found in %s", dir)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		s, err := LoadByID(name)
		if err == nil {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no valid snapshot found in %s", dir)
}

// baselineRecord is the JSON structure of baselines.json.
type baselineRecord struct {
	Baseline string `json:"baseline"`
}

// LoadBaseline loads the snapshot marked as baseline in baselines.json.
func LoadBaseline() (*Snapshot, error) {
	data, err := os.ReadFile(BaselinesFile())
	if err != nil {
		return nil, fmt.Errorf("no baseline set")
	}
	var rec baselineRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("baselines.json is invalid: %w", err)
	}
	if rec.Baseline == "" {
		return nil, fmt.Errorf("no baseline set")
	}
	return LoadByID(rec.Baseline)
}
