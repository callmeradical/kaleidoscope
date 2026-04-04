package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/callmeradical/kaleidoscope/browser"
)

// Snapshot is a point-in-time capture of one or more pages.
type Snapshot struct {
	ID      string         `json:"id"`
	TakenAt time.Time      `json:"takenAt"`
	Pages   []PageSnapshot `json:"pages"`
}

// PageSnapshot is the captured state for a single URL.
type PageSnapshot struct {
	URL         string           `json:"url"`
	Breakpoints []BreakpointShot `json:"breakpoints"`
	Audit       *AuditSummary    `json:"audit,omitempty"`
	Elements    []ElementRecord  `json:"elements,omitempty"`
}

// BreakpointShot holds screenshot metadata for one viewport breakpoint.
type BreakpointShot struct {
	Name           string `json:"name"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	ScreenshotPath string `json:"screenshotPath"`
}

// AuditSummary stores audit issue counts for delta computation.
type AuditSummary struct {
	ContrastViolations int `json:"contrastViolations"`
	TouchViolations    int `json:"touchViolations"`
	TypographyWarnings int `json:"typographyWarnings"`
	SpacingIssues      int `json:"spacingIssues"`
}

// ElementRecord stores an element's semantic identity and bounding box.
type ElementRecord struct {
	Selector string  `json:"selector"`
	Role     string  `json:"role"`
	Name     string  `json:"name"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}

func snapshotDir() (string, error) {
	stateDir, err := browser.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "snapshots"), nil
}

// LoadBaseline loads the baseline snapshot from baselines.json.
func LoadBaseline() (*Snapshot, error) {
	stateDir, err := browser.StateDir()
	if err != nil {
		return nil, err
	}
	baselinesPath := filepath.Join(stateDir, "baselines.json")
	data, err := os.ReadFile(baselinesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("no baseline set; run: ks snapshot baseline <id>")
		}
		return nil, err
	}
	var b struct {
		Default string `json:"default"`
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("invalid baselines.json: %w", err)
	}
	if b.Default == "" {
		return nil, errors.New("no baseline set; run: ks snapshot baseline <id>")
	}
	return Load(b.Default)
}

// Load loads a snapshot by ID from the snapshot directory.
func Load(id string) (*Snapshot, error) {
	dir, err := snapshotDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("snapshot not found: %s", id)
		}
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("invalid snapshot %s: %w", id, err)
	}
	return &snap, nil
}

// LoadLatest loads the most recently created snapshot.
func LoadLatest() (*Snapshot, error) {
	dir, err := snapshotDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("no snapshots found; run: ks snapshot")
		}
		return nil, err
	}
	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}
	if len(jsonFiles) == 0 {
		return nil, errors.New("no snapshots found; run: ks snapshot")
	}
	sort.Strings(jsonFiles)
	name := jsonFiles[len(jsonFiles)-1]
	id := name[:len(name)-len(filepath.Ext(name))]
	return Load(id)
}
