package cmd

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/callmeradical/kaleidoscope/analysis"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// ProjectConfig holds per-project diff settings from .ks-project.json.
type ProjectConfig struct {
	Project             string  `json:"project"`
	ScreenshotThreshold float64 `json:"screenshotThreshold"`
}

// loadProjectConfig reads .ks-project.json in the current working directory.
// Falls back to defaults when the file is absent or unreadable.
func loadProjectConfig() ProjectConfig {
	cfg := ProjectConfig{ScreenshotThreshold: 0.99}
	data, err := os.ReadFile(".ks-project.json")
	if err != nil {
		return cfg
	}
	var parsed ProjectConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return cfg
	}
	if parsed.ScreenshotThreshold == 0 {
		parsed.ScreenshotThreshold = 0.99
	}
	return parsed
}

// loadPNG opens and decodes a PNG file.
func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// findMatchingScreenshot returns the first screenshot matching url+breakpoint, or nil.
func findMatchingScreenshot(screenshots []snapshot.SnapshotScreenshot, url, breakpoint string) *snapshot.SnapshotScreenshot {
	for i := range screenshots {
		ss := &screenshots[i]
		if ss.URL == url && ss.Breakpoint == breakpoint {
			return ss
		}
	}
	return nil
}

// sanitizeDiffName builds a safe filename component from a URL and breakpoint.
func sanitizeDiffName(url, breakpoint string) string {
	// Strip scheme for cleaner names
	name := url
	name = strings.TrimPrefix(name, "https://")
	name = strings.TrimPrefix(name, "http://")
	name = name + "-" + breakpoint
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, name)
}

// diffScreenshots compares baseline vs current screenshots and returns diffs.
func diffScreenshots(baseline, current *snapshot.Snapshot, snapshotDir string, threshold float64) []snapshot.ScreenshotDiff {
	var diffs []snapshot.ScreenshotDiff

	for _, bp := range current.Screenshots {
		baselineSS := findMatchingScreenshot(baseline.Screenshots, bp.URL, bp.Breakpoint)
		if baselineSS == nil {
			// New URL since baseline — skip
			continue
		}

		imgA, errA := loadPNG(baselineSS.Path)
		imgB, errB := loadPNG(bp.Path)

		if errA != nil || errB != nil {
			diffs = append(diffs, snapshot.ScreenshotDiff{
				URL:          bp.URL,
				Breakpoint:   bp.Breakpoint,
				BaselinePath: baselineSS.Path,
				CurrentPath:  bp.Path,
				Similarity:   0.0,
				Regressed:    true,
			})
			continue
		}

		diffResult := analysis.CompareImages(imgA, imgB, 10)

		sd := snapshot.ScreenshotDiff{
			URL:                  bp.URL,
			Breakpoint:           bp.Breakpoint,
			BaselinePath:         baselineSS.Path,
			CurrentPath:          bp.Path,
			Similarity:           diffResult.Similarity,
			MismatchedDimensions: diffResult.MismatchedDimensions,
			Regressed:            diffResult.Similarity < threshold || diffResult.MismatchedDimensions,
		}

		if !diffResult.MismatchedDimensions && diffResult.DiffImage != nil {
			baseName := sanitizeDiffName(bp.URL, bp.Breakpoint)
			diffPath, err := analysis.WriteDiffImage(snapshotDir, baseName, diffResult.DiffImage)
			if err == nil {
				sd.DiffPath = diffPath
			}
		}

		diffs = append(diffs, sd)
	}

	return diffs
}

// RunDiff implements the `ks diff` command.
func RunDiff(args []string) {
	baselinePath := getFlagValue(args, "--baseline")
	currentPath := getFlagValue(args, "--current")

	if baselinePath == "" || currentPath == "" {
		output.Fail("diff", fmt.Errorf("missing required flags"), "Usage: ks diff --baseline <path> --current <path>")
		return
	}

	baseline, err := snapshot.LoadSnapshot(baselinePath)
	if err != nil {
		output.Fail("diff", fmt.Errorf("failed to load baseline: %w", err), "")
		return
	}

	current, err := snapshot.LoadSnapshot(currentPath)
	if err != nil {
		output.Fail("diff", fmt.Errorf("failed to load current: %w", err), "")
		return
	}

	config := loadProjectConfig()
	snapshotDir := filepath.Dir(currentPath)

	screenshotDiffs := diffScreenshots(baseline, current, snapshotDir, config.ScreenshotThreshold)

	screenshotRegressed := false
	for _, sd := range screenshotDiffs {
		if sd.Regressed {
			screenshotRegressed = true
			break
		}
	}

	diffOutput := snapshot.DiffOutput{
		Baseline:            baseline.CapturedAt,
		Current:             current.CapturedAt,
		ScreenshotDiffs:     screenshotDiffs,
		ScreenshotRegressed: screenshotRegressed,
		Regressed:           screenshotRegressed,
	}

	output.Success("diff", diffOutput)
}
