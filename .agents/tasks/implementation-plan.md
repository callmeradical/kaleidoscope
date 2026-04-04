# Implementation Plan: US-006 — Side-by-Side HTML Diff Report

## Overview

Introduces `ks diff-report [snapshot-id] [--output path]` — a new command that generates a self-contained HTML file comparing the latest snapshot against a baseline. Depends on US-003 (snapshot package) and US-004 (diff package) being present; this plan includes stub implementations of those interfaces to make US-006 buildable and testable independently.

---

## Phase 1: Define Shared Data Contracts (snapshot & diff packages)

These packages are owned by US-003 and US-004, but US-006 must consume them. This phase creates the package skeletons with the exact interfaces specified in the tech spec so the rest of the work compiles.

### Task 1.1 — Create `snapshot` package skeleton

**File:** `snapshot/store.go`

#### Sub-tasks
- 1.1.1 Create `snapshot/` directory and `snapshot/store.go`
- 1.1.2 Declare `package snapshot`
- 1.1.3 Define `Rect` struct: `X, Y, Width, Height float64`
- 1.1.4 Define `AXElement` struct with fields: `Role`, `Name`, `Selector` (omitempty), `Bounds *Rect` (omitempty)
- 1.1.5 Define `AuditSummary` struct: `ContrastViolations`, `TouchViolations`, `TypographyWarnings`, `SpacingIssues int` (JSON snake_case tags)
- 1.1.6 Define `BreakpointCapture` struct: `Name`, `Width int`, `Height int`, `ScreenshotPath string` (JSON tags)
- 1.1.7 Define `URLSnapshot` struct: `URL string`, `Breakpoints []BreakpointCapture`, `AuditResult AuditSummary`, `AXElements []AXElement` (JSON tags)
- 1.1.8 Define `Snapshot` struct: `ID string`, `CreatedAt time.Time`, `CommitSHA string` (omitempty), `URLs []URLSnapshot` (JSON tags)
- 1.1.9 Define `Store` interface: `Latest() (*Snapshot, error)` and `LoadByID(id string) (*Snapshot, error)`
- 1.1.10 Define `BaselineManager` interface: `ActiveBaselineID() (string, error)`

### Task 1.2 — Create `diff` package skeleton

**File:** `diff/engine.go`

#### Sub-tasks
- 1.2.1 Create `diff/` directory and `diff/engine.go`
- 1.2.2 Declare `package diff`; import `snapshot` package
- 1.2.3 Define `AuditDelta` struct with Before/After/Delta int fields for Contrast, Touch, Typography, Spacing
- 1.2.4 Define `ElementChange` struct: `Role`, `Name`, `Selector`, `Type` ("appeared"|"disappeared"|"moved"|"resized"), `Details string`
- 1.2.5 Define `PixelDiff` struct: `DiffPath string`, `DiffPercent float64`, `ChangedPixels int`, `TotalPixels int`
- 1.2.6 Define `URLDiff` struct: `URL string`, `AuditDelta AuditDelta`, `ElementChanges []ElementChange`, `PixelDiff *PixelDiff` (nil if unavailable)
- 1.2.7 Define `DiffResult` struct: `URLs []URLDiff`
- 1.2.8 Declare function signature `Compare(baseline, current *snapshot.Snapshot) (*DiffResult, error)` — implement as stub returning empty `DiffResult` (nil error) so code compiles

---

## Phase 2: Report Package — Data Model and HTML Generation

**File:** `report/diff_report.go`

### Task 2.1 — Define `DiffData` and related structs

#### Sub-tasks
- 2.1.1 Add `report/diff_report.go` to the existing `report` package
- 2.1.2 Define `ElementChangeRow` struct: `Role`, `Name`, `Selector`, `Type`, `Details string`
- 2.1.3 Define `AuditDeltaRow` struct: Before/After/Delta int for Contrast, Touch, Typography, Spacing (matches `diff.AuditDelta` shape)
- 2.1.4 Define `BreakpointDiffRow` struct:
  - `Name string`, `Width int`, `Height int`
  - `BaselineURI template.URL`, `CurrentURI template.URL`, `DiffOverlayURI template.URL`
  - `DiffPercent float64`, `HasDiff bool`
- 2.1.5 Define `URLDiffSection` struct: `URL string`, `Breakpoints []BreakpointDiffRow`, `AuditDelta AuditDeltaRow`, `ElementChanges []ElementChangeRow`
- 2.1.6 Define `DiffData` struct: `BaselineID string`, `CurrentID string`, `GeneratedAt time.Time`, `URLs []URLDiffSection`

### Task 2.2 — Implement `GenerateDiffReport`

#### Sub-tasks
- 2.2.1 Implement `GenerateDiffReport(w io.Writer, data *DiffData) error`
  - Parse `diffHTMLTemplate` with `html/template`
  - Execute template to `w`
  - Return error on parse or execute failure
- 2.2.2 Implement `WriteDiffFile(path string, data *DiffData) (string, error)`
  - Call `os.MkdirAll` on parent directory (`filepath.Dir(path)`)
  - Use `filepath.Clean(path)` to sanitize the path (security: path traversal)
  - Create the file, call `GenerateDiffReport`, remove file on error
  - Return `filepath.Abs(path)` on success

### Task 2.3 — Write `diffHTMLTemplate` HTML string constant

#### Sub-tasks
- 2.3.1 Write `<!DOCTYPE html>` shell with `<head>` and `<body>` structure
- 2.3.2 Add `<meta charset="utf-8">` and `<meta name="viewport">` tags
- 2.3.3 Add `<title>Diff Report — {{.BaselineID}} vs {{.CurrentID}}</title>`
- 2.3.4 Embed CSS block with existing design tokens (dark theme, color variables):
  - Copy CSS variables from `report/report.go` (`htmlTemplate`): `--bg`, `--surface`, `--border`, `--text`, `--text-muted`, `--accent`, `--green`, `--red`, `--yellow`, etc.
  - Add base layout styles (body, h1, h2, h3, table, badge etc.) matching existing report style
- 2.3.5 Add new CSS classes required for diff layout:
  - `.diff-row { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 0.75rem; margin: 1rem 0; }`
  - `.diff-col`, `.diff-col img`, `.diff-label` (with `var(--text-muted)`)
  - `.diff-col--overlay .diff-label { color: var(--yellow); }`
  - `.delta-positive { color: var(--red); font-weight: 600; }` (regression = bad)
  - `.delta-negative { color: var(--green); font-weight: 600; }` (improvement = good)
  - `.delta-zero { color: var(--text-muted); }`
  - `.change-appeared { color: var(--green); }`, `.change-disappeared { color: var(--red); }`, `.change-moved`, `.change-resized { color: var(--yellow); }`
  - `.no-screenshot { display:flex; align-items:center; justify-content:center; min-height:120px; color:var(--text-muted); font-style:italic; font-size:0.85rem; }`
- 2.3.6 Write report header `<h1>` showing baseline vs current IDs and generated timestamp
- 2.3.7 Write `{{range .URLs}}` loop:
  - `<h2>` with `{{.URL}}`
  - Nested `{{range .Breakpoints}}` loop per breakpoint:
    - `<h3>[breakpoint name] — [width]x[height]</h3>`
    - `<div class="diff-row">` with three `.diff-col` divs:
      - **Left** (`.diff-col--baseline`): label "Baseline ({{$.BaselineID}})" + `<img>` if `BaselineURI` non-empty, else `.no-screenshot` placeholder
      - **Center** (`.diff-col--overlay`): label "Diff ({{printf "%.1f" .DiffPercent}}% changed)" + `<img>` if `DiffOverlayURI` non-empty, else "no visual change" placeholder; hide entirely if `!.HasDiff`
      - **Right** (`.diff-col--current`): label "Current ({{$.CurrentID}})" + `<img>` if `CurrentURI` non-empty, else placeholder
  - Audit delta table:
    - Four rows (Contrast, Touch, Typography, Spacing)
    - Each cell: before, after, delta colored by sign using template `if`/`eq` logic
    - Use Go template functions to apply CSS class: `delta-positive` if delta > 0, `delta-negative` if < 0, `delta-zero` if == 0
  - Element changes table (guarded by `{{if .ElementChanges}}`):
    - Columns: Role, Name, Selector, Change Type, Details
    - Each row's Type cell uses class matching `.change-{{.Type}}`

### Task 2.4 — Add template helper functions

#### Sub-tasks
- 2.4.1 Define `template.FuncMap` with a `deltaClass` helper: `func(delta int) string` returning `"delta-positive"`, `"delta-negative"`, or `"delta-zero"`
- 2.4.2 Define `signedDelta` helper: `func(delta int) string` returning `"+N"`, `"-N"`, or `"0"` for display
- 2.4.3 Register both helpers in `template.New("diff-report").Funcs(funcMap).Parse(diffHTMLTemplate)`

---

## Phase 3: Command Handler

**File:** `cmd/diff_report.go`

### Task 3.1 — Implement `RunDiffReport(args []string)`

#### Sub-tasks
- 3.1.1 Create `cmd/diff_report.go` with `package cmd`
- 3.1.2 Parse flags:
  - `snapshotID := getArg(args)` — optional positional argument
  - `outputPath := getFlagValue(args, "--output")` — with fallback to `.kaleidoscope/diff-report.html`
  - `local := hasFlag(args, "--local")` (for future snapshot dir resolution; pass to store constructor)
- 3.1.3 Instantiate `snapshot.BaselineManager` (concrete implementation from US-003; for now, load from `.kaleidoscope/baselines.json` or `~/.kaleidoscope/baselines.json` per `--local`)
- 3.1.4 Call `mgr.ActiveBaselineID()`:
  - On error or empty string: `output.Fail("diff-report", err, "run: ks baseline set")` + `os.Exit(2)`
- 3.1.5 Instantiate `snapshot.Store` (concrete implementation from US-003)
- 3.1.6 Load baseline snapshot: `store.LoadByID(baselineID)`:
  - On error: `output.Fail("diff-report", err, "baseline snapshot not found in store")` + `os.Exit(2)`
- 3.1.7 Load current snapshot:
  - If `snapshotID != ""`: `store.LoadByID(snapshotID)`
  - Else: `store.Latest()`
  - On error or nil: `output.Fail("diff-report", err, "no snapshots found; run: ks snapshot")` + `os.Exit(2)`
- 3.1.8 Call `diff.Compare(baseline, current)` → `result`:
  - On error: `output.Fail("diff-report", err, "")` + `os.Exit(2)`
- 3.1.9 Build `report.DiffData`:
  - Call helper `buildDiffData(baseline, current, result)` → `*report.DiffData`
- 3.1.10 Call `report.WriteDiffFile(outputPath, data)` → `absPath`:
  - On error: `output.Fail("diff-report", err, "")` + `os.Exit(2)`
- 3.1.11 Compute summary counts for output JSON:
  - `urlCount`: `len(result.URLs)`
  - Aggregate `contrastDelta`, `touchDelta`, `typographyDelta`, `spacingDelta` across all URLs
  - `elementChanges`: total count across all URLs
  - `breakpointsDiffed`: total breakpoints with at least one screenshot
- 3.1.12 Emit `output.Success("diff-report", map[string]any{...})` with `path`, `baselineID`, `currentID`, `urlCount`, `summary`

### Task 3.2 — Implement `buildDiffData` helper (within `cmd/diff_report.go`)

#### Sub-tasks
- 3.2.1 Create `buildDiffData(baseline, current *snapshot.Snapshot, result *diff.DiffResult) *report.DiffData`
- 3.2.2 Build a map of `URLDiff` by URL for O(1) lookup: `diffByURL := map[string]*diff.URLDiff{}`
- 3.2.3 Build a map of baseline `URLSnapshot` by URL: `baselineByURL`
- 3.2.4 Build a map of current `URLSnapshot` by URL: `currentByURL`
- 3.2.5 Iterate over all URLs (union of baseline and current):
  - For each URL, create `report.URLDiffSection`
  - Look up corresponding `URLDiff` for audit delta and element changes
  - Convert `diff.AuditDelta` → `report.AuditDeltaRow` (field-by-field copy)
  - Convert `[]diff.ElementChange` → `[]report.ElementChangeRow`
- 3.2.6 For each breakpoint in baseline URL:
  - Find matching breakpoint by `Name` in current URL
  - Load `BaselineURI` via `report.LoadScreenshot(bp.ScreenshotPath)` — empty string on error (non-fatal)
  - Load `CurrentURI` similarly
  - Load `DiffOverlayURI` from `diff.PixelDiff.DiffPath` if non-nil and matched by breakpoint — (tech spec: `PixelDiff` is per URLDiff, not per breakpoint; use it for the first/only breakpoint or associate by index)
  - Set `HasDiff = DiffPercent > 0.0`
  - Create `report.BreakpointDiffRow`
- 3.2.7 Return assembled `*report.DiffData`

---

## Phase 4: Wire into `main.go`

### Task 4.1 — Add `diff-report` to command router

#### Sub-tasks
- 4.1.1 Add `case "diff-report": cmd.RunDiffReport(cmdArgs)` to the `switch` block in `main.go`, after the `report` case
- 4.1.2 Add usage entry under "UX Evaluation" section of `usage` string:
  ```
    diff-report [snapshot-id]  Generate side-by-side HTML diff report vs baseline
  ```

---

## Phase 5: Security Hardening

### Task 5.1 — Path traversal prevention

#### Sub-tasks
- 5.1.1 In `report.WriteDiffFile`: apply `filepath.Clean(path)` before use
- 5.1.2 Do not dereference symlinks — use `os.OpenFile` with `os.O_CREATE|os.O_WRONLY|os.O_TRUNC` (no `os.O_SYNC` needed); avoid `os.Create` on symlink targets by checking `os.Lstat` first if needed
- 5.1.3 Ensure all user-controlled strings (URL, snapshot IDs, selector names, element names, details) pass through `html/template` automatic escaping — never cast to `template.HTML`
- 5.1.4 Only `BaselineURI`, `CurrentURI`, `DiffOverlayURI` use `template.URL` — confirm each is formed exclusively from `"data:image/png;base64,"` prefix + base64 content (no user input)

---

## Phase 6: Testing

### Task 6.1 — Unit tests for `report.GenerateDiffReport`

**File:** `report/diff_report_test.go`

#### Sub-tasks
- 6.1.1 Test: empty `DiffData` (no URLs) renders without error
- 6.1.2 Test: single URL with two breakpoints, both screenshots available — verify baseline/current/overlay `<img>` tags appear in output
- 6.1.3 Test: missing screenshot (empty URI) renders `.no-screenshot` placeholder, no broken `<img>` tag
- 6.1.4 Test: positive delta (regression) produces `delta-positive` class in output
- 6.1.5 Test: negative delta (improvement) produces `delta-negative` class in output
- 6.1.6 Test: zero delta produces `delta-zero` class
- 6.1.7 Test: element changes section only appears when `ElementChanges` is non-empty
- 6.1.8 Test: element change type classes (`change-appeared`, `change-disappeared`, `change-moved`, `change-resized`) appear correctly
- 6.1.9 Test: user-controlled strings (URL, selector) are HTML-escaped (e.g., `<script>` → `&lt;script&gt;`)

### Task 6.2 — Unit tests for `cmd.buildDiffData`

**File:** `cmd/diff_report_test.go`

#### Sub-tasks
- 6.2.1 Test: baseline with 2 URLs, current with 2 URLs → output has 2 `URLDiffSection` entries
- 6.2.2 Test: `AuditDelta` fields are correctly mapped to `AuditDeltaRow`
- 6.2.3 Test: `ElementChange` slice is correctly mapped to `ElementChangeRow` slice
- 6.2.4 Test: breakpoint present in baseline but missing in current → `CurrentURI` is empty string, `BaselineURI` non-empty (assuming screenshot file exists)
- 6.2.5 Test: `PixelDiff` with `DiffPercent > 0` → `HasDiff` is true and `DiffPercent` is set

### Task 6.3 — Integration smoke test for `RunDiffReport`

#### Sub-tasks
- 6.3.1 Test: call `RunDiffReport` with stub store returning no baseline → `os.Exit(2)` (use `exec.Command` subprocess or capture via test harness)
- 6.3.2 Test: call `RunDiffReport` with valid stub baseline and current snapshot → HTML file written to `--output` path, JSON output contains `"ok": true`

### Task 6.4 — Run quality gate

#### Sub-tasks
- 6.4.1 Run `go build ./...` — must compile with zero errors
- 6.4.2 Run `go test ./...` — all tests must pass
- 6.4.3 Run `go vet ./...` — no vet errors

---

## File Manifest

| File | Action | Phase |
|------|--------|-------|
| `snapshot/store.go` | Create (package skeleton) | 1 |
| `diff/engine.go` | Create (package skeleton + stub) | 1 |
| `report/diff_report.go` | Create (DiffData, GenerateDiffReport, WriteDiffFile, template) | 2 |
| `cmd/diff_report.go` | Create (RunDiffReport, buildDiffData) | 3 |
| `main.go` | Modify (add case + usage entry) | 4 |
| `report/diff_report_test.go` | Create (report unit tests) | 6 |
| `cmd/diff_report_test.go` | Create (cmd unit tests) | 6 |

No new Go module dependencies are required.

---

## Dependency Notes

- `snapshot` and `diff` packages are created as stubs in Phase 1 so that Phases 2–4 compile independently of US-003/US-004 delivery.
- When US-003 delivers `snapshot/store.go` with real implementations, the stub file is replaced in its entirety; the interfaces remain identical.
- When US-004 delivers `diff/engine.go` with a real `Compare` function, the stub body is replaced; the signature is unchanged.
- `cmd/util.go` requires no changes — `getFlagValue`, `hasFlag`, and `getArg` already handle `--output` and positional args correctly.
