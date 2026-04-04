package diff

import (
	"image"
	"image/color"
)

// Options configures the pixel diff overlay behavior.
type Options struct {
	Threshold      uint8
	HighlightColor color.RGBA
	DimOpacity     float64
}

func defaultOptions(opts *Options) *Options {
	if opts == nil {
		opts = &Options{}
	}
	if opts.Threshold == 0 {
		opts.Threshold = 10
	}
	if opts.HighlightColor == (color.RGBA{}) {
		opts.HighlightColor = color.RGBA{R: 255, G: 0, B: 0, A: 255}
	}
	if opts.DimOpacity == 0 {
		opts.DimOpacity = 0.5
	}
	return opts
}

// pixelAt returns the 8-bit RGBA values of the pixel at (x, y), or zeros if out of bounds.
func pixelAt(img image.Image, x, y int) (r, g, b, a uint8) {
	if !(image.Point{X: x, Y: y}.In(img.Bounds())) {
		return 0, 0, 0, 0
	}
	r32, g32, b32, a32 := img.At(x, y).RGBA()
	return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8), uint8(a32 >> 8)
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func maxChannelDelta(r1, g1, b1, a1, r2, g2, b2, a2 uint8) uint8 {
	d := absDiff(r1, r2)
	if v := absDiff(g1, g2); v > d {
		d = v
	}
	if v := absDiff(b1, b2); v > d {
		d = v
	}
	if v := absDiff(a1, a2); v > d {
		d = v
	}
	return d
}

func unionBounds(b1, b2 image.Rectangle) image.Rectangle {
	minX, minY := b1.Min.X, b1.Min.Y
	maxX, maxY := b1.Max.X, b1.Max.Y
	if b2.Min.X < minX {
		minX = b2.Min.X
	}
	if b2.Min.Y < minY {
		minY = b2.Min.Y
	}
	if b2.Max.X > maxX {
		maxX = b2.Max.X
	}
	if b2.Max.Y > maxY {
		maxY = b2.Max.Y
	}
	return image.Rect(minX, minY, maxX, maxY)
}

// Overlay returns an image highlighting pixels that differ between img1 and img2.
// Different pixels are set to opts.HighlightColor; similar pixels are dimmed from img1.
func Overlay(img1, img2 image.Image, opts *Options) image.Image {
	opts = defaultOptions(opts)
	bounds := unionBounds(img1.Bounds(), img2.Bounds())
	out := image.NewRGBA(bounds)
	dim := 1.0 - opts.DimOpacity

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, a1 := pixelAt(img1, x, y)
			r2, g2, b2, a2 := pixelAt(img2, x, y)

			delta := maxChannelDelta(r1, g1, b1, a1, r2, g2, b2, a2)
			if delta > opts.Threshold {
				out.SetRGBA(x, y, opts.HighlightColor)
			} else {
				out.SetRGBA(x, y, color.RGBA{
					R: uint8(float64(r1) * dim),
					G: uint8(float64(g1) * dim),
					B: uint8(float64(b1) * dim),
					A: a1,
				})
			}
		}
	}
	return out
}

// Score returns the fraction of pixels that are identical between img1 and img2
// (max channel delta ≤ threshold). Returns 1.0 if both images are empty.
func Score(img1, img2 image.Image, threshold uint8) float64 {
	bounds := unionBounds(img1.Bounds(), img2.Bounds())
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 1.0
	}

	identical := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, a1 := pixelAt(img1, x, y)
			r2, g2, b2, a2 := pixelAt(img2, x, y)
			if maxChannelDelta(r1, g1, b1, a1, r2, g2, b2, a2) <= threshold {
				identical++
			}
		}
	}
	return float64(identical) / float64(total)
}
