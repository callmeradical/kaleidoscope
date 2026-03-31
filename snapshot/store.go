package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manifest is the metadata file (snapshot.json) at the root of each snapshot directory.
type Manifest struct {
	ID         string        `json:"id"`
	Timestamp  time.Time     `json:"timestamp"`
	CommitHash string        `json:"commitHash,omitempty"`
	Project    ProjectConfig `json:"project"`
}

// SnapshotsDir returns the base snapshots directory for a given state dir.
func SnapshotsDir(stateDir string) string {
	return filepath.Join(stateDir, "snapshots")
}

// SnapshotPath returns the path to a specific snapshot directory.
func SnapshotPath(stateDir, id string) string {
	return filepath.Join(SnapshotsDir(stateDir), id)
}

// URLToPath converts a URL path like "/dashboard" to a filesystem-safe name like "dashboard".
// The root path "/" becomes "_root".
func URLToPath(urlPath string) string {
	clean := strings.Trim(urlPath, "/")
	if clean == "" {
		return "_root"
	}
	return strings.ReplaceAll(clean, "/", "_")
}

// GenerateID creates a snapshot ID from commit hash and timestamp.
// Format: "<commit>-<unix-ms>" or "snapshot-<unix-ms>" if not in a git repo.
func GenerateID(commitHash string, t time.Time) string {
	ts := fmt.Sprintf("%d", t.UnixMilli())
	if commitHash != "" {
		return commitHash[:min(8, len(commitHash))] + "-" + ts
	}
	return "snapshot-" + ts
}

// WriteManifest writes the snapshot manifest to the snapshot directory.
func WriteManifest(snapDir string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapDir, "snapshot.json"), data, 0644)
}

// ReadManifest reads a snapshot manifest.
func ReadManifest(snapDir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(snapDir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ListSnapshots returns all snapshot manifests sorted by timestamp descending (newest first).
func ListSnapshots(stateDir string) ([]*Manifest, error) {
	dir := SnapshotsDir(stateDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var manifests []*Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := ReadManifest(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		manifests = append(manifests, m)
	}
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Timestamp.After(manifests[j].Timestamp)
	})
	return manifests, nil
}

// LatestSnapshot returns the most recent snapshot manifest, or nil if none exist.
func LatestSnapshot(stateDir string) (*Manifest, error) {
	all, err := ListSnapshots(stateDir)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all[0], nil
}

// WriteAuditJSON saves the audit result JSON to the URL's snapshot subdirectory.
func WriteAuditJSON(snapDir, urlPath string, data []byte) error {
	dir := filepath.Join(snapDir, URLToPath(urlPath))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "audit.json"), data, 0644)
}

// WriteAxTreeJSON saves the accessibility tree JSON.
func WriteAxTreeJSON(snapDir, urlPath string, data []byte) error {
	dir := filepath.Join(snapDir, URLToPath(urlPath))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "ax-tree.json"), data, 0644)
}

// WriteScreenshot saves a breakpoint screenshot.
func WriteScreenshot(snapDir, urlPath, breakpointName string, width, height int, data []byte) (string, error) {
	dir := filepath.Join(snapDir, URLToPath(urlPath))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%s-%dx%d.png", breakpointName, width, height)
	path := filepath.Join(dir, filename)
	return path, os.WriteFile(path, data, 0644)
}

// ReadAuditJSON reads the audit.json for a URL in a snapshot.
func ReadAuditJSON(stateDir, snapshotID, urlPath string) (map[string]any, error) {
	path := filepath.Join(SnapshotPath(stateDir, snapshotID), URLToPath(urlPath), "audit.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ReadAxTreeJSON reads the ax-tree.json for a URL in a snapshot.
func ReadAxTreeJSON(stateDir, snapshotID, urlPath string) (map[string]any, error) {
	path := filepath.Join(SnapshotPath(stateDir, snapshotID), URLToPath(urlPath), "ax-tree.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ScreenshotPath returns the path to a breakpoint screenshot within a snapshot.
func ScreenshotPath(stateDir, snapshotID, urlPath, breakpointName string, width, height int) string {
	filename := fmt.Sprintf("%s-%dx%d.png", breakpointName, width, height)
	return filepath.Join(SnapshotPath(stateDir, snapshotID), URLToPath(urlPath), filename)
}
