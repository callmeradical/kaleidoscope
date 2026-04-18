package diff

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// --- PNG fixture generators ---

// makeSolidPNG creates a solid-color PNG of the given size and writes it to path.
func makeSolidPNG(t *testing.T, path string, w, h int, c color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	writePNG(t, path, img)
}

// makeGradientPNG creates a horizontal gradient PNG.
func makeGradientPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(x * 255 / w)
			img.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	writePNG(t, path, img)
}

// makeGradientWithPatchPNG creates a gradient PNG with a colored patch in the center.
func makeGradientWithPatchPNG(t *testing.T, path string, w, h int, patchColor color.NRGBA, patchSize int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	cx, cy := w/2, h/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(x * 255 / w)
			img.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	// Draw patch
	half := patchSize / 2
	for y := cy - half; y < cy+half; y++ {
		for x := cx - half; x < cx+half; x++ {
			if x >= 0 && x < w && y >= 0 && y < h {
				img.SetNRGBA(x, y, patchColor)
			}
		}
	}
	writePNG(t, path, img)
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

// --- Tests ---

func TestCompareScreenshots_Identical(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")

	makeSolidPNG(t, a, 100, 80, color.NRGBA{R: 40, G: 120, B: 200, A: 255})
	makeSolidPNG(t, b, 100, 80, color.NRGBA{R: 40, G: 120, B: 200, A: 255})

	result, diffImg, err := CompareScreenshots(a, b, ScreenshotDiffConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Similarity != 1.0 {
		t.Errorf("similarity: got %f, want 1.0", result.Similarity)
	}
	if result.DiffPixelCount != 0 {
		t.Errorf("diffPixelCount: got %d, want 0", result.DiffPixelCount)
	}
	if !result.DimensionMatch {
		t.Error("expected dimension match")
	}
	if result.TotalPixels != 8000 {
		t.Errorf("totalPixels: got %d, want 8000", result.TotalPixels)
	}
	if diffImg == nil {
		t.Error("diff image should not be nil")
	}
}

func TestCompareScreenshots_SubtlyDifferent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")

	// Gradient image, then same gradient with a small patch changed
	makeGradientPNG(t, a, 200, 100)
	makeGradientWithPatchPNG(t, b, 200, 100, color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 10)

	result, diffImg, err := CompareScreenshots(a, b, ScreenshotDiffConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Similarity >= 1.0 {
		t.Error("expected similarity < 1.0 for subtly different images")
	}
	if result.Similarity < 0.9 {
		t.Errorf("expected high similarity for subtle change, got %f", result.Similarity)
	}
	if result.DiffPixelCount == 0 {
		t.Error("expected some diff pixels")
	}
	if !result.DimensionMatch {
		t.Error("expected dimension match")
	}
	if diffImg == nil {
		t.Error("diff image should not be nil")
	}

	// Verify diff image can be written
	outPath := filepath.Join(dir, "diff.png")
	if err := WriteDiffImage(diffImg, outPath); err != nil {
		t.Fatalf("writing diff image: %v", err)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat diff image: %v", err)
	}
	if info.Size() == 0 {
		t.Error("diff image file is empty")
	}
}

func TestCompareScreenshots_SignificantlyDifferent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")

	makeSolidPNG(t, a, 100, 100, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	makeSolidPNG(t, b, 100, 100, color.NRGBA{R: 0, G: 0, B: 0, A: 255})

	result, _, err := CompareScreenshots(a, b, ScreenshotDiffConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Similarity != 0.0 {
		t.Errorf("similarity: got %f, want 0.0 (all pixels different)", result.Similarity)
	}
	if result.DiffPixelCount != 10000 {
		t.Errorf("diffPixelCount: got %d, want 10000", result.DiffPixelCount)
	}
	if !result.DimensionMatch {
		t.Error("expected dimension match")
	}
}

func TestCompareScreenshots_MismatchedDimensions(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")

	makeSolidPNG(t, a, 100, 100, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
	makeSolidPNG(t, b, 200, 150, color.NRGBA{R: 128, G: 128, B: 128, A: 255})

	result, diffImg, err := CompareScreenshots(a, b, ScreenshotDiffConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Similarity != 0.0 {
		t.Errorf("similarity: got %f, want 0.0 for dimension mismatch", result.Similarity)
	}
	if result.DimensionMatch {
		t.Error("expected dimension mismatch")
	}
	if diffImg == nil {
		t.Error("diff image should still be generated for dimension mismatch")
	}
}

func TestCompareScreenshots_PixelThreshold(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")

	// Two images that differ by exactly 5 in the red channel
	makeSolidPNG(t, a, 50, 50, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	makeSolidPNG(t, b, 50, 50, color.NRGBA{R: 105, G: 100, B: 100, A: 255})

	// With threshold=0, all pixels differ
	r1, _, err := CompareScreenshots(a, b, ScreenshotDiffConfig{PixelThreshold: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.DiffPixelCount == 0 {
		t.Error("expected diffs with threshold=0")
	}

	// With threshold=10, no pixels differ
	r2, _, err := CompareScreenshots(a, b, ScreenshotDiffConfig{PixelThreshold: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2.DiffPixelCount != 0 {
		t.Errorf("expected 0 diffs with threshold=10, got %d", r2.DiffPixelCount)
	}
	if r2.Similarity != 1.0 {
		t.Errorf("expected similarity 1.0 with threshold=10, got %f", r2.Similarity)
	}
}

func TestCompareScreenshots_SimilarityRange(t *testing.T) {
	dir := t.TempDir()

	// Create images with exactly 50% of pixels different
	w, h := 100, 100
	imgA := image.NewNRGBA(image.Rect(0, 0, w, h))
	imgB := image.NewNRGBA(image.Rect(0, 0, w, h))

	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.NRGBA{R: 0, G: 0, B: 0, A: 255}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			imgA.SetNRGBA(x, y, white)
			if y < h/2 {
				imgB.SetNRGBA(x, y, white) // top half same
			} else {
				imgB.SetNRGBA(x, y, black) // bottom half different
			}
		}
	}

	aPath := filepath.Join(dir, "a.png")
	bPath := filepath.Join(dir, "b.png")
	writePNG(t, aPath, imgA)
	writePNG(t, bPath, imgB)

	result, _, err := CompareScreenshots(aPath, bPath, ScreenshotDiffConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if math.Abs(result.Similarity-0.5) > 0.01 {
		t.Errorf("similarity: got %f, want ~0.5", result.Similarity)
	}
	if result.Similarity < 0.0 || result.Similarity > 1.0 {
		t.Errorf("similarity out of range: %f", result.Similarity)
	}
}

func TestWriteDiffImage_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "sub", "dir", "diff.png")

	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	if err := WriteDiffImage(img, outPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("diff image not created: %v", err)
	}
}

func TestCompareScreenshots_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "nonexistent.png")
	b := filepath.Join(dir, "also-nonexistent.png")

	_, _, err := CompareScreenshots(a, b, ScreenshotDiffConfig{})
	if err == nil {
		t.Error("expected error for nonexistent files")
	}
}
