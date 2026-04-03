package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// BaselineEntry records the accepted baseline snapshot for one URL.
type BaselineEntry struct {
	URL        string `json:"url"`
	Path       string `json:"path"`
	SnapshotID string `json:"snapshot_id"`
	AcceptedAt string `json:"accepted_at"`
}

// Baselines is the root object stored in baselines.json.
type Baselines struct {
	Entries []BaselineEntry `json:"baselines"`
}

// Load reads baselines.json from dir. Returns empty Baselines if file is absent.
func Load(dir string) (*Baselines, error) {
	path := filepath.Join(dir, "baselines.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Baselines{}, nil
		}
		return nil, err
	}
	var b Baselines
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Save atomically writes baselines.json to dir.
func (b *Baselines) Save(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "baselines-*.json.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, filepath.Join(dir, "baselines.json"))
}

// Accept promotes snapshotID as the baseline for the given URLEntry.
// Returns true if a change was made, false if already up-to-date (idempotent).
func (b *Baselines) Accept(u snapshot.URLEntry, snapshotID string) bool {
	for i := range b.Entries {
		if b.Entries[i].Path == u.Path {
			if b.Entries[i].SnapshotID == snapshotID {
				return false
			}
			b.Entries[i].SnapshotID = snapshotID
			b.Entries[i].AcceptedAt = time.Now().UTC().Format(time.RFC3339)
			return true
		}
	}
	b.Entries = append(b.Entries, BaselineEntry{
		URL:        u.URL,
		Path:       u.Path,
		SnapshotID: snapshotID,
		AcceptedAt: time.Now().UTC().Format(time.RFC3339),
	})
	return true
}

// ForPath returns the baseline entry for the given path, or nil.
func (b *Baselines) ForPath(path string) *BaselineEntry {
	for i := range b.Entries {
		if b.Entries[i].Path == path {
			return &b.Entries[i]
		}
	}
	return nil
}
