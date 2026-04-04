package snapshot

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SnapshotsDir returns the path to the snapshots directory.
func SnapshotsDir() string {
	return filepath.Join(".kaleidoscope", "snapshots")
}

// BaselinesFile returns the path to baselines.json.
func BaselinesFile() string {
	return filepath.Join(".kaleidoscope", "baselines.json")
}

// sanitizeID rejects snapshot IDs containing path traversal characters.
func sanitizeID(id string) error {
	if strings.Contains(id, "/") {
		return fmt.Errorf("snapshot ID must not contain '/'")
	}
	if strings.Contains(id, "\\") {
		return fmt.Errorf("snapshot ID must not contain '\\'")
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("snapshot ID must not contain '..'")
	}
	if strings.ContainsRune(id, 0) {
		return fmt.Errorf("snapshot ID must not contain null bytes")
	}
	return nil
}

// snapshotPath returns the path to a snapshot directory, rejecting traversal.
func snapshotPath(id string) (string, error) {
	if err := sanitizeID(id); err != nil {
		return "", err
	}
	base := filepath.Clean(SnapshotsDir())
	p := filepath.Clean(filepath.Join(base, id))
	if !strings.HasPrefix(p, base+string(filepath.Separator)) && p != base {
		return "", fmt.Errorf("snapshot ID %q escapes snapshot directory", id)
	}
	return p, nil
}
