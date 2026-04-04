# Tech Spec: Screenshot Pixel Diff (US-004)

**Story:** US-004 — Screenshot Pixel Diff
**Depends on:** US-002 (Snapshot System)
**Status:** Ready for implementation

---

## 1. Architecture Overview

US-004 adds a pure-Go pixel-level image comparison engine to Kaleidoscope. It introduces a new `pixeldiff` package that compares two PNG screenshots and produces a highlighted diff image and a similarity score. This engine is invoked by the `ks diff` command (introduced in US-002) when comparing snapshot screenshots.

```
cmd/diff.go          — CLI entry point for `ks diff`
pixeldiff/diff.go    — Pure-Go diff engine (no Chrome dependency)
pixeldiff/diff_test.go
```

The diff engine is a pure function: it takes two `image.Image` values and a threshold, and returns a diff result struct plus a highlighted diff `image.RGBA`. It has no browser dependency and no global state.

---

## 2. Detailed Component Design

### 2.1 Package: `pixeldiff`

**File:** `pixeldiff/diff.go`

#### Core Types

```go
package pixeldiff

import "image"

// Result holds the outcome of comparing two PNG screenshots.
type Result struct {
    // SimilarityScore is between 0.0 (completely different) and 1.0 (identical).
    SimilarityScore float64 `json:"similarityScore"`
    // DiffPixels is the count of pixels that differ beyond the per-channel tolerance.
    DiffPixels int `json:"diffPixels"`
    // TotalPixels is the total number of pixels compared.
    TotalPixels int `json:"totalPixels"`
    // DimensionMismatch is true when the two images have different dimensions.
    // When true, SimilarityScore is 0.0 and no diff image is produced.
    DimensionMismatch bool `json:"dimensionMismatch,omitempty"`
    // Regressed is true when SimilarityScore < (1 - threshold).
    Regressed bool `json:"regressed"`
}

// Options controls diff engine behavior.
type Options struct {
    // Threshold is the minimum fraction of differing pixels to flag as regressed.
    // Range: 0.0–1.0. Default: 0.01 (1% pixel difference triggers regression).
    Threshold float64
    // PixelTolerance is the per-channel tolerance for considering a pixel changed.
    // Range: 0–255. Default: 10.
    PixelTolerance uint8
    // HighlightColor is the RGBA color used to mark changed pixels in the diff image.
    // Default: red (255, 0, 0, 255).
    HighlightColor [4]uint8
}
```

#### Core Function

```go
// Compare compares two images and returns a Result and a diff image.
// The diff image is nil when DimensionMismatch is true.
// Compare never panics on mismatched dimensions.
func Compare(baseline, current image.Image, opts Options) (Result, *image.RGBA)
```

**Algorithm:**

1. Check bounds. If dimensions differ, return `Result{DimensionMismatch: true, Regressed: true, SimilarityScore: 0.0}` and `nil` diff image.
2. Allocate an `*image.RGBA` output image with the same bounds.
3. Iterate every pixel. For each pixel:
   - Copy the `current` pixel color to the output image (provides visual context).
   - Compute per-channel absolute difference between `baseline` and `current` RGBA values.
   - If any channel difference exceeds `opts.PixelTolerance`, mark pixel as diff, paint it `opts.HighlightColor`, increment `diffPixels`.
4. Compute `similarityScore = 1.0 - float64(diffPixels)/float64(totalPixels)`.
5. Set `Regressed = similarityScore < (1.0 - opts.Threshold)`.
6. Return result and diff image.

#### Helper: Write Diff PNG

```go
// WriteDiffPNG encodes the diff image to a PNG file at path.
func WriteDiffPNG(path string, img *image.RGBA) error
```

Uses `image/png` encoder. Creates the file with mode `0644`.

#### Default Options

```go
func DefaultOptions() Options {
    return Options{
        Threshold:      0.01,
        PixelTolerance: 10,
        HighlightColor: [4]uint8{255, 0, 0, 255},
    }
}
```

---

### 2.2 Integration into `ks diff` (cmd/diff.go)

US-002 introduces the `ks diff` command and the snapshot data model. US-004 extends the diff output by running pixel comparison for each screenshot pair found between the baseline snapshot and the current snapshot.

#### Diff Invocation Pattern

For each URL × breakpoint pair present in both baseline and current snapshots:
1. Load baseline screenshot path and current screenshot path from snapshot metadata.
2. Decode both PNGs using `image/png`.
3. Call `pixeldiff.Compare(baseline, current, opts)`.
4. If result is not a dimension mismatch, write the diff PNG to the snapshot directory.
5. Append a `ScreenshotDiffEntry` to the diff output.

#### Configurable Threshold

The similarity threshold is read from:
1. CLI flag `--screenshot-threshold <float>` (e.g. `0.02` for 2%)
2. `.ks-project.json` field `screenshotThreshold` (if US-002 defines this config file)
3. Default: `0.01`

---

## 3. API Definitions

### 3.1 `ks diff` JSON Output Extension

The `ks diff` command (defined in US-002) outputs a JSON `Result` envelope via `output.Success("diff", ...)`. US-004 adds a `screenshots` field to the result payload.

```jsonc
{
  "ok": true,
  "command": "diff",
  "result": {
    "baselineID": "<snapshot-id>",
    "currentID":  "<snapshot-id>",
    // ... audit diff fields from US-002 ...
    "screenshots": [
      {
        "url":             "https://example.com/",
        "breakpoint":      "desktop",
        "width":           1280,
        "height":          720,
        "baselinePath":    ".kaleidoscope/snapshots/<id>/desktop-1280x720.png",
        "currentPath":     ".kaleidoscope/snapshots/<id>/desktop-1280x720.png",
        "diffPath":        ".kaleidoscope/snapshots/<id>/diff-desktop-1280x720.png",
        "similarityScore": 0.9974,
        "diffPixels":      2600,
        "totalPixels":     921600,
        "dimensionMismatch": false,
        "regressed":       false,
        "threshold":       0.01
      }
    ],
    "screenshotSummary": {
      "total":    4,
      "regressed": 1,
      "mismatch": 0
    }
  }
}
```

When `dimensionMismatch` is `true`, `diffPath` is omitted.

### 3.2 `pixeldiff` Package Public API Summary

| Symbol | Signature | Description |
|---|---|---|
| `Compare` | `(image.Image, image.Image, Options) (Result, *image.RGBA)` | Core diff function |
| `WriteDiffPNG` | `(string, *image.RGBA) error` | Write diff image to disk |
| `DefaultOptions` | `() Options` | Returns sensible defaults |
| `Result` | struct | Diff outcome |
| `Options` | struct | Diff configuration |

---

## 4. Data Model Changes

### 4.1 Snapshot Screenshot Metadata (US-002 context)

US-002 is expected to store snapshot metadata that includes screenshot paths indexed by URL and breakpoint. US-004 requires this shape (or compatible equivalent):

```go
// SnapshotScreenshot represents one captured screenshot within a snapshot.
type SnapshotScreenshot struct {
    URL        string `json:"url"`
    Breakpoint string `json:"breakpoint"`
    Width      int    `json:"width"`
    Height     int    `json:"height"`
    Path       string `json:"path"` // absolute or relative to snapshot dir
}
```

US-004 does not modify snapshot creation — it only reads existing screenshot paths during `ks diff`.

### 4.2 Diff PNG Naming Convention

Diff PNGs are written alongside the current snapshot's screenshots, using the naming pattern:

```
.kaleidoscope/snapshots/<current-snapshot-id>/diff-<breakpoint>-<width>x<height>.png
```

For multi-URL snapshots, the URL is slugified:

```
diff-<url-slug>-<breakpoint>-<width>x<height>.png
```

Where `url-slug` strips the scheme and replaces non-alphanumeric characters with `-`.

### 4.3 `.ks-project.json` Schema Addition

```json
{
  "screenshotThreshold": 0.01
}
```

This field is optional. When absent, `DefaultOptions().Threshold` (0.01) is used.

---

## 5. File Layout

```
pixeldiff/
  diff.go          Core Compare function, Result, Options types, WriteDiffPNG
  diff_test.go     Unit tests (synthetic images, dimension mismatch, zero diff, full diff)
cmd/
  diff.go          Extended to call pixeldiff for screenshot pairs (US-002 owns this file)
```

No new dependencies are introduced. All image handling uses Go stdlib: `image`, `image/color`, `image/png`, `os`.

---

## 6. Security Considerations

### 6.1 Path Traversal
Diff PNG output paths are constructed by joining a trusted snapshot directory root (controlled by the application) with a filename derived from URL and breakpoint strings. URL slugification must replace `/`, `.`, `:`, `?`, `#`, and all non-alphanumeric characters with `-` before use in file paths. This prevents any user-supplied URL from escaping the snapshot directory.

**Implementation note:** Use `strings.Map` or a regexp to strip unsafe path characters from the slug before calling `filepath.Join`.

### 6.2 Denial of Service via Large Images
Screenshots taken by Kaleidoscope are bounded by the browser viewport dimensions (max ~4K resolution). The pixel comparison loop is O(W×H), which for a 3840×2160 image is ~8M iterations — acceptable. No additional size guard is needed beyond the existing screenshot size limits.

### 6.3 Malformed PNG Input
Baseline screenshots are written by Kaleidoscope and stored locally, but future changes could allow user-supplied paths. Use `image/png.Decode` which is memory-safe and returns an error on malformed input. Errors are propagated via the `output.Fail` path, not panics.

### 6.4 File Permission
Diff PNG files are written with mode `0644`. The snapshot directory is created with `0755`. No credentials or secrets are stored in image data.

---

## 7. Testing Strategy

### Unit Tests (`pixeldiff/diff_test.go`)

| Test | Description |
|---|---|
| `TestIdenticalImages` | Two identical synthetic images → similarity 1.0, 0 diff pixels, not regressed |
| `TestCompletelyDifferent` | Two synthetic images with all pixels different → similarity ~0.0, regressed |
| `TestPartialDiff` | Images that differ in a known rectangular region → correct pixel count and score |
| `TestDimensionMismatch` | 100×100 vs 200×200 → `DimensionMismatch=true`, no panic, regressed |
| `TestThresholdBoundary` | Score just above and just below threshold → correct `Regressed` value |
| `TestPixelTolerance` | Pixels differing by exactly tolerance value are not flagged |
| `TestWriteDiffPNG` | Written file is a valid PNG with correct bounds |

Quality gate: `go test ./...`

---

## 8. Implementation Sequence

1. Create `pixeldiff/diff.go` with `Compare`, `WriteDiffPNG`, `DefaultOptions`, `Result`, `Options`.
2. Create `pixeldiff/diff_test.go` covering the test cases above.
3. Extend `cmd/diff.go` (US-002) to:
   - Accept `--screenshot-threshold` flag.
   - For each screenshot pair in both baseline and current: decode PNGs, call `pixeldiff.Compare`, write diff PNG (if no mismatch), append to output.
4. Update `cmd/diff.go` JSON output to include `screenshots` and `screenshotSummary` fields.
5. Run `go test ./...` to verify all quality gates pass.
