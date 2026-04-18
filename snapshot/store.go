// Package snapshot manages the snapshot store under .kaleidoscope/snapshots/.
// Each snapshot captures the full interface state (screenshots at all breakpoints,
// audit results, accessibility tree) for every URL path in the project.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manifest is the root metadata file stored as snapshot.json in each snapshot directory.
type Manifest struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	CommitHash string   `json:"commitHash,omitempty"`
	Project   any       `json:"project"`
	Paths     []string  `json:"paths"`
	Stats     Stats     `json:"stats"`
}

// Stats summarises what was captured in the snapshot.
type Stats struct {
	PathCount       int `json:"pathCount"`
	BreakpointCount int `json:"breakpointCount"`
	ScreenshotCount int `json:"screenshotCount"`
	AuditCount      int `json:"auditCount"`
	AxTreeCount     int `json:"axTreeCount"`
}

// BaselinesFile records which snapshot is the current baseline.
type BaselinesFile struct {
	Current string `json:"current"`
}

// PathResult holds the captured data for a single URL path.
type PathResult struct {
	Path        string
	Screenshots map[string][]byte // breakpoint name -> PNG data
	Audit       any               // audit JSON payload
	AxTree      any               // ax-tree JSON payload
}

// Store manages snapshot persistence.
type Store struct {
	// Root is the .kaleidoscope directory.
	Root string
}

// NewStore creates a store rooted at the given .kaleidoscope directory.
func NewStore(root string) *Store {
	return &Store{Root: root}
}

// snapshotsDir returns the snapshots subdirectory.
func (s *Store) snapshotsDir() string {
	return filepath.Join(s.Root, "snapshots")
}

// baselinesPath returns the path to baselines.json.
func (s *Store) baselinesPath() string {
	return filepath.Join(s.Root, "baselines.json")
}

// GenerateID creates a snapshot ID from the current time and optional git hash.
func GenerateID(ts time.Time, commitHash string) string {
	tsStr := ts.UTC().Format("20060102T150405Z")
	if commitHash != "" {
		return fmt.Sprintf("%s-%s", tsStr, commitHash)
	}
	return tsStr
}

// GitShortHash returns the current short git commit hash, or empty string if
// not in a git repo or git is unavailable.
func GitShortHash() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Create persists a new snapshot with the given results and project config.
// It returns the manifest of the created snapshot.
func (s *Store) Create(ts time.Time, commitHash string, projectCfg any, results []PathResult) (*Manifest, error) {
	id := GenerateID(ts, commitHash)
	dir := filepath.Join(s.snapshotsDir(), id)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating snapshot directory: %w", err)
	}

	paths := make([]string, 0, len(results))
	totalScreenshots := 0
	totalAudits := 0
	totalAxTrees := 0
	breakpointNames := make(map[string]bool)

	for _, pr := range results {
		paths = append(paths, pr.Path)

		// Sanitize path for directory name: "/" -> "_root", "/foo/bar" -> "foo_bar"
		pathDir := sanitizePath(pr.Path)
		pathFullDir := filepath.Join(dir, pathDir)
		if err := os.MkdirAll(pathFullDir, 0755); err != nil {
			return nil, fmt.Errorf("creating path directory %s: %w", pathDir, err)
		}

		// Write screenshots
		for bpName, pngData := range pr.Screenshots {
			breakpointNames[bpName] = true
			filename := fmt.Sprintf("%s.png", bpName)
			if err := os.WriteFile(filepath.Join(pathFullDir, filename), pngData, 0644); err != nil {
				return nil, fmt.Errorf("writing screenshot %s/%s: %w", pathDir, filename, err)
			}
			totalScreenshots++
		}

		// Write audit.json
		if pr.Audit != nil {
			data, err := json.MarshalIndent(pr.Audit, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshaling audit for %s: %w", pr.Path, err)
			}
			if err := os.WriteFile(filepath.Join(pathFullDir, "audit.json"), data, 0644); err != nil {
				return nil, fmt.Errorf("writing audit for %s: %w", pr.Path, err)
			}
			totalAudits++
		}

		// Write ax-tree.json
		if pr.AxTree != nil {
			data, err := json.MarshalIndent(pr.AxTree, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshaling ax-tree for %s: %w", pr.Path, err)
			}
			if err := os.WriteFile(filepath.Join(pathFullDir, "ax-tree.json"), data, 0644); err != nil {
				return nil, fmt.Errorf("writing ax-tree for %s: %w", pr.Path, err)
			}
			totalAxTrees++
		}
	}

	manifest := &Manifest{
		ID:         id,
		Timestamp:  ts,
		CommitHash: commitHash,
		Project:    projectCfg,
		Paths:      paths,
		Stats: Stats{
			PathCount:       len(results),
			BreakpointCount: len(breakpointNames),
			ScreenshotCount: totalScreenshots,
			AuditCount:      totalAudits,
			AxTreeCount:     totalAxTrees,
		},
	}

	// Write snapshot.json manifest
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), data, 0644); err != nil {
		return nil, fmt.Errorf("writing manifest: %w", err)
	}

	return manifest, nil
}

// PromoteBaseline writes or updates baselines.json to point at the given snapshot ID.
func (s *Store) PromoteBaseline(snapshotID string) error {
	bf := BaselinesFile{Current: snapshotID}
	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Root, 0755); err != nil {
		return err
	}
	return os.WriteFile(s.baselinesPath(), append(data, '\n'), 0644)
}

// HasBaseline returns true if baselines.json exists and has a current baseline.
func (s *Store) HasBaseline() bool {
	data, err := os.ReadFile(s.baselinesPath())
	if err != nil {
		return false
	}
	var bf BaselinesFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return false
	}
	return bf.Current != ""
}

// LoadBaseline reads the current baselines.json.
func (s *Store) LoadBaseline() (*BaselinesFile, error) {
	data, err := os.ReadFile(s.baselinesPath())
	if err != nil {
		return nil, err
	}
	var bf BaselinesFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, err
	}
	return &bf, nil
}

// List returns all snapshots in reverse chronological order (newest first).
func (s *Store) List() ([]Manifest, error) {
	snDir := s.snapshotsDir()
	entries, err := os.ReadDir(snDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var manifests []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(snDir, e.Name(), "snapshot.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue // skip malformed snapshot dirs
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		manifests = append(manifests, m)
	}

	// Sort reverse chronological
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Timestamp.After(manifests[j].Timestamp)
	})

	return manifests, nil
}

// Get loads the manifest for a specific snapshot by ID.
func (s *Store) Get(id string) (*Manifest, error) {
	manifestPath := filepath.Join(s.snapshotsDir(), id, "snapshot.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("snapshot %q not found: %w", id, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest for %q: %w", id, err)
	}
	return &m, nil
}

// sanitizePath converts a URL path to a safe directory name.
func sanitizePath(urlPath string) string {
	if urlPath == "/" || urlPath == "" {
		return "_root"
	}
	// Strip leading slash, replace remaining slashes with underscores
	p := strings.TrimPrefix(urlPath, "/")
	p = strings.ReplaceAll(p, "/", "_")
	return p
}
