package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
)

// Baselines maps a URL to the snapshot ID that is its baseline.
type Baselines map[string]string

// LoadBaselines reads baselines.json from the working directory.
// If the file does not exist, it returns an empty Baselines map (not an error).
func LoadBaselines() (Baselines, error) {
	data, err := os.ReadFile("baselines.json")
	if os.IsNotExist(err) {
		return Baselines{}, nil
	}
	if err != nil {
		return nil, err
	}
	var b Baselines
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing baselines.json: %w", err)
	}
	return b, nil
}

// SaveBaselines writes baselines.json atomically.
func SaveBaselines(b Baselines) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp := "baselines.json.tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, "baselines.json")
}

// BaselineFor returns the snapshot ID for the given URL, and whether one exists.
func (b Baselines) BaselineFor(url string) (string, bool) {
	id, ok := b[url]
	return id, ok
}
