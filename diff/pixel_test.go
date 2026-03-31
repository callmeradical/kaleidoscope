package diff

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func makeImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestDiffScreenshots_Identical(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.png")
	curr := filepath.Join(dir, "curr.png")
	diff := filepath.Join(dir, "diff.png")

	img := makeImage(10, 10, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writePNG(t, base, img)
	writePNG(t, curr, img)

	result, err := DiffScreenshots(base, curr, diff)
	if err != nil {
		t.Fatal(err)
	}
	if result.Similarity < 0.99 {
		t.Errorf("expected similarity ~1.0, got %f", result.Similarity)
	}
}

func TestDiffScreenshots_Different(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.png")
	curr := filepath.Join(dir, "curr.png")
	diff := filepath.Join(dir, "diff.png")

	img1 := makeImage(10, 10, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	img2 := makeImage(10, 10, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	writePNG(t, base, img1)
	writePNG(t, curr, img2)

	result, err := DiffScreenshots(base, curr, diff)
	if err != nil {
		t.Fatal(err)
	}
	if result.Similarity > 0.5 {
		t.Errorf("expected low similarity, got %f", result.Similarity)
	}
}

func TestDiffScreenshots_MismatchedDimensions(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.png")
	curr := filepath.Join(dir, "curr.png")
	diff := filepath.Join(dir, "diff.png")

	img1 := makeImage(10, 10, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	img2 := makeImage(20, 20, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	writePNG(t, base, img1)
	writePNG(t, curr, img2)

	result, err := DiffScreenshots(base, curr, diff)
	if err != nil {
		t.Fatal(err)
	}
	if !result.MismatchedDims {
		t.Error("expected MismatchedDims to be true")
	}
	if result.Similarity != 0.0 {
		t.Errorf("expected similarity 0.0 for mismatched dims, got %f", result.Similarity)
	}
}
