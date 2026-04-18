package diff

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// ScreenshotDiffResult holds the comparison result for a single screenshot pair.
type ScreenshotDiffResult struct {
	Breakpoint     string  `json:"breakpoint"`
	Similarity     float64 `json:"similarity"`     // 0.0–1.0
	DiffImagePath  string  `json:"diffImagePath"`  // path to the generated diff PNG
	DiffPixelCount int     `json:"diffPixelCount"` // number of changed pixels
	TotalPixels    int     `json:"totalPixels"`
	DimensionMatch bool    `json:"dimensionMatch"` // false = full regression
}

// ScreenshotDiffConfig controls diff sensitivity.
type ScreenshotDiffConfig struct {
	// PixelThreshold is the minimum per-channel delta (0–255) to consider a
	// pixel as changed. Defaults to 0 (exact match).
	PixelThreshold uint8
}

// highlightColor is the overlay color used to mark changed pixels in the diff image.
var highlightColor = color.NRGBA{R: 255, G: 0, B: 77, A: 180}

// CompareScreenshots compares two PNG images and returns a similarity score and
// diff image. If dimensions differ, similarity is 0.0 and DimensionMatch is false.
func CompareScreenshots(baselinePath, currentPath string, cfg ScreenshotDiffConfig) (*ScreenshotDiffResult, image.Image, error) {
	baseImg, err := loadPNG(baselinePath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading baseline %s: %w", baselinePath, err)
	}
	currImg, err := loadPNG(currentPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading current %s: %w", currentPath, err)
	}

	baseBounds := baseImg.Bounds()
	currBounds := currImg.Bounds()

	// Dimension mismatch → full regression
	if baseBounds.Dx() != currBounds.Dx() || baseBounds.Dy() != currBounds.Dy() {
		// Build a simple diff image at the max dimensions showing the current image
		// with a red tint to indicate complete mismatch.
		w := max(baseBounds.Dx(), currBounds.Dx())
		h := max(baseBounds.Dy(), currBounds.Dy())
		diffImg := image.NewNRGBA(image.Rect(0, 0, w, h))
		// Fill with highlight color
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				diffImg.SetNRGBA(x, y, highlightColor)
			}
		}
		return &ScreenshotDiffResult{
			Similarity:     0.0,
			DiffPixelCount: w * h,
			TotalPixels:    w * h,
			DimensionMatch: false,
		}, diffImg, nil
	}

	w := baseBounds.Dx()
	h := baseBounds.Dy()
	totalPixels := w * h

	// Create diff image: semi-transparent overlay of current with changed pixels highlighted
	diffImg := image.NewNRGBA(image.Rect(0, 0, w, h))
	diffCount := 0
	threshold := int(cfg.PixelThreshold)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			br, bg, bb, ba := baseImg.At(baseBounds.Min.X+x, baseBounds.Min.Y+y).RGBA()
			cr, cg, cb, ca := currImg.At(currBounds.Min.X+x, currBounds.Min.Y+y).RGBA()

			// Convert from 16-bit to 8-bit for comparison
			dr := absDiff16(br, cr)
			dg := absDiff16(bg, cg)
			db := absDiff16(bb, cb)
			da := absDiff16(ba, ca)

			if dr > threshold || dg > threshold || db > threshold || da > threshold {
				diffCount++
				// Blend: dim the current pixel and overlay highlight
				diffImg.SetNRGBA(x, y, blendHighlight(
					uint8(cr>>8), uint8(cg>>8), uint8(cb>>8), uint8(ca>>8),
				))
			} else {
				// Unchanged: show dimmed version of current
				diffImg.SetNRGBA(x, y, color.NRGBA{
					R: uint8(cr >> 8) / 2,
					G: uint8(cg >> 8) / 2,
					B: uint8(cb >> 8) / 2,
					A: 255,
				})
			}
		}
	}

	similarity := 1.0
	if totalPixels > 0 {
		similarity = 1.0 - float64(diffCount)/float64(totalPixels)
	}

	return &ScreenshotDiffResult{
		Similarity:     math.Round(similarity*10000) / 10000, // 4 decimal places
		DiffPixelCount: diffCount,
		TotalPixels:    totalPixels,
		DimensionMatch: true,
	}, diffImg, nil
}

// WriteDiffImage saves a diff image as PNG to the given path, creating
// directories as needed.
func WriteDiffImage(img image.Image, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// loadPNG reads and decodes a PNG file.
func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// absDiff16 returns the absolute difference of two 16-bit color values,
// scaled to 8-bit range.
func absDiff16(a, b uint32) int {
	a8 := int(a >> 8)
	b8 := int(b >> 8)
	d := a8 - b8
	if d < 0 {
		return -d
	}
	return d
}

// blendHighlight blends a pixel with the highlight color.
func blendHighlight(r, g, b, a uint8) color.NRGBA {
	hA := float64(highlightColor.A) / 255.0
	return color.NRGBA{
		R: uint8(float64(r)*(1-hA) + float64(highlightColor.R)*hA),
		G: uint8(float64(g)*(1-hA) + float64(highlightColor.G)*hA),
		B: uint8(float64(b)*(1-hA) + float64(highlightColor.B)*hA),
		A: 255,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
