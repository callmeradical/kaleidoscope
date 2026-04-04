package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
)

// DiffOutput is the top-level JSON result for ks diff.
type DiffOutput struct {
	BaselineSnapshotID string               `json:"baselineSnapshotID"`
	CurrentSnapshotID  string               `json:"currentSnapshotID"`
	ScreenshotDiffs    []ScreenshotDiffEntry `json:"screenshotDiffs"`
	AuditDiffs         []interface{}         `json:"auditDiffs"`
	ElementDiffs       []interface{}         `json:"elementDiffs"`
	Threshold          float64               `json:"threshold"`
	HasRegressions     bool                  `json:"hasRegressions"`
}

// ScreenshotDiffEntry wraps a diff result with URL and breakpoint context.
type ScreenshotDiffEntry struct {
	URL        string `json:"url"`
	Breakpoint string `json:"breakpoint"`
	diff.ScreenshotDiffResult
}

type snapshotManifest struct {
	ID          string               `json:"id"`
	CreatedAt   string               `json:"createdAt"`
	Screenshots []snapshotScreenshot `json:"screenshots"`
}

type snapshotScreenshot struct {
	URL        string `json:"url"`
	Breakpoint string `json:"breakpoint"`
	Path       string `json:"path"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type baselinesFile struct {
	CurrentBaseline string `json:"currentBaseline"`
}

var snapshotIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validateSnapshotID(id string) bool {
	return snapshotIDPattern.MatchString(id)
}

func loadSnapshotManifest(snapshotID string) (*snapshotManifest, error) {
	if !validateSnapshotID(snapshotID) {
		return nil, fmt.Errorf("invalid snapshot ID: %q", snapshotID)
	}
	path := filepath.Join(".kaleidoscope", "snapshots", snapshotID, "snapshot.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot manifest: %w", err)
	}
	var m snapshotManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing snapshot manifest: %w", err)
	}
	return &m, nil
}

func resolveBaselineID() (string, error) {
	data, err := os.ReadFile(filepath.Join(".kaleidoscope", "baselines.json"))
	if err != nil {
		return "", fmt.Errorf("reading baselines.json: %w", err)
	}
	var bf baselinesFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return "", fmt.Errorf("parsing baselines.json: %w", err)
	}
	if bf.CurrentBaseline == "" {
		return "", fmt.Errorf("baselines.json: currentBaseline is empty")
	}
	return bf.CurrentBaseline, nil
}

func resolveCurrentSnapshotID() (string, error) {
	dir := filepath.Join(".kaleidoscope", "snapshots")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading snapshots directory: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no snapshots found in %s", dir)
	}
	sort.Strings(names)
	return names[len(names)-1], nil
}

// RunDiff implements the ks diff command.
func RunDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	baseline := fs.String("baseline", "", "Baseline snapshot ID")
	current := fs.String("current", "", "Current snapshot ID")
	threshold := fs.Float64("threshold", 0.99, "Similarity threshold (0.0-1.0)")
	if err := fs.Parse(args); err != nil {
		output.Fail("diff", err, "failed to parse flags")
		return
	}

	// Resolve baseline ID
	baselineID := *baseline
	if baselineID == "" {
		id, err := resolveBaselineID()
		if err != nil {
			output.Fail("diff", err, "could not resolve baseline snapshot")
			return
		}
		baselineID = id
	} else if !validateSnapshotID(baselineID) {
		output.Fail("diff", fmt.Errorf("invalid baseline ID: %q", baselineID), "")
		return
	}

	// Resolve current ID
	currentID := *current
	if currentID == "" {
		id, err := resolveCurrentSnapshotID()
		if err != nil {
			output.Fail("diff", err, "could not resolve current snapshot")
			return
		}
		currentID = id
	} else if !validateSnapshotID(currentID) {
		output.Fail("diff", fmt.Errorf("invalid current ID: %q", currentID), "")
		return
	}

	baselineManifest, err := loadSnapshotManifest(baselineID)
	if err != nil {
		output.Fail("diff", err, "failed to load baseline manifest")
		return
	}

	currentManifest, err := loadSnapshotManifest(currentID)
	if err != nil {
		output.Fail("diff", err, "failed to load current manifest")
		return
	}

	// Build lookup map from baseline
	type key struct{ url, breakpoint string }
	baselineMap := make(map[key]snapshotScreenshot)
	for _, s := range baselineManifest.Screenshots {
		baselineMap[key{s.URL, s.Breakpoint}] = s
	}

	opts := diff.DefaultOptions()
	opts.SimilarityThreshold = *threshold

	currentSnapshotDir := filepath.Join(".kaleidoscope", "snapshots", currentID)

	var entries []ScreenshotDiffEntry
	for _, cs := range currentManifest.Screenshots {
		bs, ok := baselineMap[key{cs.URL, cs.Breakpoint}]
		if !ok {
			continue
		}

		baselineImg, err := diff.LoadPNG(bs.Path)
		if err != nil {
			output.Fail("diff", err, "failed to load baseline image: "+bs.Path)
			return
		}

		currentImg, err := diff.LoadPNG(cs.Path)
		if err != nil {
			output.Fail("diff", err, "failed to load current image: "+cs.Path)
			return
		}

		result, diffImg := diff.CompareImages(baselineImg, currentImg, opts)
		result.BaselinePath = bs.Path
		result.CurrentPath = cs.Path

		diffPath := diff.DiffImagePath(currentSnapshotDir, bs.Path, cs.Path)
		if diffImg != nil {
			if err := diff.SavePNG(diffPath, diffImg); err != nil {
				output.Fail("diff", err, "failed to save diff image")
				return
			}
		}
		result.DiffPath = diffPath

		entries = append(entries, ScreenshotDiffEntry{
			URL:                  cs.URL,
			Breakpoint:           cs.Breakpoint,
			ScreenshotDiffResult: result,
		})
	}

	hasRegressions := false
	for _, e := range entries {
		if e.Regressed {
			hasRegressions = true
			break
		}
	}

	if entries == nil {
		entries = []ScreenshotDiffEntry{}
	}

	output.Success("diff", DiffOutput{
		BaselineSnapshotID: baselineID,
		CurrentSnapshotID:  currentID,
		ScreenshotDiffs:    entries,
		AuditDiffs:         []interface{}{},
		ElementDiffs:       []interface{}{},
		Threshold:          *threshold,
		HasRegressions:     hasRegressions,
	})
}
