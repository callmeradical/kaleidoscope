package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// BaselinesFile tracks which snapshots are designated as baselines.
type BaselinesFile struct {
	Default string            `json:"default"`
	Named   map[string]string `json:"named"`
}

var validIDRegexp = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// validateSnapshotID rejects IDs that could be used for path traversal.
func validateSnapshotID(id string) error {
	if !validIDRegexp.MatchString(id) {
		return fmt.Errorf("invalid snapshot ID: %q (must match ^[a-zA-Z0-9\\-_]+$)", id)
	}
	return nil
}

// KaleidoscopeDir returns the kaleidoscope state directory.
// If local is true, returns ".kaleidoscope" relative to CWD.
// Otherwise returns "~/.kaleidoscope".
func KaleidoscopeDir(local bool) string {
	if local {
		return filepath.Join(".", ".kaleidoscope")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".kaleidoscope")
	}
	return filepath.Join(home, ".kaleidoscope")
}

const maxSnapshotBytes = 50 * 1024 * 1024 // 50 MB

// Load reads a snapshot from disk by ID.
func Load(dir, id string) (*Snapshot, error) {
	if err := validateSnapshotID(id); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "snapshots", id+".json")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot %q not found: %w", id, err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxSnapshotBytes))
	if err != nil {
		return nil, fmt.Errorf("reading snapshot %q: %w", id, err)
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing snapshot %q: %w", id, err)
	}
	return &s, nil
}

// Save writes a snapshot to disk and updates the index.
func Save(dir string, s *Snapshot) error {
	snapshotsDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	path := filepath.Join(snapshotsDir, s.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	idx, err := readIndex(dir)
	if err != nil {
		return err
	}
	idx.Entries = append(idx.Entries, SnapshotMeta{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		URL:       s.URL,
	})
	return writeIndex(dir, idx)
}

// LoadBaselines reads baselines.json from dir.
func LoadBaselines(dir string) (*BaselinesFile, error) {
	path := filepath.Join(dir, "baselines.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("baselines.json not found: no baseline set")
		}
		return nil, fmt.Errorf("reading baselines.json: %w", err)
	}
	var b BaselinesFile
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing baselines.json: %w", err)
	}
	return &b, nil
}
