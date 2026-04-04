package diff

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helpers ---

func solidRGBA(w, h int, r, g, b, a uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img
}

// --- tests ---

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.Threshold != 10 {
		t.Errorf("Threshold: want 10, got %d", opts.Threshold)
	}
	if opts.SimilarityThreshold != 0.99 {
		t.Errorf("SimilarityThreshold: want 0.99, got %f", opts.SimilarityThreshold)
	}
	if opts.HighlightColor != [4]uint8{255, 0, 0, 255} {
		t.Errorf("HighlightColor: want [255 0 0 255], got %v", opts.HighlightColor)
	}
}

func TestIdenticalImages(t *testing.T) {
	baseline := solidRGBA(10, 10, 0, 200, 0, 255)
	current := solidRGBA(10, 10, 0, 200, 0, 255)

	result, diffImg := CompareImages(baseline, current, DefaultOptions())

	if result.Similarity != 1.0 {
		t.Errorf("Similarity: want 1.0, got %f", result.Similarity)
	}
	if result.Regressed {
		t.Error("Regressed: want false for identical images")
	}
	if result.DimensionMismatch {
		t.Error("DimensionMismatch: want false for same-size images")
	}
	if diffImg == nil {
		t.Fatal("diffImg: want non-nil for same-size images")
	}

	// No pixels should be highlighted — all should match baseline green.
	bounds := diffImg.Bounds()
	highlight := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := diffImg.At(x, y).RGBA()
			got := color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			}
			if got == highlight {
				t.Errorf("pixel (%d,%d) is highlighted in diff image for identical images", x, y)
			}
		}
	}
}

func TestCompletelyDifferentImages(t *testing.T) {
	baseline := solidRGBA(10, 10, 255, 0, 0, 255) // red
	current := solidRGBA(10, 10, 0, 0, 255, 255)  // blue

	result, diffImg := CompareImages(baseline, current, DefaultOptions())

	if result.Similarity > 0.0 {
		t.Errorf("Similarity: want 0.0 for completely different images, got %f", result.Similarity)
	}
	if !result.Regressed {
		t.Error("Regressed: want true for completely different images")
	}
	if diffImg == nil {
		t.Fatal("diffImg: want non-nil for same-size images")
	}

	// All pixels should be the highlight color.
	bounds := diffImg.Bounds()
	highlight := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := diffImg.At(x, y).RGBA()
			got := color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			}
			if got != highlight {
				t.Errorf("pixel (%d,%d) not highlighted: got %v, want %v", x, y, got, highlight)
				return // don't flood with errors
			}
		}
	}
}

func TestPartialDiff(t *testing.T) {
	baseline := solidRGBA(10, 10, 255, 255, 255, 255) // white
	current := solidRGBA(10, 10, 255, 255, 255, 255)  // white base
	// Bottom row (y=9) is black.
	for x := 0; x < 10; x++ {
		current.SetRGBA(x, 9, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	}

	result, _ := CompareImages(baseline, current, DefaultOptions())

	// 10 pixels differ out of 100 => similarity = 0.90
	want := float64(90) / float64(100)
	if math.Abs(result.Similarity-want) > 1e-9 {
		t.Errorf("Similarity: want %f, got %f", want, result.Similarity)
	}
}

func TestDimensionMismatch(t *testing.T) {
	small := solidRGBA(10, 10, 255, 0, 0, 255)
	large := solidRGBA(20, 20, 0, 0, 255, 255)

	result, diffImg := CompareImages(small, large, DefaultOptions())

	if !result.DimensionMismatch {
		t.Error("DimensionMismatch: want true")
	}
	if result.Similarity != 0.0 {
		t.Errorf("Similarity: want 0.0, got %f", result.Similarity)
	}
	if !result.Regressed {
		t.Error("Regressed: want true for dimension mismatch")
	}
	if diffImg != nil {
		t.Error("diffImg: want nil for dimension mismatch")
	}
}

func TestThresholdTolerance(t *testing.T) {
	opts := DefaultOptions()
	opts.Threshold = 20

	// Diff of 15 — within threshold, should NOT count as different.
	baseline1 := solidRGBA(1, 1, 100, 0, 0, 255)
	current1 := solidRGBA(1, 1, 115, 0, 0, 255)
	r1, _ := CompareImages(baseline1, current1, opts)
	if r1.Similarity != 1.0 {
		t.Errorf("diff=15 < threshold=20: Similarity want 1.0, got %f", r1.Similarity)
	}

	// Diff of 21 — exceeds threshold, SHOULD count as different.
	baseline2 := solidRGBA(1, 1, 100, 0, 0, 255)
	current2 := solidRGBA(1, 1, 121, 0, 0, 255)
	r2, _ := CompareImages(baseline2, current2, opts)
	if r2.Similarity >= 1.0 {
		t.Errorf("diff=21 > threshold=20: Similarity want <1.0, got %f", r2.Similarity)
	}
}

func TestRegressionFlag(t *testing.T) {
	opts := DefaultOptions()
	opts.SimilarityThreshold = 0.95

	// Construct 100-pixel images where exactly 6 differ => similarity = 0.94
	baseline94 := solidRGBA(10, 10, 255, 255, 255, 255)
	current94 := solidRGBA(10, 10, 255, 255, 255, 255)
	for x := 0; x < 6; x++ {
		current94.SetRGBA(x, 0, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	}
	r94, _ := CompareImages(baseline94, current94, opts)
	if !r94.Regressed {
		t.Errorf("similarity=0.94 < threshold=0.95: Regressed want true, got false")
	}

	// Exactly 4 differ => similarity = 0.96
	baseline96 := solidRGBA(10, 10, 255, 255, 255, 255)
	current96 := solidRGBA(10, 10, 255, 255, 255, 255)
	for x := 0; x < 4; x++ {
		current96.SetRGBA(x, 0, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	}
	r96, _ := CompareImages(baseline96, current96, opts)
	if r96.Regressed {
		t.Errorf("similarity=0.96 >= threshold=0.95: Regressed want false, got true")
	}
}

func TestLoadSavePNG(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "round_trip.png")

	// Create 4×4 image with distinct colors.
	original := image.NewRGBA(image.Rect(0, 0, 4, 4))
	colors := []color.RGBA{
		{R: 255, G: 0, B: 0, A: 255},
		{R: 0, G: 255, B: 0, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
		{R: 255, G: 255, B: 0, A: 255},
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			original.SetRGBA(x, y, colors[(x+y)%len(colors)])
		}
	}

	if err := SavePNG(path, original); err != nil {
		t.Fatalf("SavePNG error: %v", err)
	}

	loaded, err := LoadPNG(path)
	if err != nil {
		t.Fatalf("LoadPNG error: %v", err)
	}

	bounds := original.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			or, og, ob, oa := original.At(x, y).RGBA()
			lr, lg, lb, la := loaded.At(x, y).RGBA()
			if or != lr || og != lg || ob != lb || oa != la {
				t.Errorf("pixel (%d,%d) mismatch: orig %v, loaded %v",
					x, y, original.At(x, y), loaded.At(x, y))
			}
		}
	}
}

func TestDiffImagePath(t *testing.T) {
	dir := "/snapshots/abc123"
	baseline := "/snapshots/old/homepage-desktop.png"
	current := "/snapshots/abc123/homepage-desktop.png"

	result := DiffImagePath(dir, baseline, current)

	if !strings.HasPrefix(result, dir) {
		t.Errorf("DiffImagePath: want prefix %q, got %q", dir, result)
	}
	if filepath.Ext(result) != ".png" {
		t.Errorf("DiffImagePath: want .png extension, got %q", result)
	}
	base := filepath.Base(result)
	if !strings.HasPrefix(base, "diff-") {
		t.Errorf("DiffImagePath: want filename starting with 'diff-', got %q", base)
	}
}

func TestLoadPNGMissingFile(t *testing.T) {
	_, err := LoadPNG("/nonexistent/path/image.png")
	if err == nil {
		t.Error("LoadPNG: want error for missing file, got nil")
	}
}

func TestSavePNGInvalidPath(t *testing.T) {
	img := solidRGBA(2, 2, 0, 0, 0, 255)
	err := SavePNG("/nonexistent/dir/output.png", img)
	if err == nil {
		t.Error("SavePNG: want error for invalid directory, got nil")
	}
}

func TestCompareImagesZeroPixels(t *testing.T) {
	// Zero-dimension images should not panic or divide by zero.
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	result, _ := CompareImages(empty, empty, DefaultOptions())
	// Similarity should be 1.0 for zero-pixel images (identical, no pixels to compare).
	if result.Similarity != 1.0 {
		t.Errorf("zero-pixel images: Similarity want 1.0, got %f", result.Similarity)
	}
}

func TestDiffImageWrittenToSnapshotDir(t *testing.T) {
	tmp := t.TempDir()
	baseline := solidRGBA(4, 4, 255, 0, 0, 255) // red
	current := solidRGBA(4, 4, 0, 0, 255, 255)  // blue

	result, diffImg := CompareImages(baseline, current, DefaultOptions())

	if diffImg == nil {
		t.Fatal("diffImg: want non-nil for same-size images")
	}

	diffPath := DiffImagePath(tmp, "baseline.png", "current-homepage.png")
	result.DiffPath = diffPath

	if err := SavePNG(diffPath, diffImg); err != nil {
		t.Fatalf("SavePNG error: %v", err)
	}

	if _, err := os.Stat(diffPath); err != nil {
		t.Errorf("diff PNG not written to snapshot dir: %v", err)
	}
	if !strings.Contains(result.DiffPath, tmp) {
		t.Errorf("DiffPath should be inside snapshotDir: %q", result.DiffPath)
	}
}
