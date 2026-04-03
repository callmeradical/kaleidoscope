package snapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/callmeradical/kaleidoscope/browser"
)

// Baseline records which snapshot is the current reference baseline.
type Baseline struct {
	SnapshotID string    `json:"snapshotId"`
	PromotedAt time.Time `json:"promotedAt"`
	PromotedBy string    `json:"promotedBy"`
}

// BaselinePath returns the path to .kaleidoscope/baselines.json in the project-local state dir.
func BaselinePath() (string, error) {
	dir, err := browser.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "baselines.json"), nil
}

// ReadBaseline reads the current baseline. Returns (nil, nil) if the file does not exist.
func ReadBaseline() (*Baseline, error) {
	path, err := BaselinePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// WriteBaseline JSON-encodes b and writes it to the baseline path.
func WriteBaseline(b *Baseline) error {
	path, err := BaselinePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// EnsureBaseline promotes snapshotID as the baseline only if no baseline exists.
// Returns (true, nil) if it created a new baseline, (false, nil) if one already existed.
func EnsureBaseline(snapshotID string) (bool, error) {
	existing, err := ReadBaseline()
	if err != nil {
		return false, err
	}
	if existing != nil {
		return false, nil
	}
	b := &Baseline{
		SnapshotID: snapshotID,
		PromotedAt: time.Now(),
		PromotedBy: "auto",
	}
	if err := WriteBaseline(b); err != nil {
		return false, err
	}
	return true, nil
}
