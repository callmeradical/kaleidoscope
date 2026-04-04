package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Snapshot represents a point-in-time capture of one or more project URLs.
type Snapshot struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"createdAt"`
	URLs      []SnapshotURL `json:"urls"`
}

// SnapshotURL holds the captured state for a single URL within a snapshot.
type SnapshotURL struct {
	URL            string `json:"url"`
	Path           string `json:"path"`
	ScreenshotPath string `json:"screenshotPath"`
	AuditResult    any    `json:"auditResult"`
	AXTree         any    `json:"axTree"`
}

// Store manages snapshot persistence in a directory.
type Store struct {
	dir string
}

// OpenStore opens (or creates) a snapshot store at the given directory.
func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Save writes a snapshot to the store as <id>.json.
func (s *Store) Save(snap *Snapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, snap.ID+".json"), data, 0644)
}

// Latest returns the most recently created snapshot, or an error if none exist.
func (s *Store) Latest() (*Snapshot, error) {
	snaps, err := s.list()
	if err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, errors.New("no snapshots found")
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].CreatedAt.After(snaps[j].CreatedAt)
	})
	return snaps[0], nil
}

// ByID returns the snapshot with the given ID, or an error if not found.
func (s *Store) ByID(id string) (*Snapshot, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("snapshot not found: %s", id)
		}
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *Store) list() ([]*Snapshot, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snaps []*Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		snap, err := s.ByID(id)
		if err != nil {
			continue
		}
		snaps = append(snaps, snap)
	}
	return snaps, nil
}
