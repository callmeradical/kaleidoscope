package cmd

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// ScreenshotDiffs aggregates pixel diff results for all screenshot pairs.
type ScreenshotDiffs struct {
	Pairs     []snapshot.ScreenshotDiffResult `json:"pairs"`
	Regressed bool                            `json:"regressed"`
}

// DiffOutput is the top-level JSON payload for the `ks diff` command.
type DiffOutput struct {
	ScreenshotDiff ScreenshotDiffs `json:"screenshotDiff"`
	Regressed      bool            `json:"regressed"`
}

// RunDiff implements the `ks diff` command.
// It compares screenshots listed in a baseline snapshot manifest against a current snapshot
// manifest and emits pixel diff results as JSON.
//
// Flags:
//
//	--baseline <path>    Path to the baseline snapshot manifest JSON file
//	--current  <path>    Path to the current snapshot manifest JSON file
//	--threshold <float>  Similarity threshold (default 0.99); pairs below this are flagged regressed
func RunDiff(args []string) {
	baselinePath := getFlagValue(args, "--baseline")
	currentPath := getFlagValue(args, "--current")
	thresholdStr := getFlagValue(args, "--threshold")

	if baselinePath == "" || currentPath == "" {
		output.Fail("diff", fmt.Errorf("--baseline and --current are required"), "Example: ks diff --baseline baseline.json --current current.json")
		os.Exit(2)
	}

	threshold := 0.99
	if thresholdStr != "" {
		v, err := strconv.ParseFloat(thresholdStr, 64)
		if err != nil {
			output.Fail("diff", fmt.Errorf("invalid --threshold value %q: %w", thresholdStr, err), "--threshold must be a float between 0.0 and 1.0")
			os.Exit(2)
		}
		threshold = v
	}

	cfg := snapshot.DiffConfig{
		SimilarityThreshold: threshold,
		HighlightColor:      color.RGBA{R: 255, G: 0, B: 0, A: 255},
	}

	baseline, err := snapshot.LoadManifest(baselinePath)
	if err != nil {
		output.Fail("diff", err, "Ensure the baseline manifest file exists and is valid JSON")
		os.Exit(2)
	}

	current, err := snapshot.LoadManifest(currentPath)
	if err != nil {
		output.Fail("diff", err, "Ensure the current manifest file exists and is valid JSON")
		os.Exit(2)
	}

	// Build a lookup map from (url, breakpoint) → current screenshot path.
	currentIndex := make(map[string]string, len(current.Screenshots))
	for _, e := range current.Screenshots {
		key := e.URL + "\x00" + e.Breakpoint
		currentIndex[key] = e.Path
	}

	var pairs []snapshot.ScreenshotDiffResult

	for _, base := range baseline.Screenshots {
		key := base.URL + "\x00" + base.Breakpoint
		curPath, found := currentIndex[key]
		if !found {
			pairs = append(pairs, snapshot.ScreenshotDiffResult{
				BaselinePath:    base.Path,
				Regressed:       true,
				SimilarityScore: 0.0,
				Error:           "screenshot missing in current snapshot",
			})
			continue
		}

		// Write diff PNG alongside the current screenshot.
		diffDir := filepath.Dir(curPath)
		result := snapshot.DiffScreenshotFiles(base.Path, curPath, diffDir, cfg)
		pairs = append(pairs, result)
	}

	screenshotDiff := ScreenshotDiffs{Pairs: pairs}
	for _, p := range pairs {
		if p.Regressed {
			screenshotDiff.Regressed = true
			break
		}
	}

	out := DiffOutput{
		ScreenshotDiff: screenshotDiff,
		Regressed:      screenshotDiff.Regressed,
	}

	output.Success("diff", out)
}
