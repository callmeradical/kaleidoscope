# Implementation Plan: Screenshot Pixel Diff (US-004)

**Story:** US-004 — Screenshot Pixel Diff
**Depends on:** US-002 (Snapshot System — must be complete before Phase 3)
**Module:** `github.com/callmeradical/kaleidoscope`
**Quality Gate:** `go test ./...`

---

## Overview

US-004 adds a pure-Go pixel-level image comparison engine to Kaleidoscope. It introduces the `pixeldiff` package (standalone, no Chrome dependency) and extends `cmd/diff.go` (introduced by US-002) to produce diff PNGs and similarity scores for each screenshot pair between a baseline and current snapshot.

No new third-party dependencies are introduced. All image handling uses Go stdlib: `image`, `image/color`, `image/png`, `os`.

---

## Phase 1: `pixeldiff` Package — Core Engine

**Goal:** Create `pixeldiff/diff.go` with all public types and functions. This phase is fully self-contained and has no dependency on US-002.

### Task 1.1: Create Package File

- **Sub-task 1.1.1:** Create `pixeldiff/diff.go` with `package pixeldiff` declaration.
- **Sub-task 1.1.2:** Add import block: `"image"`, `"image/color"`, `"image/png"`, `"os"`.

---

### Task 1.2: Define Public Types

- **Sub-task 1.2.1:** Define `Result` struct:
  ```go
  type Result struct {
      SimilarityScore   float64 `json:"similarityScore"`
      DiffPixels        int     `json:"diffPixels"`
      TotalPixels       int     `json:"totalPixels"`
      DimensionMismatch bool    `json:"dimensionMismatch,omitempty"`
      Regressed         bool    `json:"regressed"`
  }
  ```

- **Sub-task 1.2.2:** Define `Options` struct:
  ```go
  type Options struct {
      Threshold      float64   // 0.0–1.0, default 0.01
      PixelTolerance uint8     // 0–255, default 10
      HighlightColor [4]uint8  // RGBA, default {255, 0, 0, 255}
  }
  ```

- **Sub-task 1.2.3:** Implement `DefaultOptions() Options` returning `{Threshold: 0.01, PixelTolerance: 10, HighlightColor: [4]uint8{255, 0, 0, 255}}`.

---

### Task 1.3: Implement `Compare` Function

Signature: `func Compare(baseline, current image.Image, opts Options) (Result, *image.RGBA)`

- **Sub-task 1.3.1:** Extract bounds from both images using `.Bounds()`.
- **Sub-task 1.3.2:** Check dimension mismatch: if `baseline.Bounds().Size() != current.Bounds().Size()`, return `Result{DimensionMismatch: true, Regressed: true, SimilarityScore: 0.0}` and `nil`. Do not panic.
- **Sub-task 1.3.3:** Allocate `diffImg := image.NewRGBA(current.Bounds())`.
- **Sub-task 1.3.4:** Compute `totalPixels = bounds.Dx() * bounds.Dy()`.
- **Sub-task 1.3.5:** Implement nested loop over all pixels `(x, y)` within bounds:
  - Sub-task 1.3.5a: Read RGBA from `current` via `current.At(x, y).RGBA()` (returns uint32, scale to uint8 by `>> 8`).
  - Sub-task 1.3.5b: Read RGBA from `baseline` via `baseline.At(x, y).RGBA()`.
  - Sub-task 1.3.5c: Copy current pixel color to `diffImg` at `(x, y)` as visual context.
  - Sub-task 1.3.5d: Compute per-channel absolute difference for R, G, B, A channels.
  - Sub-task 1.3.5e: If any channel difference exceeds `opts.PixelTolerance`, mark pixel as diff: set `diffImg` pixel to `opts.HighlightColor`, increment `diffPixels`.
- **Sub-task 1.3.6:** Compute `similarityScore = 1.0 - float64(diffPixels)/float64(totalPixels)`. Handle `totalPixels == 0` edge case (return 1.0 for zero-size images).
- **Sub-task 1.3.7:** Set `Regressed = similarityScore < (1.0 - opts.Threshold)`.
- **Sub-task 1.3.8:** Return `Result{SimilarityScore, DiffPixels, TotalPixels, Regressed}` and `diffImg`.

---

### Task 1.4: Implement `WriteDiffPNG` Function

Signature: `func WriteDiffPNG(path string, img *image.RGBA) error`

- **Sub-task 1.4.1:** Create file at `path` using `os.Create(path)` — this sets permissions to the OS default (typically 0666 masked by umask, matching the project's 0644 intent).
- **Sub-task 1.4.2:** Defer `f.Close()`.
- **Sub-task 1.4.3:** Encode `img` to the file using `png.Encode(f, img)`.
- **Sub-task 1.4.4:** Return any encoding error.

---

## Phase 2: Unit Tests — `pixeldiff/diff_test.go`

**Goal:** Comprehensive unit tests covering all acceptance criteria. Each test uses synthetic `image.RGBA` images constructed in-memory — no filesystem or browser dependency.

### Task 2.1: Test File Setup

- **Sub-task 2.1.1:** Create `pixeldiff/diff_test.go` with `package pixeldiff` (same-package testing for access to internals if needed).
- **Sub-task 2.1.2:** Add import block: `"image"`, `"image/color"`, `"os"`, `"path/filepath"`, `"testing"`.
- **Sub-task 2.1.3:** Add helper `makeSolidImage(w, h int, c color.RGBA) *image.RGBA` that creates a solid-color synthetic image.

---

### Task 2.2: `TestIdenticalImages`

- **Sub-task 2.2.1:** Create two identical 100×100 red synthetic images using `makeSolidImage`.
- **Sub-task 2.2.2:** Call `Compare` with `DefaultOptions()`.
- **Sub-task 2.2.3:** Assert `result.SimilarityScore == 1.0`.
- **Sub-task 2.2.4:** Assert `result.DiffPixels == 0`.
- **Sub-task 2.2.5:** Assert `result.Regressed == false`.
- **Sub-task 2.2.6:** Assert `result.DimensionMismatch == false`.
- **Sub-task 2.2.7:** Assert `diffImg != nil`.

---

### Task 2.3: `TestCompletelyDifferent`

- **Sub-task 2.3.1:** Create 100×100 all-red image (baseline) and 100×100 all-blue image (current).
- **Sub-task 2.3.2:** Call `Compare` with `DefaultOptions()`.
- **Sub-task 2.3.3:** Assert `result.DiffPixels == result.TotalPixels` (all pixels differ).
- **Sub-task 2.3.4:** Assert `result.SimilarityScore == 0.0`.
- **Sub-task 2.3.5:** Assert `result.Regressed == true`.

---

### Task 2.4: `TestPartialDiff`

- **Sub-task 2.4.1:** Create 100×100 all-white baseline.
- **Sub-task 2.4.2:** Create 100×100 current image that is white except for a 10×10 block of black pixels in the top-left corner (100 diff pixels).
- **Sub-task 2.4.3:** Call `Compare` with `DefaultOptions()` (tolerance 10, threshold 0.01).
- **Sub-task 2.4.4:** Assert `result.DiffPixels == 100`.
- **Sub-task 2.4.5:** Assert `result.TotalPixels == 10000`.
- **Sub-task 2.4.6:** Assert `result.SimilarityScore == 0.99` (within float tolerance ±0.001).
- **Sub-task 2.4.7:** Assert `result.Regressed == false` (0.99 >= 0.99, just at threshold, not below).

---

### Task 2.5: `TestDimensionMismatch`

- **Sub-task 2.5.1:** Create 100×100 image (baseline) and 200×200 image (current).
- **Sub-task 2.5.2:** Call `Compare` — verify it does not panic.
- **Sub-task 2.5.3:** Assert `result.DimensionMismatch == true`.
- **Sub-task 2.5.4:** Assert `result.Regressed == true`.
- **Sub-task 2.5.5:** Assert `result.SimilarityScore == 0.0`.
- **Sub-task 2.5.6:** Assert `diffImg == nil`.

---

### Task 2.6: `TestThresholdBoundary`

- **Sub-task 2.6.1:** Create 100×100 images where exactly 1% of pixels (100) differ (use the partial diff setup from Task 2.4).
- **Sub-task 2.6.2:** Test with `opts.Threshold = 0.005` (0.5%) → score 0.99 < 0.995 → `Regressed == true`.
- **Sub-task 2.6.3:** Test with `opts.Threshold = 0.02` (2%) → score 0.99 >= 0.98 → `Regressed == false`.
- **Sub-task 2.6.4:** Test with `opts.Threshold = 0.01` (exactly 1%) → score 0.99 is NOT less than 0.99 → `Regressed == false`.

---

### Task 2.7: `TestPixelTolerance`

- **Sub-task 2.7.1:** Create baseline with a pixel at `(0,0)` with RGBA value `{100, 100, 100, 255}`.
- **Sub-task 2.7.2:** Create current with the same pixel at `(0,0)` with RGBA value `{110, 100, 100, 255}` (differs by exactly `PixelTolerance=10` on R channel).
- **Sub-task 2.7.3:** All other pixels are identical.
- **Sub-task 2.7.4:** Call `Compare` with `DefaultOptions()` (PixelTolerance = 10).
- **Sub-task 2.7.5:** Assert `result.DiffPixels == 0` (difference of exactly tolerance is not flagged — `>` not `>=`).

---

### Task 2.8: `TestWriteDiffPNG`

- **Sub-task 2.8.1:** Create a 50×50 synthetic diff image (`*image.RGBA`).
- **Sub-task 2.8.2:** Write to a temp file using `t.TempDir()` + `filepath.Join`.
- **Sub-task 2.8.3:** Call `WriteDiffPNG(path, img)` and assert no error.
- **Sub-task 2.8.4:** Open the file and decode using `png.Decode`.
- **Sub-task 2.8.5:** Assert decoded image bounds equal 50×50.
- **Sub-task 2.8.6:** Cleanup is automatic via `t.TempDir()`.

---

## Phase 3: `cmd/diff.go` — Snapshot Integration

**Prerequisite:** US-002 must be complete and provide:
- `snapshot.Snapshot` type with `Screenshots []snapshot.SnapshotScreenshot`
- `snapshot.SnapshotScreenshot` with `URL`, `Breakpoint`, `Width`, `Height`, `Path` fields
- `snapshot.LoadBaseline(projectDir string) (*snapshot.Snapshot, error)` or equivalent
- `snapshot.LoadLatest(projectDir string) (*snapshot.Snapshot, error)` or equivalent
- The `ks diff` CLI command scaffold in `cmd/diff.go`

### Task 3.1: Add `--screenshot-threshold` Flag

- **Sub-task 3.1.1:** In `RunDiff` (in `cmd/diff.go`), parse `--screenshot-threshold` flag using `getFlagValue(args, "--screenshot-threshold")`.
- **Sub-task 3.1.2:** Convert the string value to `float64` using `strconv.ParseFloat`. If parsing fails, emit `output.Fail` with a descriptive error.
- **Sub-task 3.1.3:** If flag is not provided, fall back to `.ks-project.json` field `screenshotThreshold` (see Task 3.2). If that is also absent, use `pixeldiff.DefaultOptions().Threshold` (0.01).
- **Sub-task 3.1.4:** Update `getNonFlagArgs` in `cmd/util.go` to recognize `--screenshot-threshold` as a flag that takes a value (add to the skip list).

---

### Task 3.2: Read `screenshotThreshold` from `.ks-project.json`

- **Sub-task 3.2.1:** Define a minimal `projectConfig` struct (or reuse the US-002 struct if available):
  ```go
  type projectConfig struct {
      ScreenshotThreshold float64 `json:"screenshotThreshold"`
  }
  ```
- **Sub-task 3.2.2:** Attempt to read `.ks-project.json` from the current working directory using `os.ReadFile`.
- **Sub-task 3.2.3:** If the file exists and `screenshotThreshold > 0`, use it as the threshold. If the file is absent or the field is zero, fall through to the default.
- **Sub-task 3.2.4:** Threshold resolution order: CLI flag > `.ks-project.json` > `DefaultOptions().Threshold`.

---

### Task 3.3: Implement URL Slug Sanitization

- **Sub-task 3.3.1:** Add helper `slugifyURL(u string) string` in `cmd/diff.go` (or `cmd/util.go`).
- **Sub-task 3.3.2:** Strip URL scheme (`https://`, `http://`) from the input string.
- **Sub-task 3.3.3:** Use `strings.Map` or `regexp.MustCompile("[^a-zA-Z0-9]+").ReplaceAllString` to replace all non-alphanumeric characters with `-`.
- **Sub-task 3.3.4:** Trim leading/trailing `-` characters.
- **Sub-task 3.3.5:** Limit slug length to avoid excessively long filenames (e.g., max 64 characters via `slug[:min(len(slug), 64)]`).

---

### Task 3.4: Implement Diff PNG Path Construction

- **Sub-task 3.4.1:** For each screenshot pair, determine the current snapshot directory (from US-002 snapshot metadata).
- **Sub-task 3.4.2:** Build the diff PNG filename using the convention:
  - Single URL: `diff-<breakpoint>-<width>x<height>.png`
  - Multi-URL: `diff-<url-slug>-<breakpoint>-<width>x<height>.png`
- **Sub-task 3.4.3:** Use `filepath.Join(currentSnapshotDir, filename)` to produce the full path. Do NOT use `path.Join` (wrong separator on Windows).
- **Sub-task 3.4.4:** Verify the sanitized slug cannot escape the snapshot directory (all non-alphanumeric chars replaced, no `..` or `/`).

---

### Task 3.5: Implement Screenshot Pair Iteration and Diff Execution

- **Sub-task 3.5.1:** Retrieve `baseline.Screenshots` and `current.Screenshots` from the snapshot data structures provided by US-002.
- **Sub-task 3.5.2:** Build a lookup map of `(URL, Breakpoint) → SnapshotScreenshot` from the baseline screenshots.
- **Sub-task 3.5.3:** Iterate `current.Screenshots`. For each entry:
  - Sub-task 3.5.3a: Look up matching baseline screenshot by `(URL, Breakpoint)` key. If no baseline match, skip with a note in output.
  - Sub-task 3.5.3b: Decode baseline PNG: `os.Open(baselineShot.Path)` → `png.Decode`.
  - Sub-task 3.5.3c: Decode current PNG: `os.Open(currentShot.Path)` → `png.Decode`.
  - Sub-task 3.5.3d: On decode error for either file, append an error entry to results and continue (do not abort entire diff).
  - Sub-task 3.5.3e: Build `pixeldiff.Options{Threshold: threshold, PixelTolerance: 10, HighlightColor: [4]uint8{255, 0, 0, 255}}`.
  - Sub-task 3.5.3f: Call `pixeldiff.Compare(baselineImg, currentImg, opts)`.
  - Sub-task 3.5.3g: If `result.DimensionMismatch == false`, call `pixeldiff.WriteDiffPNG(diffPath, diffImg)`. On write error, log but do not abort.
  - Sub-task 3.5.3h: Build `ScreenshotDiffEntry` (see Task 3.6) and append to results slice.
- **Sub-task 3.5.4:** Compute summary counters: `total`, `regressed` (count of entries where `Regressed == true`), `mismatch` (count where `DimensionMismatch == true`).

---

### Task 3.6: Define `ScreenshotDiffEntry` Output Type

- **Sub-task 3.6.1:** Define struct in `cmd/diff.go` (or a shared types file):
  ```go
  type ScreenshotDiffEntry struct {
      URL               string  `json:"url"`
      Breakpoint        string  `json:"breakpoint"`
      Width             int     `json:"width"`
      Height            int     `json:"height"`
      BaselinePath      string  `json:"baselinePath"`
      CurrentPath       string  `json:"currentPath"`
      DiffPath          string  `json:"diffPath,omitempty"`  // omitted on dimension mismatch
      SimilarityScore   float64 `json:"similarityScore"`
      DiffPixels        int     `json:"diffPixels"`
      TotalPixels       int     `json:"totalPixels"`
      DimensionMismatch bool    `json:"dimensionMismatch,omitempty"`
      Regressed         bool    `json:"regressed"`
      Threshold         float64 `json:"threshold"`
  }
  ```
- **Sub-task 3.6.2:** `DiffPath` must be omitted (`omitempty`) when `DimensionMismatch == true`.

---

### Task 3.7: Extend `ks diff` JSON Output

- **Sub-task 3.7.1:** Add `Screenshots []ScreenshotDiffEntry` field to the existing diff result payload struct (defined by US-002).
- **Sub-task 3.7.2:** Add `ScreenshotSummary` struct:
  ```go
  type ScreenshotSummary struct {
      Total     int `json:"total"`
      Regressed int `json:"regressed"`
      Mismatch  int `json:"mismatch"`
  }
  ```
- **Sub-task 3.7.3:** Populate `ScreenshotSummary` from the counters computed in Task 3.5.4.
- **Sub-task 3.7.4:** Pass the extended payload to `output.Success("diff", payload)`.

---

## Phase 4: Quality Gate Validation

**Goal:** Ensure `go test ./...` passes cleanly with no compilation errors or test failures.

### Task 4.1: Verify Package Compilation

- **Sub-task 4.1.1:** Run `go build ./pixeldiff/...` and confirm zero errors.
- **Sub-task 4.1.2:** Run `go build ./cmd/...` and confirm zero errors.
- **Sub-task 4.1.3:** Run `go build ./...` (entire project) and confirm zero errors.

---

### Task 4.2: Run Unit Tests

- **Sub-task 4.2.1:** Run `go test ./pixeldiff/...` and verify all 7 tests pass.
- **Sub-task 4.2.2:** Run `go test ./...` and verify no regressions in existing tests (audit, screenshot, contrast, etc.).

---

### Task 4.3: Verify Security Constraints

- **Sub-task 4.3.1:** Confirm `slugifyURL` replaces `/`, `.`, `:`, `?`, `#`, and all non-alphanumeric chars with `-`.
- **Sub-task 4.3.2:** Confirm diff paths are constructed via `filepath.Join(snapshotDir, sanitizedFilename)` only — no raw string concatenation.
- **Sub-task 4.3.3:** Confirm PNG decoding uses `image/png.Decode` with error propagation (no panic paths).

---

## File Summary

| File | Status | Owner |
|---|---|---|
| `pixeldiff/diff.go` | New | US-004 |
| `pixeldiff/diff_test.go` | New | US-004 |
| `cmd/diff.go` | Extended (owned by US-002) | US-004 extends |
| `cmd/util.go` | Minor update (`--screenshot-threshold` flag) | US-004 |

## Dependency Note

Phase 3 (cmd/diff.go integration) requires US-002 to have already delivered:
- `snapshot` package with `Snapshot` and `SnapshotScreenshot` types
- `cmd/diff.go` scaffold with `RunDiff` function and snapshot loading logic
- `.ks-project.json` config schema (the `screenshotThreshold` field extends this)

Phases 1 and 2 (`pixeldiff` package and tests) are fully independent of US-002 and can be implemented immediately.
