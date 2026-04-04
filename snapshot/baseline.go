package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/browser"
)

// BaselinePath returns the path to .kaleidoscope/baselines.json.
func BaselinePath() (string, error) {
	stateDir, err := browser.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "baselines.json"), nil
}

// LoadBaseline reads the current baseline pointer.
// Returns (nil, nil) if no baseline file exists yet.
func LoadBaseline() (*Baseline, error) {
	path, err := BaselinePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
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

// SaveBaseline writes the baseline pointer to disk.
func SaveBaseline(b *Baseline) error {
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
