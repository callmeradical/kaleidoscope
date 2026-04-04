package diff_test

import (
	"image"
	"image/color"
	"testing"

	pixeldiff "github.com/callmeradical/kaleidoscope/diff"
)

// solidImage creates a uniform-color image of the given size.
func solidImage(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestOverlay_identical(t *testing.T) {
	c := color.RGBA{R: 100, G: 150, B: 200, A: 255}
	img1 := solidImage(10, 10, c)
	img2 := solidImage(10, 10, c)

	overlay := pixeldiff.Overlay(img1, img2, nil)
	if overlay == nil {
		t.Fatal("Overlay returned nil")
	}

	score := pixeldiff.Score(img1, img2, 10)
	if score != 1.0 {
		t.Errorf("Score = %f, want 1.0 for identical images", score)
	}

	// Verify no pixels are highlighted (none should differ).
	highlightColor := color.RGBA{R: 255, G: 0, B: 0, A: 255} // default red highlight
	bounds := overlay.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := overlay.At(x, y).RGBA()
			hr, hg, hb, ha := highlightColor.RGBA()
			if r == hr && g == hg && b == hb && a == ha {
				t.Errorf("pixel (%d,%d) is highlighted but images are identical", x, y)
				return
			}
		}
	}
}

func TestOverlay_fullyDifferent(t *testing.T) {
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	img1 := solidImage(10, 10, red)
	img2 := solidImage(10, 10, blue)

	overlay := pixeldiff.Overlay(img1, img2, nil)
	if overlay == nil {
		t.Fatal("Overlay returned nil")
	}

	score := pixeldiff.Score(img1, img2, 10)
	if score != 0.0 {
		t.Errorf("Score = %f, want 0.0 for fully different images", score)
	}

	// All pixels should be highlighted with the default highlight color.
	highlightColor := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	bounds := overlay.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := overlay.At(x, y).RGBA()
			hr, hg, hb, ha := highlightColor.RGBA()
			if r != hr || g != hg || b != hb || a != ha {
				t.Errorf("pixel (%d,%d) not highlighted in fully different overlay", x, y)
				return
			}
		}
	}
}

func TestOverlay_differentSizes(t *testing.T) {
	c := color.RGBA{R: 100, G: 150, B: 200, A: 255}
	img1 := solidImage(10, 10, c)
	img2 := solidImage(10, 15, c) // taller by 5 rows

	overlay := pixeldiff.Overlay(img1, img2, nil)
	if overlay == nil {
		t.Fatal("Overlay returned nil")
	}

	// Output should be at least as tall as the taller image.
	bounds := overlay.Bounds()
	if bounds.Dy() < 15 {
		t.Errorf("Overlay height = %d, want >= 15 (max of both images)", bounds.Dy())
	}

	// Extra rows (y=10..14) where img1 is absent should be highlighted.
	highlightColor := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := 10; y < 15; y++ {
		for x := 0; x < 10; x++ {
			r, g, b, a := overlay.At(x, y).RGBA()
			hr, hg, hb, ha := highlightColor.RGBA()
			if r != hr || g != hg || b != hb || a != ha {
				t.Errorf("extra row pixel (%d,%d) should be highlighted (img1 absent)", x, y)
				return
			}
		}
	}
}

func TestScore_threshold(t *testing.T) {
	base := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	img1 := solidImage(5, 5, base)

	// Differ by exactly threshold=10 on one channel → should be identical (delta == threshold is within threshold).
	atThreshold := solidImage(5, 5, color.RGBA{R: 100 + 10, G: 100, B: 100, A: 255})
	score := pixeldiff.Score(img1, atThreshold, 10)
	if score != 1.0 {
		t.Errorf("Score(delta==threshold) = %f, want 1.0 (at-threshold pixels should be identical)", score)
	}

	// Differ by threshold+1 → all pixels different.
	overThreshold := solidImage(5, 5, color.RGBA{R: 100 + 11, G: 100, B: 100, A: 255})
	score2 := pixeldiff.Score(img1, overThreshold, 10)
	if score2 != 0.0 {
		t.Errorf("Score(delta==threshold+1) = %f, want 0.0 (over-threshold pixels should be different)", score2)
	}
}
