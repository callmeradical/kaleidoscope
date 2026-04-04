# Tech Spec: Screenshot Pixel Diff (US-004)

## Overview

Implements pure-Go pixel-level visual comparison between baseline and current screenshots. Produces a diff PNG with highlighted changed regions and a similarity score (0.0–1.0). Integrates into the `ks diff` command output alongside audit and element diffs.

**Depends on:** US-002 (snapshot system, baseline manager, `ks diff` command skeleton)

---

## Architecture Overview

```
snapshot/
  pixeldiff.go        # pure-function diff engine (no Chrome, no browser deps)

cmd/
  diff.go             # ks diff command — orchestrates audit+element+pixel diffs
                      # (partially scaffolded by US-002; extended here)
```

The pixel diff engine lives in a dedicated `snapshot` package as a pure function:
- **Input:** two `image.Image` values + config
- **Output:** diff `image.RGBA`, similarity score `float64`, error

This keeps it independently testable with no browser, filesystem, or CLI dependencies.

---

## Component Design

### 1. `snapshot` Package — `pixeldiff.go`

#### Types

```go
// DiffConfig controls diff behavior.
type DiffConfig struct {
    // SimilarityThreshold below which a pair is flagged as regressed (0.0–1.0).
    // Default: 0.99
    SimilarityThreshold float64

    // HighlightColor is the RGBA color used to paint changed pixels in the diff image.
    // Default: {255, 0, 0, 255} (opaque red)
    HighlightColor color.RGBA
}

// ScreenshotDiffResult is the output of a single image comparison.
type ScreenshotDiffResult struct {
    BaselinePath   string  `json:"baselinePath"`
    CurrentPath    string  `json:"currentPath"`
    DiffPath       string  `json:"diffPath"`        // empty if dimensions mismatch
    SimilarityScore float64 `json:"similarityScore"` // 0.0 on dimension mismatch
    Regressed      bool    `json:"regressed"`
    DimensionMismatch bool `json:"dimensionMismatch,omitempty"`
}
```

#### Core Function

```go
// DiffImages compares two images pixel by pixel.
// Returns a diff image, similarity score (0.0–1.0), and any decode error.
// If dimensions differ, returns nil image, score 0.0, and sets a sentinel error
// (callers should check and set DimensionMismatch rather than treating as fatal).
func DiffImages(baseline, current image.Image, cfg DiffConfig) (diff *image.RGBA, score float64, err error)
```

**Algorithm:**
1. Compare `baseline.Bounds()` vs `current.Bounds()`. If they differ, return `nil, 0.0, ErrDimensionMismatch`.
2. Allocate `image.RGBA` of identical bounds, initialized to the `current` image pixels (so unchanged areas show the actual UI).
3. Iterate every pixel `(x, y)`:
   - Compute per-channel absolute difference: `|R1-R2| + |G1-G2| + |B1-B2|`
   - If difference exceeds a noise floor (default: 10 per channel sum), count pixel as "changed" and paint `HighlightColor` onto the diff image at `(x, y)`.
4. `score = 1.0 - (changedPixels / totalPixels)`
5. Return diff image, score, nil.

**Sentinel error:**
```go
var ErrDimensionMismatch = errors.New("images have different dimensions")
```

#### File I/O Helper

```go
// LoadPNG decodes a PNG from disk into image.Image.
func LoadPNG(path string) (image.Image, error)

// SavePNG encodes an image.Image and writes it to path, creating dirs as needed.
func SavePNG(path string, img image.Image) error
```

#### Orchestration Helper

```go
// DiffScreenshotFiles loads two PNG paths, diffs them, writes the diff PNG,
// and returns a ScreenshotDiffResult. diffDir is where the diff PNG is written.
func DiffScreenshotFiles(baselinePath, currentPath, diffDir string, cfg DiffConfig) ScreenshotDiffResult
```

- Diff PNG filename: `diff_<url-slug>_<breakpoint>_<timestamp>.png`
- On `ErrDimensionMismatch`: sets `DimensionMismatch: true`, `SimilarityScore: 0.0`, `Regressed: true`, skips writing diff PNG.
- On other errors (file not found, corrupt PNG): sets `Regressed: true`, propagates error message into a `Error string` field on the result struct.

---

### 2. Integration into `ks diff` — `cmd/diff.go`

US-002 defines the snapshot data model and the `ks diff` skeleton. This story extends it.

#### Assumed US-002 Snapshot Model (referenced, not redefined here)

```go
// Snapshot (from US-002)
type Snapshot struct {
    ID         string            `json:"id"`
    CommitHash string            `json:"commitHash"`
    CreatedAt  time.Time         `json:"createdAt"`
    Screenshots []SnapshotScreenshot `json:"screenshots"`
    // ...audit results, ax-tree, etc.
}

type SnapshotScreenshot struct {
    URL        string `json:"url"`
    Breakpoint string `json:"breakpoint"` // e.g. "1280x720"
    Path       string `json:"path"`       // absolute or relative to .kaleidoscope/
}
```

#### Extended `DiffOutput` (add screenshot section)

```go
type ScreenshotDiffs struct {
    Pairs     []ScreenshotDiffResult `json:"pairs"`
    Regressed bool                   `json:"regressed"`
}

// Merged into the existing DiffOutput struct from US-002:
type DiffOutput struct {
    BaselineID      string          `json:"baselineId"`
    CurrentID       string          `json:"currentId"`
    AuditDiff       AuditDiff       `json:"auditDiff"`       // from US-002
    ElementDiff     ElementDiff     `json:"elementDiff"`     // from US-003
    ScreenshotDiff  ScreenshotDiffs `json:"screenshotDiff"`  // NEW (US-004)
    Regressed       bool            `json:"regressed"`
}
```

#### Diff Execution Flow (screenshot portion)

```
for each URL × breakpoint in baseline.Screenshots:
    find matching entry in current.Screenshots (same URL + breakpoint)
    if not found → mark as full regression (missing screenshot)
    else → call DiffScreenshotFiles(baseline.Path, current.Path, diffDir, cfg)
    append ScreenshotDiffResult to pairs
ScreenshotDiffs.Regressed = any pair where .Regressed == true
```

#### CLI Flag

```
ks diff [--threshold 0.99]
```

`--threshold` sets `DiffConfig.SimilarityThreshold` (float, 0.0–1.0, default 0.99). Parsed in `cmd/diff.go` via the existing `getFlagValue` helper.

---

## Data Model Changes

No new persistent files. Diff PNG files are written into the existing snapshot directory alongside the source screenshots:

```
.kaleidoscope/
  snapshots/
    <snapshot-id>/
      screenshots/
        screenshot_<slug>_<breakpoint>.png   # captured by US-002
        diff_<slug>_<breakpoint>_<ts>.png    # NEW: written by pixel diff
  baselines.json   # unchanged (committed)
  state.json       # unchanged
```

`ScreenshotDiffResult.DiffPath` stores the path to the written diff PNG, included in the `ks diff` JSON output so agents and tooling can locate it without scanning the filesystem.

---

## API Definitions

### `ks diff` JSON Output (stdout)

```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "baselineId": "snap_abc123",
    "currentId": "snap_def456",
    "auditDiff": { ... },
    "elementDiff": { ... },
    "screenshotDiff": {
      "regressed": true,
      "pairs": [
        {
          "baselinePath": ".kaleidoscope/snapshots/snap_abc123/screenshots/shot_home_1280x720.png",
          "currentPath":  ".kaleidoscope/snapshots/snap_def456/screenshots/shot_home_1280x720.png",
          "diffPath":     ".kaleidoscope/snapshots/snap_def456/screenshots/diff_home_1280x720_1712345678.png",
          "similarityScore": 0.9872,
          "regressed": true,
          "dimensionMismatch": false
        },
        {
          "baselinePath": ".kaleidoscope/snapshots/snap_abc123/screenshots/shot_home_375x812.png",
          "currentPath":  ".kaleidoscope/snapshots/snap_def456/screenshots/shot_home_375x812.png",
          "diffPath":     "",
          "similarityScore": 0.0,
          "regressed": true,
          "dimensionMismatch": true
        }
      ]
    },
    "regressed": true
  }
}
```

---

## File Layout

| File | Contents |
|------|----------|
| `snapshot/pixeldiff.go` | `DiffImages`, `DiffScreenshotFiles`, `LoadPNG`, `SavePNG`, `DiffConfig`, `ScreenshotDiffResult`, `ErrDimensionMismatch` |
| `snapshot/pixeldiff_test.go` | Unit tests: identical images → score 1.0; fully different → score ~0.0; partial diff pixel count; dimension mismatch returns sentinel error |
| `cmd/diff.go` | Extended to call `DiffScreenshotFiles` per pair and merge into `DiffOutput.ScreenshotDiff` |

---

## Security Considerations

- **Path traversal:** `DiffScreenshotFiles` resolves all input paths against the known snapshot directory. Callers in `cmd/diff.go` must construct paths from the snapshot manifest, not from user-supplied CLI strings. No raw user input reaches `os.ReadFile`/`os.WriteFile` in the diff engine.
- **Memory bounds:** Images are decoded into `image.Image` in-memory. Very large screenshots (e.g. full-page captures >20 MP) could consume significant RAM. No mitigation is required now (non-goal per PRD), but `LoadPNG` should document this.
- **No external processes:** Pure Go standard library only (`image`, `image/color`, `image/draw`, `image/png`). No shell exec, no ImageMagick, no CGo.

---

## Test Plan

| Test | Assertion |
|------|-----------|
| Identical PNGs | `score == 1.0`, diff image has no highlighted pixels |
| Fully black vs fully white | `score == 0.0`, every pixel highlighted |
| Single pixel changed in 100×100 image | `score == 0.9999`, one highlighted pixel |
| Dimension mismatch (100×100 vs 200×200) | Returns `ErrDimensionMismatch`, score 0.0 |
| Threshold check at 0.99 | Score 0.985 → `Regressed: true`; score 0.995 → `Regressed: false` |
| Corrupt PNG input | Returns error, `Regressed: true` |
| `go test ./...` passes | Quality gate from PRD |

---

## Open Questions (from PRD)

1. **Deduplication:** Not in scope for this story; diff always writes a new PNG. If US-002 adds deduplication, diff PNG paths should still be unique per run (timestamp suffix handles this).
2. **Arbitrary snapshot comparison:** `ks diff <snap-a> <snap-b>` is out of scope for US-004. The diff engine is designed as a pure function, so extending `cmd/diff.go` to accept two snapshot IDs later is straightforward.
