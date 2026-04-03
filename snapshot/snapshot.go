package snapshot

import (
	"encoding/json"
	"os"
	"time"
)

type SnapshotScreenshot struct {
	URL        string `json:"url"`
	Breakpoint string `json:"breakpoint"`
	Path       string `json:"path"`
}

type Snapshot struct {
	ID          string               `json:"id"`
	CapturedAt  time.Time            `json:"capturedAt"`
	Screenshots []SnapshotScreenshot `json:"screenshots"`
}

type ScreenshotDiff struct {
	URL                  string  `json:"url"`
	Breakpoint           string  `json:"breakpoint"`
	BaselinePath         string  `json:"baselinePath"`
	CurrentPath          string  `json:"currentPath"`
	DiffPath             string  `json:"diffPath"`
	Similarity           float64 `json:"similarity"`
	Regressed            bool    `json:"regressed"`
	MismatchedDimensions bool    `json:"mismatchedDimensions"`
}

type DiffOutput struct {
	Baseline            time.Time        `json:"baseline"`
	Current             time.Time        `json:"current"`
	ScreenshotDiffs     []ScreenshotDiff `json:"screenshotDiffs"`
	ScreenshotRegressed bool             `json:"screenshotRegressed"`
	Regressed           bool             `json:"regressed"`
}

func LoadSnapshot(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var s Snapshot
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func SaveSnapshot(path string, s *Snapshot) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
