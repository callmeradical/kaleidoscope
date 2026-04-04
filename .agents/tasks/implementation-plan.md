# Implementation Plan: US-006 — Side-by-Side HTML Diff Report

## Overview

Implements `ks diff-report [snapshot-id] [--output path]` — a self-contained HTML report comparing
a baseline snapshot to a current (or specified) snapshot with side-by-side screenshots, pixel diff
overlays, audit delta tables, and element change lists.

**Dependencies required before US-006 code can be written:**
- US-003: `snapshot/` package (data types, disk I/O, baseline management)
- US-004: `diff/` package (audit delta, element diff, pixel diff engine)

---

## Phase 1: `snapshot` Package (US-003 prerequisite)

> New package at `snapshot/`. No Chrome dependency. Pure disk I/O + JSON.

### Task 1.1 — Define core data types (`snapshot/snapshot.go`)

- **Sub-task 1.1.1** — Create `snapshot/snapshot.go` with package declaration and imports (`time`, `encoding/json`)
- **Sub-task 1.1.2** — Define `Snapshot` struct with fields: `ID string`, `CreatedAt time.Time`, `CommitSHA string`, `URLs []URLSnapshot`; add JSON tags matching the spec
- **Sub-task 1.1.3** — Define `URLSnapshot` struct: `URL string`, `Breakpoints []BreakpointSnapshot`, `Audit AuditResult`, `Elements []AXElement`
- **Sub-task 1.1.4** — Define `BreakpointSnapshot` struct: `Name`, `Width int`, `Height int`, `ScreenshotPath string` (relative to snapshot dir)
- **Sub-task 1.1.5** — Define `AuditResult` struct with all counter fields (`ContrastViolations`, `TouchViolations`, `TypographyWarnings`, `SpacingIssues`, `AXTotalNodes`, `AXActiveNodes`) and issue list fields (`ContrastIssueList`, `TouchIssueList`, etc.)
- **Sub-task 1.1.6** — Define leaf entry types: `ContrastEntry`, `TouchEntry`, `TypoEntry`, `SpacingEntry` with all fields matching the spec
- **Sub-task 1.1.7** — Define `AXElement` struct: `Role`, `Name`, `Selector string`, `X`, `Y`, `Width`, `Height float64`

### Task 1.2 — Implement storage helpers (`snapshot/store.go`)

- **Sub-task 1.2.1** — Create `snapshot/store.go`; import `os`, `path/filepath`, `encoding/json`, `sort`, `errors`
- **Sub-task 1.2.2** — Implement `SnapshotDir(id string) (string, error)`:
  - Resolve `.kaleidoscope/snapshots/<id>/` relative to the working directory
  - Validate `id` does not contain `..`, `/`, or null bytes (path traversal guard)
  - Return absolute path
- **Sub-task 1.2.3** — Implement `Save(s *Snapshot) error`:
  - Call `SnapshotDir(s.ID)` to get dir path
  - `os.MkdirAll(dir, 0700)`
  - Marshal `s` to JSON and write to `snapshot.json` with mode `0600`
- **Sub-task 1.2.4** — Implement `Load(id string) (*Snapshot, error)`:
  - Call `SnapshotDir(id)` (includes path traversal validation)
  - Read `snapshot.json`, unmarshal JSON into `*Snapshot`, return
- **Sub-task 1.2.5** — Implement `List() ([]string, error)`:
  - Read entries from `.kaleidoscope/snapshots/`
  - Return directory names sorted newest-first (lexicographic desc works given the `YYYYMMDD-HHmmss` prefix)
- **Sub-task 1.2.6** — Implement `Latest() (*Snapshot, error)`:
  - Call `List()`, take first entry
  - Return `errors.New("no snapshots found")` if list is empty
  - Call `Load(ids[0])` and return result

### Task 1.3 — Implement baseline management (`snapshot/baseline.go`)

- **Sub-task 1.3.1** — Create `snapshot/baseline.go`; define `type Baselines map[string]string`
- **Sub-task 1.3.2** — Implement `LoadBaselines() (Baselines, error)`:
  - Read `baselines.json` from working directory
  - If file does not exist (`os.IsNotExist`), return empty `Baselines{}` with nil error
  - Unmarshal JSON and return
- **Sub-task 1.3.3** — Implement `SaveBaselines(b Baselines) error`:
  - Marshal to JSON with `json.MarshalIndent`
  - Write atomically: write to temp file, then `os.Rename` to `baselines.json`
- **Sub-task 1.3.4** — Implement `(b Baselines) BaselineFor(url string) (string, bool)`:
  - Simple map lookup; return value and presence boolean

---

## Phase 2: `diff` Package (US-004 prerequisite)

> New package at `diff/`. Pure functions. No Chrome, no side effects beyond reading screenshot PNG files.

### Task 2.1 — Define diff data types (`diff/diff.go`)

- **Sub-task 2.1.1** — Create `diff/diff.go`; import `time` and `snapshot` package
- **Sub-task 2.1.2** — Define `AuditDelta` struct: `Category string`, `Before int`, `After int`, `Delta int` (After-Before; positive = regression)
- **Sub-task 2.1.3** — Define `ElementChange` struct: `Role`, `Name`, `Selector`, `Type string` (`"appeared"|"disappeared"|"moved"|"resized"`), `Details string`
- **Sub-task 2.1.4** — Define `BreakpointDiff` struct: `Name string`, `Width int`, `Height int`, `BaselinePath string`, `CurrentPath string`, `DiffPNG []byte` (not serialized; json:"-"`), `DiffPercent float64`, `ChangedPixels int`, `TotalPixels int`
- **Sub-task 2.1.5** — Define `URLDiff` struct: `URL string`, `AuditDeltas []AuditDelta`, `ElementChanges []ElementChange`, `Breakpoints []BreakpointDiff`, `HasRegression bool`
- **Sub-task 2.1.6** — Define `Result` struct: `BaselineID string`, `CurrentID string`, `GeneratedAt time.Time`, `URLs []URLDiff`, `HasRegressions bool`

### Task 2.2 — Implement audit delta computation (`diff/diff.go`)

- **Sub-task 2.2.1** — Implement `computeAuditDeltas(base, cur snapshot.AuditResult) []AuditDelta`:
  - Build one `AuditDelta` per category: Contrast, Touch, Typography, Spacing
  - Set `Before`, `After`, `Delta = After - Before`
- **Sub-task 2.2.2** — Add helper `hasAuditRegression(deltas []AuditDelta) bool`: returns true if any delta > 0

### Task 2.3 — Implement element diff (`diff/diff.go`)

- **Sub-task 2.3.1** — Implement `computeElementChanges(baseElems, curElems []snapshot.AXElement) []ElementChange`:
  - Build index of baseline elements: `map[role+"|"+name]*AXElement`
  - Build index of current elements: `map[role+"|"+name]*AXElement`
  - Iterate baseline: if key not in current → `"disappeared"`; if in current but position changed beyond 2px → `"moved"` (build Details string e.g. `"moved 14px right, 3px down"`); if size changed beyond 2px → `"resized"`
  - Iterate current: if key not in baseline → `"appeared"`
  - Return combined list

### Task 2.4 — Implement pixel diff engine (`diff/pixeldiff.go`)

- **Sub-task 2.4.1** — Create `diff/pixeldiff.go`; import only stdlib: `bytes`, `image`, `image/color`, `image/png`, `errors`, `fmt`
- **Sub-task 2.4.2** — Implement `CompareImages(baselinePNG, currentPNG []byte, threshold uint8) (diffPNG []byte, changed, total int, err error)`:
  - Decode both PNGs with `image/png`
  - Validate decoded dimensions ≤ 4096×8192; return error if exceeded (memory safety guard from security spec)
  - Determine max bounds of both images
  - Create `image.RGBA` output buffer sized to max bounds
  - Iterate all pixels in bounds:
    - Sample each image at (x, y); treat out-of-bounds pixels as black
    - Compute per-channel absolute deltas (R, G, B); sum them
    - If sum > `uint(threshold)*3`: set output pixel to red `(255, 0, 0, 255)`; increment `changed`
    - Else: copy baseline pixel at 30% opacity (`alpha = 77`) on black background
    - Increment `total`
  - Encode output RGBA image to PNG via `image/png`; return bytes and stats

### Task 2.5 — Implement `Compute` orchestrator (`diff/diff.go`)

- **Sub-task 2.5.1** — Implement `Compute(baseline, current *snapshot.Snapshot) (*Result, error)`:
  - Build a `map[url]*snapshot.URLSnapshot` for baseline URLs
  - Iterate `current.URLs`; for each URL:
    - Look up matching baseline URL (skip with no-baseline note if absent)
    - Call `computeAuditDeltas`
    - Call `computeElementChanges`
    - For each `current.Breakpoints`: find matching baseline breakpoint by name; read both PNG files from disk; call `CompareImages(baselinePNG, currentPNG, 10)`; build `BreakpointDiff`
    - Determine `HasRegression` from audit deltas
    - Append `URLDiff` to result
  - Set `HasRegressions = any URLDiff.HasRegression`
  - Return `*Result` with `BaselineID`, `CurrentID`, `GeneratedAt = time.Now()`

---

## Phase 3: `diffreport` Package (US-006 core)

> New package at `diffreport/`. No Chrome, no disk reads (caller passes data in). Pure rendering.

### Task 3.1 — Define report data types (`diffreport/report.go`)

- **Sub-task 3.1.1** — Create `diffreport/report.go`; import `encoding/base64`, `html/template`, `io`, `os`, `path/filepath`, `time` and `diff`, `snapshot` packages
- **Sub-task 3.1.2** — Define `BreakpointSection` struct: `Name string`, `Width int`, `Height int`, `BaselineDataURI template.URL`, `CurrentDataURI template.URL`, `DiffDataURI template.URL`, `DiffPercent float64`, `ChangedPixels int`, `TotalPixels int`
- **Sub-task 3.1.3** — Define `AuditDeltaRow` struct: `Category string`, `Before int`, `After int`, `Delta int`, `IsWorse bool` (Delta>0), `IsBetter bool` (Delta<0)
- **Sub-task 3.1.4** — Define `ElementChangeRow` struct: `Role`, `Name`, `Selector`, `Type`, `Details string`
- **Sub-task 3.1.5** — Define `URLSection` struct: `URL string`, `Breakpoints []BreakpointSection`, `AuditDeltas []AuditDeltaRow`, `ElementChanges []ElementChangeRow`, `HasRegression bool`
- **Sub-task 3.1.6** — Define `Data` struct: `BaselineID string`, `CurrentID string`, `BaselineTime time.Time`, `CurrentTime time.Time`, `BaselineSHA string`, `CurrentSHA string`, `GeneratedAt time.Time`, `URLs []URLSection`, `HasRegressions bool`

### Task 3.2 — Implement `Build` function (`diffreport/report.go`)

- **Sub-task 3.2.1** — Implement helper `encodeImage(path string) (template.URL, error)`:
  - Read file bytes from `path`
  - Base64-encode with `base64.StdEncoding.EncodeToString`
  - Return `template.URL("data:image/png;base64," + encoded)`
- **Sub-task 3.2.2** — Implement helper `encodeDiffImage(pngBytes []byte) template.URL`:
  - If `pngBytes == nil` or len == 0, return empty string
  - Base64-encode and return `template.URL("data:image/png;base64," + encoded)`
- **Sub-task 3.2.3** — Implement `Build(result *diff.Result, baseline, current *snapshot.Snapshot) (*Data, error)`:
  - Build baseline URL index: `map[string]*snapshot.URLSnapshot`
  - Build current URL index: `map[string]*snapshot.URLSnapshot`
  - For each `result.URLs` (URLDiff):
    - Build `[]AuditDeltaRow` from `urlDiff.AuditDeltas` (set `IsWorse`/`IsBetter`)
    - Build `[]ElementChangeRow` from `urlDiff.ElementChanges`
    - For each `urlDiff.Breakpoints` (BreakpointDiff):
      - Find matching baseline breakpoint in snapshot to get full path; call `encodeImage(baselinePath)`
      - Find matching current breakpoint in snapshot to get full path; call `encodeImage(currentPath)`
      - Call `encodeDiffImage(bpDiff.DiffPNG)` (already in memory from Compute)
      - Build `BreakpointSection`
    - Build and append `URLSection`
  - Populate `Data` fields from `result` and both snapshot headers
  - Return `*Data`

### Task 3.3 — Implement `Generate` function (`diffreport/report.go`)

- **Sub-task 3.3.1** — Implement `Generate(w io.Writer, data *Data) error`:
  - Parse `htmlTemplate` constant using `html/template` (auto-escaping by default; never use `text/template`)
  - Execute template with `data` into `w`
  - Return any template error

### Task 3.4 — Implement `WriteFile` function (`diffreport/report.go`)

- **Sub-task 3.4.1** — Implement `WriteFile(path string, data *Data) (string, error)`:
  - Resolve absolute path via `filepath.Abs(path)`
  - `os.MkdirAll(filepath.Dir(absPath), 0700)`
  - Create file with `os.Create(absPath)`
  - Call `Generate(f, data)`
  - Return `absPath`

### Task 3.5 — Implement HTML template (`diffreport/template.go`)

- **Sub-task 3.5.1** — Create `diffreport/template.go` with `const htmlTemplate = \`...\`` string
- **Sub-task 3.5.2** — Write `<head>` section:
  - `<meta charset="UTF-8">`, viewport meta tag, `<title>Kaleidoscope Diff Report</title>`
  - Embed all CSS as `<style>` block (self-contained; no external dependencies)
- **Sub-task 3.5.3** — Write CSS `:root` block (copy color variables from `report/report.go`: `--bg`, `--surface`, `--border`, `--text`, `--text-muted`, `--red`, `--red-dim`, `--green`, `--green-dim`; add any missing vars)
- **Sub-task 3.5.4** — Write base body/typography CSS (copy base styles from existing `report/report.go` template)
- **Sub-task 3.5.5** — Write diff-specific CSS additions:
  - `.screenshot-trio`: `display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 0.75rem; margin: 1rem 0`
  - `.ss-col`, `.ss-label`, `.ss-col img`, `.no-diff` styles per spec
  - `.delta-cards`: `display: grid; grid-template-columns: repeat(4, 1fr); gap: 0.75rem; margin: 1rem 0`
  - `.delta-card`, `.delta-card.worse`, `.delta-card.better`, `.delta-card.neutral`, `.delta-label`, `.delta-values`, `.delta-badge` styles per spec
  - `.url-section`, `.bp-section` styles per spec
  - Regression badge styles: `.badge-regression`, `.badge-ok`
  - Element changes table styles
- **Sub-task 3.5.6** — Write `<header>` section:
  - Title "Kaleidoscope Diff Report"
  - Baseline metadata: `{{.BaselineID}}` `{{.BaselineSHA}}` `{{.BaselineTime.Format "2006-01-02 15:04:05"}}`
  - Arrow separator `→`
  - Current metadata: `{{.CurrentID}}` `{{.CurrentSHA}}` `{{.CurrentTime.Format "2006-01-02 15:04:05"}}`
  - Conditional regression badge: `{{if .HasRegressions}}<span class="badge-regression">Regressions Detected</span>{{else}}<span class="badge-ok">No Regressions</span>{{end}}`
- **Sub-task 3.5.7** — Write `{{range .URLs}}` URL section loop:
  - `<h2>{{.URL}}` with inline regression badge `{{if .HasRegression}}`
  - Delta cards `<div class="delta-cards">` with `{{range .AuditDeltas}}` loop; apply class `{{if .IsWorse}}worse{{else if .IsBetter}}better{{else}}neutral{{end}}`; display Before → After and `{{if gt .Delta 0}}+{{end}}{{.Delta}}`
- **Sub-task 3.5.8** — Write breakpoint screenshot trio loop `{{range .Breakpoints}}`:
  - Section header with name, dimensions, and `{{printf "%.2f" .DiffPercent}}% changed`
  - `.screenshot-trio` grid with three `.ss-col` divs: Baseline, Diff Overlay, Current
  - Baseline col: `<img src="{{.BaselineDataURI}}" alt="Baseline {{.Name}}">`
  - Diff col: `{{if .DiffDataURI}}<img src="{{.DiffDataURI}}"...>{{else}}<div class="no-diff">Identical</div>{{end}}`
  - Current col: `<img src="{{.CurrentDataURI}}" alt="Current {{.Name}}">`
- **Sub-task 3.5.9** — Write audit delta detail tables (below screenshot trio):
  - Table header: Category | Before | After | Delta
  - `{{range .AuditDeltas}}` row with colored delta cell
- **Sub-task 3.5.10** — Write element change list section (below audit table):
  - Show only if `{{if .ElementChanges}}`
  - Table header: Role | Name | Selector | Change Type | Details
  - `{{range .ElementChanges}}` rows; Type cell colored by type (appeared=green, disappeared=red, moved/resized=yellow)
- **Sub-task 3.5.11** — Write `<footer>`: `Generated by kaleidoscope · {{.GeneratedAt.Format "2006-01-02 15:04:05 UTC"}}`

---

## Phase 4: Command Handler (`cmd/diff_report.go`)

### Task 4.1 — Implement `RunDiffReport` function

- **Sub-task 4.1.1** — Create `cmd/diff_report.go`; import `output`, `snapshot`, `diff`, `diffreport` packages and `strings`
- **Sub-task 4.1.2** — Implement `RunDiffReport(args []string)`:
  - Parse positional snapshot-id: `snapshotID := ""` then find first non-flag arg
  - Parse `--output` flag value; default to `".kaleidoscope/diff-report.html"` if empty
- **Sub-task 4.1.3** — Step 1 — Load baselines:
  - Call `snapshot.LoadBaselines()`
  - If error → `output.Fail("diff-report", err, "failed to load baselines")` and return
- **Sub-task 4.1.4** — Step 2 — Load current snapshot:
  - If `snapshotID != ""`: call `snapshot.Load(snapshotID)` (path traversal validation is inside `SnapshotDir`)
  - Else: call `snapshot.Latest()`
  - If error → `output.Fail("diff-report", err, "No snapshots found. Run \`ks snapshot\` first")` and return
- **Sub-task 4.1.5** — Step 3 — Find and load baseline snapshot:
  - Collect unique baseline snapshot IDs from `baselines` for all URLs in `current.URLs`
  - If no baseline IDs found → `output.Fail("diff-report", nil, "No baseline set. Run \`ks snapshot --set-baseline <id>\`")` and return
  - Load the baseline snapshot (for simplicity: use the first/common baseline ID, or load per-URL if multiple)
  - If error loading → `output.Fail` and return
- **Sub-task 4.1.6** — Step 4 — Compute diff:
  - Call `diff.Compute(baselineSnap, currentSnap)`
  - If error → `output.Fail("diff-report", err, "diff computation failed")` and return
- **Sub-task 4.1.7** — Step 5 — Build HTML data model:
  - Call `diffreport.Build(result, baselineSnap, currentSnap)`
  - If error → `output.Fail("diff-report", err, "failed to build report data")` and return
- **Sub-task 4.1.8** — Step 6 — Write file:
  - Call `diffreport.WriteFile(outputPath, data)`
  - If error → `output.Fail("diff-report", err, "failed to write report file")` and return
- **Sub-task 4.1.9** — Step 7 — Emit JSON success:
  - Call `output.Success("diff-report", map[string]any{"path": absPath, "baselineId": result.BaselineID, "currentId": result.CurrentID, "hasRegressions": result.HasRegressions, "urlCount": len(result.URLs)})`

---

## Phase 5: Integration (`main.go`)

### Task 5.1 — Register `diff-report` command in `main.go`

- **Sub-task 5.1.1** — Open `main.go`; locate the `switch` statement routing commands
- **Sub-task 5.1.2** — Add case: `case "diff-report": cmd.RunDiffReport(cmdArgs)`
- **Sub-task 5.1.3** — Locate the usage string; find the "UX Evaluation" section; add entry:
  ```
  diff-report [id] [--output path]   Side-by-side HTML diff vs baseline
  ```

---

## Phase 6: Tests

### Task 6.1 — Unit tests for `snapshot` package (`snapshot/snapshot_test.go`)

- **Sub-task 6.1.1** — Test `SnapshotDir` path traversal validation: verify that IDs containing `..`, `/`, null bytes return errors
- **Sub-task 6.1.2** — Test `Save` + `Load` round-trip: create a `Snapshot`, save it to temp dir, load it back, compare fields
- **Sub-task 6.1.3** — Test `Latest` returns error when no snapshots exist
- **Sub-task 6.1.4** — Test `LoadBaselines` returns empty map (not error) when `baselines.json` does not exist
- **Sub-task 6.1.5** — Test `SaveBaselines` + `LoadBaselines` round-trip

### Task 6.2 — Unit tests for `diff` package (`diff/diff_test.go`)

- **Sub-task 6.2.1** — Test `computeAuditDeltas`: verify delta = After - Before for all 4 categories
- **Sub-task 6.2.2** — Test `computeElementChanges`: test each of appeared, disappeared, moved, resized scenarios
- **Sub-task 6.2.3** — Test `CompareImages` with identical images: expect `changed == 0`, `diffPercent == 0`
- **Sub-task 6.2.4** — Test `CompareImages` with completely different images: expect `changed == total`
- **Sub-task 6.2.5** — Test `CompareImages` with oversized image: expect error (4096×8192 limit)
- **Sub-task 6.2.6** — Test `CompareImages` with different-sized images: verify padding behavior (no panic, result has max-dimension bounds)

### Task 6.3 — Unit tests for `diffreport` package (`diffreport/report_test.go`)

- **Sub-task 6.3.1** — Test `Build` produces correct number of `URLSection` entries from a `diff.Result`
- **Sub-task 6.3.2** — Test `AuditDeltaRow.IsWorse` is true when Delta > 0; `IsBetter` is true when Delta < 0
- **Sub-task 6.3.3** — Test `Generate` produces valid HTML output (check for key substrings: `<!DOCTYPE html>`, `Kaleidoscope Diff Report`, `screenshot-trio`)
- **Sub-task 6.3.4** — Test `WriteFile` creates file at specified path and parent directories

### Task 6.4 — Integration smoke test (`cmd/diff_report_test.go`)

- **Sub-task 6.4.1** — Test `RunDiffReport` with no snapshots: verify JSON output contains `"ok": false` and the "no snapshots" hint
- **Sub-task 6.4.2** — Test `RunDiffReport` with snapshots but no baseline: verify JSON output contains `"ok": false` and the set-baseline hint

---

## Phase 7: Security & Quality Hardening

### Task 7.1 — Path traversal validation audit

- **Sub-task 7.1.1** — Verify `SnapshotDir` rejects `..`, `/`, and null bytes; confirm the resolved path is checked with `strings.HasPrefix(absPath, snapshotRoot)` before file open

### Task 7.2 — Template safety audit

- **Sub-task 7.2.1** — Confirm `diffreport/template.go` uses `html/template` (not `text/template`) throughout
- **Sub-task 7.2.2** — Confirm no `template.HTML()` cast is used on user-derived string fields (URLs, selectors, element names, diff details)
- **Sub-task 7.2.3** — Confirm only `template.URL` is used for data URIs — and only those constructed internally from base64-encoded PNGs

### Task 7.3 — Memory safety audit

- **Sub-task 7.3.1** — Confirm `CompareImages` checks decoded image dimensions against the 4096×8192 limit before allocating the output buffer
- **Sub-task 7.3.2** — Confirm no shell-out (`exec.Command`) anywhere in `snapshot/`, `diff/`, or `diffreport/`

### Task 7.4 — File permission audit

- **Sub-task 7.4.1** — Confirm `snapshot.Save` uses `os.MkdirAll(dir, 0700)` and writes files with mode `0600`
- **Sub-task 7.4.2** — Confirm `diffreport.WriteFile` uses `os.MkdirAll(dir, 0700)` for parent directories

### Task 7.5 — Run full test suite

- **Sub-task 7.5.1** — Run `go build ./...` and fix any compilation errors
- **Sub-task 7.5.2** — Run `go test ./...` and confirm all tests pass
- **Sub-task 7.5.3** — Run `go vet ./...` and address any issues

---

## File Creation Summary

| File | Phase | Description |
|------|-------|-------------|
| `snapshot/snapshot.go` | 1 | Data types: Snapshot, URLSnapshot, BreakpointSnapshot, AuditResult, AXElement, entry types |
| `snapshot/store.go` | 1 | SnapshotDir, Save, Load, List, Latest |
| `snapshot/baseline.go` | 1 | Baselines type, LoadBaselines, SaveBaselines, BaselineFor |
| `diff/diff.go` | 2 | All diff types + Compute + audit/element diff helpers |
| `diff/pixeldiff.go` | 2 | CompareImages — pure Go pixel comparison |
| `diffreport/report.go` | 3 | Data types + Build + Generate + WriteFile |
| `diffreport/template.go` | 3 | htmlTemplate constant — full self-contained HTML |
| `cmd/diff_report.go` | 4 | RunDiffReport command handler |
| `main.go` (edit) | 5 | Add "diff-report" case + usage string entry |
| `snapshot/snapshot_test.go` | 6 | snapshot package tests |
| `diff/diff_test.go` | 6 | diff package tests |
| `diffreport/report_test.go` | 6 | diffreport package tests |
| `cmd/diff_report_test.go` | 6 | Command handler integration tests |

## Acceptance Criteria Mapping

| Acceptance Criterion | Implemented By |
|----------------------|----------------|
| `ks diff-report` generates self-contained HTML comparing latest vs baseline | Phase 4 + Phase 5 |
| Side-by-side layout: baseline (left), diff (center), current (right) per URL per breakpoint | Task 3.5.8 |
| Audit delta tables show per-category before/after/delta counts | Tasks 3.5.7, 3.5.9 |
| Element change lists show selector, type, details | Task 3.5.10 |
| `--output` flag controls path; default `.kaleidoscope/diff-report.html` | Task 4.1.2 |
| Screenshots are base64-embedded | Tasks 3.2.1–3.2.3 |
| Fails gracefully with clear error if no baseline or no snapshots exist | Tasks 4.1.3–4.1.5 |
