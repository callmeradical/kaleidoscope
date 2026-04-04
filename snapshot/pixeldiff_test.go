package snapshot_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// makeImage returns a solid-color *image.RGBA of the given dimensions.
func makeImage(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// tmpPNG writes img to a temp PNG file and returns its path.
// The file is removed when the test ends.
func tmpPNG(t *testing.T, img image.Image) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.png")
	if err != nil {
		t.Fatalf("tmpPNG: create temp file: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("tmpPNG: encode png: %v", err)
	}
	return f.Name()
}

// --- DiffImages tests ---

// TestDiffImages_Identical verifies that two identical images produce score 1.0 with no
// highlighted pixels.
func TestDiffImages_Identical(t *testing.T) {
	// Use green so it does not collide with the default red highlight colour.
	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	img := makeImage(100, 100, green)
	cfg := snapshot.DefaultDiffConfig()

	diff, score, err := snapshot.DiffImages(img, img, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 1.0 {
		t.Errorf("score = %f; want 1.0", score)
	}

	// No pixel in the diff image should equal the highlight colour.
	highlight := cfg.HighlightColor
	bounds := diff.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			got := diff.RGBAAt(x, y)
			if got == highlight {
				t.Errorf("pixel (%d,%d) is highlight colour in identical-image diff", x, y)
				return
			}
		}
	}
}

// TestDiffImages_FullyDifferent verifies that black vs white produces score 0.0 and every
// pixel in the diff image equals the highlight colour.
func TestDiffImages_FullyDifferent(t *testing.T) {
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	baseline := makeImage(100, 100, black)
	current := makeImage(100, 100, white)
	cfg := snapshot.DefaultDiffConfig()

	diff, score, err := snapshot.DiffImages(baseline, current, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0.0 {
		t.Errorf("score = %f; want 0.0", score)
	}

	highlight := cfg.HighlightColor
	bounds := diff.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			got := diff.RGBAAt(x, y)
			if got != highlight {
				t.Errorf("pixel (%d,%d) = %v; want highlight %v", x, y, got, highlight)
				return
			}
		}
	}
}

// TestDiffImages_SinglePixelChanged verifies score and exactly one highlighted pixel when
// only one pixel differs between a 100×100 pair.
func TestDiffImages_SinglePixelChanged(t *testing.T) {
	// Use green base colour to avoid colliding with the default red highlight colour.
	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}

	baseline := makeImage(100, 100, green)
	current := makeImage(100, 100, green)
	// Change exactly one pixel in current.
	current.(*image.RGBA).SetRGBA(50, 50, blue)

	cfg := snapshot.DefaultDiffConfig()
	diff, score, err := snapshot.DiffImages(baseline, current, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantScore := 1.0 - 1.0/10000.0
	if score != wantScore {
		t.Errorf("score = %f; want %f", score, wantScore)
	}

	// Count highlighted pixels — must be exactly 1.
	highlight := cfg.HighlightColor
	bounds := diff.Bounds()
	highlighted := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if diff.RGBAAt(x, y) == highlight {
				highlighted++
			}
		}
	}
	if highlighted != 1 {
		t.Errorf("highlighted pixel count = %d; want 1", highlighted)
	}
}

// TestDiffImages_DimensionMismatch verifies that mismatched image sizes return
// ErrDimensionMismatch and no diff image.
func TestDiffImages_DimensionMismatch(t *testing.T) {
	small := makeImage(100, 100, color.RGBA{R: 255, A: 255})
	large := makeImage(200, 200, color.RGBA{G: 255, A: 255})

	diff, score, err := snapshot.DiffImages(small, large, snapshot.DefaultDiffConfig())
	if err != snapshot.ErrDimensionMismatch {
		t.Errorf("err = %v; want ErrDimensionMismatch", err)
	}
	if score != 0.0 {
		t.Errorf("score = %f; want 0.0", score)
	}
	if diff != nil {
		t.Errorf("diff image should be nil on dimension mismatch")
	}
}

// TestDiffImages_NoiseFloor verifies that a pixel with delta == 10 is NOT flagged and one
// with delta == 11 IS flagged.
func TestDiffImages_NoiseFloor(t *testing.T) {
	// Create 1×2 baseline: both pixels same colour.
	baseline := image.NewRGBA(image.Rect(0, 0, 1, 2))
	baseline.SetRGBA(0, 0, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	baseline.SetRGBA(0, 1, color.RGBA{R: 100, G: 100, B: 100, A: 255})

	current := image.NewRGBA(image.Rect(0, 0, 1, 2))
	// Pixel (0,0): delta = 10 — should NOT be flagged.
	current.SetRGBA(0, 0, color.RGBA{R: 110, G: 100, B: 100, A: 255})
	// Pixel (0,1): delta = 11 — SHOULD be flagged.
	current.SetRGBA(0, 1, color.RGBA{R: 111, G: 100, B: 100, A: 255})

	cfg := snapshot.DefaultDiffConfig()
	diff, _, err := snapshot.DiffImages(baseline, current, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	highlight := cfg.HighlightColor
	if diff.RGBAAt(0, 0) == highlight {
		t.Errorf("pixel (0,0) with delta=10 was flagged; should not be")
	}
	if diff.RGBAAt(0, 1) != highlight {
		t.Errorf("pixel (0,1) with delta=11 was not flagged; should be")
	}
}

// --- DiffScreenshotFiles tests ---

// TestDiffScreenshotFiles_Regressed verifies that a pair exceeding the threshold is
// marked as regressed.
func TestDiffScreenshotFiles_Regressed(t *testing.T) {
	// 100×100 = 10,000 pixels. Change 150 pixels → score ≈ 0.985, threshold 0.99 → regressed.
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}

	baseline := makeImage(100, 100, red).(*image.RGBA)
	current := makeImage(100, 100, red).(*image.RGBA)
	for i := 0; i < 150; i++ {
		current.SetRGBA(i%100, i/100, blue)
	}

	bPath := tmpPNG(t, baseline)
	cPath := tmpPNG(t, current)
	diffDir := t.TempDir()

	cfg := snapshot.DiffConfig{SimilarityThreshold: 0.99, HighlightColor: color.RGBA{R: 255, A: 255}}
	result := snapshot.DiffScreenshotFiles(bPath, cPath, diffDir, cfg)

	if !result.Regressed {
		t.Errorf("expected Regressed=true for score %f with threshold 0.99", result.SimilarityScore)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.DiffPath == "" {
		t.Errorf("expected non-empty DiffPath")
	}
}

// TestDiffScreenshotFiles_NotRegressed verifies that a pair within the threshold is not
// marked as regressed.
func TestDiffScreenshotFiles_NotRegressed(t *testing.T) {
	// 100×100 = 10,000 pixels. Change 50 pixels → score = 0.995, threshold 0.99 → not regressed.
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}

	baseline := makeImage(100, 100, red).(*image.RGBA)
	current := makeImage(100, 100, red).(*image.RGBA)
	for i := 0; i < 50; i++ {
		current.SetRGBA(i%100, i/100, blue)
	}

	bPath := tmpPNG(t, baseline)
	cPath := tmpPNG(t, current)
	diffDir := t.TempDir()

	cfg := snapshot.DiffConfig{SimilarityThreshold: 0.99, HighlightColor: color.RGBA{R: 255, A: 255}}
	result := snapshot.DiffScreenshotFiles(bPath, cPath, diffDir, cfg)

	if result.Regressed {
		t.Errorf("expected Regressed=false for score %f with threshold 0.99", result.SimilarityScore)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

// TestDiffScreenshotFiles_DimensionMismatch verifies graceful handling of different image sizes.
func TestDiffScreenshotFiles_DimensionMismatch(t *testing.T) {
	small := makeImage(100, 100, color.RGBA{R: 255, A: 255})
	large := makeImage(200, 200, color.RGBA{G: 255, A: 255})

	bPath := tmpPNG(t, small)
	cPath := tmpPNG(t, large)
	diffDir := t.TempDir()

	result := snapshot.DiffScreenshotFiles(bPath, cPath, diffDir, snapshot.DefaultDiffConfig())

	if !result.DimensionMismatch {
		t.Errorf("expected DimensionMismatch=true")
	}
	if !result.Regressed {
		t.Errorf("expected Regressed=true on dimension mismatch")
	}
	if result.SimilarityScore != 0.0 {
		t.Errorf("score = %f; want 0.0", result.SimilarityScore)
	}
	// No diff PNG should be written for a dimension mismatch.
	if result.DiffPath != "" {
		t.Errorf("DiffPath should be empty on dimension mismatch, got %q", result.DiffPath)
	}
}

// TestDiffScreenshotFiles_CorruptPNG verifies that a corrupt baseline PNG is handled
// gracefully: Regressed=true and Error is set.
func TestDiffScreenshotFiles_CorruptPNG(t *testing.T) {
	dir := t.TempDir()
	corruptPath := dir + "/corrupt.png"
	if err := os.WriteFile(corruptPath, []byte("not a png"), 0644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	validImg := makeImage(100, 100, color.RGBA{R: 255, A: 255})
	validPath := tmpPNG(t, validImg)

	result := snapshot.DiffScreenshotFiles(corruptPath, validPath, dir, snapshot.DefaultDiffConfig())
	if !result.Regressed {
		t.Errorf("expected Regressed=true for corrupt baseline")
	}
	if result.Error == "" {
		t.Errorf("expected non-empty Error for corrupt baseline")
	}
}

// TestDiffScreenshotFiles_DiffPNGWritten verifies that a diff PNG is written and readable
// when the comparison succeeds.
func TestDiffScreenshotFiles_DiffPNGWritten(t *testing.T) {
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}

	baseline := makeImage(10, 10, red)
	current := makeImage(10, 10, blue)
	bPath := tmpPNG(t, baseline)
	cPath := tmpPNG(t, current)
	diffDir := t.TempDir()

	result := snapshot.DiffScreenshotFiles(bPath, cPath, diffDir, snapshot.DefaultDiffConfig())
	if result.DiffPath == "" {
		t.Fatalf("DiffPath is empty")
	}

	data, err := os.ReadFile(result.DiffPath)
	if err != nil {
		t.Fatalf("read diff PNG: %v", err)
	}

	// Verify the file is a valid PNG.
	_, err = png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("diff file is not a valid PNG: %v", err)
	}
}

// TestDefaultDiffConfig verifies the default values are correct.
func TestDefaultDiffConfig(t *testing.T) {
	cfg := snapshot.DefaultDiffConfig()
	if cfg.SimilarityThreshold != 0.99 {
		t.Errorf("SimilarityThreshold = %f; want 0.99", cfg.SimilarityThreshold)
	}
	wantColor := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	if cfg.HighlightColor != wantColor {
		t.Errorf("HighlightColor = %v; want %v", cfg.HighlightColor, wantColor)
	}
}
