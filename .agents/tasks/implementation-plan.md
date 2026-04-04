# Implementation Plan: Screenshot Pixel Diff (US-004)

## Context

Implements pure-Go pixel-level visual comparison between baseline and current screenshots. Produces a diff PNG with highlighted changed regions and a similarity score (0.0–1.0). Integrates into `ks diff` command output alongside audit and element diffs.

**Depends on:** US-002 (snapshot system, baseline manager, `ks diff` skeleton — assumed to exist or be scaffolded by the time US-004 is wired up).

**No external dependencies:** Uses only Go standard library (`image`, `image/color`, `image/draw`, `image/png`, `os`, `errors`).

---

## Phase 1: `snapshot` Package — Pure Diff Engine

> Goal: Create `snapshot/pixeldiff.go` as a self-contained, testable pure-function module with no browser or CLI dependencies.

### Task 1.1 — Create `snapshot/` package directory and file

- **Sub-task 1.1.1:** Create `/workspace/snapshot/pixeldiff.go` with `package snapshot` declaration and required standard-library imports (`errors`, `image`, `image/color`, `image/draw`, `image/png`, `os`, `path/filepath`, `fmt`, `time`).

### Task 1.2 — Define `DiffConfig` type

- **Sub-task 1.2.1:** Declare `DiffConfig` struct:
  ```go
  type DiffConfig struct {
      SimilarityThreshold float64    // default 0.99
      HighlightColor      color.RGBA // default {255, 0, 0, 255}
  }
  ```
- **Sub-task 1.2.2:** Add `DefaultDiffConfig()` constructor returning the default values so callers can start from sensible defaults and override selectively.

### Task 1.3 — Define `ScreenshotDiffResult` type

- **Sub-task 1.3.1:** Declare `ScreenshotDiffResult` struct with JSON tags:
  ```go
  type ScreenshotDiffResult struct {
      BaselinePath      string  `json:"baselinePath"`
      CurrentPath       string  `json:"currentPath"`
      DiffPath          string  `json:"diffPath"`
      SimilarityScore   float64 `json:"similarityScore"`
      Regressed         bool    `json:"regressed"`
      DimensionMismatch bool    `json:"dimensionMismatch,omitempty"`
      Error             string  `json:"error,omitempty"`
  }
  ```

### Task 1.4 — Define sentinel error

- **Sub-task 1.4.1:** Declare package-level sentinel:
  ```go
  var ErrDimensionMismatch = errors.New("images have different dimensions")
  ```

### Task 1.5 — Implement `LoadPNG` helper

- **Sub-task 1.5.1:** Implement `LoadPNG(path string) (image.Image, error)`:
  - Open file via `os.Open`.
  - Decode with `png.Decode`.
  - Return image or wrapped error.
  - Add doc comment noting that large screenshots (>20 MP) are decoded fully into memory.

### Task 1.6 — Implement `SavePNG` helper

- **Sub-task 1.6.1:** Implement `SavePNG(path string, img image.Image) error`:
  - Create parent directories via `os.MkdirAll(filepath.Dir(path), 0755)`.
  - Create/truncate file via `os.Create`.
  - Encode with `png.Encode`.
  - Return any error.

### Task 1.7 — Implement `DiffImages` core function

- **Sub-task 1.7.1:** Implement function signature:
  ```go
  func DiffImages(baseline, current image.Image, cfg DiffConfig) (diff *image.RGBA, score float64, err error)
  ```
- **Sub-task 1.7.2:** Dimension guard — compare `baseline.Bounds()` vs `current.Bounds()`; if they differ return `nil, 0.0, ErrDimensionMismatch`.
- **Sub-task 1.7.3:** Allocate `diff = image.NewRGBA(bounds)` and copy `current` pixels into it using `draw.Draw(diff, bounds, current, bounds.Min, draw.Src)` so unchanged areas show the actual UI.
- **Sub-task 1.7.4:** Pixel iteration loop over every `(x, y)` in bounds:
  - Fetch `baseline.At(x, y)` and `current.At(x, y)`, convert to `color.RGBA`.
  - Compute `delta = |R1-R2| + |G1-G2| + |B1-B2|` (integer arithmetic using `uint32` channel values scaled to 8-bit).
  - If `delta > 10` (noise floor), increment `changedPixels` counter and paint `cfg.HighlightColor` at `(x, y)` in diff image.
- **Sub-task 1.7.5:** Compute `score = 1.0 - float64(changedPixels) / float64(totalPixels)`. Handle edge case of `totalPixels == 0` by returning `score = 1.0`.
- **Sub-task 1.7.6:** Return `diff, score, nil`.

### Task 1.8 — Implement `DiffScreenshotFiles` orchestration helper

- **Sub-task 1.8.1:** Implement function signature:
  ```go
  func DiffScreenshotFiles(baselinePath, currentPath, diffDir string, cfg DiffConfig) ScreenshotDiffResult
  ```
- **Sub-task 1.8.2:** Initialize result with `BaselinePath` and `CurrentPath` set.
- **Sub-task 1.8.3:** Call `LoadPNG(baselinePath)`. On error: set `result.Error`, `result.Regressed = true`, return early.
- **Sub-task 1.8.4:** Call `LoadPNG(currentPath)`. On error: set `result.Error`, `result.Regressed = true`, return early.
- **Sub-task 1.8.5:** Call `DiffImages(baseline, current, cfg)`. On `ErrDimensionMismatch`: set `result.DimensionMismatch = true`, `result.SimilarityScore = 0.0`, `result.Regressed = true`, return early (no diff PNG written). On other errors: set `result.Error`, `result.Regressed = true`, return early.
- **Sub-task 1.8.6:** Determine diff PNG filename: `diff_<timestamp>.png` using `fmt.Sprintf("diff_%d.png", time.Now().Unix())`. Write to `diffDir` via `SavePNG`. On save error: set `result.Error`, `result.Regressed = true` (score still populated).
- **Sub-task 1.8.7:** Set `result.DiffPath = <written path>`, `result.SimilarityScore = score`.
- **Sub-task 1.8.8:** Set `result.Regressed = score < cfg.SimilarityThreshold`.
- **Sub-task 1.8.9:** Return result.

---

## Phase 2: Unit Tests

> Goal: Create `snapshot/pixeldiff_test.go` covering all acceptance criteria and edge cases from the tech spec test plan.

### Task 2.1 — Test helpers

- **Sub-task 2.1.1:** Add `makeImage(w, h int, c color.RGBA) image.Image` test helper that returns a solid-color `*image.RGBA`.
- **Sub-task 2.1.2:** Add `tmpPNG(t, img image.Image) string` test helper that writes image to a temp file and returns its path (cleaned up via `t.Cleanup`).

### Task 2.2 — Test: identical images → score 1.0

- **Sub-task 2.2.1:** Create 100×100 solid-red image. Call `DiffImages` with itself as both inputs.
- **Sub-task 2.2.2:** Assert `err == nil`, `score == 1.0`, diff image has no highlighted pixels (all pixels match current).

### Task 2.3 — Test: fully different images → score ~0.0

- **Sub-task 2.3.1:** Create 100×100 solid-black and solid-white images. Call `DiffImages`.
- **Sub-task 2.3.2:** Assert `score == 0.0` (all 10,000 pixels changed).
- **Sub-task 2.3.3:** Assert every pixel in diff image equals `HighlightColor` (red).

### Task 2.4 — Test: single pixel changed in 100×100

- **Sub-task 2.4.1:** Create two identical 100×100 images, manually set one pixel to a different color in the second.
- **Sub-task 2.4.2:** Call `DiffImages`. Assert `score == 1.0 - 1.0/10000.0` (i.e., `0.9999`).
- **Sub-task 2.4.3:** Assert exactly one pixel in the diff image is `HighlightColor`.

### Task 2.5 — Test: dimension mismatch

- **Sub-task 2.5.1:** Create 100×100 and 200×200 images. Call `DiffImages`.
- **Sub-task 2.5.2:** Assert `err == ErrDimensionMismatch`, `score == 0.0`, `diff == nil`.

### Task 2.6 — Test: threshold regressed/not-regressed

- **Sub-task 2.6.1:** Construct a scenario producing `score ≈ 0.985` (e.g., change 150 pixels in a 10,000-pixel image). Use `cfg.SimilarityThreshold = 0.99`.
- **Sub-task 2.6.2:** Call `DiffScreenshotFiles` via temp files. Assert `result.Regressed == true`.
- **Sub-task 2.6.3:** Construct a scenario producing `score ≈ 0.995` (change 50 pixels). Assert `result.Regressed == false`.

### Task 2.7 — Test: corrupt PNG input

- **Sub-task 2.7.1:** Write a file with invalid PNG bytes to a temp path.
- **Sub-task 2.7.2:** Call `DiffScreenshotFiles` with corrupt file as `baselinePath`. Assert `result.Regressed == true` and `result.Error != ""`.

### Task 2.8 — Test: `DiffImages` noise floor

- **Sub-task 2.8.1:** Create two images where one pixel differs by exactly `delta == 10` (should NOT be flagged) and another where `delta == 11` (SHOULD be flagged). Assert counts accordingly.

---

## Phase 3: Extend `cmd/diff.go` for Screenshot Diff Integration

> Goal: Wire the pixel diff engine into the `ks diff` command so screenshot pairs are compared and results appear in JSON output. Assumes US-002 has scaffolded `cmd/diff.go` with `DiffOutput`, snapshot loading, and `--threshold` flag stub.

### Task 3.1 — Define `ScreenshotDiffs` struct in `cmd/diff.go`

- **Sub-task 3.1.1:** Add to `cmd/diff.go`:
  ```go
  type ScreenshotDiffs struct {
      Pairs     []snapshot.ScreenshotDiffResult `json:"pairs"`
      Regressed bool                            `json:"regressed"`
  }
  ```

### Task 3.2 — Extend `DiffOutput` struct with `ScreenshotDiff` field

- **Sub-task 3.2.1:** Add `ScreenshotDiff ScreenshotDiffs \`json:"screenshotDiff"\`` field to the existing `DiffOutput` struct in `cmd/diff.go`.

### Task 3.3 — Add `--threshold` CLI flag

- **Sub-task 3.3.1:** In the `diff` command's argument parsing block, read `--threshold` flag (float, default `0.99`) using the existing `getFlagValue` helper pattern in `cmd/util.go`.
- **Sub-task 3.3.2:** Construct `snapshot.DiffConfig{SimilarityThreshold: threshold, HighlightColor: color.RGBA{255, 0, 0, 255}}`.

### Task 3.4 — Implement screenshot pair matching loop

- **Sub-task 3.4.1:** After loading baseline and current snapshots, iterate over `baseline.Screenshots`. For each entry `(URL, Breakpoint, Path)`:
  - Search `current.Screenshots` for a matching entry where `URL` and `Breakpoint` match.
  - If **no match found**: append a `ScreenshotDiffResult` with `Regressed: true`, `SimilarityScore: 0.0`, `Error: "screenshot missing in current snapshot"` to `pairs`.
  - If **match found**: determine `diffDir` as the directory of the current screenshot path; call `snapshot.DiffScreenshotFiles(baseline.Path, current.Path, diffDir, cfg)`; append result.
- **Sub-task 3.4.2:** After the loop, set `ScreenshotDiffs.Regressed = true` if any pair has `Regressed == true`.

### Task 3.5 — Merge screenshot diff into top-level `Regressed` flag

- **Sub-task 3.5.1:** Update the final `DiffOutput.Regressed` computation to be `true` if `AuditDiff.Regressed || ElementDiff.Regressed || ScreenshotDiff.Regressed`.

### Task 3.6 — Emit result via `output.Success`

- **Sub-task 3.6.1:** Confirm the populated `DiffOutput` (now including `ScreenshotDiff`) is passed to `output.Success` and printed to stdout as JSON. No changes needed if US-002 already does this; verify and confirm.

---

## Phase 4: Command Registration in `main.go`

> Goal: Ensure the `diff` command is routable from the CLI entry point.

### Task 4.1 — Verify `diff` is registered in `main.go`

- **Sub-task 4.1.1:** Read `main.go` to check if `"diff"` is already in the command dispatch switch/map (US-002 may have added it).
- **Sub-task 4.1.2:** If absent, add `case "diff": cmd.Diff(args)` (or equivalent) to the command router in `main.go` alongside existing commands.

---

## Phase 5: Quality Gate

> Goal: Ensure `go test ./...` passes and the implementation compiles cleanly.

### Task 5.1 — Verify package compiles

- **Sub-task 5.1.1:** Run `go build ./...` and resolve any compilation errors (import cycles, missing types, unused imports).

### Task 5.2 — Run full test suite

- **Sub-task 5.2.1:** Run `go test ./...` and confirm all tests pass, including the new `snapshot` package tests.
- **Sub-task 5.2.2:** If any tests fail, diagnose and fix (type mismatches, off-by-one in pixel counting, file path issues).

---

## File Summary

| File | Action | Phase |
|------|--------|-------|
| `snapshot/pixeldiff.go` | **Create** — pure diff engine, types, helpers | 1 |
| `snapshot/pixeldiff_test.go` | **Create** — unit tests for all acceptance criteria | 2 |
| `cmd/diff.go` | **Extend** — add `ScreenshotDiffs`, loop, `--threshold` flag | 3 |
| `main.go` | **Verify/extend** — register `diff` command if missing | 4 |

---

## Dependency Map

```
Phase 1 (pixeldiff.go)
    └── Phase 2 (pixeldiff_test.go)   [needs Phase 1 types]
    └── Phase 3 (cmd/diff.go)         [needs Phase 1 types and functions]
            └── Phase 4 (main.go)     [needs Phase 3 command to exist]
                    └── Phase 5 (quality gate)
```

Phases 2 and 3 can be developed in parallel once Phase 1 is complete. Phase 4 is a single verification step. Phase 5 is the final gate.

---

## Key Design Constraints (from PRD rules)

1. **Pure Go only** — no ImageMagick, no shell exec, no CGo. Use `image`, `image/color`, `image/draw`, `image/png` from stdlib.
2. **`output.Result` convention** — `ks diff` JSON output must go through `output.Success`/`output.Fail`.
3. **No snapshot logic duplication** — `cmd/diff.go` calls `snapshot.DiffScreenshotFiles`; it does not re-implement pixel comparison inline.
4. **Security** — diff PNG paths constructed from snapshot manifest data only; no raw user CLI strings reach `os.ReadFile`/`os.WriteFile` in the engine.
5. **`.kaleidoscope/snapshots/` is gitignored** — diff PNGs written there are never accidentally committed.
