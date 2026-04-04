package snapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SnapshotDir returns the absolute path to the directory for the given snapshot ID.
// It validates the ID to prevent path traversal attacks.
func SnapshotDir(id string) (string, error) {
	if id == "" {
		return "", errors.New("snapshot id must not be empty")
	}
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.ContainsRune(id, 0) {
		return "", errors.New("snapshot id contains invalid characters")
	}
	base, err := filepath.Abs(filepath.Join(".kaleidoscope", "snapshots"))
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, id)
	// Confirm resolved path is still under the snapshots root.
	if !strings.HasPrefix(dir, base+string(filepath.Separator)) {
		return "", errors.New("snapshot id escapes snapshot directory")
	}
	return dir, nil
}

// Save writes the snapshot to disk as JSON.
func Save(s *Snapshot) error {
	dir, err := SnapshotDir(s.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "snapshot.json"), data, 0600)
}

// Load reads and parses a snapshot from disk.
func Load(id string) (*Snapshot, error) {
	dir, err := SnapshotDir(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns snapshot IDs sorted newest-first (relies on YYYYMMDD-HHmmss prefix).
func List() ([]string, error) {
	dir, err := filepath.Abs(filepath.Join(".kaleidoscope", "snapshots"))
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
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

// Latest returns the most recent snapshot.
func Latest() (*Snapshot, error) {
	ids, err := List()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.New("no snapshots found")
	}
	return Load(ids[0])
}
