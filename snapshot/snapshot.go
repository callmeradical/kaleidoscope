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

	"github.com/callmeradical/kaleidoscope/project"
)

// AuditSummary holds aggregate audit counts for a single URL capture.
type AuditSummary struct {
	TotalIssues        int `json:"totalIssues"`
	ContrastViolations int `json:"contrastViolations"`
	TouchViolations    int `json:"touchViolations"`
	TypographyWarnings int `json:"typographyWarnings"`
}

// BreakpointEntry describes a single screenshot captured at one breakpoint.
type BreakpointEntry struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	File   string `json:"file"`
}

// URLEntry holds all captured data for one URL within a snapshot.
type URLEntry struct {
	URL          string            `json:"url"`
	Slug         string            `json:"slug"`
	Breakpoints  []BreakpointEntry `json:"breakpoints"`
	AuditSummary AuditSummary      `json:"auditSummary"`
	Error        string            `json:"error,omitempty"`
}

// Manifest is the root snapshot.json written for each snapshot.
type Manifest struct {
	ID         string         `json:"id"`
	Timestamp  time.Time      `json:"timestamp"`
	CommitHash string         `json:"commitHash,omitempty"`
	Project    project.Config `json:"project"`
	URLs       []URLEntry     `json:"urls"`
}

// Baselines records the snapshot ID chosen as the current baseline.
type Baselines struct {
	SnapshotID string `json:"snapshotId"`
}

// ListEntry is a summary row returned by List.
type ListEntry struct {
	ID         string       `json:"id"`
	Timestamp  time.Time    `json:"timestamp"`
	CommitHash string       `json:"commitHash,omitempty"`
	URLCount   int          `json:"urlCount"`
	Summary    AuditSummary `json:"summary"`
	IsBaseline bool         `json:"isBaseline"`
}

// gitShortHash returns the short git commit hash, or empty string if not in a git repo.
func gitShortHash() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GitShortHash is the exported version for callers outside the package.
func GitShortHash() string {
	return gitShortHash()
}

// NewID generates a snapshot ID in the form <timestamp>[-<short-commit-hash>].
func NewID() string {
	ts := time.Now().UTC().Format("20060102T150405Z")
	hash := gitShortHash()
	if hash != "" {
		return ts + "-" + hash
	}
	return ts
}

// slugify converts a raw URL into a filesystem-safe slug.
func slugify(rawURL string) string {
	// Strip scheme (http://, https://)
	s := rawURL
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	// Replace path separators and port colons with dashes
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	// Strip leading/trailing dashes
	s = strings.Trim(s, "-")
	// Truncate to 80 characters
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// UniqueSlug returns a collision-free slug for rawURL, tracking used slugs in seen.
func UniqueSlug(rawURL string, seen map[string]int) string {
	return uniqueSlug(rawURL, seen)
}

func uniqueSlug(rawURL string, seen map[string]int) string {
	base := slugify(rawURL)
	if _, exists := seen[base]; !exists {
		seen[base] = 1
		return base
	}
	seen[base]++
	candidate := fmt.Sprintf("%s-%d", base, seen[base])
	for {
		if _, exists := seen[candidate]; !exists {
			seen[candidate] = 1
			return candidate
		}
		seen[base]++
		candidate = fmt.Sprintf("%s-%d", base, seen[base])
	}
}

// SnapshotRoot returns the path to .kaleidoscope/snapshots/, creating it if needed.
func SnapshotRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	root := filepath.Join(cwd, ".kaleidoscope", "snapshots")
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", fmt.Errorf("creating snapshot root: %w", err)
	}
	return root, nil
}

// Store writes the manifest as snapshot.json inside <snapshotRoot>/<manifest.ID>/.
func Store(manifest *Manifest) error {
	root, err := SnapshotRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, manifest.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating snapshot dir: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "snapshot.json"), data, 0644)
}

// List reads all snapshots and returns them sorted descending by timestamp.
func List() ([]ListEntry, error) {
	root, err := SnapshotRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot root: %w", err)
	}
	var list []ListEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(root, entry.Name(), "snapshot.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		var summary AuditSummary
		for _, u := range m.URLs {
			summary.TotalIssues += u.AuditSummary.TotalIssues
			summary.ContrastViolations += u.AuditSummary.ContrastViolations
			summary.TouchViolations += u.AuditSummary.TouchViolations
			summary.TypographyWarnings += u.AuditSummary.TypographyWarnings
		}
		list = append(list, ListEntry{
			ID:         m.ID,
			Timestamp:  m.Timestamp,
			CommitHash: m.CommitHash,
			URLCount:   len(m.URLs),
			Summary:    summary,
			IsBaseline: false,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Timestamp.After(list[j].Timestamp)
	})
	return list, nil
}

// LoadBaselines reads .kaleidoscope/baselines.json; returns nil, nil if absent.
func LoadBaselines() (*Baselines, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	path := filepath.Join(cwd, ".kaleidoscope", "baselines.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading baselines: %w", err)
	}
	var b Baselines
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing baselines: %w", err)
	}
	return &b, nil
}

// SaveBaselines writes b to .kaleidoscope/baselines.json.
func SaveBaselines(b *Baselines) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	dir := filepath.Join(cwd, ".kaleidoscope")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating .kaleidoscope dir: %w", err)
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling baselines: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "baselines.json"), data, 0644)
}
