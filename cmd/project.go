package cmd

import (
	"encoding/json"
	"errors"
	"os"
)

// ProjectConfig holds the parsed contents of .ks-project.json.
type ProjectConfig struct {
	Version int      `json:"version"`
	URLs    []string `json:"urls"`
}

// ReadProjectConfig reads .ks-project.json from the current working directory.
func ReadProjectConfig() (*ProjectConfig, error) {
	data, err := os.ReadFile(".ks-project.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(".ks-project.json not found in current directory")
		}
		return nil, err
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, errors.New("malformed .ks-project.json: " + err.Error())
	}
	return &cfg, nil
}
