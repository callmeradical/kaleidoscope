# Implementation Plan: US-006 — Side-by-Side HTML Diff Report

## Overview

Implements `ks diff-report [snapshot-id] [--output path]`, generating a self-contained HTML report
comparing a baseline snapshot vs. a current snapshot. Adds pixel-diff overlay generation, a new
diff-report HTML template, and a new CLI command handler wired into `main.go`.

**Depends on:** US-003 (`snapshot` package), US-004 (`diff` package) — consumed as given interfaces.

---

## Phase 1: Pixel Diff Engine (`diff/pixel.go`)

Create the pure-Go pixel-diff library with no external dependencies.

### Task 1.1 — Create `diff/pixel.go`

**File:** `/workspace/diff/pixel.go`

- **Sub-task 1.1.1** — Declare package `pixeldiff` and import `image`, `image/color`, `image/draw`, `image/png`, `bytes`, `math`
- **Sub-task 1.1.2** — Define `Options` struct:
  - `Threshold uint8` (default 10)
  - `HighlightColor color.RGBA` (default red: 255,0,0,255)
  - `DimOpacity float64` (default 0.5)
- **Sub-task 1.1.3** — Implement helper `defaultOptions(opts *Options) *Options` that fills zero-value fields with defaults
- **Sub-task 1.1.4** — Implement `Overlay(img1, img2 image.Image, opts *Options) image.Image`:
  1. Call `defaultOptions`
  2. Compute output bounds: union of both images' bounds (max width, max height)
  3. Allocate `*image.RGBA` output image
  4. Loop over every (x, y) in output bounds:
     - Read RGBA from img1 (zero if out-of-bounds)
     - Read RGBA from img2 (zero if out-of-bounds)
     - Compute per-channel absolute difference; find max channel delta
     - If max delta > `Threshold`: set output pixel to `HighlightColor`
     - Else: dim img1 pixel by `DimOpacity` (multiply each channel by `1 - DimOpacity`) and set output pixel
  5. Return output image
- **Sub-task 1.1.5** — Implement `Score(img1, img2 image.Image, threshold uint8) float64`:
  1. Determine bounds (max of both images)
  2. Count total pixels and identical pixels (max channel delta ≤ threshold)
  3. Return `identical / total` as float64; return 1.0 if total == 0

### Task 1.2 — Create `diff/pixel_test.go`

**File:** `/workspace/diff/pixel_test.go`

- **Sub-task 1.2.1** — `TestOverlay_identical`: create two identical solid-color 10×10 images → `Score == 1.0`, all non-highlighted pixels in overlay
- **Sub-task 1.2.2** — `TestOverlay_fullyDifferent`: solid red vs solid blue → `Score == 0.0`, all output pixels == HighlightColor
- **Sub-task 1.2.3** — `TestOverlay_differentSizes`: img1 is 10×10, img2 is 10×15 (taller) → extra rows in overlay are highlighted (treated as blank in img1)
- **Sub-task 1.2.4** — `TestScore_threshold`: create two images that differ by exactly Threshold on one channel → pixel is identical; differ by Threshold+1 → pixel is different

---

## Phase 2: Diff Report Data Model and HTML Template (`report/diff_report.go`)

Mirrors `report/report.go` patterns with a 3-column layout.

### Task 2.1 — Create `report/diff_report.go`

**File:** `/workspace/report/diff_report.go`

- **Sub-task 2.1.1** — Declare package `report`; imports: `html/template`, `io`, `os`, `path/filepath`, `time`
- **Sub-task 2.1.2** — Define data model structs (no import cycle; duplicate delta types locally):
  - `DiffData { GeneratedAt time.Time; BaselineID, CurrentID string; BaselineAt, CurrentAt time.Time; Pages []DiffPage }`
  - `DiffPage { URL string; Breakpoints []DiffBreakpoint; AuditDelta AuditDelta; ElementChanges []ElementChangeRow }`
  - `DiffBreakpoint { Name string; Width, Height int; BaselineURI, CurrentURI, OverlayURI template.URL; DiffScore float64 }`
  - `AuditDelta { Contrast, Touch, Typography, Spacing CategoryDelta }`
  - `CategoryDelta { Before, After, Delta int }`
  - `ElementChangeRow { Selector, ChangeType, Details string }`
- **Sub-task 2.1.3** — Define template helper functions map:
  - `deltaClass(n int) string`: returns `"delta-positive"`, `"delta-negative"`, or `"delta-zero"`
  - `deltaSign(n int) string`: returns `"+N"`, `"−N"`, or `"0"`
  - `scoreClass(f float64) string`: returns `"diff-score-good"` (≥0.99), `"diff-score-warn"` (0.95–0.99), `"diff-score-bad"` (<0.95)
  - `pct(f float64) string`: formats as `"97.3%"`
- **Sub-task 2.1.4** — Write `diffReportTmpl` as a `const string` (raw string literal) with the full HTML template:
  - `<head>`: charset, viewport, `<title>Kaleidoscope Diff Report</title>`, inline `<style>` block reusing dark-theme CSS variables from `report.go` plus new `.diff-row`, `.diff-col`, `.diff-col-label`, `.diff-score-good/warn/bad`, `.delta-positive/negative/zero`, `.empty-state` rules
  - `<body>`:
    - `<h1>` header + meta row (baseline ID | current ID | generated at)
    - Range over `.Pages`:
      - `<h2>` URL
      - Range over `.Breakpoints`:
        - `<h3>` name + dimensions
        - `<div class="diff-row">` with 3 `<div class="diff-col">` columns:
          - Left: label "Baseline", `<img>` if `BaselineURI` non-empty, else `.empty-state`
          - Center: label "Diff Overlay (similarity: X%)", `<img>` if `OverlayURI` non-empty, else omit column or show `.empty-state`
          - Right: label "Current", `<img>` if `CurrentURI` non-empty, else `.empty-state`
      - `<h3>Audit Delta</h3>` + `<table>` with columns: Category | Before | After | Delta (badge)
      - `<h3>Element Changes</h3>` + `<table>` with columns: Selector | Type | Details; show "No element changes" if empty
- **Sub-task 2.1.5** — Implement `GenerateDiff(w io.Writer, data *DiffData) error`:
  1. Parse `diffReportTmpl` with `html/template.New("diff-report").Funcs(funcMap).Parse(...)`
  2. Execute template with `data` into `w`
  3. Return any error
- **Sub-task 2.1.6** — Implement `WriteDiffFile(path string, data *DiffData) (string, error)`:
  1. `filepath.Clean(path)` on input
  2. `os.MkdirAll(filepath.Dir(path), 0755)`
  3. `os.Create(path)` → defer close
  4. Call `GenerateDiff(f, data)`
  5. Return `filepath.Abs(path)`

### Task 2.2 — Create `report/diff_report_test.go`

**File:** `/workspace/report/diff_report_test.go`

- **Sub-task 2.2.1** — `TestGenerateDiff_smoke`: build minimal `DiffData` with one `DiffPage` and one `DiffBreakpoint` (all URIs non-empty, DiffScore 0.98). Call `GenerateDiff` into a `bytes.Buffer`. Assert no error and output contains `"Kaleidoscope Diff Report"`, `"Baseline"`, `"Current"`, `"Diff Overlay"`.
- **Sub-task 2.2.2** — `TestGenerateDiff_missingImages`: set all three URIs to `""`. Assert no error and output contains `"No screenshot at this breakpoint"` or the empty-state placeholder text. No template panics.
- **Sub-task 2.2.3** — `TestGenerateDiff_deltaColors`: set `CategoryDelta{Before:1, After:3, Delta:2}`. Assert output contains `"delta-positive"`. Set `Delta:-1` → contains `"delta-negative"`. Set `Delta:0` → contains `"delta-zero"`.

---

## Phase 3: Command Handler (`cmd/diff_report.go`)

### Task 3.1 — Create `cmd/diff_report.go`

**File:** `/workspace/cmd/diff_report.go`

- **Sub-task 3.1.1** — Declare package `cmd`; imports: `diff` (pixeldiff), `diff` (compare), `snapshot`, `report`, `output`, `browser`, `encoding/base64`, `fmt`, `image/png`, `os`, `bytes`, `path/filepath`, `strings`
- **Sub-task 3.1.2** — Implement argument parsing at top of `RunDiffReport(args []string)`:
  - `snapshotID = getArg(args)` (first non-flag arg)
  - `outputPath = getFlagValue(args, "--output")`
  - If `outputPath == ""`: resolve default as `browser.StateDir() + "/diff-report.html"` (using existing `browser.StateDir()`)
- **Sub-task 3.1.3** — Load baseline:
  ```go
  baseline, err := snapshot.LoadBaseline()
  if err != nil {
      output.Fail("diff-report", err, "no baseline set; run: ks snapshot baseline <id>")
      return
  }
  ```
- **Sub-task 3.1.4** — Load current snapshot:
  ```go
  var current *snapshot.Snapshot
  if snapshotID != "" {
      current, err = snapshot.Load(snapshotID)
      if err != nil {
          output.Fail("diff-report", err, "snapshot not found: "+snapshotID)
          return
      }
  } else {
      current, err = snapshot.LoadLatest()
      if err != nil {
          output.Fail("diff-report", err, "no snapshots found; run: ks snapshot")
          return
      }
  }
  ```
- **Sub-task 3.1.5** — Guard: `if baseline.ID == current.ID` → `output.Fail("diff-report", ..., "no diff: baseline and current are the same snapshot")`
- **Sub-task 3.1.6** — Compute diff: `diffResult, err := diff.Compare(baseline, current)` → fail on error
- **Sub-task 3.1.7** — Build `report.DiffData`:
  - Set `GeneratedAt`, `BaselineID`, `CurrentID`, `BaselineAt`, `CurrentAt`
  - Helper `findPage(pages []snapshot.PageSnapshot, url string) *snapshot.PageSnapshot`
  - Helper `loadImageB64(path string) (template.URL, image.Image, error)` — reads PNG file, returns base64 data URI and decoded image
  - For each `pageDiff` in `diffResult.Pages`:
    - Find `baselinePage` and `currentPage` by URL
    - For each breakpoint name in `[]string{"mobile","tablet","desktop","wide"}`:
      - Find matching `BreakpointShot` in each page (nil if absent)
      - Load baseline image → `baseURI, img1`
      - Load current image → `curURI, img2`
      - If both images available: `overlay = pixeldiff.Overlay(img1, img2, nil)`; encode overlay to PNG bytes → base64 data URI; `score = pixeldiff.Score(img1, img2, 10)`
      - Else: `overlayURI = ""`, `score = 0`
      - Append `report.DiffBreakpoint`
    - Map `pageDiff.AuditDelta` → `report.AuditDelta` (field-by-field copy)
    - Map `pageDiff.ElementChanges` → `[]report.ElementChangeRow`:
      - `Selector = ec.Selector`
      - `ChangeType = ec.ChangeType`
      - `Details = formatDetails(ec.Details)` (helper: stringify map as `key=value` pairs joined by `, `)
    - Append `report.DiffPage`
- **Sub-task 3.1.8** — Clean output path: `outputPath = filepath.Clean(outputPath)`
- **Sub-task 3.1.9** — Write report: `absPath, err := report.WriteDiffFile(outputPath, &diffData)` → fail on error
- **Sub-task 3.1.10** — Output success:
  ```go
  output.Success("diff-report", map[string]any{
      "path":       absPath,
      "baselineId": baseline.ID,
      "currentId":  current.ID,
      "pages":      len(diffData.Pages),
  })
  ```
- **Sub-task 3.1.11** — Implement private helpers in same file:
  - `findPage(pages []snapshot.PageSnapshot, url string) *snapshot.PageSnapshot`
  - `loadImageB64(path string) (template.URL, image.Image, error)`: reads file, decodes PNG with `image/png.Decode`, re-encodes raw bytes to base64 data URI `"data:image/png;base64,..."`, returns both URI and decoded image
  - `encodeOverlayB64(img image.Image) (template.URL, error)`: encodes image to PNG bytes buffer, returns base64 data URI
  - `formatDetails(m map[string]any) string`: formats map as readable string

---

## Phase 4: Wire Into `main.go` and `cmd/usage.go`

### Task 4.1 — Update `main.go`

**File:** `/workspace/main.go`

- **Sub-task 4.1.1** — Add case to the command switch:
  ```go
  case "diff-report":
      cmd.RunDiffReport(cmdArgs)
  ```
  Place after the existing `"report"` case in the UX Evaluation section.
- **Sub-task 4.1.2** — Add to the usage/help string under UX Evaluation section:
  ```
    diff-report [snapshot-id]  Side-by-side HTML diff vs baseline
  ```

### Task 4.2 — Update `cmd/usage.go`

**File:** `/workspace/cmd/usage.go`

- **Sub-task 4.2.1** — Add case to the `CommandUsage` map or `GetUsage` switch:
  ```go
  case "diff-report":
      return `Usage: ks diff-report [snapshot-id] [--output path]

  Generate a self-contained HTML report comparing baseline vs current screenshot
  with pixel diff overlays, audit delta tables, and element change lists.

  Options:
    [snapshot-id]      Snapshot to compare (default: latest)
    --output <path>    Output file path (default: .kaleidoscope/diff-report.html)
  `
  ```

---

## Phase 5: `.gitignore` Update

### Task 5.1 — Add diff-report to `.gitignore`

**File:** `/workspace/.gitignore` (create or append)

- **Sub-task 5.1.1** — Add `.kaleidoscope/diff-report.html` to `.gitignore`
- **Sub-task 5.1.2** — Verify `.kaleidoscope/snapshots/` and `.kaleidoscope/baselines.json` rules match the PRD (snapshots gitignored, baselines committed)

---

## Phase 6: Quality Gate

### Task 6.1 — Run and fix tests

- **Sub-task 6.1.1** — Run `go build ./...` to check compilation errors
- **Sub-task 6.1.2** — Run `go test ./...` and fix any failures
- **Sub-task 6.1.3** — Check `diff/pixel_test.go` all 4 tests pass
- **Sub-task 6.1.4** — Check `report/diff_report_test.go` all 3 tests pass

---

## File Creation Summary

| File | Status | Description |
|------|--------|-------------|
| `diff/pixel.go` | NEW | `Overlay()`, `Score()`, `Options` |
| `diff/pixel_test.go` | NEW | 4 unit tests for pixel diff |
| `report/diff_report.go` | NEW | `DiffData`, `GenerateDiff()`, `WriteDiffFile()`, HTML template |
| `report/diff_report_test.go` | NEW | 3 unit tests for diff report |
| `cmd/diff_report.go` | NEW | `RunDiffReport()` command handler |
| `main.go` | MODIFIED | Add `"diff-report"` case + help text |
| `cmd/usage.go` | MODIFIED | Add `"diff-report"` usage text |
| `.gitignore` | MODIFIED | Add `diff-report.html` |

---

## Key Constraints and Notes

1. **No external dependencies**: pixel diff uses only Go stdlib `image/*` packages.
2. **No import cycles**: `report/diff_report.go` duplicates `AuditDelta`/`CategoryDelta` types rather than importing `diff` package.
3. **Graceful degradation**: missing screenshots produce placeholder cards, not crashes.
4. **Security**: `filepath.Clean` on `--output`; all user strings go through `html/template` auto-escaping; no `template.HTML()` casts for user data.
5. **Pattern consistency**: command follows existing `Run<Command>(args []string)` pattern with `getArg`/`getFlagValue`/`output.Success`/`output.Fail`.
6. **Default output path**: resolved via `browser.StateDir()` (already handles local vs global `.kaleidoscope/` selection).
7. **Overlay encoding**: `encodeOverlayB64` must encode the `image.Image` returned by `pixeldiff.Overlay` to PNG bytes before base64-encoding.
