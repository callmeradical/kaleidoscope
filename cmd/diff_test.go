package cmd

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeSolidPNG creates PNG bytes for a solid-color image.
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

// setupSnapshotDir creates a minimal snapshot directory tree in dir and
// returns the state dir path.
func setupSnapshotDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create snapshot dirs
	baselineDir := filepath.Join(dir, "snapshots", "baseline-001", "screenshots")
	snapDir := filepath.Join(dir, "snapshots", "snap-002", "screenshots")
	if err := os.MkdirAll(baselineDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write PNGs
	baselinePNG := makeSolidPNG(100, 100, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	snapPNG := makeSolidPNG(100, 100, color.RGBA{R: 200, G: 50, B: 50, A: 255})

	if err := os.WriteFile(filepath.Join(baselineDir, "1280x720.png"), baselinePNG, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "1280x720.png"), snapPNG, 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

// writeBaselinesJSON writes a baselines.json file in the state dir.
func writeBaselinesJSON(t *testing.T, stateDir, defaultBaseline string) {
	t.Helper()
	data := []byte(`{"defaultBaseline":"` + defaultBaseline + `"}`)
	if err := os.WriteFile(filepath.Join(stateDir, "baselines.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

// captureStdout captures stdout output produced by fn.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.Bytes()
}

// overrideStateDir temporarily overrides the state directory by creating a
// .kaleidoscope symlink in the current directory pointing to dir, then
// restores after the test.
func overrideStateDir(t *testing.T, dir string) {
	t.Helper()

	// Create .kaleidoscope dir in the temp dir itself; we rely on StateDir()
	// looking for ".kaleidoscope" relative to CWD. We'll create a local dir.
	localKS := filepath.Join(".", ".kaleidoscope-test-"+t.Name())
	_ = os.Remove(localKS)

	// Change CWD to dir so StateDir() picks up .kaleidoscope inside it.
	// The simplest approach: create .kaleidoscope inside dir and chdir to dir.
	ksDir := filepath.Join(dir, ".kaleidoscope")
	if err := os.MkdirAll(ksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy all subdirs into .kaleidoscope
	for _, sub := range []string{"snapshots", "baselines.json"} {
		src := filepath.Join(dir, sub)
		dst := filepath.Join(ksDir, sub)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				// ignore if it doesn't exist
				_ = err
			}
		}
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
}

// TestDiffJSONOutput verifies that RunDiff produces valid JSON output with
// screenshotDiffs including a similarityScore when given two snapshot dirs.
func TestDiffJSONOutput(t *testing.T) {
	dir := setupSnapshotDir(t)
	writeBaselinesJSON(t, dir, "baseline-001")
	overrideStateDir(t, dir)

	out := captureStdout(t, func() {
		RunDiff([]string{"--snapshot", "snap-002", "--baseline", "baseline-001"})
	})

	var resp struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Result  struct {
			SnapshotID      string `json:"snapshotID"`
			BaselineID      string `json:"baselineID"`
			ScreenshotDiffs []struct {
				SimilarityScore *float64 `json:"similarityScore"`
				Breakpoint      struct {
					Width  int `json:"width"`
					Height int `json:"height"`
				} `json:"breakpoint"`
			} `json:"screenshotDiffs"`
			Threshold    float64 `json:"threshold"`
			AnyRegressed bool    `json:"anyRegressed"`
		} `json:"result"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, out)
	}
	if !resp.OK {
		t.Errorf("ok = false; want true. error: %s", resp.Error)
	}
	if resp.Command != "diff" {
		t.Errorf("command = %q; want \"diff\"", resp.Command)
	}
	if len(resp.Result.ScreenshotDiffs) != 1 {
		t.Errorf("screenshotDiffs count = %d; want 1", len(resp.Result.ScreenshotDiffs))
	} else {
		if resp.Result.ScreenshotDiffs[0].SimilarityScore == nil {
			t.Errorf("similarityScore is nil; want a numeric value")
		}
		if resp.Result.ScreenshotDiffs[0].Breakpoint.Width != 1280 {
			t.Errorf("breakpoint.width = %d; want 1280", resp.Result.ScreenshotDiffs[0].Breakpoint.Width)
		}
	}
	if resp.Result.Threshold == 0 {
		t.Errorf("threshold = 0; want non-zero default")
	}
	if resp.Result.SnapshotID != "snap-002" {
		t.Errorf("snapshotID = %q; want \"snap-002\"", resp.Result.SnapshotID)
	}
	if resp.Result.BaselineID != "baseline-001" {
		t.Errorf("baselineID = %q; want \"baseline-001\"", resp.Result.BaselineID)
	}
}

// TestDiffMissingBaseline verifies that RunDiff fails gracefully when
// baselines.json is absent and --baseline is not specified.
// Uses a subprocess to handle os.Exit without killing the test process.
func TestDiffMissingBaseline(t *testing.T) {
	// Subprocess mode: actually run the command in a subprocess.
	if os.Getenv("KS_TEST_SUBPROCESS") == "TestDiffMissingBaseline" {
		dir := os.Getenv("KS_TEST_DIR")
		ksDir := filepath.Join(dir, ".kaleidoscope")
		if err := os.MkdirAll(ksDir, 0755); err != nil {
			os.Exit(3)
		}
		// Create snap-002 dir
		snapDir := filepath.Join(ksDir, "snapshots", "snap-002", "screenshots")
		os.MkdirAll(snapDir, 0755)
		solidPNG := makeSolidPNG(50, 50, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		os.WriteFile(filepath.Join(snapDir, "1280x720.png"), solidPNG, 0644)
		os.Chdir(dir)
		// No baselines.json — RunDiff should fail
		RunDiff([]string{"--snapshot", "snap-002"})
		os.Exit(0)
	}

	dir := t.TempDir()

	// Re-run this test in subprocess mode
	cmd := exec.Command(os.Args[0], "-test.run=TestDiffMissingBaseline", "-test.v")
	cmd.Env = append(os.Environ(),
		"KS_TEST_SUBPROCESS=TestDiffMissingBaseline",
		"KS_TEST_DIR="+dir,
	)
	out, err := cmd.Output()
	// Expect non-zero exit due to os.Exit(2) call inside RunDiff
	if err == nil {
		t.Logf("subprocess stdout: %s", out)
		// If somehow it exited 0, check that ok=false was printed
	}

	// Parse JSON output to verify ok=false
	outStr := string(out)
	// Find the JSON line (may be prefixed by test output)
	for _, line := range bytes.Split([]byte(outStr), []byte("\n")) {
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var resp struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(line, &resp); jsonErr == nil {
			if resp.OK {
				t.Errorf("ok = true; want false when baseline is missing")
			}
			if resp.Error == "" {
				t.Errorf("error is empty; want descriptive message")
			}
			return
		}
	}
	// If no JSON found in output, check that subprocess exited non-zero
	if err == nil {
		t.Errorf("expected non-zero exit when baseline is missing, but subprocess exited 0")
	}
}

// TestDiffThresholdFlag verifies that the --threshold flag value is reflected
// in the JSON output result.
func TestDiffThresholdFlag(t *testing.T) {
	dir := setupSnapshotDir(t)

	// Use identical images for both so score == 1.0
	ksDir := filepath.Join(dir, ".kaleidoscope")
	if err := os.MkdirAll(ksDir, 0755); err != nil {
		t.Fatal(err)
	}
	snapDir := filepath.Join(ksDir, "snapshots", "snap-001", "screenshots")
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		t.Fatal(err)
	}
	solidPNG := makeSolidPNG(50, 50, color.RGBA{R: 128, G: 128, B: 128, A: 255})
	os.WriteFile(filepath.Join(snapDir, "800x600.png"), solidPNG, 0644)

	// Also create baseline dir
	baseDir := filepath.Join(ksDir, "snapshots", "base-001", "screenshots")
	os.MkdirAll(baseDir, 0755)
	os.WriteFile(filepath.Join(baseDir, "800x600.png"), solidPNG, 0644)

	writeBaselinesJSON(t, ksDir, "base-001")

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	for _, tc := range []struct {
		threshold string
		wantVal   float64
	}{
		{"0.99", 0.99},
		{"0.95", 0.95},
	} {
		t.Run("threshold="+tc.threshold, func(t *testing.T) {
			out := captureStdout(t, func() {
				RunDiff([]string{"--snapshot", "snap-001", "--baseline", "base-001", "--threshold", tc.threshold})
			})

			var resp struct {
				OK     bool `json:"ok"`
				Result struct {
					Threshold float64 `json:"threshold"`
				} `json:"result"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(out, &resp); err != nil {
				t.Fatalf("output not valid JSON: %v\nraw: %s", err, out)
			}
			if !resp.OK {
				t.Errorf("ok = false; error: %s", resp.Error)
			}
			if resp.Result.Threshold != tc.wantVal {
				t.Errorf("threshold = %f; want %f", resp.Result.Threshold, tc.wantVal)
			}
		})
	}
}
