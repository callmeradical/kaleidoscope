package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry records which snapshot is the accepted baseline for a URL path.
type Entry struct {
	SnapshotID string    `json:"snapshotId"`
	AcceptedAt time.Time `json:"acceptedAt"`
}

// BaselinesFile is the on-disk representation of baselines.json.
type BaselinesFile struct {
	Version   int               `json:"version"`
	Updated   time.Time         `json:"updated"`
	Baselines map[string]Entry  `json:"baselines"`
}

// Manager reads and writes .kaleidoscope/baselines.json.
type Manager struct {
	path string
}

// NewManager returns a Manager targeting kaleidoscopeDir/baselines.json.
func NewManager(kaleidoscopeDir string) *Manager {
	return &Manager{path: filepath.Join(kaleidoscopeDir, "baselines.json")}
}

// Load reads baselines.json from disk. If the file does not exist it returns
// an empty BaselinesFile (not an error).
func (m *Manager) Load() (*BaselinesFile, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &BaselinesFile{Version: 1, Baselines: map[string]Entry{}}, nil
		}
		return nil, err
	}
	var f BaselinesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Baselines == nil {
		f.Baselines = map[string]Entry{}
	}
	return &f, nil
}

// Save writes f to baselines.json using an atomic rename.
func (m *Manager) Save(f *BaselinesFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// Accept promotes snapshotID as the baseline for the given URL paths.
// Paths already pointing to snapshotID are skipped (idempotent). The updated
// BaselinesFile is returned.
func (m *Manager) Accept(snapshotID string, paths []string) (*BaselinesFile, error) {
	f, err := m.Load()
	if err != nil {
		return nil, err
	}
	changed := false
	for _, p := range paths {
		if existing, ok := f.Baselines[p]; ok && existing.SnapshotID == snapshotID {
			continue // already baseline — no-op
		}
		f.Baselines[p] = Entry{SnapshotID: snapshotID, AcceptedAt: time.Now().UTC()}
		changed = true
	}
	if changed {
		f.Updated = time.Now().UTC()
		if err := m.Save(f); err != nil {
			return nil, err
		}
	}
	return f, nil
}
