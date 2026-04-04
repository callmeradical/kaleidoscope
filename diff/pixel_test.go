package diff_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/callmeradical/kaleidoscope/diff"
)

// writePNG writes a solid-color PNG image to a temp file and returns the path.
func writePNG(t *testing.T, dir, name string, c color.Color, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// TestPixelDiff_IdenticalImages verifies score == 0 for two identical images.
func TestPixelDiff_IdenticalImages(t *testing.T) {
	dir := t.TempDir()
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	p1 := writePNG(t, dir, "a.png", red, 10, 10)
	p2 := writePNG(t, dir, "b.png", red, 10, 10)

	score, overlay, err := diff.PixelDiff(p1, p2)
	if err != nil {
		t.Fatalf("PixelDiff error: %v", err)
	}
	if score != 0.0 {
		t.Errorf("expected score 0.0 for identical images, got %f", score)
	}
	if len(overlay) == 0 {
		t.Error("expected non-empty overlay PNG for identical images")
	}
}

// TestPixelDiff_TotallyDifferent verifies score ≈ 1.0 for red vs blue images.
func TestPixelDiff_TotallyDifferent(t *testing.T) {
	dir := t.TempDir()
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	p1 := writePNG(t, dir, "red.png", red, 10, 10)
	p2 := writePNG(t, dir, "blue.png", blue, 10, 10)

	score, _, err := diff.PixelDiff(p1, p2)
	if err != nil {
		t.Fatalf("PixelDiff error: %v", err)
	}
	if score < 0.99 {
		t.Errorf("expected score ≈ 1.0 for red vs blue, got %f", score)
	}
}

// TestPixelDiff_MissingFile verifies an error is returned when a file is missing.
func TestPixelDiff_MissingFile(t *testing.T) {
	dir := t.TempDir()
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	p1 := writePNG(t, dir, "exists.png", red, 10, 10)

	_, _, err := diff.PixelDiff(p1, filepath.Join(dir, "missing.png"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
