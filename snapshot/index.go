package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/callmeradical/kaleidoscope/browser"
)

// SnapshotMeta holds metadata for a single snapshot (US-003 contract).
type SnapshotMeta struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	CommitSHA string    `json:"commitSha"`
	URLs      []string  `json:"urls"`
}

// Index is the top-level container for the snapshot index file (US-003 contract).
type Index struct {
	Snapshots []SnapshotMeta `json:"snapshots"`
}

// LoadIndex reads .kaleidoscope/snapshots/index.json and returns the snapshot index.
// Returns an error if the file does not exist or cannot be parsed.
func LoadIndex() (*Index, error) {
	dir, err := browser.StateDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "snapshots", "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("snapshot index not found at %s", path)
		}
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot index: %w", err)
	}
	return &idx, nil
}
