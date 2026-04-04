package diff

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

const maxWidth = 4096
const maxHeight = 8192

// CompareImages compares two PNG byte slices and returns a diff PNG, the count
// of changed pixels, the total pixel count, and any error.
// Pixels where the sum of per-channel absolute differences exceeds threshold*3
// are highlighted in red. Otherwise the baseline pixel is rendered at 30% opacity.
func CompareImages(baselinePNG, currentPNG []byte, threshold uint8) (diffPNG []byte, changed, total int, err error) {
	baseImg, err := png.Decode(bytes.NewReader(baselinePNG))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decoding baseline PNG: %w", err)
	}
	curImg, err := png.Decode(bytes.NewReader(currentPNG))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decoding current PNG: %w", err)
	}

	baseBounds := baseImg.Bounds()
	curBounds := curImg.Bounds()

	if baseBounds.Dx() > maxWidth || baseBounds.Dy() > maxHeight {
		return nil, 0, 0, errors.New("baseline image exceeds maximum allowed dimensions")
	}
	if curBounds.Dx() > maxWidth || curBounds.Dy() > maxHeight {
		return nil, 0, 0, errors.New("current image exceeds maximum allowed dimensions")
	}

	// Use max bounds of both images.
	maxW := baseBounds.Dx()
	if curBounds.Dx() > maxW {
		maxW = curBounds.Dx()
	}
	maxH := baseBounds.Dy()
	if curBounds.Dy() > maxH {
		maxH = curBounds.Dy()
	}

	out := image.NewRGBA(image.Rect(0, 0, maxW, maxH))

	thresh := uint32(threshold) * 3

	for y := 0; y < maxH; y++ {
		for x := 0; x < maxW; x++ {
			var br, bg, bb uint32
			if x < baseBounds.Dx() && y < baseBounds.Dy() {
				r, g, b, _ := baseImg.At(x, y).RGBA()
				br, bg, bb = r>>8, g>>8, b>>8
			}
			var cr, cg, cb uint32
			if x < curBounds.Dx() && y < curBounds.Dy() {
				r, g, b, _ := curImg.At(x, y).RGBA()
				cr, cg, cb = r>>8, g>>8, b>>8
			}

			dr := absDiff(br, cr)
			dg := absDiff(bg, cg)
			db := absDiff(bb, cb)
			sum := dr + dg + db

			if sum > thresh {
				out.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
				changed++
			} else {
				// Render baseline at 30% opacity on black background.
				out.SetRGBA(x, y, color.RGBA{
					R: uint8(br * 77 / 255),
					G: uint8(bg * 77 / 255),
					B: uint8(bb * 77 / 255),
					A: 77,
				})
			}
			total++
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, 0, 0, fmt.Errorf("encoding diff PNG: %w", err)
	}
	return buf.Bytes(), changed, total, nil
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
