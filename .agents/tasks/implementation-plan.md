# Implementation Plan: US-006 — Side-by-Side HTML Diff Report

## Overview

`ks diff-report [snapshot-id] [--output path]` generates a self-contained HTML file
comparing a baseline snapshot against the latest (or specified) snapshot. No browser
required. Pure Go.

**Dependency chain:** snapshot package → diff package → report/diff package → cmd/diff_report → main.go

---

## Phase 1: Snapshot Data Model and Storage

### Task 1.1: Create `snapshot/snapshot.go` — Core Types

**Sub-tasks:**
- Define `Snapshot` struct with fields: `ID string`, `CreatedAt time.Time`, `CommitSHA string`, `CommitMsg string`, `ProjectFile string`, `Pages []PageSnapshot`
- Define `PageSnapshot` struct with fields: `URL string`, `Title string`, `Breakpoints []BreakpointCapture`, `AuditResult AuditResult`, `AXNodes []AXNodeRecord`
- Define `BreakpointCapture` struct with fields: `Name string`, `Width int`, `Height int`, `ScreenshotFile string`
- Define `AuditResult` struct with count fields (`ContrastViolations`, `TouchViolations`, `TypographyWarnings`, `SpacingIssues int`) and issue-list fields (`ContrastIssues []ContrastRecord`, `TouchIssues []TouchRecord`, `TypographyIssues []TypoRecord`, `SpacingIssues_ []SpacingRecord`)
- Define `AXNodeRecord` struct with fields: `Role string`, `Name string`, `Selector string`, `Bounds Rect`, `Ignored bool`
- Define `Rect` struct with fields: `X, Y, Width, Height float64`
- Define concrete sub-types `ContrastRecord`, `TouchRecord`, `TypoRecord`, `SpacingRecord` (mirroring `report` package types but in `snapshot` package, no cross-dependency)
- Add `package snapshot` declaration and required imports (`time`, `encoding/json`)

### Task 1.2: Create `snapshot/store.go` — File I/O and Index Helpers

**Sub-tasks:**
- Implement `Load(dir, id string) (*Snapshot, error)`:
  - Validate `id` is safe (use `filepath.Clean` + `strings.HasPrefix` check against snapshots dir to prevent path traversal)
  - Read `<dir>/snapshots/<id>/snapshot.json`
  - Unmarshal JSON into `*Snapshot`
  - Return descriptive error if file not found
- Implement `Latest(dir string) (*Snapshot, error)`:
  - List all entries under `<dir>/snapshots/`
  - For each entry, attempt to read `snapshot.json` and extract `CreatedAt`
  - Sort by `CreatedAt` descending
  - Return first valid snapshot; error if none found
- Implement `LoadBaseline(dir string) (*Snapshot, error)`:
  - Read `<dir>/baselines.json`
  - Unmarshal into `map[string]string`
  - Extract `"default"` key; return error with hint `"Run: ks baseline set"` if missing
  - Call `Load(dir, id)`
- Implement `ScreenshotPath(dir, snapshotID, filename string) string`:
  - Return `filepath.Join(dir, "snapshots", snapshotID, filename)`

---

## Phase 2: Diff Engine

### Task 2.1: Create `diff/diff.go` — Audit and Element Diff

**Sub-tasks:**
- Define `SnapshotDiff` struct: `BaselineID string`, `CurrentID string`, `Pages []PageDiff`
- Define `PageDiff` struct: `URL string`, `AuditDelta []CategoryDelta`, `ElementChanges []ElementChange`, `BreakpointDiffs []BreakpointDiff`
- Define `CategoryDelta` struct: `Category string`, `Before int`, `After int`, `Delta int`
- Define `ElementChange` struct: `Role string`, `Name string`, `Selector string`, `Type string`, `Details string`
- Define `BreakpointDiff` struct: `Name string`, `BaselinePath string`, `CurrentPath string`, `DiffScore float64`, `DiffImageBytes []byte` (json:"-")
- Implement `Compare(baseline, current *snapshot.Snapshot) (*SnapshotDiff, error)`:
  - Build URL-keyed map of baseline pages
  - Iterate current pages; match by URL string equality
  - For each matched pair call `compareAudit` and `compareElements`
  - For each matched breakpoint pair, populate `BreakpointDiff` with absolute paths via `snapshot.ScreenshotPath`
- Implement `compareAudit(base, cur snapshot.AuditResult) []CategoryDelta` (pure arithmetic):
  - Return four `CategoryDelta` rows for categories: `"contrast"`, `"touch"`, `"typography"`, `"spacing"`
  - `Delta = After - Before`
- Implement `compareElements(base, cur []snapshot.AXNodeRecord) []ElementChange`:
  - Build map keyed by `(Role, Name)` tuple for baseline nodes
  - Walk current nodes; nodes absent from baseline → `"appeared"`
  - Remaining unmatched baseline nodes → `"disappeared"`
  - Matched nodes with changed `Bounds` (threshold > 2px):
    - Width or Height changed → `"resized"`, details: `"WxH → WxH"`
    - Only X/Y changed → `"moved"`, details: `"Npx right/left/up/down"`
- Implement helper `countRegressions(d *SnapshotDiff) int`:
  - Sum all `CategoryDelta.Delta > 0` across all pages
- Add `package diff` declaration and imports (`snapshot`, `fmt`, `math`)

### Task 2.2: Create `diff/pixel.go` — Pure-Go Pixel Diff

**Sub-tasks:**
- Implement `PixelDiff(baselinePath, currentPath string) (score float64, overlayPNG []byte, err error)`:
  - Open and decode both PNG files using `image/png`
  - If either file is missing/unreadable, return error
  - Resize smaller image to match larger dimensions using nearest-neighbour algorithm (in-place, pure Go)
  - Walk every pixel: compute Euclidean distance in RGB space: `sqrt((dr²+dg²+db²))`
  - Count pixels where distance > threshold (default `10.0/255.0` in normalized space, i.e. threshold `10` on 0–255 scale)
  - Compute `score = float64(differentPixels) / float64(totalPixels)`
  - Build overlay image:
    - Create new `image.RGBA` sized to current image dimensions
    - Copy current image pixels as base
    - For different pixels: set to semi-transparent red `color.RGBA{R:255, G:0, B:0, A:128}`
  - Encode overlay to PNG bytes using `image/png`
  - Return score, PNG bytes, nil
- Add `package diff` declaration and imports (`image`, `image/color`, `image/png`, `bytes`, `math`, `os`)

---

## Phase 3: Diff Report Package

### Task 3.1: Create `report/diff_report.go` — View Model and Rendering

**Sub-tasks:**
- Define `DiffData` struct:
  - `GeneratedAt time.Time`
  - `BaselineID string`, `CurrentID string`
  - `BaselineCommit string`, `CurrentCommit string`
  - `Pages []DiffPageData`
- Define `DiffPageData` struct:
  - `URL string`
  - `Breakpoints []DiffBreakpointData`
  - `AuditDelta []DiffCategoryRow`
  - `ElementChanges []DiffElementRow`
- Define `DiffBreakpointData` struct:
  - `Name string`, `Width int`, `Height int`
  - `BaselineDataURI template.URL`
  - `CurrentDataURI template.URL`
  - `DiffDataURI template.URL` (empty string if images identical)
  - `DiffScore float64`
- Define `DiffCategoryRow` struct:
  - `Category string`, `Before int`, `After int`, `Delta int`
  - `Trend string` — `"better"` | `"worse"` | `"same"`
- Define `DiffElementRow` struct:
  - `Role string`, `Name string`, `Selector string`
  - `Type string`, `Details string`
- Implement helper `loadAndEncode(path string) (template.URL, error)`:
  - Read file bytes, base64-encode, return `template.URL("data:image/png;base64," + encoded)`
  - Return empty string (not error) if file path is empty
- Implement `BuildDiffData(d *diff.SnapshotDiff, baseline, current *snapshot.Snapshot) (*DiffData, error)`:
  - Populate header fields from snapshot metadata
  - For each `PageDiff`:
    - Build `[]DiffBreakpointData`: for each `BreakpointDiff`, load baseline + current screenshots via `loadAndEncode`; base64-encode `DiffImageBytes` overlay if non-empty; compute `DiffScore`
    - Build `[]DiffCategoryRow`: map `CategoryDelta` → `DiffCategoryRow`; set `Trend` based on `Delta` sign
    - Build `[]DiffElementRow`: map `ElementChange` → `DiffElementRow`
  - Return `*DiffData`
- Implement `GenerateDiff(w io.Writer, data *DiffData) error`:
  - Parse `diffHTMLTemplate` constant using `html/template`
  - Execute template with `data`
- Implement `WriteDiffFile(outputPath, dir string, data *DiffData) (string, error)`:
  - If `outputPath` non-empty: use that path directly
  - Otherwise: default to `filepath.Join(dir, fmt.Sprintf("diff-report-%d.html", time.Now().UnixMilli()))`
  - Ensure parent directory exists via `os.MkdirAll`
  - Create file, call `GenerateDiff`, close; remove file on error
  - Return absolute path
- Add `package report` declaration and imports (`diff`, `snapshot`, `html/template`, `encoding/base64`, `io`, `os`, `path/filepath`, `time`, `fmt`)

### Task 3.2: Create `report/diff_template.go` — HTML Template

**Sub-tasks:**
- Declare `package report` and `var diffHTMLTemplate = \`...\``
- Implement report header section:
  - `<h1>Kaleidoscope Diff Report</h1>`
  - Meta block: baseline ID + commit, current ID + commit, generated timestamp
- Implement per-page section (`{{range .Pages}}`):
  - `<section class="page-section">` with `<h2>{{.URL}}</h2>`
  - Per-breakpoint triptych (`{{range .Breakpoints}}`):
    - `<div class="diff-breakpoint">` with `<h3>name · WxH</h3>`
    - `<div class="diff-triptych">` containing three `<figure>` elements:
      - Baseline (left): `<img src="{{.BaselineDataURI}}">` + `<figcaption>Baseline</figcaption>`
      - Diff overlay (center): show if `DiffDataURI` non-empty, else show "Identical"; `<figcaption>Diff ({{printf "%.1f" (mul .DiffScore 100)}}% changed)</figcaption>`
      - Current (right): `<img src="{{.CurrentDataURI}}">` + `<figcaption>Current</figcaption>`
    - Empty state if no screenshots: muted "No screenshots captured"
  - Audit delta table below breakpoints:
    - `<h3>Audit Delta</h3>`
    - Columns: Category, Before, After, Delta, Trend
    - Delta cell: red if positive, green if negative, muted if zero
    - Trend badge: `better` = green, `worse` = red, `same` = muted
    - Empty state: "No audit changes detected" if all deltas zero
  - Element changes table:
    - `<h3>Element Changes</h3>`
    - Columns: Role, Name, Type, Details
    - Type badge: `appeared` = green, `disappeared` = red, `moved`/`resized` = yellow
    - Empty state: "No element changes detected"
- Implement CSS (reusing existing CSS variables from `htmlTemplate`):
  - Copy `:root` CSS variables and base styles
  - Add `.diff-triptych` layout: CSS grid with three equal columns, gap
  - Add `.diff-pane` styles: `figure` with border, rounded corners, overflow hidden
  - Add `.diff-baseline`, `.diff-overlay`, `.diff-current` variant styles
  - Add `.audit-delta` and `.element-changes` table styles
  - Add `.trend-better` (green), `.trend-worse` (red), `.trend-same` (muted) badge styles
  - Add `.change-appeared` (green), `.change-disappeared` (red), `.change-moved`/`.change-resized` (yellow) badge styles
  - Add `.page-section` separator styling
  - Add `footer` styles matching existing report
- Add template `mul` function registration for percentage calculation (or use pre-computed field in `DiffBreakpointData`)
  - **Note:** `html/template` does not support arithmetic; add `DiffScorePct string` field (pre-formatted `"4.2%"`) to `DiffBreakpointData` instead

---

## Phase 4: CLI Command

### Task 4.1: Create `cmd/diff_report.go`

**Sub-tasks:**
- Implement `RunDiffReport(args []string)`:
  - Parse positional arg 0 as optional `snapshotID`
  - Parse `--output <path>` flag for optional output path
  - Resolve state directory via `browser.StateDir()`; on error call `output.Fail` and `os.Exit(2)`
  - Load baseline via `snapshot.LoadBaseline(dir)`:
    - On "baselines.json not found" error: `output.Fail("diff-report", err, "Run: ks baseline set")` + `os.Exit(2)`
  - Load current snapshot:
    - If `snapshotID` non-empty: `snapshot.Load(dir, snapshotID)` → on error: `output.Fail` with hint `"Run: ks snapshot list to see available IDs"` + `os.Exit(2)`
    - Else: `snapshot.Latest(dir)` → on error: `output.Fail` with hint `"Run: ks snapshot to capture one"` + `os.Exit(2)`
  - Run diff engine: `diff.Compare(baseline, current)`
  - Run pixel diff for each breakpoint:
    - Iterate `d.Pages[pi].BreakpointDiffs[bi]`
    - Call `diff.PixelDiff(bd.BaselinePath, bd.CurrentPath)`
    - On error: log warning to stderr, continue (non-fatal), leave `DiffImageBytes` nil
  - Build view model: `report.BuildDiffData(d, baseline, current)`
  - Write HTML: `report.WriteDiffFile(outputPath, dir, data)` → on error: `output.Fail` + `os.Exit(2)`
  - Emit success: `output.Success("diff-report", map[string]any{"path": reportPath, "baselineId": baseline.ID, "currentId": current.ID, "pages": len(d.Pages), "regressions": diff.CountRegressions(d)})`
- Add `package cmd` declaration and imports (`snapshot`, `diff`, `report`, `output`, `browser`, `fmt`, `os`)

### Task 4.2: Update `main.go` — Route `diff-report` Command

**Sub-tasks:**
- Add `case "diff-report":` to the `switch command` block before the `default:` case
- Call `cmd.RunDiffReport(cmdArgs)`
- Add `diff-report` entry to the usage string under "UX Evaluation" section

---

## Phase 5: Unit Tests

### Task 5.1: `snapshot` Package Tests (`snapshot/store_test.go`)

**Sub-tasks:**
- Test `Load` with a valid snapshot JSON on disk → returns correct `*Snapshot`
- Test `Load` with a path-traversal ID (`"../etc"`) → returns error (path safety check)
- Test `Latest` with two snapshots of different `CreatedAt` → returns newer one
- Test `LoadBaseline` with a `baselines.json` missing `"default"` key → returns error

### Task 5.2: `diff` Package Tests (`diff/diff_test.go`)

**Sub-tasks:**
- Test `Compare` with identical snapshots → all `CategoryDelta.Delta` zero, no element changes
- Test `Compare` where current has one additional contrast violation → one delta with `Delta = +1`, category `"contrast"`
- Test `compareElements` (exported or white-box via same package) with a node present only in current → one `"appeared"` change
- Test `compareElements` with a node present only in baseline → one `"disappeared"` change
- Test `compareElements` with a node whose `Bounds.Width` changed by >2px → one `"resized"` change
- Test `compareElements` with a node whose only `Bounds.X` changed by >2px → one `"moved"` change

### Task 5.3: `diff` Package Tests (`diff/pixel_test.go`)

**Sub-tasks:**
- Test `PixelDiff` with two identical in-memory PNGs (generated via `image/png` encode) → `score == 0.0`, overlay returned
- Test `PixelDiff` with a fully red PNG vs a fully blue PNG → `score ≈ 1.0`
- Test `PixelDiff` with a missing file path → returns error

### Task 5.4: `report` Package Tests (`report/diff_report_test.go`)

**Sub-tasks:**
- Test `GenerateDiff` with a zero-value `&DiffData{}` → executes without template error (no panic, no error returned)
- Test `BuildDiffData` with valid diff and snapshots → `DiffData.Pages` count matches `SnapshotDiff.Pages` count
- Test `DiffCategoryRow.Trend` field: `Delta > 0` → `"worse"`, `Delta < 0` → `"better"`, `Delta == 0` → `"same"`

### Task 5.5: Verify `go test ./...` Passes

**Sub-tasks:**
- Run `go test ./...` from `/workspace`
- Fix any compilation errors (import cycles, unused imports, type mismatches)
- Fix any test failures

---

## Phase 6: Integration Verification

### Task 6.1: Compile Check

**Sub-tasks:**
- Run `go build ./...` to verify all packages compile cleanly
- Confirm no import cycles between `snapshot`, `diff`, `report`, `cmd`

### Task 6.2: Error Path Verification

**Sub-tasks:**
- Verify `RunDiffReport` exits with code 2 and emits `{"ok":false,...}` JSON when no `baselines.json` exists
- Verify `RunDiffReport` exits with code 2 and emits `{"ok":false,...}` JSON when no snapshots directory exists
- Verify `RunDiffReport` exits with code 2 when a specified `snapshot-id` is not found

---

## File Summary

| File | Action | Notes |
|---|---|---|
| `snapshot/snapshot.go` | Create | All snapshot data types |
| `snapshot/store.go` | Create | Load, Latest, LoadBaseline, ScreenshotPath |
| `diff/diff.go` | Create | Compare, compareAudit, compareElements, CountRegressions |
| `diff/pixel.go` | Create | PixelDiff — pure image/png |
| `report/diff_report.go` | Create | DiffData types, BuildDiffData, GenerateDiff, WriteDiffFile |
| `report/diff_template.go` | Create | diffHTMLTemplate constant |
| `cmd/diff_report.go` | Create | RunDiffReport |
| `main.go` | Edit | Add `case "diff-report"` + usage string entry |
| `snapshot/store_test.go` | Create | snapshot package unit tests |
| `diff/diff_test.go` | Create | diff engine unit tests |
| `diff/pixel_test.go` | Create | pixel diff unit tests |
| `report/diff_report_test.go` | Create | report/diff unit tests |

## Dependency Order

```
snapshot/snapshot.go   (no internal deps)
snapshot/store.go      (depends on: snapshot/snapshot.go)
diff/diff.go           (depends on: snapshot/)
diff/pixel.go          (depends on: image/png only — no internal deps)
report/diff_report.go  (depends on: diff/, snapshot/)
report/diff_template.go (no internal deps)
cmd/diff_report.go     (depends on: snapshot/, diff/, report/, output/, browser/)
main.go                (depends on: cmd/)
```

## Key Constraints (from Tech Spec)

- All user-controlled strings go through `html/template` auto-escaping; only `DataURI` fields use `template.URL`
- `snapshot.Load` must validate ID against path traversal before opening files
- Pixel diff is pure Go (`image` + `image/png`); no ImageMagick or external binaries
- `diff.Compare` has no Chrome dependency — pure data transformation
- Missing pixel diff (e.g. screenshot file absent) is non-fatal; report continues with empty overlay
- Default output path: `.kaleidoscope/diff-report.html` (resolved relative to state dir)
