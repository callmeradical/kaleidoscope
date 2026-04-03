package snapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// SnapshotMeta is a lightweight entry in the snapshot index.
type SnapshotMeta struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	URL       string    `json:"url"`
}

// SnapshotIndex tracks all saved snapshots in order.
type SnapshotIndex struct {
	Entries []SnapshotMeta `json:"entries"`
}

func indexPath(dir string) string {
	return filepath.Join(dir, "snapshots", "index.json")
}

// readIndex reads the snapshot index from disk. Returns an empty index if the file is absent.
func readIndex(dir string) (*SnapshotIndex, error) {
	path := indexPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SnapshotIndex{}, nil
		}
		return nil, err
	}
	var idx SnapshotIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// writeIndex marshals and writes the index to disk.
func writeIndex(dir string, idx *SnapshotIndex) error {
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath(dir), data, 0644)
}

// LatestID returns the ID of the most recently saved snapshot.
func LatestID(dir string) (string, error) {
	idx, err := readIndex(dir)
	if err != nil {
		return "", err
	}
	if len(idx.Entries) == 0 {
		return "", errors.New("no snapshots found")
	}
	return idx.Entries[len(idx.Entries)-1].ID, nil
}
