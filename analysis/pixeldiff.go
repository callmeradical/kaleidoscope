package analysis

import (
	"image"
	"image/color"
)

// ImageDiffResult holds the result of a pixel-level image comparison.
type ImageDiffResult struct {
	Similarity           float64     `json:"similarity"`
	DifferentPixels      int         `json:"differentPixels"`
	TotalPixels          int         `json:"totalPixels"`
	DiffImage            image.Image `json:"-"`
	MismatchedDimensions bool        `json:"mismatchedDimensions"`
}

// CompareImages compares two images pixel-by-pixel and returns a diff result.
// channelThreshold is the per-channel difference required to count a pixel as different.
// A pixel is flagged as different if any channel diff is strictly greater than channelThreshold.
// Diff image: changed pixels are red (255,0,0,255); unchanged pixels use original color at alpha=128.
// If dimensions are mismatched, returns MismatchedDimensions=true, Similarity=0.0, DiffImage=nil.
func CompareImages(a, b image.Image, channelThreshold uint8) ImageDiffResult {
	boundsA := a.Bounds()
	boundsB := b.Bounds()

	if boundsA.Dx() != boundsB.Dx() || boundsA.Dy() != boundsB.Dy() {
		return ImageDiffResult{
			MismatchedDimensions: true,
			Similarity:           0.0,
		}
	}

	w := boundsA.Dx()
	h := boundsA.Dy()
	total := w * h

	diff := image.NewNRGBA(image.Rect(0, 0, w, h))
	different := 0

	for y := boundsA.Min.Y; y < boundsA.Max.Y; y++ {
		for x := boundsA.Min.X; x < boundsA.Max.X; x++ {
			pa := color.NRGBAModel.Convert(a.At(x, y)).(color.NRGBA)
			pb := color.NRGBAModel.Convert(b.At(x, y)).(color.NRGBA)

			// dx, dy are offsets into the output diff image
			dx := x - boundsA.Min.X
			dy := y - boundsA.Min.Y

			isDiff := absDiff(pa.R, pb.R) > channelThreshold ||
				absDiff(pa.G, pb.G) > channelThreshold ||
				absDiff(pa.B, pb.B) > channelThreshold ||
				absDiff(pa.A, pb.A) > channelThreshold

			if isDiff {
				different++
				diff.SetNRGBA(dx, dy, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
			} else {
				diff.SetNRGBA(dx, dy, color.NRGBA{R: pa.R, G: pa.G, B: pa.B, A: 128})
			}
		}
	}

	var similarity float64
	if total == 0 {
		similarity = 1.0
	} else {
		similarity = float64(total-different) / float64(total)
	}

	return ImageDiffResult{
		Similarity:      similarity,
		DifferentPixels: different,
		TotalPixels:     total,
		DiffImage:       diff,
	}
}

// absDiff returns the absolute difference between two uint8 values.
func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
