package snapshot

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/callmeradical/kaleidoscope/browser"
)

// SnapshotsDir returns the path to the snapshots directory, creating it if needed.
func SnapshotsDir() (string, error) {
	dir, err := browser.StateDir()
	if err != nil {
		return "", err
	}
	ssDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(ssDir, 0755); err != nil {
		return "", err
	}
	return ssDir, nil
}

// GenerateID produces a snapshot ID in the format <unix-ms>[-<short-git-hash>].
// Outside a git repo, returns timestamp-only format.
func GenerateID() string {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ts
	}
	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return ts
	}
	return ts + "-" + hash
}

// CreateDir creates the snapshot directory for the given ID and returns its path.
func CreateDir(id string) (string, error) {
	dir, err := SnapshotsDir()
	if err != nil {
		return "", err
	}
	snapshotDir := filepath.Join(dir, id)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return "", err
	}
	return snapshotDir, nil
}

// URLDir converts a raw URL into a safe directory name component.
// Returns "root" for "/" and sanitises non-alphanumeric characters to underscores.
func URLDir(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "root"
	}
	// Only use path component (strips query/fragment)
	path := u.Path
	// Strip leading slash
	path = strings.TrimPrefix(path, "/")
	// Replace non-[a-zA-Z0-9_-] chars with underscore
	var sb strings.Builder
	for _, c := range path {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			sb.WriteRune(c)
		} else {
			sb.WriteRune('_')
		}
	}
	result := sb.String()
	// Collapse consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	// Trim leading/trailing underscores
	result = strings.Trim(result, "_")
	if result == "" {
		return "root"
	}
	return result
}

// WriteManifest JSON-encodes m and writes it to <snapshotDir>/snapshot.json.
func WriteManifest(snapshotDir string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotDir, "snapshot.json"), data, 0644)
}

// ReadManifest reads and JSON-decodes <snapshotDir>/snapshot.json.
func ReadManifest(snapshotDir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(snapshotDir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// List returns all snapshot manifests sorted by timestamp descending.
func List() ([]*Manifest, error) {
	dir, err := SnapshotsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
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
