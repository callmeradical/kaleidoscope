# Tech Spec: US-004 — Screenshot Pixel Diff

**Story:** As a developer, I want pixel-level visual comparison between baseline and current screenshots that produces a highlight overlay image, so that subtle visual changes are surfaced.

**Status:** open
**Depends on:** US-002 (Snapshot system — provides baseline screenshot paths)

---

## Architecture Overview

The pixel diff feature is a pure-Go image comparison engine with no external dependencies beyond the Go standard library (`image`, `image/png`, `image/draw`). It operates as:

1. A **`diff` package** (`/workspace/diff/`) — stateless, pure-function pixel comparison engine.
2. A **`ks diff` command** (`/workspace/cmd/diff.go`) — CLI integration that loads baseline and current snapshot screenshots, invokes the diff engine, writes diff PNG output, and emits JSON results.

The diff engine has zero dependencies on Chrome/rod and can be unit-tested entirely with synthetic images.

This spec covers the pixel diff subsystem. Audit/element diff integration points (also part of `ks diff`) are noted where relevant but not fully specified here.

---

## Component Design

### 1. `diff` Package (`/workspace/diff/pixel.go`)

**Responsibility:** Pure-function pixel comparison. Takes two `image.Image` values, returns a diff result including a highlighted diff image and a similarity score.

#### Types

```go
package diff

import "image"

// ScreenshotDiffResult holds the outcome of comparing two screenshots.
type ScreenshotDiffResult struct {
    // BaselinePath is the source path of the baseline screenshot.
    BaselinePath string `json:"baselinePath"`
    // CurrentPath is the source path of the current screenshot.
    CurrentPath string `json:"currentPath"`
    // DiffPath is the output path of the generated diff PNG.
    DiffPath string `json:"diffPath"`
    // Similarity is a value between 0.0 (completely different) and 1.0 (identical).
    Similarity float64 `json:"similarity"`
    // Regressed is true when Similarity is below the configured threshold.
    Regressed bool `json:"regressed"`
    // DimensionMismatch is true when the two images have different sizes.
    DimensionMismatch bool `json:"dimensionMismatch,omitempty"`
    // Width and Height of the comparison (baseline dimensions, or 0 on mismatch).
    Width  int `json:"width"`
    Height int `json:"height"`
}

// Options controls diff behavior.
type Options struct {
    // Threshold is the per-channel tolerance for considering two pixels different.
    // Range: 0–255. Default: 10.
    Threshold uint8
    // SimilarityThreshold is the minimum score to NOT flag as regressed.
    // Range: 0.0–1.0. Default: 0.99.
    SimilarityThreshold float64
    // HighlightColor is the RGBA color used to mark changed pixels in the diff image.
    // Default: {255, 0, 0, 255} (opaque red).
    HighlightColor [4]uint8
}

// DefaultOptions returns sensible default Options.
func DefaultOptions() Options

// CompareImages compares two images and returns a ScreenshotDiffResult.
// It does NOT write the diff image to disk; that is the caller's responsibility.
// If dimensions differ, DimensionMismatch is set to true, Similarity is 0.0,
// and the returned diff image is nil.
func CompareImages(baseline, current image.Image, opts Options) (ScreenshotDiffResult, image.Image)
```

#### Algorithm: `CompareImages`

1. **Dimension check:** If `baseline.Bounds()` != `current.Bounds()`, return `DimensionMismatch: true`, `Similarity: 0.0`, `Regressed: true`, and a `nil` diff image. No panic.

2. **Pixel iteration:** For each pixel `(x, y)` in the image bounds:
   - Extract RGBA components of both baseline and current pixels using `image.At(x,y).RGBA()` (returns 16-bit values; shift right by 8 to get 8-bit).
   - A pixel is considered *different* if any channel (R, G, B) differs by more than `opts.Threshold`.
   - Count different pixels as `diffCount`.

3. **Diff image construction:** Create a new `image.RGBA` of the same dimensions.
   - For *identical* pixels: copy the baseline pixel value.
   - For *different* pixels: fill with `opts.HighlightColor`.

4. **Similarity score:** `similarity = 1.0 - (float64(diffCount) / float64(totalPixels))`

5. **Regression flag:** `regressed = similarity < opts.SimilarityThreshold`

6. Return `ScreenshotDiffResult` (without `DiffPath` set — caller sets that) and the diff `image.Image`.

#### File I/O Helper (`/workspace/diff/io.go`)

```go
// LoadPNG reads and decodes a PNG file.
func LoadPNG(path string) (image.Image, error)

// SavePNG encodes and writes an image as PNG to path.
func SavePNG(path string, img image.Image) error

// DiffImagePath derives the output path for a diff PNG given a snapshot directory
// and the two source filenames.
// Example: DiffImagePath("/snap/dir", "baseline-mobile.png", "current-mobile.png")
//          => "/snap/dir/diff-mobile-<timestamp>.png"
func DiffImagePath(snapshotDir, baselineFile, currentFile string) string
```

---

### 2. `ks diff` Command (`/workspace/cmd/diff.go`)

**Responsibility:** Load baseline and current snapshot data (from US-002 snapshot storage), run the pixel diff engine for each screenshot pair, write diff PNGs, and emit the combined JSON result.

> **Note:** US-002 defines the snapshot storage format. This spec assumes snapshots are stored as JSON manifests under `.kaleidoscope/snapshots/<id>/snapshot.json` containing screenshot paths keyed by URL + breakpoint. The exact schema from US-002 may vary; the integration points below describe the expected interface contract.

#### Command Signature

```
ks diff [--baseline <snapshot-id>] [--current <snapshot-id>] [--threshold <0.0-1.0>]
```

| Flag | Default | Description |
|---|---|---|
| `--baseline` | latest baseline in `baselines.json` | Snapshot ID to use as baseline |
| `--current` | most recent snapshot | Snapshot ID to compare against |
| `--threshold` | `0.99` | Similarity threshold below which a screenshot pair is flagged |

#### Execution Flow

1. Resolve baseline snapshot and current snapshot IDs (from flags or defaults).
2. Load baseline and current snapshot manifests (JSON from US-002 snapshot storage).
3. For each `(url, breakpoint)` pair present in **both** snapshots:
   a. Locate the baseline screenshot path and current screenshot path.
   b. Invoke `diff.LoadPNG()` on each.
   c. Call `diff.CompareImages(baseline, current, opts)`.
   d. Derive diff output path using `diff.DiffImagePath()` (stored in the **current** snapshot's directory).
   e. If diff image is non-nil, call `diff.SavePNG(diffPath, diffImg)`.
   f. Set `result.DiffPath = diffPath`.
   g. Collect `ScreenshotDiffResult` into results slice.
4. Emit `output.Success("diff", DiffOutput{...})`.

---

## API Definitions

### JSON Output Schema (`ks diff`)

```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "baselineSnapshotID": "snap-20240404-001",
    "currentSnapshotID":  "snap-20240404-002",
    "screenshotDiffs": [
      {
        "url":               "https://example.com",
        "breakpoint":        "mobile",
        "baselinePath":      ".kaleidoscope/snapshots/snap-001/mobile-375x812.png",
        "currentPath":       ".kaleidoscope/snapshots/snap-002/mobile-375x812.png",
        "diffPath":          ".kaleidoscope/snapshots/snap-002/diff-mobile-1712192400.png",
        "similarity":        0.9923,
        "regressed":         false,
        "dimensionMismatch": false,
        "width":             375,
        "height":            812
      },
      {
        "url":               "https://example.com",
        "breakpoint":        "desktop",
        "baselinePath":      ".kaleidoscope/snapshots/snap-001/desktop-1280x720.png",
        "currentPath":       ".kaleidoscope/snapshots/snap-002/desktop-1280x720.png",
        "diffPath":          ".kaleidoscope/snapshots/snap-002/diff-desktop-1712192401.png",
        "similarity":        0.8712,
        "regressed":         true,
        "dimensionMismatch": false,
        "width":             1280,
        "height":            720
      }
    ],
    "auditDiffs":   [],
    "elementDiffs": [],
    "threshold":    0.99,
    "hasRegressions": true
  }
}
```

**Top-level `result` fields:**

| Field | Type | Description |
|---|---|---|
| `baselineSnapshotID` | string | ID of the baseline snapshot |
| `currentSnapshotID` | string | ID of the current snapshot |
| `screenshotDiffs` | `[]ScreenshotDiffResult` | Per-URL per-breakpoint diff results |
| `auditDiffs` | array | Audit issue diffs (US-002 scope, empty slice for now) |
| `elementDiffs` | array | Element/ax-tree diffs (US-002 scope, empty slice for now) |
| `threshold` | float64 | Configured similarity threshold used for this run |
| `hasRegressions` | bool | True if any diff (screenshot or audit) is regressed |

### Go Types in `diff` Package

```go
// Public surface — all types serializable to JSON.

type ScreenshotDiffResult struct { ... }  // see above
type Options struct { ... }               // see above

func DefaultOptions() Options
func CompareImages(baseline, current image.Image, opts Options) (ScreenshotDiffResult, image.Image)
func LoadPNG(path string) (image.Image, error)
func SavePNG(path string, img image.Image) error
func DiffImagePath(snapshotDir, baselineFile, currentFile string) string
```

---

## Data Model Changes

### New Files

| Path | Purpose |
|---|---|
| `/workspace/diff/pixel.go` | Core pixel comparison engine |
| `/workspace/diff/io.go` | PNG load/save helpers and path derivation |
| `/workspace/diff/pixel_test.go` | Unit tests for the diff engine |
| `/workspace/cmd/diff.go` | `ks diff` command implementation |

### Modified Files

| Path | Change |
|---|---|
| `/workspace/main.go` | Add `case "diff": cmd.RunDiff(cmdArgs)` to switch |

### No schema/database changes.
The diff images are written to the existing snapshot directory (`.kaleidoscope/snapshots/<id>/`), following the established convention. No new directories or config files are introduced by this story alone.

---

## Snapshot Storage Assumptions (US-002 Interface Contract)

US-004 consumes snapshot data produced by US-002. The following interface is assumed; if US-002's schema differs, `cmd/diff.go` must adapt its loading logic:

```go
// Assumed snapshot manifest (from US-002):
type Snapshot struct {
    ID          string               `json:"id"`
    CreatedAt   time.Time            `json:"createdAt"`
    Screenshots []SnapshotScreenshot `json:"screenshots"`
    // ...audit data, element data
}

type SnapshotScreenshot struct {
    URL        string `json:"url"`
    Breakpoint string `json:"breakpoint"`
    Path       string `json:"path"`
    Width      int    `json:"width"`
    Height     int    `json:"height"`
}
```

Snapshots are loaded from `.kaleidoscope/snapshots/<id>/snapshot.json`. The baseline snapshot ID is resolved from `.kaleidoscope/baselines.json`.

---

## Security Considerations

1. **Path traversal:** `--baseline` and `--current` flag values must be validated to contain only alphanumeric characters, hyphens, and underscores before being used to construct file paths. Reject values containing `..`, `/`, or `\`.

2. **File size limits:** PNG loading should be protected against decompression bombs. The standard `image/png` decoder is safe in this regard (streams pixel data), but callers should be aware that very large images (e.g., 10,000×10,000 full-page screenshots) will allocate proportionally large buffers. No explicit limit is required for the initial implementation, but this is noted for future hardening.

3. **Output directory:** Diff images are written only to the snapshot directory (`.kaleidoscope/snapshots/<id>/`). The path is derived programmatically via `DiffImagePath()`, not from user input, preventing write-anywhere attacks.

4. **No external processes:** All image processing uses pure Go standard library (`image`, `image/png`, `image/draw`). No shelling out to ImageMagick or other tools.

---

## Unit Test Plan (`/workspace/diff/pixel_test.go`)

| Test | Description |
|---|---|
| `TestIdenticalImages` | Two identical synthetic images → similarity 1.0, no highlighted pixels |
| `TestCompletelyDifferentImages` | Solid red vs solid blue → similarity near 0.0, all pixels highlighted |
| `TestPartialDiff` | One row different → similarity = `(H-1)/H`, highlighted pixels = width |
| `TestDimensionMismatch` | Images of different sizes → `DimensionMismatch: true`, `Similarity: 0.0`, nil diff image |
| `TestThresholdTolerance` | Pixels differing by exactly `Threshold` → not counted as different |
| `TestRegressionFlag` | Similarity below `SimilarityThreshold` → `Regressed: true` |
| `TestDefaultOptions` | `DefaultOptions()` returns expected values |
| `TestLoadSavePNG` | Round-trip PNG write/read preserves pixel values |

All tests use synthetic `image.RGBA` images constructed in memory — no disk fixtures required except for the PNG round-trip test (uses `t.TempDir()`).

---

## Implementation Notes

- **No new Go dependencies.** Use only `image`, `image/color`, `image/draw`, `image/png` from the standard library.
- **Alpha channel:** Alpha is not factored into the difference calculation (only R, G, B channels are compared). Alpha is preserved in the diff image for identical pixels.
- **Highlight color in diff image:** Changed pixels are set to a solid highlight color (default red). The diff image does NOT blend with the original — it is a flat color overlay. This keeps the algorithm simple and the output easy to interpret.
- **`ks diff` is a read-only command** with respect to snapshot storage — it only writes new diff PNG files. It does not modify snapshot manifests.
- **Mismatched screenshot sets:** If a URL/breakpoint exists in the baseline but not in the current (or vice versa), skip it and do not include it in `screenshotDiffs`. Future work can surface missing screenshots explicitly.
- **Exit code:** `ks diff` exits with code 0 regardless of regression detection (regressions are reported in JSON, not via exit code). This is consistent with `ks audit` behavior.
