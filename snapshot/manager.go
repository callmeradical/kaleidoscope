package snapshot

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/callmeradical/kaleidoscope/browser"
)

// ID is the string identifier for a snapshot (e.g. "1712345678-abc1234").
type ID = string

// Manifest is the root snapshot.json written inside each snapshot directory.
type Manifest struct {
	ID           string       `json:"id"`
	Timestamp    time.Time    `json:"timestamp"`
	CommitHash   string       `json:"commitHash,omitempty"`
	ProjectURLs  []string     `json:"projectURLs"`
	ProjectName  string       `json:"projectName,omitempty"`
	BaseURL      string       `json:"baseURL,omitempty"`
	URLSummaries []URLSummary `json:"urlSummaries"`
}

// URLSummary holds per-URL audit summary data stored in the manifest.
type URLSummary struct {
	URL                string `json:"url"`
	Slug               string `json:"slug"`
	ContrastViolations int    `json:"contrastViolations"`
	TouchViolations    int    `json:"touchViolations"`
	TypographyWarnings int    `json:"typographyWarnings"`
	AXActiveNodes      int    `json:"axActiveNodes"`
	AXTotalNodes       int    `json:"axTotalNodes"`
}

// AuditResult holds the per-URL audit output returned by the internal audit helper.
type AuditResult struct {
	URL                string
	ContrastViolations int
	TouchViolations    int
	TypographyWarnings int
	AXActiveNodes      int
	AXTotalNodes       int
	TotalIssues        int
}

// AXNode represents a single node from the accessibility tree.
type AXNode struct {
	NodeID     string         `json:"nodeId"`
	Role       string         `json:"role"`
	Name       string         `json:"name"`
	Children   []string       `json:"children,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// BaselinesFile is the structure of .kaleidoscope/baselines.json.
type BaselinesFile struct {
	DefaultBaseline string            `json:"defaultBaseline"`
	URLBaselines    map[string]string `json:"urlBaselines,omitempty"`
}

// SnapshotsDir returns the path to the snapshots directory, creating it if needed.
func SnapshotsDir() (string, error) {
	dir, err := browser.StateDir()
	if err != nil {
		return "", err
	}
	snapshotsDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		return "", fmt.Errorf("creating snapshots directory: %w", err)
	}
	return snapshotsDir, nil
}

// SnapshotPath returns the path to a specific snapshot directory, creating it if needed.
func SnapshotPath(id ID) (string, error) {
	dir, err := SnapshotsDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, id)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("creating snapshot directory: %w", err)
	}
	return path, nil
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// URLSlug converts a raw URL into a filesystem-safe slug using host+path.
// Characters outside [a-zA-Z0-9._-] are replaced with underscores.
// The result is capped at 200 characters.
func URLSlug(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "_"
	}
	slug := u.Host + u.Path
	slug = strings.ReplaceAll(slug, "/", "_")
	slug = nonAlnum.ReplaceAllString(slug, "_")
	slug = strings.TrimRight(slug, "_")
	if len(slug) > 200 {
		slug = slug[:200]
	}
	return slug
}

// URLDir returns the per-URL subdirectory inside a snapshot, creating it if needed.
func URLDir(snapshotID ID, slug string) (string, error) {
	snapshotPath, err := SnapshotPath(snapshotID)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(snapshotPath, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating URL directory: %w", err)
	}
	return dir, nil
}

// NewID generates a snapshot ID.
// Format: "<unix_seconds>-<hash>" when hash is non-empty, "<unix_seconds>" otherwise.
func NewID(hash string) ID {
	ts := time.Now().Unix()
	if hash != "" {
		return fmt.Sprintf("%d-%s", ts, hash)
	}
	return fmt.Sprintf("%d", ts)
}

// WriteManifest serialises m to snapshot.json inside the snapshot directory for id.
func WriteManifest(id ID, m *Manifest) error {
	path, err := SnapshotPath(id)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(path, "snapshot.json"), data, 0644)
}

// ReadManifest reads and deserialises snapshot.json from the snapshot directory for id.
func ReadManifest(id ID) (*Manifest, error) {
	path, err := SnapshotPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(path, "snapshot.json"))
	if err != nil {
		return nil, fmt.Errorf("reading snapshot.json: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing snapshot.json: %w", err)
	}
	return &m, nil
}

// ListIDs returns snapshot IDs in descending order (newest first).
func ListIDs() ([]ID, error) {
	dir, err := SnapshotsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ID{}, nil
		}
		return nil, err
	}
	var ids []ID
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, nil
}

// ReadBaselines reads .kaleidoscope/baselines.json.
// Returns nil, nil when the file does not exist.
func ReadBaselines() (*BaselinesFile, error) {
	dir, err := browser.StateDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "baselines.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var b BaselinesFile
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing baselines.json: %w", err)
	}
	return &b, nil
}

// WriteBaselines writes b to .kaleidoscope/baselines.json.
func WriteBaselines(b *BaselinesFile) error {
	dir, err := browser.StateDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling baselines: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "baselines.json"), data, 0644)
}
