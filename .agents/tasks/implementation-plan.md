# Implementation Plan: US-006 — Side-by-Side HTML Diff Report

**Command:** `ks diff-report [snapshot-id] [--output path]`
**Dependencies:** US-003 (snapshot package), US-004 (diff package)
**New files:** `cmd/diff_report.go`, `report/diff_report.go`
**Modified files:** `main.go`, `cmd/usage.go`

---

## Phase 1: Understand Dependency Interfaces

> Establish the exact API surface of the snapshot and diff packages (US-003, US-004) before writing any code, so that integration code is written correctly the first time.

### Task 1.1 — Verify snapshot package API
- Sub-task 1.1.1: Read `snapshot/` package files to confirm field names for `Snapshot`, `URLSnapshot`, `BreakpointSnapshot`, `AuditSummary`, `AXNode`.
- Sub-task 1.1.2: Confirm `snapshot.Load(stateDir, id string) (*Snapshot, error)` signature and behavior for `"latest"` resolution.
- Sub-task 1.1.3: Confirm `snapshot.LoadBaseline(stateDir string) (map[string]string, error)` return type (url → snapshot-id).
- Sub-task 1.1.4: Note any divergence from the tech spec assumptions (Section 2) and record what adjustments are needed in `cmd/diff_report.go`.

### Task 1.2 — Verify diff package API
- Sub-task 1.2.1: Read `diff/` package files to confirm field names for `SnapshotDiff`, `URLDiff`, `BreakpointDiff`, `AuditDelta`, `ElementChange`.
- Sub-task 1.2.2: Confirm `diff.Compare(baseline, current *snapshot.Snapshot) *SnapshotDiff` signature.
- Sub-task 1.2.3: Confirm whether `BreakpointDiff.DiffImagePath` is populated when images are identical (empty string) or always set.
- Sub-task 1.2.4: Note any divergence from the tech spec assumptions (Section 3) and record adjustments needed.

### Task 1.3 — Verify existing report package helpers
- Sub-task 1.3.1: Read `report/report.go` to confirm the signature of `LoadScreenshot(path string) (template.URL, error)` (or equivalent).
- Sub-task 1.3.2: Confirm that `report/report.go` uses `html/template` (not `text/template`) so auto-escaping is in place.
- Sub-task 1.3.3: Identify the CSS variable names (`--border`, `--surface`, `--text-muted`, `--bg`, badge classes `badge-pass`, `badge-fail`, `badge-warn`) from the existing HTML template for consistency.
- Sub-task 1.3.4: Note whether `LoadScreenshot` returns an error on missing file (needed for error handling design).

### Task 1.4 — Verify main.go routing pattern
- Sub-task 1.4.1: Read `main.go` switch block to confirm the exact pattern for adding a new case (`case "diff-report": cmd.RunDiffReport(cmdArgs)`).
- Sub-task 1.4.2: Confirm where the usage string is embedded and how `diff-report` should be added to it.
- Sub-task 1.4.3: Confirm `cmdArgs` variable name used when delegating to cmd functions.

### Task 1.5 — Verify browser.StateDir helper
- Sub-task 1.5.1: Read `browser/state.go` to confirm `browser.StateDir()` exists and returns the `.kaleidoscope/` directory path (used for default output path and loading snapshots/baselines).

---

## Phase 2: Create `report/diff_report.go`

> Build the data types, builder function, renderer, and self-contained HTML template. This is the largest file and has no runtime dependency on Chrome — it can be compiled and unit-tested independently.

### Task 2.1 — Define types
- Sub-task 2.1.1: Add package declaration `package report` and required imports (`html/template`, `io`, `os`, `path/filepath`, `time`, `fmt`, and the `diff` package).
- Sub-task 2.1.2: Define `DiffScreenshotSet` struct with fields: `Breakpoint string`, `Width int`, `Height int`, `BaselineURI template.URL`, `CurrentURI template.URL`, `DiffURI template.URL`, `DiffPercent float64`.
- Sub-task 2.1.3: Define `AuditDeltaRow` struct with fields: `Category string`, `Before int`, `After int`, `Delta int`.
- Sub-task 2.1.4: Define `ElementChangeRow` struct with fields: `Selector string`, `Role string`, `Name string`, `Type string`, `Details string`.
- Sub-task 2.1.5: Define `URLDiffSection` struct with fields: `URL string`, `ScreenshotSets []DiffScreenshotSet`, `AuditDeltas []AuditDeltaRow`, `ElementChanges []ElementChangeRow`, `HasRegressions bool`.
- Sub-task 2.1.6: Define `DiffData` struct with fields: `BaselineID string`, `CurrentID string`, `GeneratedAt time.Time`, `URLs []URLDiffSection`, `TotalRegressions int`.

### Task 2.2 — Implement helper functions
- Sub-task 2.2.1: Implement `buildAuditDeltaRows(d diff.AuditDelta) []AuditDeltaRow` — returns exactly 4 rows (Contrast, Touch Targets, Typography, Spacing) each with Before, After, Delta (After-Before) computed from the `AuditDelta` fields.
- Sub-task 2.2.2: Implement `buildElementChangeRows(changes []diff.ElementChange) []ElementChangeRow` — maps each `diff.ElementChange` to `ElementChangeRow` (direct field mapping).
- Sub-task 2.2.3: Implement `anyPositiveDelta(rows []AuditDeltaRow) bool` — returns true if any row's `Delta > 0`.

### Task 2.3 — Implement `BuildDiffData`
- Sub-task 2.3.1: Start function signature: `func BuildDiffData(d *diff.SnapshotDiff) (*DiffData, error)`.
- Sub-task 2.3.2: Initialize `DiffData` with `BaselineID`, `CurrentID`, `GeneratedAt: time.Now()`.
- Sub-task 2.3.3: Loop over `d.URLs`; for each `URLDiff`, create a `URLDiffSection{URL: u.URL}`.
- Sub-task 2.3.4: Inside the URL loop, iterate `u.Breakpoints`; for each `BreakpointDiff`:
  - Call `LoadScreenshot(b.BaselineImagePath)` → `baseURI`.
  - Call `LoadScreenshot(b.CurrentImagePath)` → `currURI`.
  - If `b.DiffImagePath != ""`, call `LoadScreenshot(b.DiffImagePath)` → `diffURI`; otherwise set `diffURI = ""`.
  - Propagate any `LoadScreenshot` error immediately with a descriptive message including the file path.
  - Append `DiffScreenshotSet{Breakpoint: b.Name, Width: b.Width, Height: b.Height, BaselineURI: baseURI, CurrentURI: currURI, DiffURI: diffURI, DiffPercent: b.DiffPercent}`.
- Sub-task 2.3.5: Call `buildAuditDeltaRows(u.AuditDelta)` and assign to `section.AuditDeltas`.
- Sub-task 2.3.6: Call `buildElementChangeRows(u.ElementChanges)` and assign to `section.ElementChanges`.
- Sub-task 2.3.7: Call `anyPositiveDelta(section.AuditDeltas)` and assign to `section.HasRegressions`.
- Sub-task 2.3.8: Append the section to `data.URLs`.
- Sub-task 2.3.9: After the URL loop, compute `data.TotalRegressions` by counting sections where `HasRegressions == true`.
- Sub-task 2.3.10: Return `&data, nil`.

### Task 2.4 — Implement `GenerateDiff`
- Sub-task 2.4.1: Parse the `diffHtmlTemplate` constant with `html/template` and add a `formatTime` template function (format: `"2006-01-02 15:04:05 UTC"`).
- Sub-task 2.4.2: Execute the parsed template against `data`, writing to `w io.Writer`.
- Sub-task 2.4.3: Return any template execution error.

### Task 2.5 — Implement `WriteDiffFile`
- Sub-task 2.5.1: Signature: `func WriteDiffFile(path string, data *DiffData) (string, error)`.
- Sub-task 2.5.2: Call `filepath.Clean(path)` on the given path.
- Sub-task 2.5.3: Call `os.MkdirAll(filepath.Dir(cleanPath), 0755)` to ensure the parent directory exists; return error on failure.
- Sub-task 2.5.4: Create the file with `os.Create(cleanPath)`; return error on failure.
- Sub-task 2.5.5: Defer `f.Close()`.
- Sub-task 2.5.6: Call `GenerateDiff(f, data)`; return error on failure.
- Sub-task 2.5.7: Resolve absolute path with `filepath.Abs(cleanPath)` and return it.

### Task 2.6 — Write HTML template constant `diffHtmlTemplate`
- Sub-task 2.6.1: Add `<!DOCTYPE html>` shell with `<head>` containing `<title>Kaleidoscope Diff Report — baseline {{.BaselineID}} → {{.CurrentID}}</title>`.
- Sub-task 2.6.2: Add `<style>` block — copy all existing CSS variables from `report/report.go`'s `htmlTemplate` to ensure visual consistency (dark theme, typography, badge classes).
- Sub-task 2.6.3: Add diff-specific CSS rules: `.url-section`, `.diff-grid` (3-column CSS grid), `.diff-col`, `.diff-col img`, `.diff-col-label`, `.diff-identical` per the tech spec (Section 5 CSS Additions).
- Sub-task 2.6.4: Add `<body>` with `<h1>Kaleidoscope Diff Report</h1>` and meta block showing `{{.BaselineID}}` → `{{.CurrentID}}`, generated time via `{{.GeneratedAt | formatTime}}`, and conditional regression badge (`badge-fail` if `{{.TotalRegressions}}`, `badge-pass` otherwise).
- Sub-task 2.6.5: Add `{{range .URLs}}` loop rendering each `URLDiffSection`:
  - `<h2>` with URL and optional `badge-fail` regression badge.
  - `{{range .ScreenshotSets}}` loop: `<h3>` with breakpoint/dimensions, then `.diff-grid` div containing three `.diff-col` divs: Baseline (left), Diff overlay with percent badge (center), Current (right). Center column shows `<img>` if `DiffURI` non-empty, else `.diff-identical` div.
  - `<h3>Audit Delta</h3>` followed by table with columns `Category | Before | After | Delta`. Delta cell uses `badge-fail` for `+N regression`, `badge-pass` for `-N resolved`, plain `—` for zero.
  - `{{if .ElementChanges}}` block with `<h3>Element Changes</h3>` and table with columns `Selector | Role | Name | Change | Details`. Change column uses `badge-pass` for `appeared`, `badge-fail` for `disappeared`, `badge-warn` for `moved` or `resized`.
- Sub-task 2.6.6: Add `<footer>` with "Generated by kaleidoscope · AI agent front-end design toolkit".
- Sub-task 2.6.7: Verify all user-controlled data (URL strings, snapshot IDs, element names/selectors) is rendered through `html/template` text nodes (not raw `template.HTML`) to ensure auto-escaping.

---

## Phase 3: Create `cmd/diff_report.go`

> Implement the CLI handler that orchestrates loading, diffing, and rendering. This file depends on Phase 2 (`report` package) and the snapshot/diff packages verified in Phase 1.

### Task 3.1 — File skeleton
- Sub-task 3.1.1: Create `cmd/diff_report.go` with `package cmd` declaration and imports: `os`, `path/filepath`, `regexp`, `fmt`, and packages `browser`, `snapshot`, `diff`, `output`, `report`.

### Task 3.2 — Implement `RunDiffReport(args []string)`
- Sub-task 3.2.1: Parse `snapshotID` — call `getArg(args)` (existing util); default to `"latest"` if empty.
- Sub-task 3.2.2: Parse `outputPath` — call `getFlagValue(args, "--output")` (existing util); leave empty for now (default resolved later).
- Sub-task 3.2.3: Validate `snapshotID` against `regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)` — if invalid, call `output.Fail("diff-report", "invalid snapshot ID", "Snapshot IDs may only contain alphanumeric characters, underscores, and hyphens")` and `os.Exit(2)`.
- Sub-task 3.2.4: Resolve state directory via `browser.StateDir()`.
- Sub-task 3.2.5: Load current snapshot:
  ```go
  current, err := snapshot.Load(stateDir, snapshotID)
  ```
  On error, call `output.Fail("diff-report", err.Error(), "Run 'ks snapshot' first")` and `os.Exit(2)`.
- Sub-task 3.2.6: Load baseline map:
  ```go
  baselines, err := snapshot.LoadBaseline(stateDir)
  ```
  On error, call `output.Fail("diff-report", err.Error(), "Run 'ks baseline set' first")` and `os.Exit(2)`.
- Sub-task 3.2.7: Resolve baseline snapshot ID — check if `baselines` map has a single entry (use its value as `baselineID`) or entries keyed by URL matching URLs in `current`. If no URL in `current` has a baseline entry, call `output.Fail("diff-report", "no baseline found for any URL in current snapshot", "Run 'ks baseline set' first")` and `os.Exit(2)`.
- Sub-task 3.2.8: Load baseline snapshot:
  ```go
  baselineSnap, err := snapshot.Load(stateDir, baselineID)
  ```
  On error, call `output.Fail("diff-report", fmt.Sprintf("snapshot '%s' not found: %v", baselineID, err), "Run 'ks snapshot list' to see available snapshots")` and `os.Exit(2)`.
- Sub-task 3.2.9: Compute diff:
  ```go
  snapshotDiff := diff.Compare(baselineSnap, current)
  ```
- Sub-task 3.2.10: Build report data:
  ```go
  data, err := report.BuildDiffData(snapshotDiff)
  ```
  On error, call `output.Fail("diff-report", err.Error(), "")` and `os.Exit(2)`.
- Sub-task 3.2.11: Resolve output path — if `outputPath == ""`, set `outputPath = filepath.Join(stateDir, "diff-report.html")`.
- Sub-task 3.2.12: Write report:
  ```go
  absPath, err := report.WriteDiffFile(outputPath, data)
  ```
  On error, call `output.Fail("diff-report", err.Error(), "")` and `os.Exit(2)`.
- Sub-task 3.2.13: Call `output.Success`:
  ```go
  output.Success("diff-report", map[string]any{
      "path":       absPath,
      "baselineId": snapshotDiff.BaselineID,
      "currentId":  snapshotDiff.CurrentID,
      "urlCount":   len(snapshotDiff.URLs),
  })
  ```

---

## Phase 4: Update Existing Files

> Wire the new command into the CLI router and document it in the usage registry. These are small, targeted edits.

### Task 4.1 — Update `main.go`
- Sub-task 4.1.1: Add a new case to the `switch` block:
  ```go
  case "diff-report":
      cmd.RunDiffReport(cmdArgs)
  ```
  Place it in the "UX Evaluation" section alongside `report`, `audit`, `contrast`, `spacing`.
- Sub-task 4.1.2: Add `diff-report [snap-id]   Side-by-side HTML diff report vs baseline` to the usage string under the "UX Evaluation:" section.

### Task 4.2 — Update `cmd/usage.go`
- Sub-task 4.2.1: Add a new key `"diff-report"` to the `CommandUsage` map with the full usage string from the tech spec (Section 6), including argument description, `--output` flag documentation, and three examples.

---

## Phase 5: Testing and Validation

> Verify the implementation compiles and the acceptance criteria pass.

### Task 5.1 — Compilation check
- Sub-task 5.1.1: Run `go build ./...` and fix any type mismatches between `report.DiffData` fields and what `BuildDiffData` populates.
- Sub-task 5.1.2: Fix any import cycle issues (e.g., if `diff` package imports `report` or vice versa).
- Sub-task 5.1.3: Ensure `html/template` is used (not `text/template`) in `report/diff_report.go`.

### Task 5.2 — Unit tests for `report/diff_report.go`
- Sub-task 5.2.1: Write `report/diff_report_test.go` with a test for `buildAuditDeltaRows` — verify exactly 4 rows returned with correct Delta computation (positive, negative, zero cases).
- Sub-task 5.2.2: Write a test for `anyPositiveDelta` — verify returns true when any Delta > 0, false when all zero or negative.
- Sub-task 5.2.3: Write a test for `GenerateDiff` with a minimal `DiffData` (no screenshot files needed — use empty `template.URL`) — verify the HTML output contains `Kaleidoscope Diff Report`, the baseline and current IDs, and does not panic.
- Sub-task 5.2.4: Write a test for `BuildDiffData` with a `SnapshotDiff` whose `BreakpointDiff` entries use real temporary PNG files — verify base64 data URIs are embedded and `TotalRegressions` is counted correctly.

### Task 5.3 — Unit tests for `cmd/diff_report.go`
- Sub-task 5.3.1: Write a test for snapshot ID validation — confirm that IDs with path traversal characters (e.g., `../etc/passwd`) are rejected before any file I/O.
- Sub-task 5.3.2: Write a test for the default output path logic — confirm `outputPath` defaults to `<stateDir>/diff-report.html` when `--output` is not supplied.

### Task 5.4 — Quality gate
- Sub-task 5.4.1: Run `go test ./...` and confirm all tests pass.
- Sub-task 5.4.2: Verify `ks diff-report --help` outputs the usage string (or equivalent help display).
- Sub-task 5.4.3: Verify that running `ks diff-report` with no snapshots exits with code 2 and a JSON error containing the hint `"Run 'ks snapshot' first"`.
- Sub-task 5.4.4: Verify that running `ks diff-report` with no baseline exits with code 2 and a JSON error containing the hint `"Run 'ks baseline set' first"`.

---

## Phase 6: Acceptance Criteria Verification

> Cross-check each acceptance criterion from US-006 against the implementation.

| AC | Implementation Verified By |
|---|---|
| `ks diff-report` generates a self-contained HTML file | `WriteDiffFile` writes file; `BuildDiffData` base64-encodes all images |
| Side-by-side layout (baseline left / diff center / current right) per URL per breakpoint | `.diff-grid` 3-column CSS grid in `diffHtmlTemplate` |
| Audit delta tables below each URL section | `AuditDeltaRow` slice rendered as `<table>` per `URLDiffSection` |
| Element change lists with selector, type, details | `ElementChangeRow` slice rendered as `<table>` when non-empty |
| `--output` controls path; defaults to `.kaleidoscope/diff-report.html` | Parsed in `RunDiffReport`; fallback to `filepath.Join(stateDir, "diff-report.html")` |
| Screenshots are base64-embedded | `BuildDiffData` calls `report.LoadScreenshot` for each image path |
| Graceful failure with clear error if no baseline or snapshots | Explicit `output.Fail` calls with actionable hints for each error condition |

---

## File Summary

| File | Action | Phase |
|---|---|---|
| `report/diff_report.go` | **Create** | 2 |
| `cmd/diff_report.go` | **Create** | 3 |
| `main.go` | **Modify** (add case + usage string) | 4.1 |
| `cmd/usage.go` | **Modify** (add `"diff-report"` entry) | 4.2 |
| `report/diff_report_test.go` | **Create** | 5.2 |
