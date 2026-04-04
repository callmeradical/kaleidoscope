// Package snapshot provides pixel-level screenshot comparison and snapshot manifest management.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
)

// ScreenshotEntry represents a single screenshot captured at a specific URL and breakpoint.
type ScreenshotEntry struct {
	URL        string `json:"url"`
	Breakpoint string `json:"breakpoint"`
	Path       string `json:"path"`
}

// SnapshotManifest holds metadata for a set of captured screenshots.
type SnapshotManifest struct {
	Screenshots []ScreenshotEntry `json:"screenshots"`
}

// LoadManifest reads and parses a JSON snapshot manifest from the given file path.
func LoadManifest(path string) (*SnapshotManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m SnapshotManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return &m, nil
}
