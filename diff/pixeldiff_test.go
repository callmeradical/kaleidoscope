package diff

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// makeSolidPNG returns PNG bytes for a solid-color image of given dimensions.
func makeSolidPNG(width, height int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic("makeSolidPNG: " + err.Error())
	}
	return buf.Bytes()
}

// TestIdenticalImages verifies that comparing two identical PNGs yields
// SimilarityScore==1.0, ChangedPixels==0, and Regressed==false.
func TestIdenticalImages(t *testing.T) {
	c := color.RGBA{R: 100, G: 150, B: 200, A: 255}
	base := makeSolidPNG(100, 100, c)
	curr := makeSolidPNG(100, 100, c)

	result, err := CompareBytes(base, curr, PixelDiffOptions{})
	if err != nil {
		t.Fatalf("CompareBytes returned unexpected error: %v", err)
	}
	if result.SimilarityScore != 1.0 {
		t.Errorf("SimilarityScore = %f; want 1.0", result.SimilarityScore)
	}
	if result.ChangedPixels != 0 {
		t.Errorf("ChangedPixels = %d; want 0", result.ChangedPixels)
	}
	if result.Regressed {
		t.Errorf("Regressed = true; want false")
	}
}

// TestCompletelyDifferent verifies that solid white vs solid black images
// yield SimilarityScore near 0.0 and Regressed==true.
func TestCompletelyDifferent(t *testing.T) {
	white := makeSolidPNG(100, 100, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	black := makeSolidPNG(100, 100, color.RGBA{R: 0, G: 0, B: 0, A: 255})

	result, err := CompareBytes(white, black, PixelDiffOptions{})
	if err != nil {
		t.Fatalf("CompareBytes returned unexpected error: %v", err)
	}
	if result.SimilarityScore >= 0.01 {
		t.Errorf("SimilarityScore = %f; want < 0.01", result.SimilarityScore)
	}
	if !result.Regressed {
		t.Errorf("Regressed = false; want true")
	}
}

// TestPartialChange verifies that a 10x10 region change in a 100x100 image
// yields approximately 100 changed pixels and a score between 0.9 and 1.0.
func TestPartialChange(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 100, 100))
	curr := image.NewRGBA(image.Rect(0, 0, 100, 100))

	// Fill both with green
	green := color.RGBA{R: 0, G: 200, B: 0, A: 255}
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			base.SetRGBA(x, y, green)
			curr.SetRGBA(x, y, green)
		}
	}
	// Paint a 10x10 region red in current (pixels 0..9 x 0..9 = 100 pixels)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			curr.SetRGBA(x, y, red)
		}
	}

	var bufBase, bufCurr bytes.Buffer
	png.Encode(&bufBase, base)
	png.Encode(&bufCurr, curr)

	result, err := CompareBytes(bufBase.Bytes(), bufCurr.Bytes(), PixelDiffOptions{})
	if err != nil {
		t.Fatalf("CompareBytes returned unexpected error: %v", err)
	}
	if result.ChangedPixels != 100 {
		t.Errorf("ChangedPixels = %d; want 100", result.ChangedPixels)
	}
	if result.SimilarityScore <= 0.9 || result.SimilarityScore >= 1.0 {
		t.Errorf("SimilarityScore = %f; want (0.9, 1.0)", result.SimilarityScore)
	}
}

// TestDimensionMismatch verifies that comparing PNGs with different dimensions
// does not panic, does not return an error, and sets DimensionMismatch=true
// with SimilarityScore=0.0 and Regressed=true.
func TestDimensionMismatch(t *testing.T) {
	a := makeSolidPNG(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	b := makeSolidPNG(200, 200, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	result, err := CompareBytes(a, b, PixelDiffOptions{})
	if err != nil {
		t.Fatalf("CompareBytes returned unexpected error for dimension mismatch: %v", err)
	}
	if !result.DimensionMismatch {
		t.Errorf("DimensionMismatch = false; want true")
	}
	if result.SimilarityScore != 0.0 {
		t.Errorf("SimilarityScore = %f; want 0.0", result.SimilarityScore)
	}
	if !result.Regressed {
		t.Errorf("Regressed = false; want true")
	}
}

// TestThresholdFlag verifies that the Threshold option controls the Regressed flag.
// Uses a small 100x100 image with a 3% change (~300 changed pixels → score ~0.97).
func TestThresholdFlag(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 100, 100))
	curr := image.NewRGBA(image.Rect(0, 0, 100, 100))

	green := color.RGBA{R: 0, G: 200, B: 0, A: 255}
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			base.SetRGBA(x, y, green)
			curr.SetRGBA(x, y, green)
		}
	}
	// Change 300 pixels (rows 0-2, all 100 columns)
	for y := 0; y < 3; y++ {
		for x := 0; x < 100; x++ {
			curr.SetRGBA(x, y, red)
		}
	}

	var bufBase, bufCurr bytes.Buffer
	png.Encode(&bufBase, base)
	png.Encode(&bufCurr, curr)

	baseBytes := bufBase.Bytes()
	currBytes := bufCurr.Bytes()

	// With threshold 0.98, a score of ~0.97 should be regressed
	result98, err := CompareBytes(baseBytes, currBytes, PixelDiffOptions{Threshold: 0.98})
	if err != nil {
		t.Fatalf("CompareBytes(0.98) returned unexpected error: %v", err)
	}
	if !result98.Regressed {
		t.Errorf("Threshold 0.98: Regressed = false; want true (score=%f)", result98.SimilarityScore)
	}

	// With threshold 0.95, a score of ~0.97 should NOT be regressed
	result95, err := CompareBytes(baseBytes, currBytes, PixelDiffOptions{Threshold: 0.95})
	if err != nil {
		t.Fatalf("CompareBytes(0.95) returned unexpected error: %v", err)
	}
	if result95.Regressed {
		t.Errorf("Threshold 0.95: Regressed = true; want false (score=%f)", result95.SimilarityScore)
	}
}

// TestDiffImageOutput verifies that the diff PNG output is valid and highlights
// changed pixels with the DefaultHighlightColor.
func TestDiffImageOutput(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 10, 10))
	curr := image.NewRGBA(image.Rect(0, 0, 10, 10))

	blue := color.RGBA{R: 0, G: 0, B: 200, A: 255}
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			base.SetRGBA(x, y, blue)
			curr.SetRGBA(x, y, blue)
		}
	}
	// Change pixel (0,0) to red in current
	curr.SetRGBA(0, 0, red)

	var bufBase, bufCurr bytes.Buffer
	png.Encode(&bufBase, base)
	png.Encode(&bufCurr, curr)

	result, err := CompareBytes(bufBase.Bytes(), bufCurr.Bytes(), PixelDiffOptions{})
	if err != nil {
		t.Fatalf("CompareBytes returned unexpected error: %v", err)
	}
	if len(result.DiffImageBytes) == 0 {
		t.Fatal("DiffImageBytes is empty; want a valid PNG")
	}

	// Verify DiffImageBytes is valid PNG
	diffImg, err := png.Decode(bytes.NewReader(result.DiffImageBytes))
	if err != nil {
		t.Fatalf("DiffImageBytes is not a valid PNG: %v", err)
	}

	// Changed pixel at (0,0) should be DefaultHighlightColor
	got := diffImg.At(0, 0)
	want := DefaultHighlightColor
	r, g, b, _ := got.RGBA()
	wr, wg, wb, _ := color.RGBA(want).RGBA()
	if r != wr || g != wg || b != wb {
		t.Errorf("diff pixel at (0,0) = %v; want %v (DefaultHighlightColor)", got, want)
	}
}

// TestCompareFiles verifies that CompareFiles matches CompareBytes results and
// writes the diff PNG to the specified output path.
func TestCompareFiles(t *testing.T) {
	dir := t.TempDir()
	c1 := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	c2 := color.RGBA{R: 200, G: 50, B: 50, A: 255}

	baseBytes := makeSolidPNG(50, 50, c1)
	currBytes := makeSolidPNG(50, 50, c2)

	basePath := filepath.Join(dir, "base.png")
	currPath := filepath.Join(dir, "curr.png")
	diffPath := filepath.Join(dir, "diff.png")

	if err := os.WriteFile(basePath, baseBytes, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currPath, currBytes, 0644); err != nil {
		t.Fatal(err)
	}

	opts := PixelDiffOptions{OutputPath: diffPath}
	fileResult, err := CompareFiles(basePath, currPath, opts)
	if err != nil {
		t.Fatalf("CompareFiles returned unexpected error: %v", err)
	}

	// Compare with direct CompareBytes call
	bytesResult, err := CompareBytes(baseBytes, currBytes, opts)
	if err != nil {
		t.Fatalf("CompareBytes returned unexpected error: %v", err)
	}

	if fileResult.SimilarityScore != bytesResult.SimilarityScore {
		t.Errorf("SimilarityScore mismatch: CompareFiles=%f, CompareBytes=%f",
			fileResult.SimilarityScore, bytesResult.SimilarityScore)
	}
	if fileResult.ChangedPixels != bytesResult.ChangedPixels {
		t.Errorf("ChangedPixels mismatch: CompareFiles=%d, CompareBytes=%d",
			fileResult.ChangedPixels, bytesResult.ChangedPixels)
	}

	// Diff PNG should exist on disk
	if _, err := os.Stat(diffPath); os.IsNotExist(err) {
		t.Errorf("diff PNG not written to %s", diffPath)
	}
}
