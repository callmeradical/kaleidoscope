package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
)

// validIDPattern restricts snapshot IDs to safe characters only.
var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ScreenshotDiffEntry holds the comparison result for a single screenshot pair.
type ScreenshotDiffEntry struct {
	URL               string   `json:"url"`
	Breakpoint        Breakpoint `json:"breakpoint"`
	BaselinePath      string   `json:"baselinePath"`
	CurrentPath       string   `json:"currentPath"`
	DiffPath          string   `json:"diffPath,omitempty"`
	SimilarityScore   *float64 `json:"similarityScore"`
	ChangedPixels     int      `json:"changedPixels"`
	TotalPixels       int      `json:"totalPixels"`
	DimensionMismatch bool     `json:"dimensionMismatch"`
	Regressed         bool     `json:"regressed"`
}

// Breakpoint represents a viewport width/height pair.
type Breakpoint struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// DiffResult is the top-level JSON output of the diff command.
type DiffResult struct {
	SnapshotID      string                `json:"snapshotID"`
	BaselineID      string                `json:"baselineID"`
	ScreenshotDiffs []ScreenshotDiffEntry `json:"screenshotDiffs"`
	Threshold       float64               `json:"threshold"`
	AnyRegressed    bool                  `json:"anyRegressed"`
}

// baselines is the structure of .kaleidoscope/baselines.json.
type baselines struct {
	DefaultBaseline string `json:"defaultBaseline"`
}

// RunDiff implements the `ks diff` command.
// STUB: not yet fully implemented.
func RunDiff(args []string) {
	snapshotID := getFlagValue(args, "--snapshot")
	baselineID := getFlagValue(args, "--baseline")
	thresholdStr := getFlagValue(args, "--threshold")

	threshold := diff.DefaultSimilarityThreshold
	if thresholdStr != "" {
		var err error
		threshold, err = strconv.ParseFloat(thresholdStr, 64)
		if err != nil {
			output.Fail("diff", fmt.Errorf("invalid --threshold value: %s", thresholdStr), "Provide a float between 0.0 and 1.0")
			os.Exit(2)
		}
	}

	stateDir, err := browser.StateDir()
	if err != nil {
		output.Fail("diff", fmt.Errorf("resolving state directory: %w", err), "")
		os.Exit(2)
	}

	snapshotsDir := filepath.Join(stateDir, "snapshots")

	// Resolve snapshot ID
	if snapshotID == "" || snapshotID == "latest" {
		snapshotID, err = resolveLatestSnapshot(snapshotsDir)
		if err != nil {
			output.Fail("diff", fmt.Errorf("resolving latest snapshot: %w", err), "Run `ks snapshot` to create a snapshot first")
			os.Exit(2)
		}
	}

	// Validate snapshot ID
	if !validIDPattern.MatchString(snapshotID) {
		output.Fail("diff", errors.New("invalid snapshot ID: must match [a-zA-Z0-9_-]"), "")
		os.Exit(2)
	}

	// Resolve baseline ID
	if baselineID == "" {
		baselineID, err = readDefaultBaseline(stateDir)
		if err != nil {
			output.Fail("diff", fmt.Errorf("reading baseline: %w", err), "Set a baseline with `ks baseline set <id>` or pass --baseline <id>")
			os.Exit(2)
		}
	}

	// Validate baseline ID
	if !validIDPattern.MatchString(baselineID) {
		output.Fail("diff", errors.New("invalid baseline ID: must match [a-zA-Z0-9_-]"), "")
		os.Exit(2)
	}

	currentDir := filepath.Join(snapshotsDir, snapshotID, "screenshots")
	baselineDir := filepath.Join(snapshotsDir, baselineID, "screenshots")
	diffsDir := filepath.Join(snapshotsDir, snapshotID, "diffs")

	if err := os.MkdirAll(diffsDir, 0755); err != nil {
		output.Fail("diff", fmt.Errorf("creating diffs directory: %w", err), "")
		os.Exit(2)
	}

	currentFiles, err := listPNGs(currentDir)
	if err != nil && !os.IsNotExist(err) {
		output.Fail("diff", fmt.Errorf("reading current snapshot screenshots: %w", err), "")
		os.Exit(2)
	}

	baselineFiles, err := listPNGs(baselineDir)
	if err != nil && !os.IsNotExist(err) {
		output.Fail("diff", fmt.Errorf("reading baseline screenshots: %w", err), "")
		os.Exit(2)
	}

	// Build filename sets
	currentSet := toSet(currentFiles)
	baselineSet := toSet(baselineFiles)

	var diffs []ScreenshotDiffEntry
	anyRegressed := false

	opts := diff.PixelDiffOptions{Threshold: threshold}

	// Process files in both snapshots
	for name := range currentSet {
		if !baselineSet[name] {
			// Only in current — new screenshot
			bp := parseBreakpoint(name)
			score := (*float64)(nil)
			diffs = append(diffs, ScreenshotDiffEntry{
				Breakpoint:   bp,
				CurrentPath:  filepath.Join(currentDir, name),
				SimilarityScore: score,
			})
			continue
		}

		bp := parseBreakpoint(name)
		diffPNGPath := filepath.Join(diffsDir, strings.TrimSuffix(name, ".png")+"-diff.png")
		opts.OutputPath = diffPNGPath

		res, err := diff.CompareFiles(
			filepath.Join(baselineDir, name),
			filepath.Join(currentDir, name),
			opts,
		)
		if err != nil {
			output.Fail("diff", fmt.Errorf("comparing %s: %w", name, err), "")
			os.Exit(2)
		}

		score := res.SimilarityScore
		entry := ScreenshotDiffEntry{
			Breakpoint:        bp,
			BaselinePath:      filepath.Join(baselineDir, name),
			CurrentPath:       filepath.Join(currentDir, name),
			SimilarityScore:   &score,
			ChangedPixels:     res.ChangedPixels,
			TotalPixels:       res.TotalPixels,
			DimensionMismatch: res.DimensionMismatch,
			Regressed:         res.Regressed,
		}
		if !res.DimensionMismatch && len(res.DiffImageBytes) > 0 {
			entry.DiffPath = diffPNGPath
		}
		if res.Regressed {
			anyRegressed = true
		}
		diffs = append(diffs, entry)
	}

	// Files only in baseline — removed screenshots
	for name := range baselineSet {
		if !currentSet[name] {
			bp := parseBreakpoint(name)
			diffs = append(diffs, ScreenshotDiffEntry{
				Breakpoint:   bp,
				BaselinePath: filepath.Join(baselineDir, name),
				SimilarityScore: nil,
				Regressed:    true,
			})
			anyRegressed = true
		}
	}

	result := DiffResult{
		SnapshotID:      snapshotID,
		BaselineID:      baselineID,
		ScreenshotDiffs: diffs,
		Threshold:       threshold,
		AnyRegressed:    anyRegressed,
	}
	output.Success("diff", result)
}

// resolveLatestSnapshot returns the lexicographically last snapshot directory name.
func resolveLatestSnapshot(snapshotsDir string) (string, error) {
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		return "", err
	}
	var last string
	for _, e := range entries {
		if e.IsDir() && e.Name() > last {
			last = e.Name()
		}
	}
	if last == "" {
		return "", errors.New("no snapshots found")
	}
	return last, nil
}

// readDefaultBaseline reads the defaultBaseline field from baselines.json.
func readDefaultBaseline(stateDir string) (string, error) {
	path := filepath.Join(stateDir, "baselines.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("baselines.json not found; no default baseline configured")
		}
		return "", err
	}
	var b baselines
	if err := json.Unmarshal(data, &b); err != nil {
		return "", fmt.Errorf("parsing baselines.json: %w", err)
	}
	if b.DefaultBaseline == "" {
		return "", errors.New("baselines.json has no defaultBaseline set")
	}
	return b.DefaultBaseline, nil
}

// listPNGs returns PNG filenames (basename only) in the given directory.
func listPNGs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".png") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// toSet converts a slice of strings to a map for O(1) lookup.
func toSet(names []string) map[string]bool {
	s := make(map[string]bool, len(names))
	for _, n := range names {
		s[n] = true
	}
	return s
}

// parseBreakpoint parses a filename like "1280x720.png" into a Breakpoint.
// Returns zero-value Breakpoint on parse failure.
func parseBreakpoint(name string) Breakpoint {
	base := strings.TrimSuffix(name, ".png")
	parts := strings.SplitN(base, "x", 2)
	if len(parts) != 2 {
		return Breakpoint{}
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return Breakpoint{}
	}
	return Breakpoint{Width: w, Height: h}
}
