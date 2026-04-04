# Tech Spec: Screenshot Pixel Diff (US-004)

## Overview

Implements pure Go pixel-level visual comparison between baseline and current screenshots. Produces a diff PNG with highlighted difference regions, a similarity score (0.0–1.0), and integrates into the `ks diff` JSON output. Depends on US-002 (snapshot system) for baseline/snapshot storage conventions.

---

## Architecture Overview

```
diff/
  pixeldiff.go        # Pure function: compare two images, produce diff PNG + score
cmd/
  diff.go             # `ks diff` command — calls pixeldiff, formats JSON output
```

The pixel diff engine is a **pure function module** with no Chrome/browser dependency. It accepts two PNG byte slices (or file paths), computes per-pixel differences, and returns structured results including a highlighted diff PNG. Integration with `ks diff` reads files from the snapshot directory established by US-002.

---

## Component Design

### 1. `diff` Package — `diff/pixeldiff.go`

#### Types

```go
package diff

// PixelDiffResult holds the outcome of comparing two PNG screenshots.
type PixelDiffResult struct {
    SimilarityScore  float64 // 0.0 (completely different) to 1.0 (identical)
    ChangedPixels    int     // absolute count of pixels that changed
    TotalPixels      int     // total pixel count (from baseline dimensions)
    DiffImagePath    string  // path where diff PNG was written (empty if not written)
    DiffImageBytes   []byte  // diff PNG bytes (nil if dimensions mismatched)
    DimensionMismatch bool   // true when images have different dimensions
    Regressed        bool    // true when SimilarityScore < threshold
}

// PixelDiffOptions controls comparison behaviour.
type PixelDiffOptions struct {
    Threshold      float64 // similarity below this value is flagged as regressed (default 0.95)
    HighlightColor color.RGBA // colour used to paint changed pixels in diff PNG (default bright red)
    OutputPath     string  // where to write diff PNG; if empty, caller handles DiffImageBytes
}
```

#### Functions

```go
// CompareFiles loads two PNG files from disk and compares them.
// Writes the diff PNG to opts.OutputPath if provided.
func CompareFiles(baselinePath, currentPath string, opts PixelDiffOptions) (PixelDiffResult, error)

// CompareBytes compares two in-memory PNG payloads.
// Returns a PixelDiffResult; does NOT write to disk (caller manages DiffImageBytes).
func CompareBytes(baselinePNG, currentPNG []byte, opts PixelDiffOptions) (PixelDiffResult, error)

// renderDiff builds a new RGBA image the same size as the baseline.
// Each pixel that differs by more than the perPixelTolerance (Euclidean distance
// in RGB space, 0–255 per channel) is painted highlightColor;
// unchanged pixels are painted the baseline colour at 30% opacity for context.
func renderDiff(baseline, current image.Image, highlight color.RGBA) *image.RGBA

// pixelDistance returns the Euclidean distance between two colours (0–441.67).
func pixelDistance(a, b color.Color) float64
```

#### Algorithm

1. Decode both PNGs using `image/png`.
2. **Dimension check**: if bounds differ, return `PixelDiffResult{DimensionMismatch: true, SimilarityScore: 0.0, Regressed: true}` — no panic.
3. Iterate over every pixel; compute Euclidean RGB distance. Pixel is "changed" if distance > `perPixelTolerance` (internal constant: 10.0 out of 441.67 max, ~2.3%).
4. `SimilarityScore = 1.0 - (changedPixels / totalPixels)`.
5. Build diff image via `renderDiff`: changed pixels → `HighlightColor`, unchanged → baseline colour at 30% alpha blend.
6. Encode diff image to PNG bytes via `image/png`.
7. If `opts.OutputPath` is set, write diff PNG to disk.
8. Set `Regressed = SimilarityScore < opts.Threshold`.

**Packages used**: `image`, `image/color`, `image/draw`, `image/png`, `math`, `os`. No third-party dependencies.

---

### 2. `cmd/diff.go` — `ks diff` Command

Extends (or creates) the `ks diff` command. When screenshot pairs exist in the snapshot, pixel diff runs automatically and its results are included in the JSON output.

#### CLI Interface

```
ks diff [--snapshot <id>] [--baseline <id>] [--threshold <0.0-1.0>]
```

| Flag | Default | Description |
|---|---|---|
| `--snapshot` | latest | Snapshot ID to compare |
| `--baseline` | configured baseline | Baseline snapshot ID |
| `--threshold` | `0.95` | Similarity threshold below which a screenshot is flagged as regressed |

#### JSON Output Shape

```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "snapshotID": "abc123",
    "baselineID": "def456",
    "screenshotDiffs": [
      {
        "url": "https://example.com",
        "breakpoint": { "width": 1280, "height": 720 },
        "baselinePath": ".kaleidoscope/snapshots/def456/screenshots/1280x720.png",
        "currentPath":  ".kaleidoscope/snapshots/abc123/screenshots/1280x720.png",
        "diffPath":     ".kaleidoscope/snapshots/abc123/diffs/1280x720-diff.png",
        "similarityScore": 0.9823,
        "changedPixels": 1456,
        "totalPixels": 921600,
        "dimensionMismatch": false,
        "regressed": false
      }
    ],
    "threshold": 0.95,
    "anyRegressed": false
  }
}
```

The `screenshotDiffs` array is appended to any existing audit/element diff results that US-002's `ks diff` already produces.

---

## Data Model Changes

### Snapshot Directory Layout (extends US-002)

```
.kaleidoscope/
  snapshots/
    <snapshot-id>/
      screenshots/
        <width>x<height>.png     # source screenshots (written by ks snapshot)
      diffs/                     # NEW — created by ks diff
        <width>x<height>-diff.png
  baselines.json                 # committed; maps URL+breakpoint → baseline snapshot ID
```

No new JSON schema fields on the snapshot metadata itself. The diff PNG paths are computed on the fly and reported in the command output; they are not persisted to `baselines.json` or a separate index.

### `PixelDiffOptions` Defaults

Stored as package-level constants in `diff/pixeldiff.go`:

```go
const (
    DefaultSimilarityThreshold = 0.95
    perPixelTolerance          = 10.0  // Euclidean RGB distance (0–441.67)
)

var DefaultHighlightColor = color.RGBA{R: 255, G: 0, B: 80, A: 255} // crimson
```

The threshold is overridden via `--threshold` CLI flag; future `.ks-project.json` integration (out of scope for US-004) may source it from config.

---

## API Definitions

### `diff.CompareFiles`

```
Input:
  baselinePath string  — absolute or relative path to baseline PNG
  currentPath  string  — absolute or relative path to current PNG
  opts         PixelDiffOptions

Output:
  PixelDiffResult, error

Errors:
  - file not found / unreadable
  - not a valid PNG
  (dimension mismatch is NOT an error — it is surfaced as DimensionMismatch=true in result)
```

### `diff.CompareBytes`

```
Input:
  baselinePNG []byte
  currentPNG  []byte
  opts        PixelDiffOptions

Output:
  PixelDiffResult, error

Errors:
  - invalid PNG data
```

### `cmd.RunDiff` (internal)

Called by `main.go` switch case `"diff"`. Orchestrates:
1. Resolve snapshot and baseline IDs from flags/state.
2. Enumerate screenshot pairs (URL × breakpoint) present in both snapshots.
3. For each pair: call `diff.CompareFiles`, write diff PNG to `diffs/` subdirectory.
4. Aggregate results; call `output.Success("diff", result)`.

---

## Security Considerations

- **Path traversal**: snapshot/baseline IDs used to construct file paths must be validated to contain only alphanumeric characters, hyphens, and underscores before path joining. Reject IDs containing `..` or `/`.
- **PNG decode safety**: `image/png` from the Go standard library handles malformed input without panicking; errors are returned and surfaced via `output.Fail`.
- **File write scope**: diff PNGs are written only inside `.kaleidoscope/snapshots/<id>/diffs/`, scoped to the project-local or home state directory. No writes outside that boundary.
- **No shell execution**: the pixel diff engine uses only pure Go; no `exec.Command` or external binaries are invoked.

---

## Testing Strategy

All tests via `go test ./...` (quality gate from PRD).

### `diff/pixeldiff_test.go`

| Test | Description |
|---|---|
| `TestIdenticalImages` | Two identical PNGs → score=1.0, changedPixels=0, Regressed=false |
| `TestCompletelyDifferent` | Solid white vs solid black → score≈0.0, Regressed=true |
| `TestPartialChange` | Image with small region changed → score between 0.9 and 1.0, changedPixels matches region size |
| `TestDimensionMismatch` | Different-sized PNGs → DimensionMismatch=true, score=0.0, no panic |
| `TestThresholdFlag` | score=0.97 with threshold=0.98 → Regressed=true; with threshold=0.95 → Regressed=false |
| `TestDiffImageOutput` | Valid diff PNG is produced; can be re-decoded; changed pixels are highlighted colour |
| `TestCompareFiles` | Round-trip: write PNGs to temp files, CompareFiles returns consistent result with CompareBytes |

### `cmd/diff_test.go`

| Test | Description |
|---|---|
| `TestDiffJSONOutput` | Mocked snapshot dir with two screenshot pairs → JSON output includes screenshotDiffs array |
| `TestDiffMissingBaseline` | No baseline configured → output.Fail with descriptive message |
| `TestDiffThresholdFlag` | `--threshold 0.99` overrides default in JSON result |

---

## Implementation Notes

- The `diffs/` subdirectory is created on demand inside `cmd/diff.go` using `os.MkdirAll`. No new browser state changes required.
- Diff PNG filename pattern: `<width>x<height>-diff.png` mirrors the source screenshot naming convention.
- When a screenshot exists in the current snapshot but not in the baseline (new URL or new breakpoint), report `similarityScore: null` and `regressed: false` (new, not regressed).
- When a screenshot exists in the baseline but not in the current snapshot (removed URL/breakpoint), report `similarityScore: null` and `regressed: true` (disappeared).
- The `ks diff` command exits with code 2 on any fatal error (consistent with other commands). It exits with code 0 even when `anyRegressed: true` — the caller (agent or pre-commit hook) interprets the JSON.
