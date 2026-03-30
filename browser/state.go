package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	WSEndpoint string    `json:"wsEndpoint"`
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"startedAt"`
	CurrentURL string    `json:"currentUrl,omitempty"`
	Viewport   *Viewport `json:"viewport,omitempty"`
}

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// StateDir returns the path to the state directory.
// Checks for .kaleidoscope/ in CWD first, falls back to ~/.kaleidoscope/.
func StateDir() (string, error) {
	// Check project-local first
	local := filepath.Join(".", ".kaleidoscope")
	if info, err := os.Stat(local); err == nil && info.IsDir() {
		return local, nil
	}

	// Fall back to global
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kaleidoscope"), nil
}

// EnsureStateDir creates the state directory if it doesn't exist.
func EnsureStateDir(local bool) (string, error) {
	var dir string
	if local {
		dir = filepath.Join(".", ".kaleidoscope")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".kaleidoscope")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func statePath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

func ReadState() (*State, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func WriteState(s *State) error {
	dir, err := StateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "state.json")
	return os.WriteFile(path, data, 0644)
}

func RemoveState() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// ScreenshotDir returns the screenshot output directory, creating it if needed.
func ScreenshotDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	ssDir := filepath.Join(dir, "screenshots")
	if err := os.MkdirAll(ssDir, 0755); err != nil {
		return "", err
	}
	return ssDir, nil
}
