package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/callmeradical/kaleidoscope/browser"
)

// stateDirOverride allows tests to redirect the state directory.
var stateDirOverride string

// SetStateDirOverride sets the state directory override for testing.
// Pass an empty string to clear the override.
func SetStateDirOverride(dir string) {
	stateDirOverride = dir
}

func snapshotsDir() (string, error) {
	dir, err := resolveStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "snapshots"), nil
}

func resolveStateDir() (string, error) {
	if stateDirOverride != "" {
		return stateDirOverride, nil
	}
	return browser.StateDir()
}

// ListSnapshots returns all snapshots sorted by CreatedAt ascending.
// Returns an empty slice (not error) when the snapshots directory does not exist.
func ListSnapshots() ([]Snapshot, error) {
	dir, err := snapshotsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Snapshot{}, nil
		}
		return nil, fmt.Errorf("reading snapshots dir: %w", err)
	}

	var snaps []Snapshot
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(dir, e.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue // skip entries without a meta.json
		}
		var s Snapshot
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		snaps = append(snaps, s)
	}

	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].CreatedAt.Before(snaps[j].CreatedAt)
	})

	return snaps, nil
}

// LatestSnapshot returns the most recently created snapshot.
func LatestSnapshot() (*Snapshot, error) {
	snaps, err := ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}
	s := snaps[len(snaps)-1]
	return &s, nil
}

// LatestSnapshotForURL returns the most recent snapshot for the given URL path.
func LatestSnapshotForURL(urlPath string) (*Snapshot, error) {
	snaps, err := ListSnapshots()
	if err != nil {
		return nil, err
	}
	var filtered []Snapshot
	for _, s := range snaps {
		if s.URLPath == urlPath {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no snapshots found for URL: %s", urlPath)
	}
	s := filtered[len(filtered)-1]
	return &s, nil
}

var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// GetSnapshot looks up a snapshot by ID. Returns "invalid snapshot ID" for
// IDs containing path traversal or disallowed characters.
func GetSnapshot(id string) (*Snapshot, error) {
	if !validIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid snapshot ID")
	}
	dir, err := snapshotsDir()
	if err != nil {
		return nil, err
	}
	metaPath := filepath.Join(dir, id, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("snapshot not found: %s", id)
		}
		return nil, fmt.Errorf("reading snapshot %s: %w", id, err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing snapshot %s: %w", id, err)
	}
	return &s, nil
}
