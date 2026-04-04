package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func baselinesPath() (string, error) {
	dir, err := resolveStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "baselines.json"), nil
}

// ReadBaselines loads the baselines map from disk.
// Returns an empty map (not error) if the file does not exist yet.
func ReadBaselines() (Baselines, error) {
	path, err := baselinesPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Baselines{}, nil
		}
		return nil, err
	}
	var b Baselines
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return b, nil
}

// WriteBaselines atomically persists the baselines map to disk.
func WriteBaselines(b Baselines) error {
	path, err := baselinesPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "baselines-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// AcceptSnapshot returns a new Baselines map with the given snapshot promoted.
//
//   - urlPath == "*": update every existing key plus snap.URLPath.
//   - urlPath == "": use snap.URLPath as the target.
//   - otherwise: use urlPath (normalized) as the target.
//
// The bool return value is true when the operation was a no-op (nothing changed).
func AcceptSnapshot(current Baselines, snap *Snapshot, urlPath string) (Baselines, bool, error) {
	// Copy current into updated.
	updated := make(Baselines, len(current))
	for k, v := range current {
		updated[k] = v
	}

	if urlPath == "*" {
		// Update every existing key.
		for k := range updated {
			updated[k] = snap.ID
		}
		// Also ensure the snapshot's own URL path is covered.
		updated[snap.URLPath] = snap.ID
	} else {
		targetPath := urlPath
		if targetPath == "" {
			targetPath = snap.URLPath
		}
		// Normalize: must start with /, strip trailing / (unless root).
		if len(targetPath) > 0 && targetPath[0] != '/' {
			targetPath = "/" + targetPath
		}
		if len(targetPath) > 1 {
			targetPath = strings.TrimRight(targetPath, "/")
		}
		updated[targetPath] = snap.ID
	}

	wasNoOp := mapsEqual(current, updated)
	return updated, wasNoOp, nil
}

func mapsEqual(a, b Baselines) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
