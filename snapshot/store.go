package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

const (
	snapshotsDir = ".kaleidoscope/snapshots"
	baselineFile = ".kaleidoscope/baselines.json"
)

// SnapshotsDir returns (and creates) the absolute path to the snapshots directory.
func SnapshotsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path := filepath.Join(cwd, snapshotsDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

// SnapshotPath returns the absolute path for a snapshot ID (does not create directory).
func SnapshotPath(id SnapshotID) (string, error) {
	dir, err := SnapshotsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id), nil
}

// URLDir creates and returns the subdirectory for a URL within a snapshot.
func URLDir(snapshotPath, rawURL string) (string, error) {
	key := URLToKey(rawURL)
	path := filepath.Join(snapshotPath, key)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

// WriteManifest marshals m to indented JSON and writes it to snapshot.json.
func WriteManifest(snapshotPath string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotPath, "snapshot.json"), data, 0644)
}

// ReadManifest reads and unmarshals snapshot.json from the given snapshot directory.
func ReadManifest(snapshotPath string) (*Manifest, error) {
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

// ListSnapshotIDs returns snapshot IDs in descending order (newest first).
func ListSnapshotIDs() ([]SnapshotID, error) {
	dir, err := SnapshotsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var ids []SnapshotID
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	// Sort descending (newest first) by lexicographic reverse order
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] > ids[j]
	})
	return ids, nil
}

// ReadBaselineManifest reads .kaleidoscope/baselines.json.
// Returns nil, nil if the file does not exist.
func ReadBaselineManifest() (*BaselineManifest, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(cwd, baselineFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var b BaselineManifest
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// WriteBaselineManifest writes b to .kaleidoscope/baselines.json.
func WriteBaselineManifest(b *BaselineManifest) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dir := filepath.Join(cwd, ".kaleidoscope")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cwd, baselineFile), data, 0644)
}
