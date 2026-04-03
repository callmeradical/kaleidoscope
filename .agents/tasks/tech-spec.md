# Tech Spec: US-004 — Screenshot Pixel Diff

**Story:** As a developer, I want pixel-level visual comparison between baseline and current screenshots that produces a highlight overlay image, so that subtle visual changes are surfaced.

**Depends on:** US-002 (Snapshot Infrastructure & `ks diff` command framework)

**Status:** Ready for implementation

---

## Architecture Overview

US-004 adds a pure-Go pixel-diff engine to the existing `ks diff` command introduced by US-002. The engine is a standalone analysis module with no Chrome dependency. It ingests two `image.Image` values, computes per-pixel differences, writes a highlight overlay PNG to the snapshot directory, and returns a `ScreenshotDiffResult` that the `ks diff` command embeds in its JSON output.

```
ks diff
  └─ cmd/diff.go (US-002)
        ├─ audit diff (US-002)
        ├─ element diff via ax-tree (US-003)
        └─ screenshot diff  ← US-004
              └─ analysis/pixeldiff.go
                    ├─ CompareImages()   pure function, no I/O
                    └─ WriteHighlight()  writes diff PNG to snapshot dir
```

No new top-level commands are introduced. All changes are additive to the diff engine and `cmd/diff.go`.

---

## Detailed Component Design

### 1. `analysis/pixeldiff.go` — Pure Diff Engine

**Package:** `github.com/callmeradical/kaleidoscope/analysis`

This file is a pure-function module. It must not import `browser`, `cmd`, or any I/O packages. The only allowed imports are:

```go
import (
    "fmt"
    "image"
    "image/color"
    "math"
)
```

#### Types

```go
// ImageDiffResult is the return value of CompareImages.
type ImageDiffResult struct {
    // Similarity is in [0.0, 1.0]; 1.0 = identical, 0.0 = fully different.
    Similarity float64 `json:"similarity"`

    // DifferentPixels is the raw count of pixels that exceed the per-channel threshold.
    DifferentPixels int `json:"differentPixels"`

    // TotalPixels is width × height of the reference image.
    TotalPixels int `json:"totalPixels"`

    // DiffImage is the highlight overlay ready to be encoded as PNG.
    // Nil if images had mismatched dimensions (see MismatchedDimensions).
    DiffImage image.Image `json:"-"`

    // MismatchedDimensions is true when the two inputs have different bounds.
    // In this case Similarity is 0.0 and DiffImage is nil.
    MismatchedDimensions bool `json:"mismatchedDimensions"`
}
```

#### Functions

```go
// CompareImages compares two images pixel-by-pixel.
//
// channelThreshold controls how much per-channel (R/G/B) difference is
// tolerated before a pixel is counted as "different". Range 0–255;
// recommended default: 10.
//
// Returns an ImageDiffResult. If the images have different dimensions,
// MismatchedDimensions is set to true, Similarity is 0.0, and DiffImage
// is nil — the function does NOT panic.
func CompareImages(a, b image.Image, channelThreshold uint8) ImageDiffResult
```

**Algorithm:**
1. If `a.Bounds() != b.Bounds()`, return `ImageDiffResult{MismatchedDimensions: true, Similarity: 0.0}`.
2. Allocate an `*image.NRGBA` of the same bounds for the diff overlay (default: dark background, e.g. `color.NRGBA{0, 0, 0, 255}`).
3. Iterate every `(x, y)` pixel:
   - Decode both pixels via `color.NRGBAModel.Convert()`.
   - Compute per-channel absolute difference for R, G, B.
   - If any channel difference > `channelThreshold`:
     - Increment `differentPixels`.
     - Paint the overlay pixel **red** (`color.NRGBA{255, 0, 0, 255}`) to highlight the change.
   - Else: copy the original pixel from image `a` at 50 % alpha (`A: 128`) so unchanged areas are visible but dimmed.
4. `Similarity = 1.0 - float64(differentPixels)/float64(totalPixels)`.
5. Return result with `DiffImage` set to the overlay.

**Why this algorithm:** Red highlights on a dimmed reference image give maximum visual saliency for changed regions, matching common visual regression tools (Playwright, Percy). Using the standard `image` package and no external libs satisfies the PRD constraint.

---

### 2. `analysis/pixeldiff_writer.go` — Overlay PNG Writer

**Package:** `github.com/callmeradical/kaleidoscope/analysis`

Separating I/O from the pure function keeps `CompareImages` trivially testable.

```go
import (
    "fmt"
    "image/png"
    "os"
    "path/filepath"
)

// WriteDiffImage encodes diffImg as a PNG and writes it to dir.
// The filename follows the pattern: diff-<baseName>.png
// Returns the absolute path of the written file.
func WriteDiffImage(dir, baseName string, diffImg image.Image) (string, error)
```

Implementation: `os.Create` → `png.Encode` → close. Returns error if dir does not exist (caller is responsible for creating snapshot dir).

---

### 3. Data Model Changes

#### `ScreenshotDiff` (new struct, lives in the snapshot package introduced by US-002)

```go
// ScreenshotDiff is the result of comparing one screenshot pair
// (baseline vs. current) for a given URL + breakpoint.
type ScreenshotDiff struct {
    URL          string  `json:"url"`
    Breakpoint   string  `json:"breakpoint"`   // "mobile", "tablet", "desktop", "wide"
    BaselinePath string  `json:"baselinePath"`
    CurrentPath  string  `json:"currentPath"`
    DiffPath     string  `json:"diffPath"`      // path to highlight overlay PNG; empty if mismatched dims
    Similarity   float64 `json:"similarity"`    // 0.0–1.0
    Regressed    bool    `json:"regressed"`     // similarity < threshold
    MismatchedDimensions bool `json:"mismatchedDimensions"`
}
```

#### `DiffOutput` (extended from US-002)

US-002 defines a `DiffOutput` struct returned by `ks diff`. US-004 adds a new field:

```go
type DiffOutput struct {
    // ... existing fields from US-002 (audit diffs, element diffs, etc.) ...

    // ScreenshotDiffs contains one entry per (URL, breakpoint) pair compared.
    // Added by US-004.
    ScreenshotDiffs []ScreenshotDiff `json:"screenshotDiffs"`

    // ScreenshotRegressed is true if any ScreenshotDiff.Regressed == true.
    ScreenshotRegressed bool `json:"screenshotRegressed"`
}
```

---

### 4. Integration into `cmd/diff.go` (US-002 foundation)

US-002 creates `cmd/diff.go` and the `RunDiff(args []string)` function. US-004 adds the screenshot comparison phase at the end of `RunDiff`:

```
Phase 1: Load baseline snapshot      (US-002)
Phase 2: Capture current snapshot    (US-002)
Phase 3: Diff audit results          (US-002)
Phase 4: Diff ax-tree elements       (US-003)
Phase 5: Diff screenshots            ← US-004
Phase 6: Assemble DiffOutput + emit  (US-002 / US-004 extended)
```

**Phase 5 pseudocode:**

```go
func diffScreenshots(baseline, current Snapshot, snapshotDir string, threshold float64) []ScreenshotDiff {
    var results []ScreenshotDiff
    for _, bp := range current.Screenshots {  // []SnapshotScreenshot{URL, Breakpoint, Path}
        baselineSS := findMatching(baseline.Screenshots, bp.URL, bp.Breakpoint)
        if baselineSS == nil {
            // No baseline for this pair — skip or mark as new
            continue
        }

        imgA, _ := loadPNG(baselineSS.Path)
        imgB, _ := loadPNG(bp.Path)

        diffResult := analysis.CompareImages(imgA, imgB, 10)

        diffPath := ""
        if !diffResult.MismatchedDimensions {
            baseName := fmt.Sprintf("%s-%s", sanitize(bp.URL), bp.Breakpoint)
            diffPath, _ = analysis.WriteDiffImage(snapshotDir, baseName, diffResult.DiffImage)
        }

        results = append(results, ScreenshotDiff{
            URL:                  bp.URL,
            Breakpoint:           bp.Breakpoint,
            BaselinePath:         baselineSS.Path,
            CurrentPath:          bp.Path,
            DiffPath:             diffPath,
            Similarity:           diffResult.Similarity,
            Regressed:            diffResult.Similarity < threshold,
            MismatchedDimensions: diffResult.MismatchedDimensions,
        })
    }
    return results
}
```

`loadPNG` reads a file with `os.Open` + `png.Decode`. Must return `(image.Image, error)`.

---

### 5. Configuration

The similarity threshold is read from `.ks-project.json` (introduced by US-002):

```json
{
  "project": "my-app",
  "screenshotThreshold": 0.99
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `screenshotThreshold` | float64 | `0.99` | Similarity below this value marks a screenshot pair as regressed |

If `.ks-project.json` is absent or the field is missing, the default `0.99` is used (99 % similarity required to pass).

The `channelThreshold` (per-pixel tolerance) is hard-coded at `10` (out of 255) and is not user-configurable in this story. It can be exposed in a future story.

---

### 6. `ks diff` JSON Output (Extended)

```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "baseline": "2024-01-15T10:00:00Z",
    "current":  "2024-01-15T11:00:00Z",
    "auditDiffs": [ ... ],
    "elementDiffs": [ ... ],
    "screenshotDiffs": [
      {
        "url": "https://example.com",
        "breakpoint": "desktop",
        "baselinePath": ".kaleidoscope/snapshots/baseline/desktop.png",
        "currentPath":  ".kaleidoscope/snapshots/current/desktop.png",
        "diffPath":     ".kaleidoscope/snapshots/current/diff-example.com-desktop.png",
        "similarity":   0.9987,
        "regressed":    false,
        "mismatchedDimensions": false
      },
      {
        "url": "https://example.com",
        "breakpoint": "mobile",
        "baselinePath": ".kaleidoscope/snapshots/baseline/mobile.png",
        "currentPath":  ".kaleidoscope/snapshots/current/mobile.png",
        "diffPath":     "",
        "similarity":   0.0,
        "regressed":    true,
        "mismatchedDimensions": true
      }
    ],
    "screenshotRegressed": true,
    "regressed": true
  }
}
```

---

### 7. New Files Summary

| File | Package | Purpose |
|---|---|---|
| `analysis/pixeldiff.go` | `analysis` | Pure `CompareImages()` function |
| `analysis/pixeldiff_writer.go` | `analysis` | `WriteDiffImage()` I/O helper |
| `analysis/pixeldiff_test.go` | `analysis` | Unit tests (see below) |

**Modified files** (US-002 baseline):

| File | Change |
|---|---|
| `snapshot/snapshot.go` | Add `ScreenshotDiff` struct |
| `cmd/diff.go` | Add Phase 5 screenshot diff logic, extend `DiffOutput` |
| `cmd/usage.go` | Document new `screenshotDiffs` fields in `diff` usage string |

---

## API Definitions

### Internal Go API

```go
// analysis package — pure functions, no side effects

func CompareImages(a, b image.Image, channelThreshold uint8) ImageDiffResult
func WriteDiffImage(dir, baseName string, diffImg image.Image) (string, error)
```

No new CLI flags are added to `ks diff` in this story. The threshold is read from `.ks-project.json`.

---

## Security Considerations

1. **Path traversal:** `baseName` passed to `WriteDiffImage` is derived from URL + breakpoint name. URLs must be sanitized (strip scheme, replace `/`, `:`, `?`, `#` with `-`) before use as filenames. A helper `sanitizeFilename(s string) string` should be implemented using only `strings.Map` and a whitelist of `[a-zA-Z0-9._-]`.

2. **Memory:** Large screenshots at 1920×1080 (wide breakpoint) with 4 bytes per pixel = ~8 MB per image. Three images (baseline, current, diff) = ~24 MB per URL/breakpoint pair. For a typical 4-breakpoint diff on a single URL this is ~96 MB peak, which is acceptable. If multiple URLs are compared concurrently in future, consider streaming rather than holding all images in memory simultaneously.

3. **File writes are local only:** Diff images are written to `.kaleidoscope/snapshots/`, which is gitignored per PRD rules. No data is transmitted externally.

4. **No shell execution:** The pixel diff is pure Go — no `exec.Command`, no ImageMagick, no external binaries. This eliminates command-injection surface.

---

## Testing Plan

**File:** `analysis/pixeldiff_test.go`

Required test cases (must pass `go test ./...`):

| Test | Scenario |
|---|---|
| `TestCompareImages_Identical` | Two identical solid-color images → Similarity == 1.0, DifferentPixels == 0 |
| `TestCompareImages_TotallyDifferent` | Image A solid white, image B solid black → Similarity == 0.0, DifferentPixels == TotalPixels |
| `TestCompareImages_PartialDiff` | Image A solid white, image B half white / half red → Similarity ≈ 0.5 |
| `TestCompareImages_MismatchedDimensions` | 100×100 vs 200×200 → MismatchedDimensions == true, does not panic |
| `TestCompareImages_ChannelThreshold` | Pixels differ by exactly threshold → not counted; differ by threshold+1 → counted |
| `TestWriteDiffImage_Roundtrip` | Write diff image to tmpdir, decode PNG back, verify bounds match input |
| `TestCompareImages_DiffImageIsRed` | Known changed pixel → corresponding overlay pixel is red |
| `TestCompareImages_DiffImageIsAtten` | Known unchanged pixel → overlay pixel has A ≈ 128 |

All tests use only `image`, `image/color`, `image/png`, `os`, and `testing` — no test helpers requiring Chrome or a running browser.

---

## Assumptions & Constraints

- US-002 is complete and merged before this story begins. Specifically:
  - `Snapshot` struct exists with a `Screenshots []SnapshotScreenshot` field.
  - `RunDiff` in `cmd/diff.go` exists and handles the command lifecycle.
  - `.ks-project.json` loading is implemented (config struct with `ScreenshotThreshold float64`).
  - The snapshot directory layout is established.

- Screenshot file format is always PNG (enforced by `browser.ScreenshotDir` and existing screenshot commands).

- The diff image format is PNG (standard library `image/png`, no lossy compression).

- Breakpoints are the canonical four defined in `cmd/breakpoints.go`: mobile (375×812), tablet (768×1024), desktop (1280×720), wide (1920×1080). The diff logic matches by `(URL, breakpoint name)` string pair.

- `ks diff` is invoked after a new snapshot has been captured. US-004 does not capture screenshots itself; it reads paths from the snapshot metadata.
