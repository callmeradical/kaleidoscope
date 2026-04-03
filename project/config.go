package project

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ProjectConfig represents the .ks-project.json configuration file.
type ProjectConfig struct {
	Name        string   `json:"name"`
	URLs        []string `json:"urls"`
	Breakpoints []string `json:"breakpoints"`
}

// LoadConfig reads and JSON-decodes the project config at the given path.
func LoadConfig(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// FindConfig walks up from the current working directory looking for
// .ks-project.json. Returns (nil, nil) if not found.
func FindConfig() (*ProjectConfig, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		candidate := filepath.Join(dir, ".ks-project.json")
		if _, err := os.Stat(candidate); err == nil {
			return LoadConfig(candidate)
		}
		// Stop at git root or filesystem root
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, nil
}
