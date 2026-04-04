package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/pixeldiff"
)

// ScreenshotDiffEntry holds the result of comparing a single screenshot pair.
type ScreenshotDiffEntry struct {
	URL               string  `json:"url,omitempty"`
	Breakpoint        string  `json:"breakpoint,omitempty"`
	Width             int     `json:"width,omitempty"`
	Height            int     `json:"height,omitempty"`
	BaselinePath      string  `json:"baselinePath"`
	CurrentPath       string  `json:"currentPath"`
	DiffPath          string  `json:"diffPath,omitempty"`
	SimilarityScore   float64 `json:"similarityScore"`
	DiffPixels        int     `json:"diffPixels"`
	TotalPixels       int     `json:"totalPixels"`
	DimensionMismatch bool    `json:"dimensionMismatch,omitempty"`
	Regressed         bool    `json:"regressed"`
	Threshold         float64 `json:"threshold"`
}

// ScreenshotSummary aggregates counts across all compared screenshot pairs.
type ScreenshotSummary struct {
	Total     int `json:"total"`
	Regressed int `json:"regressed"`
	Mismatch  int `json:"mismatch"`
}

// diffResult is the top-level payload emitted by RunDiff.
type diffResult struct {
	Screenshots       []ScreenshotDiffEntry `json:"screenshots"`
	ScreenshotSummary ScreenshotSummary     `json:"screenshotSummary"`
}

// projectConfig holds relevant fields from .ks-project.json.
type projectConfig struct {
	ScreenshotThreshold float64 `json:"screenshotThreshold"`
}

// nonAlphanumRE matches sequences of non-alphanumeric characters for slugification.
var nonAlphanumRE = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// slugifyURL converts a URL into a filesystem-safe slug.
// It strips the scheme, replaces non-alphanumeric chars with hyphens,
// trims leading/trailing hyphens, and limits the result to 64 characters.
func slugifyURL(u string) string {
	// Strip scheme
	for _, scheme := range []string{"https://", "http://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	slug := nonAlphanumRE.ReplaceAllString(u, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}

// resolveThreshold determines the diff threshold using the precedence:
// CLI flag > .ks-project.json > pixeldiff.DefaultOptions().Threshold
func resolveThreshold(args []string) float64 {
	if val := getFlagValue(args, "--screenshot-threshold"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}

	// Try reading .ks-project.json
	data, err := os.ReadFile(".ks-project.json")
	if err == nil {
		var cfg projectConfig
		if jsonErr := json.Unmarshal(data, &cfg); jsonErr == nil && cfg.ScreenshotThreshold > 0 {
			return cfg.ScreenshotThreshold
		}
	}

	return pixeldiff.DefaultOptions().Threshold
}

// RunDiff compares one or more baseline/current screenshot pairs and emits JSON diff results.
//
// Usage:
//
//	ks diff --baseline <path> --current <path> [--output-dir <dir>]
//	        [--url <url>] [--breakpoint <name>]
//	        [--screenshot-threshold <0.0-1.0>]
func RunDiff(args []string) {
	baselinePath := getFlagValue(args, "--baseline")
	currentPath := getFlagValue(args, "--current")
	outputDir := getFlagValue(args, "--output-dir")
	urlLabel := getFlagValue(args, "--url")
	breakpointLabel := getFlagValue(args, "--breakpoint")

	if baselinePath == "" || currentPath == "" {
		output.Fail("diff", errors.New("--baseline and --current flags are required"), "provide paths to two PNG files to compare")
		return
	}

	threshold := resolveThreshold(args)
	if threshStr := getFlagValue(args, "--screenshot-threshold"); threshStr != "" {
		if _, err := strconv.ParseFloat(threshStr, 64); err != nil {
			output.Fail("diff", fmt.Errorf("invalid --screenshot-threshold value %q: %w", threshStr, err), "value must be a float between 0.0 and 1.0")
			return
		}
	}

	opts := pixeldiff.Options{
		Threshold:      threshold,
		PixelTolerance: pixeldiff.DefaultOptions().PixelTolerance,
		HighlightColor: pixeldiff.DefaultOptions().HighlightColor,
	}

	entry, err := compareScreenshotPair(baselinePath, currentPath, outputDir, urlLabel, breakpointLabel, opts)
	if err != nil {
		output.Fail("diff", err, "failed to compare screenshots")
		return
	}

	summary := ScreenshotSummary{Total: 1}
	if entry.Regressed {
		summary.Regressed++
	}
	if entry.DimensionMismatch {
		summary.Mismatch++
	}

	output.Success("diff", diffResult{
		Screenshots:       []ScreenshotDiffEntry{entry},
		ScreenshotSummary: summary,
	})
}

// compareScreenshotPair performs the pixel diff for a single baseline/current pair.
func compareScreenshotPair(baselinePath, currentPath, outputDir, urlLabel, breakpointLabel string, opts pixeldiff.Options) (ScreenshotDiffEntry, error) {
	entry := ScreenshotDiffEntry{
		URL:          urlLabel,
		Breakpoint:   breakpointLabel,
		BaselinePath: baselinePath,
		CurrentPath:  currentPath,
		Threshold:    opts.Threshold,
	}

	// Decode baseline
	bf, err := os.Open(baselinePath)
	if err != nil {
		return entry, fmt.Errorf("opening baseline %q: %w", baselinePath, err)
	}
	defer bf.Close()
	baselineImg, err := png.Decode(bf)
	if err != nil {
		return entry, fmt.Errorf("decoding baseline PNG %q: %w", baselinePath, err)
	}

	// Decode current
	cf, err := os.Open(currentPath)
	if err != nil {
		return entry, fmt.Errorf("opening current %q: %w", currentPath, err)
	}
	defer cf.Close()
	currentImg, err := png.Decode(cf)
	if err != nil {
		return entry, fmt.Errorf("decoding current PNG %q: %w", currentPath, err)
	}

	b := baselineImg.Bounds()
	entry.Width = b.Dx()
	entry.Height = b.Dy()

	result, diffImg := pixeldiff.Compare(baselineImg, currentImg, opts)
	entry.SimilarityScore = result.SimilarityScore
	entry.DiffPixels = result.DiffPixels
	entry.TotalPixels = result.TotalPixels
	entry.DimensionMismatch = result.DimensionMismatch
	entry.Regressed = result.Regressed

	// Write diff PNG when dimensions match
	if !result.DimensionMismatch && diffImg != nil {
		diffPath := buildDiffPath(baselinePath, outputDir, urlLabel, breakpointLabel, b.Dx(), b.Dy())
		if writeErr := pixeldiff.WriteDiffPNG(diffPath, diffImg); writeErr == nil {
			entry.DiffPath = diffPath
		}
		// Non-fatal: if write fails, DiffPath is omitted
	}

	return entry, nil
}

// buildDiffPath constructs the output path for a diff PNG.
// It writes alongside the current screenshot when no output-dir is specified.
func buildDiffPath(currentPath, outputDir, urlLabel, breakpoint string, width, height int) string {
	var filename string
	if urlLabel != "" {
		slug := slugifyURL(urlLabel)
		if breakpoint != "" {
			filename = fmt.Sprintf("diff-%s-%s-%dx%d.png", slug, breakpoint, width, height)
		} else {
			filename = fmt.Sprintf("diff-%s-%dx%d.png", slug, width, height)
		}
	} else if breakpoint != "" {
		filename = fmt.Sprintf("diff-%s-%dx%d.png", breakpoint, width, height)
	} else {
		filename = fmt.Sprintf("diff-%dx%d.png", width, height)
	}

	dir := outputDir
	if dir == "" {
		dir = filepath.Dir(currentPath)
	}
	return filepath.Join(dir, filename)
}
