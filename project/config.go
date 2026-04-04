package project

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the project configuration loaded from .ks-project.json.
type Config struct {
	Name    string   `json:"name"`
	BaseURL string   `json:"baseURL"`
	URLs    []string `json:"urls"`
}

// Load reads .ks-project.json from the current working directory and returns the parsed Config.
// Returns an error wrapping a hint to run 'ks init' if the file is missing.
// Returns an error if the urls field is empty or the JSON is malformed.
func Load() (*Config, error) {
	data, err := os.ReadFile(".ks-project.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no .ks-project.json found; run 'ks init' first")
		}
		return nil, fmt.Errorf("reading .ks-project.json: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing .ks-project.json: %w", err)
	}
	if len(c.URLs) == 0 {
		return nil, fmt.Errorf("urls field is empty in .ks-project.json")
	}
	return &c, nil
}
