package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/callmeradical/kaleidoscope/browser"
)

// SnapshotsDir returns the path to .kaleidoscope/snapshots/.
// The directory is NOT created here; callers are responsible for os.MkdirAll.
func SnapshotsDir() (string, error) {
	stateDir, err := browser.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "snapshots"), nil
}

// SnapshotPath returns the directory path for a specific snapshot ID.
func SnapshotPath(id string) (string, error) {
	snapshotsDir, err := SnapshotsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(snapshotsDir, id), nil
}

// Save writes the manifest to <snapshotsDir>/<id>/snapshot.json.
func Save(m *Manifest) error {
	snapshotDir, err := SnapshotPath(m.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("creating snapshot dir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotDir, "snapshot.json"), data, 0644)
}

// Load reads and parses the manifest for the given snapshot ID.
func Load(id string) (*Manifest, error) {
	snapshotPath, err := SnapshotPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(snapshotPath, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// List returns all valid manifests sorted by Timestamp descending (newest first).
// If the snapshots directory doesn't exist, an empty slice is returned without error.
func List() ([]*Manifest, error) {
	snapshotsDir, err := SnapshotsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Manifest{}, nil
		}
		return nil, err
	}

	var manifests []*Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := Load(entry.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping corrupt snapshot %s: %v\n", entry.Name(), err)
			continue
		}
		manifests = append(manifests, m)
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Timestamp.After(manifests[j].Timestamp)
	})

	return manifests, nil
}
