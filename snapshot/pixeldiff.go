// Package snapshot provides pixel-level screenshot comparison using only Go standard library.
// No external dependencies: uses image, image/color, image/draw, image/png from stdlib.
package snapshot

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// ErrDimensionMismatch is returned by DiffImages when the two images have different bounds.
var ErrDimensionMismatch = errors.New("images have different dimensions")

// DiffConfig controls pixel diff behaviour.
type DiffConfig struct {
	// SimilarityThreshold is the minimum score (0.0–1.0) below which a pair is flagged regressed.
	// Default: 0.99
	SimilarityThreshold float64
	// HighlightColor is the RGBA colour painted over changed pixels in the diff image.
	// Default: {255, 0, 0, 255} (opaque red)
	HighlightColor color.RGBA
}

// DefaultDiffConfig returns a DiffConfig with sensible defaults.
func DefaultDiffConfig() DiffConfig {
	return DiffConfig{
		SimilarityThreshold: 0.99,
		HighlightColor:      color.RGBA{R: 255, G: 0, B: 0, A: 255},
	}
}

// ScreenshotDiffResult holds the comparison outcome for a single screenshot pair.
type ScreenshotDiffResult struct {
	BaselinePath      string  `json:"baselinePath"`
	CurrentPath       string  `json:"currentPath"`
	DiffPath          string  `json:"diffPath,omitempty"`
	SimilarityScore   float64 `json:"similarityScore"`
	Regressed         bool    `json:"regressed"`
	DimensionMismatch bool    `json:"dimensionMismatch,omitempty"`
	Error             string  `json:"error,omitempty"`
}

// LoadPNG opens and decodes a PNG file.
// Note: large screenshots (>20 MP) are decoded fully into memory.
func LoadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

// SavePNG encodes img as PNG and writes it to path, creating parent directories as needed.
func SavePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}

// DiffImages compares baseline and current pixel-by-pixel and returns:
//   - diff: an RGBA image showing current pixels with changed pixels highlighted
//   - score: similarity ratio in [0.0, 1.0] (1.0 = identical)
//   - err: ErrDimensionMismatch if bounds differ, or nil
//
// Pixels whose per-channel manhattan distance exceeds 10 are considered changed.
func DiffImages(baseline, current image.Image, cfg DiffConfig) (diff *image.RGBA, score float64, err error) {
	bb := baseline.Bounds()
	cb := current.Bounds()
	if bb != cb {
		return nil, 0.0, ErrDimensionMismatch
	}

	bounds := bb
	totalPixels := bounds.Dx() * bounds.Dy()
	if totalPixels == 0 {
		return image.NewRGBA(bounds), 1.0, nil
	}

	diff = image.NewRGBA(bounds)
	// Seed diff with current image so unchanged areas show the real UI.
	draw.Draw(diff, bounds, current, bounds.Min, draw.Src)

	changedPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			br, bg, bb8, _ := baseline.At(x, y).RGBA()
			cr, cg, cb8, _ := current.At(x, y).RGBA()

			// RGBA() returns values in [0, 65535]; scale to [0, 255].
			delta := absDiff(br>>8, cr>>8) + absDiff(bg>>8, cg>>8) + absDiff(bb8>>8, cb8>>8)
			if delta > 10 {
				changedPixels++
				diff.SetRGBA(x, y, cfg.HighlightColor)
			}
		}
	}

	score = 1.0 - float64(changedPixels)/float64(totalPixels)
	return diff, score, nil
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// DiffScreenshotFiles loads two PNG files, computes a pixel diff, writes the diff PNG to
// diffDir, and returns a ScreenshotDiffResult. All errors are captured in the result; this
// function never panics or returns a Go error.
func DiffScreenshotFiles(baselinePath, currentPath, diffDir string, cfg DiffConfig) ScreenshotDiffResult {
	result := ScreenshotDiffResult{
		BaselinePath: baselinePath,
		CurrentPath:  currentPath,
	}

	baseline, err := LoadPNG(baselinePath)
	if err != nil {
		result.Error = err.Error()
		result.Regressed = true
		return result
	}

	current, err := LoadPNG(currentPath)
	if err != nil {
		result.Error = err.Error()
		result.Regressed = true
		return result
	}

	diffImg, score, err := DiffImages(baseline, current, cfg)
	if err != nil {
		if errors.Is(err, ErrDimensionMismatch) {
			result.DimensionMismatch = true
			result.SimilarityScore = 0.0
			result.Regressed = true
			return result
		}
		result.Error = err.Error()
		result.Regressed = true
		return result
	}

	result.SimilarityScore = score

	diffFilename := fmt.Sprintf("diff_%d.png", time.Now().Unix())
	diffPath := filepath.Join(diffDir, diffFilename)
	if err := SavePNG(diffPath, diffImg); err != nil {
		result.Error = err.Error()
		result.Regressed = true
		return result
	}
	result.DiffPath = diffPath
	result.Regressed = score < cfg.SimilarityThreshold
	return result
}
