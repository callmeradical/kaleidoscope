package diff

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// PixelDiffResult holds the result of comparing two screenshots.
type PixelDiffResult struct {
	Similarity     float64 `json:"similarity"`
	DiffPath       string  `json:"diffPath,omitempty"`
	MismatchedDims bool    `json:"mismatchedDims,omitempty"`
}

// DiffScreenshots compares two PNG files and writes a diff image to diffPath.
// Returns similarity score (1.0 = identical, 0.0 = completely different).
func DiffScreenshots(baselinePath, currentPath, diffPath string) (*PixelDiffResult, error) {
	baseImg, err := loadPNG(baselinePath)
	if err != nil {
		return nil, err
	}
	currImg, err := loadPNG(currentPath)
	if err != nil {
		return nil, err
	}

	baseBounds := baseImg.Bounds()
	currBounds := currImg.Bounds()

	if baseBounds.Dx() != currBounds.Dx() || baseBounds.Dy() != currBounds.Dy() {
		return &PixelDiffResult{Similarity: 0.0, MismatchedDims: true}, nil
	}

	width := baseBounds.Dx()
	height := baseBounds.Dy()
	total := width * height
	if total == 0 {
		return &PixelDiffResult{Similarity: 1.0}, nil
	}

	diffImg := image.NewRGBA(baseBounds)
	diffCount := 0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			bc := baseImg.At(baseBounds.Min.X+x, baseBounds.Min.Y+y)
			cc := currImg.At(currBounds.Min.X+x, currBounds.Min.Y+y)
			br, bg, bb, _ := bc.RGBA()
			cr, cg, cb, _ := cc.RGBA()

			dr := math.Abs(float64(br>>8) - float64(cr>>8))
			dg := math.Abs(float64(bg>>8) - float64(cg>>8))
			db := math.Abs(float64(bb>>8) - float64(cb>>8))
			pixelDiff := (dr + dg + db) / 3.0

			if pixelDiff > 10 {
				diffCount++
				diffImg.Set(baseBounds.Min.X+x, baseBounds.Min.Y+y, color.RGBA{R: 255, G: 0, B: 0, A: 200})
			} else {
				r, g, b, _ := bc.RGBA()
				diffImg.Set(baseBounds.Min.X+x, baseBounds.Min.Y+y, color.RGBA{
					R: uint8(r >> 9),
					G: uint8(g >> 9),
					B: uint8(b >> 9),
					A: 255,
				})
			}
		}
	}

	similarity := 1.0 - float64(diffCount)/float64(total)

	f, err := os.Create(diffPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := png.Encode(f, diffImg); err != nil {
		return nil, err
	}

	return &PixelDiffResult{Similarity: similarity, DiffPath: diffPath}, nil
}

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
