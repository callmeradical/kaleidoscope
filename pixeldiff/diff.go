package pixeldiff

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// Result holds the outcome of a pixel-level image comparison.
type Result struct {
	SimilarityScore   float64 `json:"similarityScore"`
	DiffPixels        int     `json:"diffPixels"`
	TotalPixels       int     `json:"totalPixels"`
	DimensionMismatch bool    `json:"dimensionMismatch,omitempty"`
	Regressed         bool    `json:"regressed"`
}

// Options controls the behavior of the comparison engine.
type Options struct {
	Threshold      float64  // 0.0–1.0; fraction of pixels allowed to differ before flagging as regressed
	PixelTolerance uint8    // 0–255; per-channel difference threshold below which pixels are considered identical
	HighlightColor [4]uint8 // RGBA color used to mark differing pixels in the output image
}

// DefaultOptions returns sensible defaults for screenshot diffing.
func DefaultOptions() Options {
	return Options{
		Threshold:      0.01,
		PixelTolerance: 10,
		HighlightColor: [4]uint8{255, 0, 0, 255},
	}
}

// Compare performs a pixel-level comparison of baseline and current images.
// Returns a Result and a diff image (nil when dimensions mismatch).
func Compare(baseline, current image.Image, opts Options) (Result, *image.RGBA) {
	bb := baseline.Bounds()
	cb := current.Bounds()

	if bb.Size() != cb.Size() {
		return Result{
			DimensionMismatch: true,
			Regressed:         true,
			SimilarityScore:   0.0,
		}, nil
	}

	totalPixels := bb.Dx() * bb.Dy()
	if totalPixels == 0 {
		return Result{
			SimilarityScore: 1.0,
			TotalPixels:     0,
		}, image.NewRGBA(cb)
	}

	diffImg := image.NewRGBA(cb)
	diffPixels := 0

	highlight := color.RGBA{
		R: opts.HighlightColor[0],
		G: opts.HighlightColor[1],
		B: opts.HighlightColor[2],
		A: opts.HighlightColor[3],
	}

	for y := cb.Min.Y; y < cb.Max.Y; y++ {
		for x := cb.Min.X; x < cb.Max.X; x++ {
			cr32, cg32, cb32, ca32 := current.At(x, y).RGBA()
			br32, bg32, bb32, ba32 := baseline.At(x, y).RGBA()

			cr := uint8(cr32 >> 8)
			cg := uint8(cg32 >> 8)
			cb8 := uint8(cb32 >> 8)
			ca := uint8(ca32 >> 8)

			br := uint8(br32 >> 8)
			bg := uint8(bg32 >> 8)
			bb8 := uint8(bb32 >> 8)
			ba := uint8(ba32 >> 8)

			// Copy current pixel as visual context
			diffImg.SetRGBA(x, y, color.RGBA{R: cr, G: cg, B: cb8, A: ca})

			// Compute per-channel absolute differences
			dr := absDiff(cr, br)
			dg := absDiff(cg, bg)
			db := absDiff(cb8, bb8)
			da := absDiff(ca, ba)

			if dr > opts.PixelTolerance || dg > opts.PixelTolerance || db > opts.PixelTolerance || da > opts.PixelTolerance {
				diffImg.SetRGBA(x, y, highlight)
				diffPixels++
			}
		}
	}

	similarityScore := 1.0 - float64(diffPixels)/float64(totalPixels)
	regressed := similarityScore < (1.0 - opts.Threshold)

	return Result{
		SimilarityScore: similarityScore,
		DiffPixels:      diffPixels,
		TotalPixels:     totalPixels,
		Regressed:       regressed,
	}, diffImg
}

// WriteDiffPNG writes a diff image to disk as a PNG file.
func WriteDiffPNG(path string, img *image.RGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func absDiff(a, b uint8) uint8 {
	if a >= b {
		return a - b
	}
	return b - a
}
