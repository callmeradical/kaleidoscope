package pixeldiff

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// makeSolidImage creates a solid-color synthetic image of the given dimensions.
func makeSolidImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestIdenticalImages(t *testing.T) {
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	baseline := makeSolidImage(100, 100, red)
	current := makeSolidImage(100, 100, red)

	result, diffImg := Compare(baseline, current, DefaultOptions())

	if result.SimilarityScore != 1.0 {
		t.Errorf("expected SimilarityScore 1.0, got %f", result.SimilarityScore)
	}
	if result.DiffPixels != 0 {
		t.Errorf("expected DiffPixels 0, got %d", result.DiffPixels)
	}
	if result.Regressed {
		t.Error("expected Regressed false, got true")
	}
	if result.DimensionMismatch {
		t.Error("expected DimensionMismatch false, got true")
	}
	if diffImg == nil {
		t.Error("expected diffImg to be non-nil")
	}
}

func TestCompletelyDifferent(t *testing.T) {
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	baseline := makeSolidImage(100, 100, red)
	current := makeSolidImage(100, 100, blue)

	result, _ := Compare(baseline, current, DefaultOptions())

	if result.DiffPixels != result.TotalPixels {
		t.Errorf("expected all pixels to differ: DiffPixels=%d TotalPixels=%d", result.DiffPixels, result.TotalPixels)
	}
	if result.SimilarityScore != 0.0 {
		t.Errorf("expected SimilarityScore 0.0, got %f", result.SimilarityScore)
	}
	if !result.Regressed {
		t.Error("expected Regressed true, got false")
	}
}

func TestPartialDiff(t *testing.T) {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	baseline := makeSolidImage(100, 100, white)

	// Current: white except 10×10 black block in top-left (100 diff pixels)
	current := makeSolidImage(100, 100, white)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			current.SetRGBA(x, y, black)
		}
	}

	result, diffImg := Compare(baseline, current, DefaultOptions())

	if result.DiffPixels != 100 {
		t.Errorf("expected DiffPixels 100, got %d", result.DiffPixels)
	}
	if result.TotalPixels != 10000 {
		t.Errorf("expected TotalPixels 10000, got %d", result.TotalPixels)
	}
	if math.Abs(result.SimilarityScore-0.99) > 0.001 {
		t.Errorf("expected SimilarityScore ~0.99, got %f", result.SimilarityScore)
	}
	// score 0.99 is NOT less than 0.99, so Regressed == false
	if result.Regressed {
		t.Errorf("expected Regressed false (score %.4f >= threshold boundary 0.99)", result.SimilarityScore)
	}
	if diffImg == nil {
		t.Error("expected diffImg to be non-nil")
	}
}

func TestDimensionMismatch(t *testing.T) {
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	baseline := makeSolidImage(100, 100, red)
	current := makeSolidImage(200, 200, red)

	// Must not panic
	result, diffImg := Compare(baseline, current, DefaultOptions())

	if !result.DimensionMismatch {
		t.Error("expected DimensionMismatch true")
	}
	if !result.Regressed {
		t.Error("expected Regressed true on dimension mismatch")
	}
	if result.SimilarityScore != 0.0 {
		t.Errorf("expected SimilarityScore 0.0, got %f", result.SimilarityScore)
	}
	if diffImg != nil {
		t.Error("expected diffImg nil on dimension mismatch")
	}
}

func TestThresholdBoundary(t *testing.T) {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	baseline := makeSolidImage(100, 100, white)
	current := makeSolidImage(100, 100, white)
	// Exactly 1% of pixels (100 out of 10000) differ
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			current.SetRGBA(x, y, black)
		}
	}

	// Threshold 0.5%: score 0.99 < 0.995 → Regressed
	opts1 := Options{Threshold: 0.005, PixelTolerance: 10, HighlightColor: [4]uint8{255, 0, 0, 255}}
	r1, _ := Compare(baseline, current, opts1)
	if !r1.Regressed {
		t.Errorf("threshold=0.005: expected Regressed true, score=%f", r1.SimilarityScore)
	}

	// Threshold 2%: score 0.99 >= 0.98 → Not regressed
	opts2 := Options{Threshold: 0.02, PixelTolerance: 10, HighlightColor: [4]uint8{255, 0, 0, 255}}
	r2, _ := Compare(baseline, current, opts2)
	if r2.Regressed {
		t.Errorf("threshold=0.02: expected Regressed false, score=%f", r2.SimilarityScore)
	}

	// Threshold exactly 1%: score 0.99 is NOT less than 0.99 → Not regressed
	opts3 := Options{Threshold: 0.01, PixelTolerance: 10, HighlightColor: [4]uint8{255, 0, 0, 255}}
	r3, _ := Compare(baseline, current, opts3)
	if r3.Regressed {
		t.Errorf("threshold=0.01: expected Regressed false (score %f >= 0.99)", r3.SimilarityScore)
	}
}

func TestPixelTolerance(t *testing.T) {
	// 1×1 images to isolate single pixel behavior
	baseline := image.NewRGBA(image.Rect(0, 0, 1, 1))
	baseline.SetRGBA(0, 0, color.RGBA{R: 100, G: 100, B: 100, A: 255})

	// Current differs by exactly PixelTolerance=10 on R channel — should NOT be flagged
	current := image.NewRGBA(image.Rect(0, 0, 1, 1))
	current.SetRGBA(0, 0, color.RGBA{R: 110, G: 100, B: 100, A: 255})

	result, _ := Compare(baseline, current, DefaultOptions())

	if result.DiffPixels != 0 {
		t.Errorf("expected DiffPixels 0 (diff == tolerance, not > tolerance), got %d", result.DiffPixels)
	}
}

func TestWriteDiffPNG(t *testing.T) {
	img := makeSolidImage(50, 50, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "diff.png")

	if err := WriteDiffPNG(path, img); err != nil {
		t.Fatalf("WriteDiffPNG returned error: %v", err)
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
	if bounds.Dx() != 50 || bounds.Dy() != 50 {
		t.Errorf("expected 50×50 image, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
