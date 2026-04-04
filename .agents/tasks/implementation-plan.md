# Implementation Plan: US-004 — Screenshot Pixel Diff

## Overview

Implement a pure-Go pixel-level image comparison engine for the `ks diff` command. The feature produces highlighted diff PNG images and similarity scores when comparing baseline vs. current screenshots.

**New files:** `diff/pixel.go`, `diff/io.go`, `diff/pixel_test.go`, `cmd/diff.go`
**Modified files:** `main.go`
**No new external dependencies.**

---

## Phase 1: `diff` Package — Core Engine

### Task 1.1 — Create `diff/pixel.go`

**Sub-tasks:**

1.1.1 Create `/workspace/diff/pixel.go` with `package diff` declaration.

1.1.2 Define `ScreenshotDiffResult` struct:
```go
type ScreenshotDiffResult struct {
    BaselinePath      string  `json:"baselinePath"`
    CurrentPath       string  `json:"currentPath"`
    DiffPath          string  `json:"diffPath"`
    Similarity        float64 `json:"similarity"`
    Regressed         bool    `json:"regressed"`
    DimensionMismatch bool    `json:"dimensionMismatch,omitempty"`
    Width             int     `json:"width"`
    Height            int     `json:"height"`
}
```

1.1.3 Define `Options` struct:
```go
type Options struct {
    Threshold           uint8
    SimilarityThreshold float64
    HighlightColor      [4]uint8
}
```

1.1.4 Implement `DefaultOptions() Options` returning:
- `Threshold: 10`
- `SimilarityThreshold: 0.99`
- `HighlightColor: [4]uint8{255, 0, 0, 255}` (opaque red)

1.1.5 Implement `CompareImages(baseline, current image.Image, opts Options) (ScreenshotDiffResult, image.Image)`:
- Step A — Dimension check: compare `baseline.Bounds()` to `current.Bounds()`; if they differ, return `DimensionMismatch: true`, `Similarity: 0.0`, `Regressed: true`, and `nil` image.
- Step B — Initialize a new `image.RGBA` of the same bounds as baseline for the diff output image.
- Step C — Pixel iteration loop over all `(x, y)` in bounds:
  - Call `.RGBA()` on both pixels (returns uint32 in 0–65535 range); shift right by 8 to get 8-bit values.
  - Compare R, G, B channels against `opts.Threshold`; if any channel difference exceeds threshold, increment `diffCount` and write `opts.HighlightColor` to the diff image at `(x, y)`.
  - Otherwise, copy the baseline pixel to the diff image (preserving alpha).
- Step D — Calculate `similarity = 1.0 - float64(diffCount)/float64(totalPixels)`; handle zero-pixel edge case to avoid division by zero.
- Step E — Set `regressed = similarity < opts.SimilarityThreshold`.
- Step F — Populate and return `ScreenshotDiffResult` (leave `DiffPath` empty — caller sets it) and the diff `image.RGBA`.

---

### Task 1.2 — Create `diff/io.go`

**Sub-tasks:**

1.2.1 Create `/workspace/diff/io.go` with `package diff` declaration.

1.2.2 Implement `LoadPNG(path string) (image.Image, error)`:
- Open the file at `path` using `os.Open`.
- Decode with `png.Decode`.
- Return the decoded image or the error.

1.2.3 Implement `SavePNG(path string, img image.Image) error`:
- Create the file at `path` using `os.Create`.
- Encode with `png.Encode`.
- Return any error.

1.2.4 Implement `DiffImagePath(snapshotDir, baselineFile, currentFile string) string`:
- Extract the base filenames (without extension) from `baselineFile` and `currentFile` using `filepath.Base` and trimming `.png`.
- Derive a suffix from `currentFile`'s base name (e.g., strip any "current-" prefix; or simply use the current file's base minus extension as a label).
- Append a Unix timestamp (via `time.Now().Unix()`) to make the filename unique.
- Return `filepath.Join(snapshotDir, fmt.Sprintf("diff-%s-%d.png", label, timestamp))`.
  - The label should be derived from `currentFile`'s base name (without path or extension).

---

## Phase 2: Unit Tests

### Task 2.1 — Create `diff/pixel_test.go`

**Sub-tasks:**

2.1.1 Create `/workspace/diff/pixel_test.go` with `package diff` (white-box tests) or `package diff_test`.

2.1.2 Implement `TestDefaultOptions`: verify `Threshold == 10`, `SimilarityThreshold == 0.99`, `HighlightColor == [4]uint8{255, 0, 0, 255}`.

2.1.3 Implement `TestIdenticalImages`:
- Create two 10×10 solid-green `image.RGBA` images.
- Call `CompareImages` with default options.
- Assert `Similarity == 1.0`, `Regressed == false`, `DimensionMismatch == false`.
- Assert diff image is non-nil and no pixels are highlighted (all pixels match baseline color).

2.1.4 Implement `TestCompletelyDifferentImages`:
- Create a 10×10 solid-red image (baseline) and a 10×10 solid-blue image (current).
- Call `CompareImages` with default options.
- Assert `Similarity` is near 0.0 (all pixels different), `Regressed == true`.
- Assert all pixels in diff image are highlight color.

2.1.5 Implement `TestPartialDiff`:
- Create a 10×10 solid-white baseline; create a current image that is identical except the entire bottom row (y=9) is black.
- Call `CompareImages` with default options.
- Assert `diffCount == 10` (one row of width 10).
- Assert `Similarity == float64(90)/float64(100)` = `0.9`.

2.1.6 Implement `TestDimensionMismatch`:
- Create a 10×10 image and a 20×20 image.
- Call `CompareImages`.
- Assert `DimensionMismatch == true`, `Similarity == 0.0`, `Regressed == true`.
- Assert returned diff image is `nil`.

2.1.7 Implement `TestThresholdTolerance`:
- Set `opts.Threshold = 20`.
- Create baseline pixel with R=100; create current pixel with R=115 (diff = 15 < 20).
- Assert pixel is NOT counted as different (similarity remains 1.0).
- Also test pixel with R=121 (diff = 21 > 20) IS counted as different.

2.1.8 Implement `TestRegressionFlag`:
- Use `opts.SimilarityThreshold = 0.95`.
- Construct images where similarity computes to exactly 0.94 (flag regressed) and 0.96 (not flagged).
- Assert `Regressed` matches expectation in each case.

2.1.9 Implement `TestLoadSavePNG`:
- Use `t.TempDir()` for the temporary file path.
- Create a small `image.RGBA` (e.g., 4×4) with distinct pixel colors.
- Call `SavePNG(tmpPath, img)` and assert no error.
- Call `LoadPNG(tmpPath)` and assert no error.
- Decode and compare pixel values to verify round-trip fidelity.

---

## Phase 3: `ks diff` CLI Command

### Task 3.1 — Create `cmd/diff.go`

**Sub-tasks:**

3.1.1 Create `/workspace/cmd/diff.go` with `package cmd` declaration and necessary imports (`flag`, `fmt`, `encoding/json`, `os`, `path/filepath`, `regexp`, `github.com/callmeradical/kaleidoscope/diff`, `github.com/callmeradical/kaleidoscope/output`).

3.1.2 Define the `DiffOutput` struct (top-level JSON result):
```go
type DiffOutput struct {
    BaselineSnapshotID string                      `json:"baselineSnapshotID"`
    CurrentSnapshotID  string                      `json:"currentSnapshotID"`
    ScreenshotDiffs    []ScreenshotDiffEntry       `json:"screenshotDiffs"`
    AuditDiffs         []interface{}               `json:"auditDiffs"`
    ElementDiffs       []interface{}               `json:"elementDiffs"`
    Threshold          float64                     `json:"threshold"`
    HasRegressions     bool                        `json:"hasRegressions"`
}
```

3.1.3 Define `ScreenshotDiffEntry` wrapping `diff.ScreenshotDiffResult` with additional `URL` and `Breakpoint` fields:
```go
type ScreenshotDiffEntry struct {
    URL       string `json:"url"`
    Breakpoint string `json:"breakpoint"`
    diff.ScreenshotDiffResult
}
```

3.1.4 Define local snapshot manifest types matching US-002's assumed schema:
```go
type snapshotManifest struct {
    ID          string               `json:"id"`
    CreatedAt   string               `json:"createdAt"`
    Screenshots []snapshotScreenshot `json:"screenshots"`
}

type snapshotScreenshot struct {
    URL        string `json:"url"`
    Breakpoint string `json:"breakpoint"`
    Path       string `json:"path"`
    Width      int    `json:"width"`
    Height     int    `json:"height"`
}
```

3.1.5 Define `baselines.json` loading struct:
```go
type baselinesFile struct {
    CurrentBaseline string `json:"currentBaseline"`
}
```

3.1.6 Implement input validation helper `validateSnapshotID(id string) bool`:
- Allow only alphanumeric characters, hyphens, and underscores using regexp `^[a-zA-Z0-9_-]+$`.
- Return false for any value containing `..`, `/`, `\`, or not matching the pattern.

3.1.7 Implement `loadSnapshotManifest(snapshotID string) (*snapshotManifest, error)`:
- Validate `snapshotID` via `validateSnapshotID`; return error if invalid.
- Construct path: `.kaleidoscope/snapshots/<snapshotID>/snapshot.json`.
- Read and JSON-decode the file; return the manifest or error.

3.1.8 Implement `resolveBaselineID() (string, error)`:
- Read `.kaleidoscope/baselines.json`.
- Decode into `baselinesFile`.
- Return `currentBaseline` field or an error if the file is missing/malformed.

3.1.9 Implement `resolveCurrentSnapshotID() (string, error)`:
- List entries in `.kaleidoscope/snapshots/` directory.
- Sort by name (snapshot IDs are expected to be timestamp-based and lexicographically ordered).
- Return the last entry's name as the most recent snapshot ID.
- Return an error if no snapshots exist.

3.1.10 Implement `RunDiff(args []string)`:
- Parse flags: `--baseline`, `--current`, `--threshold` (default 0.99) using `flag.FlagSet`.
- Resolve baseline ID: use flag value if provided (validate it); otherwise call `resolveBaselineID()`.
- Resolve current ID: use flag value if provided (validate it); otherwise call `resolveCurrentSnapshotID()`.
- Load baseline manifest via `loadSnapshotManifest`.
- Load current manifest via `loadSnapshotManifest`.
- Build a lookup map from the baseline manifest: `key=(url,breakpoint) -> snapshotScreenshot`.
- Build diff options from threshold: `opts := diff.DefaultOptions(); opts.SimilarityThreshold = threshold`.
- Iterate over current manifest screenshots:
  - Look up the matching baseline screenshot by `(url, breakpoint)` key; skip if not found.
  - Call `diff.LoadPNG` on baseline path; on error, emit `output.Fail` and return.
  - Call `diff.LoadPNG` on current path; on error, emit `output.Fail` and return.
  - Call `diff.CompareImages(baselineImg, currentImg, opts)`.
  - Derive `diffPath` using `diff.DiffImagePath(currentSnapshotDir, baselinePath, currentPath)`.
  - If diff image is non-nil, call `diff.SavePNG(diffPath, diffImg)`.
  - Set `result.DiffPath = diffPath`.
  - Set `result.BaselinePath = baseline screenshot path`.
  - Set `result.CurrentPath = current screenshot path`.
  - Build `ScreenshotDiffEntry{URL: ..., Breakpoint: ..., ScreenshotDiffResult: result}`.
  - Append to results slice.
- Determine `hasRegressions`: true if any `entry.Regressed == true`.
- Emit `output.Success("diff", DiffOutput{...})`.

---

## Phase 4: Wire into `main.go`

### Task 4.1 — Add `diff` case to `main.go` command router

**Sub-tasks:**

4.1.1 Read `/workspace/main.go` to locate the command switch statement.

4.1.2 Add `case "diff":` branch calling `cmd.RunDiff(cmdArgs)` in the switch block, grouped with other evaluation commands.

4.1.3 Add `"diff"` to the usage/help text section in `main.go` (if there is an inline help block — consult `cmd/usage.go`).

---

## Phase 5: Usage Documentation

### Task 5.1 — Add `diff` command to `cmd/usage.go`

**Sub-tasks:**

5.1.1 Read `/workspace/cmd/usage.go` to understand the `CommandUsage` map format.

5.1.2 Add a `"diff"` entry to `CommandUsage` describing:
- Synopsis: `ks diff [--baseline <id>] [--current <id>] [--threshold <0.0-1.0>]`
- Description: Compares baseline vs. current screenshots pixel-by-pixel, writes diff PNGs, and outputs similarity scores.
- Flags: `--baseline`, `--current`, `--threshold`.
- Output: JSON with `screenshotDiffs`, `hasRegressions`, `threshold`.

---

## Phase 6: Validation

### Task 6.1 — Run tests

**Sub-tasks:**

6.1.1 Run `go test ./diff/...` to confirm all 8 unit tests pass.

6.1.2 Run `go build ./...` to confirm the full project compiles without errors.

6.1.3 Run `go vet ./...` to check for any static analysis issues.

---

## Dependency & Ordering Summary

```
Phase 1 (diff package)
  └── Task 1.1 (pixel.go) ──────┐
  └── Task 1.2 (io.go)   ──────┤
                                 ↓
Phase 2 (tests) ── depends on Phase 1
                                 ↓
Phase 3 (cmd/diff.go) ── depends on Phase 1
                                 ↓
Phase 4 (main.go wire-up) ── depends on Phase 3
Phase 5 (usage docs) ── depends on Phase 3 (can run in parallel with Phase 4)
                                 ↓
Phase 6 (validation) ── depends on Phases 1–5
```

Tasks 1.1 and 1.2 can be written in parallel. Phase 2 and Phase 3 can begin once Phase 1 is complete. Phases 4 and 5 can run in parallel after Phase 3.

---

## File Inventory

| File | Action | Phase |
|---|---|---|
| `/workspace/diff/pixel.go` | Create | 1 |
| `/workspace/diff/io.go` | Create | 1 |
| `/workspace/diff/pixel_test.go` | Create | 2 |
| `/workspace/cmd/diff.go` | Create | 3 |
| `/workspace/main.go` | Modify (add case "diff") | 4 |
| `/workspace/cmd/usage.go` | Modify (add diff entry) | 5 |

---

## Key Constraints (from spec)

- **No external dependencies** — only `image`, `image/color`, `image/draw`, `image/png`, `os`, `path/filepath`, `fmt`, `time`, `regexp`, `encoding/json` from the standard library plus existing project packages.
- **Alpha channel excluded** from pixel difference calculation (R, G, B only); alpha preserved in diff image for identical pixels.
- **Diff image is flat color overlay** — no blending with original; changed pixels are solid highlight color.
- **Exit code 0** regardless of regressions (regressions reported in JSON only).
- **`ks diff` is read-only** with respect to snapshot manifests — it only writes diff PNG files.
- **Path traversal protection** — snapshot IDs validated against `^[a-zA-Z0-9_-]+$` before use in file paths.
- **Mismatched screenshot sets** — silently skip URLs/breakpoints present in one snapshot but not the other.
