package diff_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// helpers

func makeAudit(contrast, touch, typo, spacing int) snapshot.AuditResult {
	return snapshot.AuditResult{
		ContrastViolations: contrast,
		TouchViolations:    touch,
		TypographyWarnings: typo,
		SpacingIssues:      spacing,
	}
}

func solidPNG(w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// --- computeAuditDeltas (tested via Compute's URLDiff.AuditDeltas) ---

func TestComputeAuditDeltas_DeltaIsAfterMinusBefore(t *testing.T) {
	base := &snapshot.Snapshot{
		ID: "base-001",
		URLs: []snapshot.URLSnapshot{
			{URL: "https://example.com", Audit: makeAudit(1, 2, 3, 4)},
		},
	}
	cur := &snapshot.Snapshot{
		ID: "cur-001",
		URLs: []snapshot.URLSnapshot{
			{URL: "https://example.com", Audit: makeAudit(3, 1, 5, 4)},
		},
	}

	result, err := diff.Compute(base, cur)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.URLs) != 1 {
		t.Fatalf("expected 1 URLDiff, got %d", len(result.URLs))
	}

	want := map[string]int{
		"Contrast":   2,  // 3-1
		"Touch":      -1, // 1-2
		"Typography": 2,  // 5-3
		"Spacing":    0,  // 4-4
	}
	for _, d := range result.URLs[0].AuditDeltas {
		expected, ok := want[d.Category]
		if !ok {
			t.Errorf("unexpected category %q", d.Category)
			continue
		}
		if d.Delta != expected {
			t.Errorf("category %q: Delta=%d, want %d", d.Category, d.Delta, expected)
		}
	}
}

func TestComputeAuditDeltas_HasRegressionWhenDeltaPositive(t *testing.T) {
	base := &snapshot.Snapshot{ID: "base", URLs: []snapshot.URLSnapshot{{URL: "https://example.com", Audit: makeAudit(0, 0, 0, 0)}}}
	cur := &snapshot.Snapshot{ID: "cur", URLs: []snapshot.URLSnapshot{{URL: "https://example.com", Audit: makeAudit(1, 0, 0, 0)}}}
	result, err := diff.Compute(base, cur)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if !result.URLs[0].HasRegression {
		t.Error("expected HasRegression=true when contrast increased")
	}
	if !result.HasRegressions {
		t.Error("expected result.HasRegressions=true")
	}
}

func TestComputeAuditDeltas_NoRegressionWhenDeltaZeroOrNegative(t *testing.T) {
	base := &snapshot.Snapshot{ID: "base", URLs: []snapshot.URLSnapshot{{URL: "https://example.com", Audit: makeAudit(2, 1, 3, 4)}}}
	cur := &snapshot.Snapshot{ID: "cur", URLs: []snapshot.URLSnapshot{{URL: "https://example.com", Audit: makeAudit(1, 1, 2, 4)}}}
	result, err := diff.Compute(base, cur)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if result.URLs[0].HasRegression {
		t.Error("expected HasRegression=false when no category increased")
	}
}

// --- computeElementChanges ---

func newElem(role, name, sel string, x, y, w, h float64) snapshot.AXElement {
	return snapshot.AXElement{Role: role, Name: name, Selector: sel, X: x, Y: y, Width: w, Height: h}
}

func computeElementChangesViaCompute(base, cur []snapshot.AXElement) []diff.ElementChange {
	baseSnap := &snapshot.Snapshot{ID: "b", URLs: []snapshot.URLSnapshot{{URL: "http://x", Elements: base}}}
	curSnap := &snapshot.Snapshot{ID: "c", URLs: []snapshot.URLSnapshot{{URL: "http://x", Elements: cur}}}
	result, err := diff.Compute(baseSnap, curSnap)
	if err != nil {
		panic(err)
	}
	if len(result.URLs) == 0 {
		return nil
	}
	return result.URLs[0].ElementChanges
}

func findChange(changes []diff.ElementChange, changeType string) *diff.ElementChange {
	for i := range changes {
		if changes[i].Type == changeType {
			return &changes[i]
		}
	}
	return nil
}

func TestElementChanges_Appeared(t *testing.T) {
	base := []snapshot.AXElement{}
	cur := []snapshot.AXElement{newElem("button", "Submit", "button#submit", 0, 0, 100, 44)}
	changes := computeElementChangesViaCompute(base, cur)
	c := findChange(changes, "appeared")
	if c == nil {
		t.Fatal("expected 'appeared' change, got none")
	}
	if c.Name != "Submit" {
		t.Errorf("Name: got %q want %q", c.Name, "Submit")
	}
}

func TestElementChanges_Disappeared(t *testing.T) {
	base := []snapshot.AXElement{newElem("button", "Login", "button#login", 0, 0, 80, 44)}
	cur := []snapshot.AXElement{}
	changes := computeElementChangesViaCompute(base, cur)
	c := findChange(changes, "disappeared")
	if c == nil {
		t.Fatal("expected 'disappeared' change, got none")
	}
	if c.Name != "Login" {
		t.Errorf("Name: got %q want %q", c.Name, "Login")
	}
}

func TestElementChanges_Moved(t *testing.T) {
	base := []snapshot.AXElement{newElem("link", "Home", "a.home", 10, 10, 80, 20)}
	cur := []snapshot.AXElement{newElem("link", "Home", "a.home", 50, 10, 80, 20)}
	changes := computeElementChangesViaCompute(base, cur)
	c := findChange(changes, "moved")
	if c == nil {
		t.Fatal("expected 'moved' change, got none")
	}
}

func TestElementChanges_Resized(t *testing.T) {
	base := []snapshot.AXElement{newElem("img", "Logo", "img.logo", 0, 0, 100, 50)}
	cur := []snapshot.AXElement{newElem("img", "Logo", "img.logo", 0, 0, 200, 50)}
	changes := computeElementChangesViaCompute(base, cur)
	c := findChange(changes, "resized")
	if c == nil {
		t.Fatal("expected 'resized' change, got none")
	}
}

func TestElementChanges_WithinThresholdNotReported(t *testing.T) {
	// Movement of 1px should NOT be reported (threshold is 2px).
	base := []snapshot.AXElement{newElem("p", "Text", "p", 10, 10, 100, 20)}
	cur := []snapshot.AXElement{newElem("p", "Text", "p", 11, 10, 100, 20)}
	changes := computeElementChangesViaCompute(base, cur)
	if len(changes) != 0 {
		t.Errorf("expected no changes for sub-threshold movement, got %d", len(changes))
	}
}

// --- CompareImages ---

func TestCompareImages_IdenticalImages(t *testing.T) {
	img := solidPNG(10, 10, color.RGBA{128, 64, 32, 255})
	_, changed, total, err := diff.CompareImages(img, img, 10)
	if err != nil {
		t.Fatalf("CompareImages: %v", err)
	}
	if changed != 0 {
		t.Errorf("identical images: expected 0 changed pixels, got %d", changed)
	}
	if total != 100 {
		t.Errorf("expected 100 total pixels, got %d", total)
	}
}

func TestCompareImages_CompletelyDifferent(t *testing.T) {
	white := solidPNG(10, 10, color.RGBA{255, 255, 255, 255})
	black := solidPNG(10, 10, color.RGBA{0, 0, 0, 255})
	_, changed, total, err := diff.CompareImages(white, black, 10)
	if err != nil {
		t.Fatalf("CompareImages: %v", err)
	}
	if changed != total {
		t.Errorf("completely different images: expected changed==total (%d), got changed=%d", total, changed)
	}
}

func TestCompareImages_OversizedBaseline(t *testing.T) {
	// 4097×1 exceeds the 4096 width limit.
	big := image.NewRGBA(image.Rect(0, 0, 4097, 1))
	var buf bytes.Buffer
	_ = png.Encode(&buf, big)
	small := solidPNG(10, 10, color.RGBA{0, 0, 0, 255})
	_, _, _, err := diff.CompareImages(buf.Bytes(), small, 10)
	if err == nil {
		t.Fatal("expected error for oversized baseline image, got nil")
	}
}

func TestCompareImages_OversizedCurrent(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 1, 8193))
	var buf bytes.Buffer
	_ = png.Encode(&buf, big)
	small := solidPNG(10, 10, color.RGBA{0, 0, 0, 255})
	_, _, _, err := diff.CompareImages(small, buf.Bytes(), 10)
	if err == nil {
		t.Fatal("expected error for oversized current image, got nil")
	}
}

func TestCompareImages_DifferentSizes_NoPanic(t *testing.T) {
	small := solidPNG(10, 10, color.RGBA{255, 0, 0, 255})
	large := solidPNG(20, 15, color.RGBA{0, 0, 255, 255})
	diffPNG, _, total, err := diff.CompareImages(small, large, 10)
	if err != nil {
		t.Fatalf("CompareImages: %v", err)
	}
	if total != 20*15 {
		t.Errorf("expected total == max bounds (%d), got %d", 20*15, total)
	}
	if len(diffPNG) == 0 {
		t.Error("expected non-empty diff PNG")
	}
}

func TestCompareImages_ReturnsDiffPNG(t *testing.T) {
	white := solidPNG(10, 10, color.RGBA{255, 255, 255, 255})
	black := solidPNG(10, 10, color.RGBA{0, 0, 0, 255})
	diffPNG, _, _, err := diff.CompareImages(white, black, 10)
	if err != nil {
		t.Fatalf("CompareImages: %v", err)
	}
	// Verify returned bytes are a valid PNG.
	_, err = png.Decode(bytes.NewReader(diffPNG))
	if err != nil {
		t.Errorf("returned diff is not a valid PNG: %v", err)
	}
}
