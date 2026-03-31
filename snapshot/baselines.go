package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Baselines maps URL path -> snapshot ID (the accepted baseline).
type Baselines map[string]string

const BaselinesFile = "baselines.json"

// BaselinesPath returns the path to baselines.json in the state directory.
func BaselinesPath(stateDir string) string {
	return filepath.Join(stateDir, BaselinesFile)
}

// LoadBaselines reads baselines.json. Returns empty map if file doesn't exist.
func LoadBaselines(stateDir string) (Baselines, error) {
	data, err := os.ReadFile(BaselinesPath(stateDir))
	if os.IsNotExist(err) {
		return Baselines{}, nil
	}
	if err != nil {
		return nil, err
	}
	var b Baselines
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return b, nil
}

// SaveBaselines writes baselines.json to the state directory.
func SaveBaselines(stateDir string, b Baselines) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(BaselinesPath(stateDir), data, 0644)
}
