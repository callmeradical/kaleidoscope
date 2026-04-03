package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// URLEntry represents a single URL captured in a snapshot.
type URLEntry struct {
	URL  string `json:"url"`
	Path string `json:"path"`
}

// SnapshotEntry represents a single snapshot in the index.
type SnapshotEntry struct {
	ID        string     `json:"id"`
	CreatedAt string     `json:"created_at"`
	URLs      []URLEntry `json:"urls"`
}

// Index holds all snapshots for a project.
type Index struct {
	Snapshots []SnapshotEntry `json:"snapshots"`
}

// LoadIndex reads the snapshot index from dir/.kaleidoscope/snapshots/index.json.
// Returns an empty Index (no error) if the file does not exist.
func LoadIndex(dir string) (*Index, error) {
	path := filepath.Join(dir, "snapshots", "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Index{}, nil
		}
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// Latest returns the most recently added snapshot, or nil if the index is empty.
func (idx *Index) Latest() *SnapshotEntry {
	if len(idx.Snapshots) == 0 {
		return nil
	}
	return &idx.Snapshots[len(idx.Snapshots)-1]
}

// ByID returns the snapshot with the given ID, or nil if not found.
func (idx *Index) ByID(id string) *SnapshotEntry {
	for i := range idx.Snapshots {
		if idx.Snapshots[i].ID == id {
			return &idx.Snapshots[i]
		}
	}
	return nil
}
