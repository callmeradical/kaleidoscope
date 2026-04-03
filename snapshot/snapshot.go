package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AXNode represents a node in the accessibility tree.
type AXNode struct {
	Role     string            `json:"role"`
	Name     string            `json:"name"`
	Selector string            `json:"selector,omitempty"`
	Props    map[string]string `json:"props,omitempty"`
	Children []AXNode          `json:"children,omitempty"`
}

// AuditSummary holds per-category issue counts captured during a snapshot.
type AuditSummary struct {
	ContrastViolations int `json:"contrastViolations"`
	TouchViolations    int `json:"touchViolations"`
	TypographyWarnings int `json:"typographyWarnings"`
	SpacingIssues      int `json:"spacingIssues"`
}

// BreakpointSnapshot holds data captured at a single viewport breakpoint.
type BreakpointSnapshot struct {
	Name        string       `json:"name"`
	Width       int          `json:"width"`
	Height      int          `json:"height"`
	ImagePath   string       `json:"imagePath"`
	AuditResult AuditSummary `json:"auditResult"`
	AXNodes     []AXNode     `json:"axNodes,omitempty"`
}

// URLSnapshot holds all breakpoint data for a single URL.
type URLSnapshot struct {
	URL         string               `json:"url"`
	CapturedAt  time.Time            `json:"capturedAt"`
	Breakpoints []BreakpointSnapshot `json:"breakpoints"`
}

// Snapshot is a complete point-in-time capture of one or more URLs.
type Snapshot struct {
	ID          string        `json:"id"`
	CapturedAt  time.Time     `json:"capturedAt"`
	URLs        []URLSnapshot `json:"urls"`
}

// Load reads a snapshot from stateDir by ID. Pass "latest" to resolve the
// most-recently written snapshot.
func Load(stateDir, id string) (*Snapshot, error) {
	if id == "latest" {
		resolved, err := resolveLatest(stateDir)
		if err != nil {
			return nil, fmt.Errorf("resolving latest snapshot: %w", err)
		}
		id = resolved
	}
	path := filepath.Join(stateDir, "snapshots", id, "snapshot.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot %s: %w", id, err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing snapshot %s: %w", id, err)
	}
	return &s, nil
}

// LoadBaseline reads the baselines.json file and returns a map of url → snapshot-id.
func LoadBaseline(stateDir string) (map[string]string, error) {
	path := filepath.Join(stateDir, "baselines.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading baselines: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing baselines: %w", err)
	}
	return m, nil
}

// resolveLatest finds the most-recently created snapshot directory.
func resolveLatest(stateDir string) (string, error) {
	snapshotsDir := filepath.Join(stateDir, "snapshots")
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		return "", fmt.Errorf("no snapshots found in %s: %w", snapshotsDir, err)
	}
	var latest string
	var latestTime time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = e.Name()
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no snapshots found in %s", snapshotsDir)
	}
	return latest, nil
}
