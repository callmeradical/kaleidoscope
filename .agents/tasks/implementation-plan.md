# Implementation Plan: Screenshot Pixel Diff (US-004)

## Overview

Implements pure Go pixel-level visual comparison between baseline and current screenshots. Produces a diff PNG with highlighted difference regions, a similarity score (0.0–1.0), and integrates into `ks diff` JSON output. No third-party dependencies — uses only Go standard library (`image`, `image/color`, `image/draw`, `image/png`, `math`, `os`).

**Depends on**: US-002 snapshot directory layout (`.kaleidoscope/snapshots/<id>/screenshots/<WxH>.png`)

---

## Phase 1: Core Pixel Diff Engine (`diff` package)

### Task 1.1 — Create `diff/pixeldiff.go`

**Sub-tasks:**

1. **Create the `diff` package directory** at `/workspace/diff/`.

2. **Define package-level constants and defaults:**
   - `DefaultSimilarityThreshold = 0.95`
   - `perPixelTolerance = 10.0` (Euclidean RGB distance, 0–441.67 range)
   - `var DefaultHighlightColor = color.RGBA{R: 255, G: 0, B: 80, A: 255}` (crimson)

3. **Define `PixelDiffResult` struct:**
   ```go
   type PixelDiffResult struct {
       SimilarityScore   float64
       ChangedPixels     int
       TotalPixels       int
       DiffImagePath     string
       DiffImageBytes    []byte
       DimensionMismatch bool
       Regressed         bool
   }
   ```

4. **Define `PixelDiffOptions` struct:**
   ```go
   type PixelDiffOptions struct {
       Threshold      float64
       HighlightColor color.RGBA
       OutputPath     string
   }
   ```

5. **Implement `pixelDistance(a, b color.Color) float64`:**
   - Extract R, G, B channels from both colors (cast via `color.RGBAModel`)
   - Compute Euclidean distance: `sqrt((r1-r2)^2 + (g1-g2)^2 + (b1-b2)^2)`
   - Return float64 in range 0–441.67

6. **Implement `renderDiff(baseline, current image.Image, highlight color.RGBA) *image.RGBA`:**
   - Allocate new `*image.RGBA` with same bounds as baseline
   - For each pixel (x, y):
     - If `pixelDistance(baseline pixel, current pixel) > perPixelTolerance`: paint `highlight` color
     - Else: paint baseline pixel at 30% alpha blend (blend with black background at 30% opacity to dim unchanged areas for context)
   - Return resulting `*image.RGBA`

7. **Implement `CompareBytes(baselinePNG, currentPNG []byte, opts PixelDiffOptions) (PixelDiffResult, error)`:**
   - Decode both byte slices with `image/png.Decode`
   - Return error on invalid PNG
   - **Dimension mismatch check**: if bounds differ, return `PixelDiffResult{DimensionMismatch: true, SimilarityScore: 0.0, Regressed: true}` — no error, no panic
   - Iterate all pixels; count those where `pixelDistance > perPixelTolerance`
   - Compute `SimilarityScore = 1.0 - float64(changedPixels) / float64(totalPixels)`
   - Call `renderDiff` to build diff image
   - Encode diff image to PNG bytes via `image/png.Encode`
   - Apply defaults for zero-value opts fields (threshold → `DefaultSimilarityThreshold`, highlight → `DefaultHighlightColor`)
   - If `opts.OutputPath != ""`: write `DiffImageBytes` to disk
   - Set `Regressed = SimilarityScore < opts.Threshold`
   - Populate and return `PixelDiffResult`

8. **Implement `CompareFiles(baselinePath, currentPath string, opts PixelDiffOptions) (PixelDiffResult, error)`:**
   - Read both files with `os.ReadFile`
   - Return error on file not found / unreadable
   - Delegate to `CompareBytes`
   - If `opts.OutputPath` is set, it propagates through `CompareBytes` for disk write

---

## Phase 2: Unit Tests for `diff` Package

### Task 2.1 — Create `diff/pixeldiff_test.go`

**Sub-tasks:**

1. **`TestIdenticalImages`:**
   - Generate two identical in-memory PNGs (e.g., solid color 100x100)
   - Call `CompareBytes`; assert `SimilarityScore == 1.0`, `ChangedPixels == 0`, `Regressed == false`

2. **`TestCompletelyDifferent`:**
   - Generate solid white PNG and solid black PNG of same dimensions
   - Call `CompareBytes`; assert `SimilarityScore < 0.01`, `Regressed == true`

3. **`TestPartialChange`:**
   - Generate a base 100x100 PNG (solid color)
   - Copy it and paint a 10x10 region a different color (100 pixels changed out of 10000)
   - Call `CompareBytes`; assert `0.9 < SimilarityScore < 1.0`, `ChangedPixels == 100`

4. **`TestDimensionMismatch`:**
   - Generate two PNGs with different dimensions (e.g., 100x100 vs 200x200)
   - Call `CompareBytes`; assert `DimensionMismatch == true`, `SimilarityScore == 0.0`, no panic, no error

5. **`TestThresholdFlag`:**
   - Generate images with ~97% similarity (small change)
   - Call `CompareBytes` with `Threshold=0.98` → assert `Regressed == true`
   - Call `CompareBytes` with `Threshold=0.95` → assert `Regressed == false`

6. **`TestDiffImageOutput`:**
   - Generate two different PNGs; call `CompareBytes`
   - Assert `DiffImageBytes` is non-nil
   - Re-decode `DiffImageBytes` with `image/png.Decode` → assert no error (valid PNG)
   - Sample pixels at known changed locations → assert they equal `DefaultHighlightColor`

7. **`TestCompareFiles`:**
   - Write two PNGs to `t.TempDir()`
   - Call `CompareFiles` with a temp output path
   - Assert result matches a direct `CompareBytes` call on the same data
   - Assert diff PNG file exists on disk at the specified output path

---

## Phase 3: `ks diff` Command (`cmd/diff.go`)

### Task 3.1 — Create `cmd/diff.go`

**Sub-tasks:**

1. **Define the `ScreenshotDiffEntry` result struct (local to `cmd` package):**
   ```go
   type ScreenshotDiffEntry struct {
       URL               string      `json:"url"`
       Breakpoint        Breakpoint  `json:"breakpoint"`
       BaselinePath      string      `json:"baselinePath"`
       CurrentPath       string      `json:"currentPath"`
       DiffPath          string      `json:"diffPath,omitempty"`
       SimilarityScore   *float64    `json:"similarityScore"`  // nil when no comparison possible
       ChangedPixels     int         `json:"changedPixels"`
       TotalPixels       int         `json:"totalPixels"`
       DimensionMismatch bool        `json:"dimensionMismatch"`
       Regressed         bool        `json:"regressed"`
   }

   type Breakpoint struct {
       Width  int `json:"width"`
       Height int `json:"height"`
   }

   type DiffResult struct {
       SnapshotID      string                `json:"snapshotID"`
       BaselineID      string                `json:"baselineID"`
       ScreenshotDiffs []ScreenshotDiffEntry `json:"screenshotDiffs"`
       Threshold       float64               `json:"threshold"`
       AnyRegressed    bool                  `json:"anyRegressed"`
   }
   ```

2. **Implement flag parsing in `RunDiff(args []string)`:**
   - `--snapshot <id>`: snapshot to compare (default: "latest", resolved at runtime)
   - `--baseline <id>`: baseline snapshot ID (default: read from `.kaleidoscope/baselines.json`)
   - `--threshold <float>`: similarity threshold (default: `diff.DefaultSimilarityThreshold` = 0.95)
   - Register `--snapshot`, `--baseline`, `--threshold` in `cmd/util.go`'s `getNonFlagArgs` skip list

3. **Implement snapshot ID resolution:**
   - Determine state dir via `browser.StateDir()`
   - If `--snapshot` is `"latest"` or empty: enumerate `snapshots/` subdirectories, pick the lexicographically last directory name
   - If `--baseline` is empty: read `.kaleidoscope/baselines.json` and extract the default baseline ID; fail with descriptive `output.Fail` if not configured
   - **Validate IDs**: reject snapshot/baseline IDs containing `..`, `/`, or characters outside `[a-zA-Z0-9_-]` to prevent path traversal

4. **Implement screenshot pair enumeration:**
   - Scan `<stateDir>/snapshots/<snapshotID>/screenshots/` for `*.png` files
   - Scan `<stateDir>/snapshots/<baselineID>/screenshots/` for `*.png` files
   - Parse filenames as `<width>x<height>.png` → extract `Breakpoint`
   - Build three sets: in-both, only-in-current (new), only-in-baseline (removed)

5. **Implement diff execution loop:**
   - For each screenshot pair **in both** snapshots:
     - Compute output diff path: `<stateDir>/snapshots/<snapshotID>/diffs/<width>x<height>-diff.png`
     - Create `diffs/` directory via `os.MkdirAll`
     - Call `diff.CompareFiles(baselinePath, currentPath, opts)` where `opts.OutputPath` = diff path, `opts.Threshold` = parsed threshold
     - Build `ScreenshotDiffEntry` from result; set `DiffPath` to diff path
   - For each screenshot **only in current** (new):
     - Append entry with `SimilarityScore: nil`, `Regressed: false`
   - For each screenshot **only in baseline** (removed):
     - Append entry with `SimilarityScore: nil`, `Regressed: true`

6. **Compute `AnyRegressed`** by scanning all entries for `Regressed == true`.

7. **Call `output.Success("diff", result)`** with the assembled `DiffResult`.

8. **Exit with code 2** on fatal errors (missing baseline config, unreadable snapshot dir, invalid IDs); use `output.Fail` + `os.Exit(2)`.

---

## Phase 4: Register `diff` Command in `main.go`

### Task 4.1 — Wire `RunDiff` into the command router

**Sub-tasks:**

1. **Add `case "diff":` to the `switch` in `main.go`:**
   ```go
   case "diff":
       cmd.RunDiff(cmdArgs)
   ```

2. **Add `diff` to the usage string** in `main.go` under a new "Regression Detection" section:
   ```
   Regression Detection:
     diff [options]          Compare snapshots for visual/audit regressions
   ```

3. **Add `--snapshot`, `--baseline`, `--threshold` to the flag skip list** in `cmd/util.go`'s `getNonFlagArgs` so these flags are not treated as positional arguments.

---

## Phase 5: Command Tests (`cmd/diff_test.go`)

### Task 5.1 — Create `cmd/diff_test.go`

**Sub-tasks:**

1. **`TestDiffJSONOutput`:**
   - Set up a mock snapshot directory tree in `t.TempDir()`:
     - `snapshots/baseline-001/screenshots/1280x720.png` (solid color A)
     - `snapshots/snap-002/screenshots/1280x720.png` (solid color B, slightly different)
     - `baselines.json` with `{"defaultBaseline": "baseline-001"}`
   - Call `RunDiff` with `--snapshot snap-002 --baseline baseline-001`
   - Capture stdout; unmarshal JSON; assert `ok == true`, `screenshotDiffs` has one entry, `similarityScore` is present

2. **`TestDiffMissingBaseline`:**
   - Set up snapshot dir with no `baselines.json`
   - Call `RunDiff` without `--baseline`
   - Assert JSON output has `ok == false` and a descriptive `error` message

3. **`TestDiffThresholdFlag`:**
   - Set up snapshots with known similarity (e.g., identical images → score 1.0)
   - Call with `--threshold 0.99`; assert `threshold == 0.99` in JSON result
   - Call with `--threshold 0.95`; assert `threshold == 0.95` in JSON result

---

## Phase 6: Verification

### Task 6.1 — Run quality gate

**Sub-tasks:**

1. **Run `go build ./...`** to confirm the package compiles without errors.
2. **Run `go test ./...`** to confirm all tests pass (the PRD quality gate).
3. **Fix any compilation or test failures** before marking complete.

---

## File Checklist

| File | Action |
|---|---|
| `diff/pixeldiff.go` | Create (new package) |
| `diff/pixeldiff_test.go` | Create (unit tests) |
| `cmd/diff.go` | Create (new command) |
| `cmd/diff_test.go` | Create (command tests) |
| `cmd/util.go` | Edit — add `--snapshot`, `--baseline`, `--threshold` to flag skip list |
| `main.go` | Edit — add `case "diff"` and usage string entry |

---

## Key Constraints (from PRD rules & tech spec)

- **Pure Go only**: no `exec.Command`, no ImageMagick, no third-party image libraries
- **Packages**: `image`, `image/color`, `image/draw`, `image/png`, `math`, `os` only
- **No panic on dimension mismatch**: return `DimensionMismatch=true` in result
- **Path traversal safety**: validate snapshot IDs before constructing file paths
- **Exit codes**: exit 2 on fatal error; exit 0 even when `anyRegressed: true`
- **Diff PNGs**: written only inside `.kaleidoscope/snapshots/<id>/diffs/`
- **Output convention**: all output via `output.Success` / `output.Fail` (JSON)
- **`SimilarityScore`**: pointer (`*float64`) to support `null` JSON for unmatched screenshots
