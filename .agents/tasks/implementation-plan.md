# Implementation Plan: US-004 — Screenshot Pixel Diff

## Context

**Story:** As a developer, I want pixel-level visual comparison between baseline and current screenshots that produces a highlight overlay image, so that subtle visual changes are surfaced.

**Module:** `github.com/callmeradical/kaleidoscope`
**Quality Gate:** `go test ./...` must pass

### Current State (from codebase exploration)

- `analysis/` package exists with contrast, overlap, spacing, touch targets, typography modules — **no pixel diff module yet**
- `snapshot/` package does **not** exist — must be created (US-002 prerequisite)
- `cmd/diff.go` does **not** exist — must be created
- `diff` command is **not** registered in `main.go`
- `cmd/breakpoints.go` defines `defaultBreakpoints` (mobile 375×812, tablet 768×1024, desktop 1280×720, wide 1920×1080)
- All commands use `output.Success` / `output.Fail` JSON convention
- Screenshots are always PNG; `browser.ScreenshotDir()` returns the screenshot directory

### Files to Create

| File | Purpose |
|---|---|
| `snapshot/snapshot.go` | `Snapshot`, `SnapshotScreenshot`, `ScreenshotDiff`, `DiffOutput` types |
| `analysis/pixeldiff.go` | Pure `CompareImages()` diff engine |
| `analysis/pixeldiff_writer.go` | `WriteDiffImage()` I/O helper |
| `analysis/pixeldiff_test.go` | Unit tests for diff engine |
| `cmd/diff.go` | `RunDiff()` command — Phase 5 (screenshot diff) |

### Files to Modify

| File | Change |
|---|---|
| `main.go` | Register `diff` command in switch + usage string |
| `cmd/usage.go` | Add `diff` usage entry with `screenshotDiffs` field documentation |

---

## Phase 1 — Snapshot Package (US-002 Prerequisite Types)

Create the minimum snapshot infrastructure needed by US-004. This is a data-only package with no Chrome dependency.

### Task 1.1 — Create `snapshot/snapshot.go`

**File:** `/workspace/snapshot/snapshot.go`
**Package:** `github.com/callmeradical/kaleidoscope/snapshot`

#### Sub-task 1.1.1 — Declare package and imports
- Package declaration: `package snapshot`
- Imports: `encoding/json`, `os`, `time`

#### Sub-task 1.1.2 — Define `SnapshotScreenshot` struct
```go
type SnapshotScreenshot struct {
    URL        string `json:"url"`
    Breakpoint string `json:"breakpoint"`
    Path       string `json:"path"`
}
```

#### Sub-task 1.1.3 — Define `Snapshot` struct
```go
type Snapshot struct {
    ID          string               `json:"id"`
    CapturedAt  time.Time            `json:"capturedAt"`
    Screenshots []SnapshotScreenshot `json:"screenshots"`
}
```

#### Sub-task 1.1.4 — Define `ScreenshotDiff` struct
```go
type ScreenshotDiff struct {
    URL                  string  `json:"url"`
    Breakpoint           string  `json:"breakpoint"`
    BaselinePath         string  `json:"baselinePath"`
    CurrentPath          string  `json:"currentPath"`
    DiffPath             string  `json:"diffPath"`
    Similarity           float64 `json:"similarity"`
    Regressed            bool    `json:"regressed"`
    MismatchedDimensions bool    `json:"mismatchedDimensions"`
}
```

#### Sub-task 1.1.5 — Define `DiffOutput` struct
```go
type DiffOutput struct {
    Baseline            time.Time        `json:"baseline"`
    Current             time.Time        `json:"current"`
    ScreenshotDiffs     []ScreenshotDiff `json:"screenshotDiffs"`
    ScreenshotRegressed bool             `json:"screenshotRegressed"`
    Regressed           bool             `json:"regressed"`
}
```

#### Sub-task 1.1.6 — Implement `LoadSnapshot(path string) (*Snapshot, error)`
- Open JSON file at `path`, decode into `*Snapshot`, return
- Returns `(nil, error)` on any I/O or parse failure

#### Sub-task 1.1.7 — Implement `SaveSnapshot(path string, s *Snapshot) error`
- Marshal `*Snapshot` to indented JSON, write to `path` with `0644` permissions

---

## Phase 2 — Pure Pixel Diff Engine

Create the pure-function diff module with no I/O, no browser, no external dependencies.

### Task 2.1 — Create `analysis/pixeldiff.go`

**File:** `/workspace/analysis/pixeldiff.go`
**Package:** `github.com/callmeradical/kaleidoscope/analysis`
**Allowed imports:** `image`, `image/color`, `math` (no I/O packages)

#### Sub-task 2.1.1 — Define `ImageDiffResult` struct
```go
type ImageDiffResult struct {
    Similarity           float64     `json:"similarity"`
    DifferentPixels      int         `json:"differentPixels"`
    TotalPixels          int         `json:"totalPixels"`
    DiffImage            image.Image `json:"-"`
    MismatchedDimensions bool        `json:"mismatchedDimensions"`
}
```

#### Sub-task 2.1.2 — Implement `CompareImages(a, b image.Image, channelThreshold uint8) ImageDiffResult`

Step-by-step algorithm:

1. **Dimension check:** Compare `a.Bounds()` vs `b.Bounds()`. If unequal, return `ImageDiffResult{MismatchedDimensions: true, Similarity: 0.0}` immediately — no panic.

2. **Allocate overlay:** Create `*image.NRGBA` with same bounds as `a`. Initialize all pixels to black `color.NRGBA{R:0, G:0, B:0, A:255}` (dark background for visual contrast).

3. **Pixel iteration loop** over every `(x, y)` in `a.Bounds()`:
   - Decode pixel A: `color.NRGBAModel.Convert(a.At(x, y)).(color.NRGBA)`
   - Decode pixel B: `color.NRGBAModel.Convert(b.At(x, y)).(color.NRGBA)`
   - Compute per-channel absolute difference using `math.Abs` for R, G, B (cast to float64, then back)
   - **If any channel diff > `channelThreshold`:**
     - Increment `differentPixels`
     - Paint overlay pixel **red**: `color.NRGBA{R:255, G:0, B:0, A:255}`
   - **Else (unchanged pixel):**
     - Copy pixel A at 50% alpha: `color.NRGBA{R:pA.R, G:pA.G, B:pA.B, A:128}`
   - Set overlay pixel with `overlay.SetNRGBA(x, y, overlayPixel)`

4. **Compute similarity:** `similarity = 1.0 - float64(differentPixels)/float64(totalPixels)`

5. **Return** `ImageDiffResult{Similarity: similarity, DifferentPixels: differentPixels, TotalPixels: totalPixels, DiffImage: overlay}`

#### Sub-task 2.1.3 — Implement `absDiff(a, b uint8) uint8` helper
- Returns `|a - b|` as `uint8` without overflow (use int intermediary)
- Used internally by `CompareImages` to avoid subtraction underflow

---

### Task 2.2 — Create `analysis/pixeldiff_writer.go`

**File:** `/workspace/analysis/pixeldiff_writer.go`
**Package:** `github.com/callmeradical/kaleidoscope/analysis`
**Imports:** `fmt`, `image`, `image/png`, `os`, `path/filepath`

#### Sub-task 2.2.1 — Implement `sanitizeFilename(s string) string`
- Use `strings.Map` with a whitelist function: allow `[a-zA-Z0-9._-]`, replace everything else with `-`
- This prevents path traversal when URL is used as part of filename
- Add `strings` to imports

#### Sub-task 2.2.2 — Implement `WriteDiffImage(dir, baseName string, diffImg image.Image) (string, error)`
- Sanitize `baseName` via `sanitizeFilename`
- Build output path: `filepath.Join(dir, fmt.Sprintf("diff-%s.png", sanitizedName))`
- Open file with `os.Create` (caller must ensure `dir` exists)
- Encode `diffImg` to PNG with `png.Encode`
- Close file, return absolute path
- Return error if `os.Create` or `png.Encode` fails

---

## Phase 3 — Unit Tests

### Task 3.1 — Create `analysis/pixeldiff_test.go`

**File:** `/workspace/analysis/pixeldiff_test.go`
**Package:** `analysis`
**Allowed imports:** `image`, `image/color`, `image/draw`, `image/png`, `os`, `testing`

#### Sub-task 3.1.1 — Helper: `solidImage(w, h int, c color.NRGBA) *image.NRGBA`
- Creates a uniform solid-color image of dimensions `w×h`
- Used across multiple tests to avoid boilerplate

#### Sub-task 3.1.2 — `TestCompareImages_Identical`
- Create image A: 100×100 solid blue
- Create image B: 100×100 solid blue (identical)
- Call `CompareImages(a, b, 10)`
- Assert: `result.Similarity == 1.0`
- Assert: `result.DifferentPixels == 0`
- Assert: `result.MismatchedDimensions == false`
- Assert: `result.DiffImage != nil`

#### Sub-task 3.1.3 — `TestCompareImages_TotallyDifferent`
- Image A: 10×10 solid white `{255, 255, 255, 255}`
- Image B: 10×10 solid black `{0, 0, 0, 255}`
- Call `CompareImages(a, b, 10)`
- Assert: `result.Similarity == 0.0`
- Assert: `result.DifferentPixels == 100` (10×10)
- Assert: `result.TotalPixels == 100`

#### Sub-task 3.1.4 — `TestCompareImages_PartialDiff`
- Image A: 10×10 solid white
- Image B: 10×10, left half white, right half red `{255, 0, 0, 255}`
- Call `CompareImages(a, b, 10)`
- Assert: `result.Similarity` is approximately 0.5 (±0.01 tolerance)
- Use `math.Abs(result.Similarity - 0.5) < 0.01`

#### Sub-task 3.1.5 — `TestCompareImages_MismatchedDimensions`
- Image A: 100×100 solid white
- Image B: 200×200 solid white
- Call `CompareImages(a, b, 10)` — must **not** panic
- Assert: `result.MismatchedDimensions == true`
- Assert: `result.Similarity == 0.0`
- Assert: `result.DiffImage == nil`

#### Sub-task 3.1.6 — `TestCompareImages_ChannelThreshold`
- Image A: 10×10 solid `{100, 100, 100, 255}`
- Image B same except one pixel at (0,0): `{110, 100, 100, 255}` (diff = 10, exactly at threshold)
- Call with `channelThreshold = 10`: pixel at (0,0) should NOT be counted as different
- Call with `channelThreshold = 9`: pixel at (0,0) SHOULD be counted as different
- Assert `DifferentPixels == 0` for threshold 10; `DifferentPixels == 1` for threshold 9

#### Sub-task 3.1.7 — `TestCompareImages_DiffImageIsRed`
- Image A: 1×1 solid white `{255, 255, 255, 255}`
- Image B: 1×1 solid black `{0, 0, 0, 255}`
- Call `CompareImages(a, b, 10)`
- Assert overlay pixel at (0,0) is `color.NRGBA{R:255, G:0, B:0, A:255}` (red)

#### Sub-task 3.1.8 — `TestCompareImages_DiffImageIsAttenuated`
- Image A: 1×1 solid `{200, 100, 50, 255}`
- Image B: 1×1 solid same `{200, 100, 50, 255}` (identical)
- Call `CompareImages(a, b, 10)`
- Assert overlay pixel at (0,0) has `A == 128` (50% alpha, dimmed unchanged area)
- Assert R, G, B match the original pixel values

#### Sub-task 3.1.9 — `TestWriteDiffImage_Roundtrip`
- Create a 50×30 solid green diff image
- Write to `t.TempDir()` via `WriteDiffImage`
- Open written file, decode PNG
- Assert decoded image bounds == 50×30
- Assert no error at any step
- Verify returned path ends with `diff-*.png`

---

## Phase 4 — `ks diff` Command Integration

### Task 4.1 — Create `cmd/diff.go`

**File:** `/workspace/cmd/diff.go`
**Package:** `cmd`
**Imports:** `encoding/json`, `fmt`, `image/png`, `os`, `strings`, `github.com/callmeradical/kaleidoscope/analysis`, `github.com/callmeradical/kaleidoscope/output`, `github.com/callmeradical/kaleidoscope/snapshot`

#### Sub-task 4.1.1 — Define `ProjectConfig` struct
```go
type ProjectConfig struct {
    Project             string  `json:"project"`
    ScreenshotThreshold float64 `json:"screenshotThreshold"`
}
```

#### Sub-task 4.1.2 — Implement `loadProjectConfig() ProjectConfig`
- Look for `.ks-project.json` in the current working directory
- If absent or unreadable: return `ProjectConfig{ScreenshotThreshold: 0.99}` (default)
- If `ScreenshotThreshold` is zero (field missing): set to `0.99`
- Parse via `encoding/json`

#### Sub-task 4.1.3 — Implement `loadPNG(path string) (image.Image, error)`
- Open file with `os.Open`
- Decode with `png.Decode`
- Close file, return `(image.Image, error)`

#### Sub-task 4.1.4 — Implement `findMatchingScreenshot(screenshots []snapshot.SnapshotScreenshot, url, breakpoint string) *snapshot.SnapshotScreenshot`
- Iterate `screenshots`, return pointer to first entry where `ss.URL == url && ss.Breakpoint == breakpoint`
- Return `nil` if not found

#### Sub-task 4.1.5 — Implement `sanitizeDiffName(url, breakpoint string) string`
- Combine `url + "-" + breakpoint`
- Apply same character whitelist as `analysis.sanitizeFilename` (only `[a-zA-Z0-9._-]`)
- Strip URL scheme prefix (`https://`, `http://`) first for cleaner names
- Use `strings.Map` with whitelist — no shell execution, no `regexp` import needed

#### Sub-task 4.1.6 — Implement `diffScreenshots(baseline, current *snapshot.Snapshot, snapshotDir string, threshold float64) []snapshot.ScreenshotDiff`
- Iterate `current.Screenshots` (one entry per URL+breakpoint)
- For each entry, call `findMatchingScreenshot` against `baseline.Screenshots`
- If no baseline match: skip (no entry added — URL is new since baseline)
- Load baseline PNG via `loadPNG(baselineSS.Path)`
- Load current PNG via `loadPNG(bp.Path)`
- If either load fails: produce a `ScreenshotDiff` with `Similarity: 0.0`, `Regressed: true`, and an empty `DiffPath`
- Call `analysis.CompareImages(imgA, imgB, 10)` (channel threshold hard-coded 10)
- If `!diffResult.MismatchedDimensions`:
  - Build `baseName` via `sanitizeDiffName(bp.URL, bp.Breakpoint)`
  - Call `analysis.WriteDiffImage(snapshotDir, baseName, diffResult.DiffImage)`
  - Store returned path in `DiffPath`
- Append `snapshot.ScreenshotDiff{...}` with all fields populated
- Return `[]snapshot.ScreenshotDiff`

#### Sub-task 4.1.7 — Implement `RunDiff(args []string)`
- Parse args: `--baseline <path>` and `--current <path>` flags via `getFlagValue`
- Fail early if either path is empty: `output.Fail("diff", ..., "Usage: ks diff --baseline <path> --current <path>")`
- Load baseline snapshot via `snapshot.LoadSnapshot(baselinePath)`
- Load current snapshot via `snapshot.LoadSnapshot(currentPath)`
- Load project config via `loadProjectConfig()`
- Determine `snapshotDir`: directory of the current snapshot path (`filepath.Dir(currentPath)`)
- Call `diffScreenshots(baseline, current, snapshotDir, config.ScreenshotThreshold)`
- Compute `screenshotRegressed`: `true` if any `ScreenshotDiff.Regressed == true`
- Assemble `snapshot.DiffOutput{...}` and emit via `output.Success("diff", diffOutput)`

---

## Phase 5 — Wire Up in `main.go`

### Task 5.1 — Register `diff` command in `main.go`

#### Sub-task 5.1.1 — Add `diff` case to the switch statement
```go
case "diff":
    cmd.RunDiff(cmdArgs)
```

#### Sub-task 5.1.2 — Add `diff` to the usage string in `main.go`
Add under `UX Evaluation:` section or a new `Snapshots:` section:
```
Snapshots:
  diff [options]          Compare baseline vs current snapshot screenshots
```

---

## Phase 6 — Document Usage

### Task 6.1 — Update `cmd/usage.go`

#### Sub-task 6.1.1 — Add `diff` usage entry
Find the usage map in `cmd/usage.go` (the `PrintUsage` function's dispatch map) and add:
```
"diff": usageDiff
```
where `usageDiff` describes flags and output fields:

```
ks diff --baseline <snapshot.json> --current <snapshot.json>

Compares baseline and current snapshots for screenshot pixel differences.

Flags:
  --baseline <path>   Path to the baseline snapshot JSON file
  --current <path>    Path to the current snapshot JSON file

Output fields (screenshotDiffs[]):
  url                 Page URL for this diff
  breakpoint          Breakpoint name (mobile, tablet, desktop, wide)
  baselinePath        Path to baseline screenshot PNG
  currentPath         Path to current screenshot PNG
  diffPath            Path to diff overlay PNG (empty if mismatched dimensions)
  similarity          Score from 0.0 (fully different) to 1.0 (identical)
  regressed           true if similarity < screenshotThreshold in .ks-project.json
  mismatchedDimensions true if images had different pixel dimensions

screenshotRegressed:  true if any screenshot pair regressed
```

---

## Dependency & Ordering Notes

- **Phases 1, 2, 3 are independent** and can be implemented in parallel. Phase 1 creates types; Phase 2 creates the pure engine; Phase 3 tests Phase 2.
- **Phase 4 depends on Phases 1 and 2** — `cmd/diff.go` imports both `snapshot` and `analysis` packages.
- **Phase 5 depends on Phase 4** — `main.go` must reference `cmd.RunDiff`.
- **Phase 6 depends on Phase 4** — usage documentation follows the command signature.
- Tests (Phase 3) must be written before or alongside Phase 2 implementation to satisfy the TDD requirement.

## Acceptance Criteria Mapping

| Acceptance Criterion | Covered By |
|---|---|
| Given two PNGs of same dimensions → diff PNG highlighting changed pixels | Phase 2 Task 2.1 (`CompareImages` red highlight algorithm) + Phase 2 Task 2.2 (`WriteDiffImage`) |
| Returns similarity score (0.0–1.0) per screenshot pair | Phase 2 Task 2.1 Sub-task 2.1.2 step 4 |
| Diff images written to snapshot directory alongside source screenshots | Phase 4 Task 4.1 Sub-task 4.1.6 |
| `ks diff` JSON output includes screenshot diff results | Phase 4 Task 4.1 Sub-task 4.1.7 + Phase 5 |
| Handles mismatched dimensions gracefully (no panic) | Phase 2 Task 2.1 Sub-task 2.1.2 step 1 + Phase 3 Task 3.1 Sub-task 3.1.5 |
| Configurable similarity threshold from `.ks-project.json` | Phase 4 Task 4.1 Sub-tasks 4.1.1–4.1.2 |
