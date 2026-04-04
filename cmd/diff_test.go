package cmd

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/callmeradical/kaleidoscope/pixeldiff"
)

// writePNG creates a solid-color PNG at the given path and returns the path.
func writePNG(t *testing.T, dir, name string, w, h int, c color.RGBA) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create PNG %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode PNG %s: %v", path, err)
	}
	return path
}

func TestSlugifyURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://example.com/page", "example-com-page"},
		{"http://localhost:3000/", "localhost-3000"},
		{"https://foo.bar/a/b?c=d#e", "foo-bar-a-b-c-d-e"},
		{"", ""},
	}
	for _, tc := range cases {
		got := slugifyURL(tc.input)
		if got != tc.want {
			t.Errorf("slugifyURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSlugifyURLMaxLength(t *testing.T) {
	long := "https://" + strings.Repeat("a", 100) + ".com"
	slug := slugifyURL(long)
	if len(slug) > 64 {
		t.Errorf("slug length %d exceeds 64", len(slug))
	}
}

func TestCompareScreenshotPairIdentical(t *testing.T) {
	dir := t.TempDir()
	red := color.RGBA{R: 255, A: 255}
	base := writePNG(t, dir, "base.png", 50, 50, red)
	curr := writePNG(t, dir, "curr.png", 50, 50, red)

	opts := defaultDiffOpts()
	entry, err := compareScreenshotPair(base, curr, dir, "", "", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.SimilarityScore != 1.0 {
		t.Errorf("expected SimilarityScore 1.0, got %f", entry.SimilarityScore)
	}
	if entry.Regressed {
		t.Error("expected Regressed false")
	}
	if entry.DiffPath == "" {
		t.Error("expected DiffPath to be set for identical images")
	}
	// Verify diff PNG was actually written
	if _, err := os.Stat(entry.DiffPath); err != nil {
		t.Errorf("diff PNG not found at %s: %v", entry.DiffPath, err)
	}
}

func TestCompareScreenshotPairDimMismatch(t *testing.T) {
	dir := t.TempDir()
	red := color.RGBA{R: 255, A: 255}
	base := writePNG(t, dir, "base.png", 50, 50, red)
	curr := writePNG(t, dir, "curr.png", 100, 100, red)

	opts := defaultDiffOpts()
	entry, err := compareScreenshotPair(base, curr, dir, "https://example.com", "desktop", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !entry.DimensionMismatch {
		t.Error("expected DimensionMismatch true")
	}
	if !entry.Regressed {
		t.Error("expected Regressed true on mismatch")
	}
	if entry.DiffPath != "" {
		t.Error("expected DiffPath empty on dimension mismatch")
	}
}

func TestBuildDiffPath(t *testing.T) {
	cases := []struct {
		currentPath string
		outputDir   string
		urlLabel    string
		breakpoint  string
		width       int
		height      int
		wantSuffix  string
	}{
		{"/snap/curr.png", "", "", "", 100, 200, "diff-100x200.png"},
		{"/snap/curr.png", "", "", "mobile", 375, 812, "diff-mobile-375x812.png"},
		{"/snap/curr.png", "", "https://example.com/page", "desktop", 1280, 800, "diff-example-com-page-desktop-1280x800.png"},
		{"/snap/curr.png", "/out", "https://foo.bar/", "", 800, 600, "diff-foo-bar-800x600.png"},
	}
	for _, tc := range cases {
		got := buildDiffPath(tc.currentPath, tc.outputDir, tc.urlLabel, tc.breakpoint, tc.width, tc.height)
		if !strings.HasSuffix(got, tc.wantSuffix) {
			t.Errorf("buildDiffPath(%q, %q, %q, %q, %d, %d) = %q, want suffix %q",
				tc.currentPath, tc.outputDir, tc.urlLabel, tc.breakpoint, tc.width, tc.height, got, tc.wantSuffix)
		}
	}
}

func TestRunDiffMissingFlags(t *testing.T) {
	// Capture output to verify error is emitted
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	RunDiff([]string{})

	w.Close()
	os.Stdout = old

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	out := string(buf[:n])

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if result["ok"] != false {
		t.Errorf("expected ok=false, got %v", result["ok"])
	}
}

func TestRunDiffSuccess(t *testing.T) {
	dir := t.TempDir()
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	base := writePNG(t, dir, "base.png", 20, 20, red)
	curr := writePNG(t, dir, "curr.png", 20, 20, blue)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	RunDiff([]string{
		"--baseline", base,
		"--current", curr,
		"--output-dir", dir,
		"--screenshot-threshold", "0.01",
	})

	w.Close()
	os.Stdout = old

	var buf [65536]byte
	n, _ := r.Read(buf[:])
	out := string(buf[:n])

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v\nOutput: %s", result["ok"], out)
	}

	payload, ok := result["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result field missing or wrong type")
	}
	screenshots, ok := payload["screenshots"].([]interface{})
	if !ok || len(screenshots) == 0 {
		t.Fatal("screenshots array missing or empty")
	}
	shot := screenshots[0].(map[string]interface{})
	if shot["regressed"] != true {
		t.Errorf("expected regressed=true for fully-different images, got %v", shot["regressed"])
	}
	if shot["similarityScore"].(float64) != 0.0 {
		t.Errorf("expected similarityScore 0.0, got %v", shot["similarityScore"])
	}
}

// defaultDiffOpts returns pixeldiff options for use in tests.
func defaultDiffOpts() pixeldiff.Options {
	return pixeldiff.DefaultOptions()
}
