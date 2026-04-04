package diff

import (
	"image"
	"image/color"
)

// ScreenshotDiffResult holds the outcome of comparing two screenshot images.
type ScreenshotDiffResult struct {
	BaselinePath      string  `json:"baselinePath"`
	CurrentPath       string  `json:"currentPath"`
	DiffPath          string  `json:"diffPath"`
	Similarity        float64 `json:"similarity"`
	Regressed         bool    `json:"regressed"`
	DimensionMismatch bool    `json:"dimensionMismatch,omitempty"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
}

// Options controls how image comparison is performed.
type Options struct {
	Threshold           uint8
	SimilarityThreshold float64
	HighlightColor      [4]uint8
}

// DefaultOptions returns sensible defaults for image comparison.
func DefaultOptions() Options {
	return Options{
		Threshold:           10,
		SimilarityThreshold: 0.99,
		HighlightColor:      [4]uint8{255, 0, 0, 255},
	}
}

// CompareImages performs a pixel-level comparison of baseline and current images.
// Returns the diff result and a diff image (nil if dimensions mismatch).
func CompareImages(baseline, current image.Image, opts Options) (ScreenshotDiffResult, image.Image) {
	bb := baseline.Bounds()
	cb := current.Bounds()

	// Step A — dimension check
	if bb != cb {
		return ScreenshotDiffResult{
			DimensionMismatch: true,
			Similarity:        0.0,
			Regressed:         true,
			Width:             bb.Dx(),
			Height:            bb.Dy(),
		}, nil
	}

	w := bb.Dx()
	h := bb.Dy()
	totalPixels := w * h

	// Step B — initialize diff image
	diffImg := image.NewRGBA(bb)

	// Step C — pixel iteration
	diffCount := 0
	highlight := color.RGBA{
		R: opts.HighlightColor[0],
		G: opts.HighlightColor[1],
		B: opts.HighlightColor[2],
		A: opts.HighlightColor[3],
	}

	for y := bb.Min.Y; y < bb.Max.Y; y++ {
		for x := bb.Min.X; x < bb.Max.X; x++ {
			br, bg, bb2, ba := baseline.At(x, y).RGBA()
			cr, cg, cb2, _ := current.At(x, y).RGBA()

			// shift from 16-bit to 8-bit
			bR, bG, bB := uint8(br>>8), uint8(bg>>8), uint8(bb2>>8)
			cR, cG, cB := uint8(cr>>8), uint8(cg>>8), uint8(cb2>>8)

			dR := absDiff(bR, cR)
			dG := absDiff(bG, cG)
			dB := absDiff(bB, cB)

			if dR > opts.Threshold || dG > opts.Threshold || dB > opts.Threshold {
				diffCount++
				diffImg.SetRGBA(x, y, highlight)
			} else {
				// copy baseline pixel
				diffImg.SetRGBA(x, y, color.RGBA{R: bR, G: bG, B: bB, A: uint8(ba >> 8)})
			}
		}
	}

	// Step D — similarity
	var similarity float64
	if totalPixels == 0 {
		similarity = 1.0
	} else {
		similarity = 1.0 - float64(diffCount)/float64(totalPixels)
	}

	// Step E — regression flag
	regressed := similarity < opts.SimilarityThreshold

	return ScreenshotDiffResult{
		Similarity: similarity,
		Regressed:  regressed,
		Width:      w,
		Height:     h,
	}, diffImg
}

func absDiff(a, b uint8) uint8 {
	if a >= b {
		return a - b
	}
	return b - a
}
