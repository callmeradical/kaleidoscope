// Package diff provides pure-Go pixel-level image comparison.
// Uses only standard library packages: image, image/color, image/png, math, os.
package diff

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// DefaultSimilarityThreshold is the default threshold below which a screenshot
// pair is considered regressed.
const DefaultSimilarityThreshold = 0.95

// perPixelTolerance is the Euclidean RGB distance above which a pixel is
// considered changed (range 0–441.67).
const perPixelTolerance = 10.0

// DefaultHighlightColor is the crimson color used to highlight changed pixels.
var DefaultHighlightColor = color.RGBA{R: 255, G: 0, B: 80, A: 255}

// PixelDiffResult holds the outcome of a pixel-level comparison.
type PixelDiffResult struct {
	SimilarityScore   float64
	ChangedPixels     int
	TotalPixels       int
	DiffImagePath     string
	DiffImageBytes    []byte
	DimensionMismatch bool
	Regressed         bool
}

// PixelDiffOptions configures a pixel comparison operation.
type PixelDiffOptions struct {
	Threshold      float64
	HighlightColor color.RGBA
	OutputPath     string
}

// pixelDistance returns the Euclidean RGB distance between two colors (0–441.67).
func pixelDistance(a, b color.Color) float64 {
	ra, ga, ba, _ := color.RGBAModel.Convert(a).RGBA()
	rb, gb, bb, _ := color.RGBAModel.Convert(b).RGBA()
	// RGBA() returns 0-65535; shift right 8 to normalize to 0-255
	r1 := float64(ra >> 8)
	g1 := float64(ga >> 8)
	b1 := float64(ba >> 8)
	r2 := float64(rb >> 8)
	g2 := float64(gb >> 8)
	b2 := float64(bb >> 8)
	return math.Sqrt((r1-r2)*(r1-r2) + (g1-g2)*(g1-g2) + (b1-b2)*(b1-b2))
}

// renderDiff produces a diff image: changed pixels are painted with highlight;
// unchanged pixels are dimmed to 30% opacity against black for context.
func renderDiff(baseline, current image.Image, highlight color.RGBA) *image.RGBA {
	bounds := baseline.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			bp := baseline.At(x, y)
			cp := current.At(x, y)
			if pixelDistance(bp, cp) > perPixelTolerance {
				out.SetRGBA(x, y, highlight)
			} else {
				r, g, b, _ := bp.RGBA()
				out.SetRGBA(x, y, color.RGBA{
					R: uint8(float64(r>>8) * 0.30),
					G: uint8(float64(g>>8) * 0.30),
					B: uint8(float64(b>>8) * 0.30),
					A: 255,
				})
			}
		}
	}
	return out
}

// CompareBytes compares two PNG byte slices and returns a PixelDiffResult.
// On dimension mismatch, returns DimensionMismatch=true with SimilarityScore=0
// rather than an error or panic.
func CompareBytes(baselinePNG, currentPNG []byte, opts PixelDiffOptions) (PixelDiffResult, error) {
	baseImg, err := png.Decode(bytes.NewReader(baselinePNG))
	if err != nil {
		return PixelDiffResult{}, fmt.Errorf("decoding baseline PNG: %w", err)
	}
	currImg, err := png.Decode(bytes.NewReader(currentPNG))
	if err != nil {
		return PixelDiffResult{}, fmt.Errorf("decoding current PNG: %w", err)
	}

	// Apply defaults for zero-value fields
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = DefaultSimilarityThreshold
	}
	highlight := opts.HighlightColor
	if highlight == (color.RGBA{}) {
		highlight = DefaultHighlightColor
	}

	// Dimension mismatch: report as full regression without panic or error
	if baseImg.Bounds() != currImg.Bounds() {
		return PixelDiffResult{
			DimensionMismatch: true,
			SimilarityScore:   0.0,
			Regressed:         true,
		}, nil
	}

	bounds := baseImg.Bounds()
	totalPixels := (bounds.Max.X - bounds.Min.X) * (bounds.Max.Y - bounds.Min.Y)
	changedPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if pixelDistance(baseImg.At(x, y), currImg.At(x, y)) > perPixelTolerance {
				changedPixels++
			}
		}
	}

	similarityScore := 1.0
	if totalPixels > 0 {
		similarityScore = 1.0 - float64(changedPixels)/float64(totalPixels)
	}

	diffImg := renderDiff(baseImg, currImg, highlight)
	var buf bytes.Buffer
	if err := png.Encode(&buf, diffImg); err != nil {
		return PixelDiffResult{}, fmt.Errorf("encoding diff PNG: %w", err)
	}
	diffBytes := buf.Bytes()

	result := PixelDiffResult{
		SimilarityScore: similarityScore,
		ChangedPixels:   changedPixels,
		TotalPixels:     totalPixels,
		DiffImageBytes:  diffBytes,
		Regressed:       similarityScore < threshold,
	}

	if opts.OutputPath != "" {
		if err := os.WriteFile(opts.OutputPath, diffBytes, 0644); err != nil {
			return PixelDiffResult{}, fmt.Errorf("writing diff PNG to %s: %w", opts.OutputPath, err)
		}
		result.DiffImagePath = opts.OutputPath
	}

	return result, nil
}

// CompareFiles reads two PNG files and delegates to CompareBytes.
// If opts.OutputPath is set, the diff PNG is written to that path.
func CompareFiles(baselinePath, currentPath string, opts PixelDiffOptions) (PixelDiffResult, error) {
	baseBytes, err := os.ReadFile(baselinePath)
	if err != nil {
		return PixelDiffResult{}, fmt.Errorf("reading baseline file %s: %w", baselinePath, err)
	}
	currBytes, err := os.ReadFile(currentPath)
	if err != nil {
		return PixelDiffResult{}, fmt.Errorf("reading current file %s: %w", currentPath, err)
	}
	return CompareBytes(baseBytes, currBytes, opts)
}
