package analysis

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
)

// solidImage creates a uniform solid-color NRGBA image of dimensions w×h.
func solidImage(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	return img
}

func TestCompareImages_Identical(t *testing.T) {
	blue := color.NRGBA{R: 0, G: 0, B: 255, A: 255}
	a := solidImage(100, 100, blue)
	b := solidImage(100, 100, blue)

	result := CompareImages(a, b, 10)

	if result.Similarity != 1.0 {
		t.Errorf("expected Similarity=1.0, got %v", result.Similarity)
	}
	if result.DifferentPixels != 0 {
		t.Errorf("expected DifferentPixels=0, got %d", result.DifferentPixels)
	}
	if result.MismatchedDimensions {
		t.Error("expected MismatchedDimensions=false")
	}
	if result.DiffImage == nil {
		t.Error("expected DiffImage to be non-nil")
	}
}

func TestCompareImages_TotallyDifferent(t *testing.T) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	a := solidImage(10, 10, white)
	b := solidImage(10, 10, black)

	result := CompareImages(a, b, 10)

	if result.Similarity != 0.0 {
		t.Errorf("expected Similarity=0.0, got %v", result.Similarity)
	}
	if result.DifferentPixels != 100 {
		t.Errorf("expected DifferentPixels=100, got %d", result.DifferentPixels)
	}
	if result.TotalPixels != 100 {
		t.Errorf("expected TotalPixels=100, got %d", result.TotalPixels)
	}
}

func TestCompareImages_PartialDiff(t *testing.T) {
	// Image A: 10×10 solid white
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	red := color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	a := solidImage(10, 10, white)

	// Image B: left half white, right half red
	b := solidImage(10, 10, white)
	for x := 5; x < 10; x++ {
		for y := 0; y < 10; y++ {
			b.SetNRGBA(x, y, red)
		}
	}

	result := CompareImages(a, b, 10)

	if math.Abs(result.Similarity-0.5) >= 0.01 {
		t.Errorf("expected Similarity≈0.5, got %v", result.Similarity)
	}
}

func TestCompareImages_MismatchedDimensions(t *testing.T) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	a := solidImage(100, 100, white)
	b := solidImage(200, 200, white)

	// Must not panic
	result := CompareImages(a, b, 10)

	if !result.MismatchedDimensions {
		t.Error("expected MismatchedDimensions=true")
	}
	if result.Similarity != 0.0 {
		t.Errorf("expected Similarity=0.0, got %v", result.Similarity)
	}
	if result.DiffImage != nil {
		t.Error("expected DiffImage=nil for mismatched dimensions")
	}
}

func TestCompareImages_ChannelThreshold(t *testing.T) {
	base := color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	a := solidImage(10, 10, base)
	b := solidImage(10, 10, base)
	// Pixel at (0,0) differs by exactly 10 in the R channel
	b.SetNRGBA(0, 0, color.NRGBA{R: 110, G: 100, B: 100, A: 255})

	// threshold=10: diff of 10 should NOT be counted (> threshold means flagged)
	result10 := CompareImages(a, b, 10)
	if result10.DifferentPixels != 0 {
		t.Errorf("threshold=10: expected DifferentPixels=0, got %d", result10.DifferentPixels)
	}

	// threshold=9: diff of 10 IS > 9, should be counted
	result9 := CompareImages(a, b, 9)
	if result9.DifferentPixels != 1 {
		t.Errorf("threshold=9: expected DifferentPixels=1, got %d", result9.DifferentPixels)
	}
}

func TestCompareImages_DiffImageIsRed(t *testing.T) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	a := solidImage(1, 1, white)
	b := solidImage(1, 1, black)

	result := CompareImages(a, b, 10)

	if result.DiffImage == nil {
		t.Fatal("expected DiffImage to be non-nil")
	}
	pixel := color.NRGBAModel.Convert(result.DiffImage.At(0, 0)).(color.NRGBA)
	expected := color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	if pixel != expected {
		t.Errorf("expected overlay pixel to be red %v, got %v", expected, pixel)
	}
}

func TestCompareImages_DiffImageIsAttenuated(t *testing.T) {
	c := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	a := solidImage(1, 1, c)
	b := solidImage(1, 1, c) // identical

	result := CompareImages(a, b, 10)

	if result.DiffImage == nil {
		t.Fatal("expected DiffImage to be non-nil")
	}
	pixel := color.NRGBAModel.Convert(result.DiffImage.At(0, 0)).(color.NRGBA)
	if pixel.A != 128 {
		t.Errorf("expected alpha=128 for unchanged pixel, got %d", pixel.A)
	}
	if pixel.R != 200 || pixel.G != 100 || pixel.B != 50 {
		t.Errorf("expected RGB to match original {200,100,50}, got {%d,%d,%d}", pixel.R, pixel.G, pixel.B)
	}
}

func TestWriteDiffImage_Roundtrip(t *testing.T) {
	green := color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	img := solidImage(50, 30, green)

	dir := t.TempDir()
	path, err := WriteDiffImage(dir, "test-page-desktop", img)
	if err != nil {
		t.Fatalf("WriteDiffImage failed: %v", err)
	}
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("expected path to end with .png, got %s", path)
	}
	if !strings.Contains(path, "diff-") {
		t.Errorf("expected path to contain 'diff-', got %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open written file: %v", err)
	}
	defer f.Close()

	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != 50 || bounds.Dy() != 30 {
		t.Errorf("expected bounds 50×30, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
