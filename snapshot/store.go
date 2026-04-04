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

// Load reads a snapshot by ID from the given state directory.
// It validates the ID to prevent path traversal.
func Load(dir, id string) (*Snapshot, error) {
	snapshotsDir := filepath.Join(dir, "snapshots")
	target := filepath.Clean(filepath.Join(snapshotsDir, id))
	if !strings.HasPrefix(target, snapshotsDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid snapshot id %q: path traversal detected", id)
	}

	jsonPath := filepath.Join(target, "snapshot.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("snapshot %q not found", id)
		}
		return nil, fmt.Errorf("reading snapshot %q: %w", id, err)
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing snapshot %q: %w", id, err)
	}
	return &s, nil
}

// Latest returns the most recently created snapshot in the state directory.
func Latest(dir string) (*Snapshot, error) {
	snapshotsDir := filepath.Join(dir, "snapshots")
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("no snapshots found: run 'ks snapshot' to capture one")
		}
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}

	var snapshots []*Snapshot
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := Load(dir, e.Name())
		if err != nil {
			continue
		}
		snapshots = append(snapshots, s)
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found: run 'ks snapshot' to capture one")
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})
	return snapshots[0], nil
}

// LoadBaseline reads baselines.json from the state directory and loads the default baseline snapshot.
func LoadBaseline(dir string) (*Snapshot, error) {
	baselinesPath := filepath.Join(dir, "baselines.json")
	data, err := os.ReadFile(baselinesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("baselines.json not found: run 'ks baseline set'")
		}
		return nil, fmt.Errorf("reading baselines.json: %w", err)
	}

	var baselines map[string]string
	if err := json.Unmarshal(data, &baselines); err != nil {
		return nil, fmt.Errorf("parsing baselines.json: %w", err)
	}

	id, ok := baselines["default"]
	if !ok || id == "" {
		return nil, fmt.Errorf("no default baseline set: run 'ks baseline set'")
	}

	return Load(dir, id)
}

// ScreenshotPath returns the absolute path to a screenshot file within a snapshot.
func ScreenshotPath(dir, snapshotID, filename string) string {
	return filepath.Join(dir, "snapshots", snapshotID, filename)
}
