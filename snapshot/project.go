package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadProjectConfig reads and validates .ks-project.json from the current directory.
func LoadProjectConfig() (*ProjectConfig, error) {
	data, err := os.ReadFile(".ks-project.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(".ks-project.json not found — create it with a 'name' and 'urls' array: %w", err)
		}
		return nil, err
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing .ks-project.json: %w", err)
	}
	if len(cfg.URLs) == 0 {
		return nil, fmt.Errorf("no URLs defined in .ks-project.json")
	}
	return &cfg, nil
}
