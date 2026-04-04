package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/browser"
)

// BaselineEntry maps a URL to the snapshot that is its current baseline.
type BaselineEntry struct {
	URL        string `json:"url"`
	SnapshotID string `json:"snapshotId"`
}

// Baselines holds the full set of baseline entries, persisted in baselines.json.
type Baselines struct {
	Entries []BaselineEntry `json:"baselines"`
}

// LoadBaselines reads .kaleidoscope/baselines.json.
// If the file does not exist it returns an empty Baselines (not an error).
func LoadBaselines() (*Baselines, error) {
	dir, err := browser.StateDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "baselines.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Baselines{Entries: []BaselineEntry{}}, nil
		}
		return nil, err
	}
	var b Baselines
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	if b.Entries == nil {
		b.Entries = []BaselineEntry{}
	}
	return &b, nil
}

// SaveBaselines writes b to .kaleidoscope/baselines.json with 2-space indentation.
func SaveBaselines(b *Baselines) error {
	dir, err := browser.StateDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "baselines.json")
	return os.WriteFile(path, data, 0644)
}

// Accept promotes meta as the baseline for the given urls (or all meta.URLs if urls is nil/empty).
// It is a pure function: current is never mutated. Returns the updated Baselines and the list
// of URLs whose baseline actually changed (empty if this was a no-op).
func Accept(current *Baselines, meta *SnapshotMeta, urls []string) (updated *Baselines, changed []string) {
	if len(urls) == 0 {
		urls = meta.URLs
	}

	// Deep-copy entries so we don't mutate current.
	entries := make([]BaselineEntry, len(current.Entries))
	copy(entries, current.Entries)
	updated = &Baselines{Entries: entries}
	changed = []string{}

	for _, url := range urls {
		found := false
		for i := range updated.Entries {
			if updated.Entries[i].URL == url {
				found = true
				if updated.Entries[i].SnapshotID != meta.ID {
					updated.Entries[i].SnapshotID = meta.ID
					changed = append(changed, url)
				}
				break
			}
		}
		if !found {
			updated.Entries = append(updated.Entries, BaselineEntry{URL: url, SnapshotID: meta.ID})
			changed = append(changed, url)
		}
	}

	return updated, changed
}
