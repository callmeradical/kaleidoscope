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

// PixelDiff compares two PNG files and returns a difference score and overlay image.
// score is the fraction of pixels that differ (0.0 = identical, 1.0 = all different).
// overlayPNG is a PNG-encoded image with differing pixels highlighted in semi-transparent red.
// If either file cannot be read or decoded, an error is returned.
func PixelDiff(baselinePath, currentPath string) (score float64, overlayPNG []byte, err error) {
	baseImg, err := loadPNG(baselinePath)
	if err != nil {
		return 0, nil, fmt.Errorf("loading baseline image %q: %w", baselinePath, err)
	}
	curImg, err := loadPNG(currentPath)
	if err != nil {
		return 0, nil, fmt.Errorf("loading current image %q: %w", currentPath, err)
	}

	// Work in RGBA for uniform pixel access.
	baseRGBA := toRGBA(baseImg)
	curRGBA := toRGBA(curImg)

	// Use current image dimensions as the reference.
	bounds := curRGBA.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y

	overlay := image.NewRGBA(bounds)

	const threshold = 10.0 // on 0–255 scale
	total := w * h
	different := 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cr, cg, cb, ca := curRGBA.At(x, y).RGBA()
			var br, bg, bb uint32
			if x < baseRGBA.Bounds().Max.X && y < baseRGBA.Bounds().Max.Y {
				br, bg, bb, _ = baseRGBA.At(x, y).RGBA()
			}

			// Convert from 16-bit to 8-bit.
			dr := float64(cr>>8) - float64(br>>8)
			dg := float64(cg>>8) - float64(bg>>8)
			db := float64(cb>>8) - float64(bb>>8)
			dist := math.Sqrt(dr*dr + dg*dg + db*db)

			if dist > threshold {
				different++
				overlay.SetRGBA(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 128})
			} else {
				// Copy current pixel.
				overlay.SetRGBA(x, y, color.RGBA{
					R: uint8(cr >> 8),
					G: uint8(cg >> 8),
					B: uint8(cb >> 8),
					A: uint8(ca >> 8),
				})
			}
		}
	}

	score = float64(different) / float64(total)

	var buf bytes.Buffer
	if err := png.Encode(&buf, overlay); err != nil {
		return 0, nil, fmt.Errorf("encoding overlay PNG: %w", err)
	}

	return score, buf.Bytes(), nil
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding PNG: %w", err)
	}
	return img, nil
}

func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba
}
