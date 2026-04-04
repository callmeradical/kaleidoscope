# Tech Spec: US-006 — Side-by-Side HTML Diff Report

## Overview

Introduces `ks diff-report [snapshot-id] [--output path]`, a new command that generates a self-contained HTML file comparing the latest snapshot against a baseline. The report extends the existing `report/` template engine and embeds all screenshots as base64 data URIs.

This spec assumes US-003 (snapshot storage) and US-004 (diff engine) are implemented. It describes the interfaces those stories must expose for US-006 to consume.

---

## Architecture Overview

```
main.go
  └── case "diff-report" → cmd.RunDiffReport(args)

cmd/diff_report.go
  ├── loads snapshot store (snapshot.Store)
  ├── loads baseline config (snapshot.BaselineManager)
  ├── calls diff engine (diff.Compare)
  └── calls report.GenerateDiffReport(w, *report.DiffData)

report/diff_report.go
  ├── report.DiffData struct
  ├── report.GenerateDiffReport(w io.Writer, data *DiffData) error
  └── diffHTMLTemplate (self-contained HTML with embedded CSS)

snapshot/          (owned by US-003)
  ├── store.go     — Snapshot, Store, Load/Save
  └── baseline.go  — BaselineManager, LoadBaseline

diff/              (owned by US-004)
  └── engine.go    — Compare(baseline, current Snapshot) → DiffResult
```

No Chrome/browser dependency. The command reads from disk only.

---

## Detailed Component Design

### 1. `cmd/diff_report.go`

**Function:** `RunDiffReport(args []string)`

**Flags:**
- Positional arg `[snapshot-id]` — optional; defaults to latest snapshot
- `--output <path>` — output file path; defaults to `.kaleidoscope/diff-report.html`

**Logic:**
1. Resolve snapshot directory (global or `--local` `.kaleidoscope/snapshots/`).
2. Load `BaselineManager` and retrieve the active baseline snapshot ID. If no baseline exists, call `output.Fail` with message `"no baseline set; run: ks baseline set"` and exit 2.
3. Load baseline `Snapshot` from store. If not found, call `output.Fail` and exit 2.
4. Load current `Snapshot`: if snapshot-id arg provided, load by ID; otherwise load latest. If none exists, call `output.Fail` with message `"no snapshots found; run: ks snapshot"` and exit 2.
5. Call `diff.Compare(baseline, current)` → `DiffResult`.
6. Build `report.DiffData` from baseline, current, and DiffResult (including base64-encoded screenshots).
7. Write HTML to output path via `report.GenerateDiffReport`.
8. Emit `output.Success("diff-report", {...})` with path, snapshotID, baselineID, and summary counts.

**Error handling:** All errors use `output.Fail("diff-report", err, hint)` followed by `os.Exit(2)`. No panics.

---

### 2. Interfaces Consumed from US-003 (snapshot package)

```go
// snapshot/store.go

type Snapshot struct {
    ID          string              `json:"id"`           // e.g. "20260404-153012"
    CreatedAt   time.Time           `json:"createdAt"`
    CommitSHA   string              `json:"commitSha,omitempty"`
    URLs        []URLSnapshot       `json:"urls"`
}

type URLSnapshot struct {
    URL         string              `json:"url"`
    Breakpoints []BreakpointCapture `json:"breakpoints"`
    AuditResult AuditSummary        `json:"auditResult"`
    AXElements  []AXElement         `json:"axElements"`
}

type BreakpointCapture struct {
    Name        string `json:"name"`           // "mobile", "tablet", "desktop", "wide"
    Width       int    `json:"width"`
    Height      int    `json:"height"`
    ScreenshotPath string `json:"screenshotPath"` // absolute or relative to snapshot dir
}

type AuditSummary struct {
    ContrastViolations int `json:"contrastViolations"`
    TouchViolations    int `json:"touchViolations"`
    TypographyWarnings int `json:"typographyWarnings"`
    SpacingIssues      int `json:"spacingIssues"`
}

type AXElement struct {
    Role     string `json:"role"`
    Name     string `json:"name"`
    Selector string `json:"selector,omitempty"`
    Bounds   *Rect  `json:"bounds,omitempty"`
}

type Rect struct {
    X, Y, Width, Height float64
}

// Store interface
type Store interface {
    Latest() (*Snapshot, error)
    LoadByID(id string) (*Snapshot, error)
}

// BaselineManager interface
type BaselineManager interface {
    ActiveBaselineID() (string, error) // returns "" if none set
}
```

---

### 3. Interfaces Consumed from US-004 (diff package)

```go
// diff/engine.go

type DiffResult struct {
    URLs []URLDiff
}

type URLDiff struct {
    URL         string
    AuditDelta  AuditDelta
    ElementChanges []ElementChange
    PixelDiff   *PixelDiff // nil if no screenshots available for both sides
}

type AuditDelta struct {
    ContrastBefore, ContrastAfter, ContrastDelta     int
    TouchBefore,    TouchAfter,    TouchDelta         int
    TypographyBefore, TypographyAfter, TypographyDelta int
    SpacingBefore,  SpacingAfter,  SpacingDelta      int
}

type ElementChange struct {
    Role     string // AX role
    Name     string // AX name
    Selector string
    Type     string // "appeared" | "disappeared" | "moved" | "resized"
    Details  string // human-readable description, e.g. "moved from (10,20) to (15,25)"
}

type PixelDiff struct {
    DiffPath        string  // path to generated diff overlay PNG
    DiffPercent     float64 // 0.0–100.0 percentage of changed pixels
    ChangedPixels   int
    TotalPixels     int
}

// Pure function — no Chrome dependency
func Compare(baseline, current *snapshot.Snapshot) (*DiffResult, error)
```

---

### 4. `report/diff_report.go`

#### Data Model

```go
package report

// DiffData holds all data needed to render the HTML diff report.
type DiffData struct {
    BaselineID  string
    CurrentID   string
    GeneratedAt time.Time
    URLs        []URLDiffSection
}

type URLDiffSection struct {
    URL         string
    Breakpoints []BreakpointDiffRow
    AuditDelta  AuditDeltaRow
    ElementChanges []ElementChangeRow
}

type BreakpointDiffRow struct {
    Name           string
    Width          int
    Height         int
    BaselineURI    template.URL // base64 data URI, empty string if missing
    CurrentURI     template.URL // base64 data URI, empty string if missing
    DiffOverlayURI template.URL // base64 data URI of pixel diff PNG, empty if no diff
    DiffPercent    float64
    HasDiff        bool
}

type AuditDeltaRow struct {
    ContrastBefore, ContrastAfter, ContrastDelta     int
    TouchBefore,    TouchAfter,    TouchDelta         int
    TypographyBefore, TypographyAfter, TypographyDelta int
    SpacingBefore,  SpacingAfter,  SpacingDelta      int
}

type ElementChangeRow struct {
    Role     string
    Name     string
    Selector string
    Type     string // "appeared" | "disappeared" | "moved" | "resized"
    Details  string
}
```

#### Functions

```go
// GenerateDiffReport writes the HTML diff report to w.
func GenerateDiffReport(w io.Writer, data *DiffData) error

// WriteDiffFile generates the diff report and writes it to path.
// Creates parent directories as needed.
func WriteDiffFile(path string, data *DiffData) (string, error)
```

`GenerateDiffReport` follows the same pattern as `Generate`: parse `diffHTMLTemplate` and call `tmpl.Execute(w, data)`.

---

### 5. HTML Template Design (`report/diff_report.go`)

The template is a single Go string constant `diffHTMLTemplate` embedded in `diff_report.go`. It reuses the same CSS design tokens as `htmlTemplate` (dark theme, same color variables, same card/table styles).

**Layout per URL section:**

```
<h2>[URL]</h2>

  For each breakpoint:
    <h3>[breakpoint] — [width]x[height]</h3>
    <div class="diff-row">
      <div class="diff-col diff-col--baseline">
        <div class="diff-label">Baseline ([baselineID])</div>
        <img src="[BaselineURI]" />          <!-- or placeholder if missing -->
      </div>
      <div class="diff-col diff-col--overlay">
        <div class="diff-label">Diff ([DiffPercent]% changed)</div>
        <img src="[DiffOverlayURI]" />       <!-- or "no visual change" -->
      </div>
      <div class="diff-col diff-col--current">
        <div class="diff-label">Current ([currentID])</div>
        <img src="[CurrentURI]" />
      </div>
    </div>

  Audit Delta Table:
    | Category    | Baseline | Current | Delta |
    |-------------|----------|---------|-------|
    | Contrast    | N        | N       | ±N    |
    | Touch       | N        | N       | ±N    |
    | Typography  | N        | N       | ±N    |
    | Spacing     | N        | N       | ±N    |

  Element Changes Table (if any):
    | Role | Name | Selector | Change Type | Details |
```

Delta values are colored: positive delta (regression) = red, zero = muted, negative delta (improvement) = green.

Missing screenshots display a styled placeholder `<div>` instead of `<img>`.

**CSS additions** (appended to existing CSS variables block):
```css
.diff-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 0.75rem;
  margin: 1rem 0;
}
.diff-col { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.diff-col img { width: 100%; height: auto; display: block; }
.diff-label { padding: 0.5rem 0.75rem; font-size: 0.8rem; color: var(--text-muted); border-bottom: 1px solid var(--border); }
.diff-col--overlay .diff-label { color: var(--yellow); }
.delta-positive { color: var(--red); font-weight: 600; }
.delta-negative { color: var(--green); font-weight: 600; }
.delta-zero { color: var(--text-muted); }
.change-appeared { color: var(--green); }
.change-disappeared { color: var(--red); }
.change-moved, .change-resized { color: var(--yellow); }
.no-screenshot { display:flex; align-items:center; justify-content:center; min-height:120px; color:var(--text-muted); font-style:italic; font-size:0.85rem; }
```

---

### 6. `main.go` Changes

Add to the `switch` block:

```go
case "diff-report":
    cmd.RunDiffReport(cmdArgs)
```

Add to the usage string under "UX Evaluation":

```
  diff-report [snapshot-id]  Generate side-by-side HTML diff report vs baseline
```

Add `"--output"` handling note: `getFlagValue` already supports `--output` (it's in `cmd/util.go`'s skip list).

---

### 7. `cmd/util.go` Changes

None required — `getFlagValue`, `hasFlag`, `getArg` already cover the needed flag parsing.

---

## API Definitions

`ks diff-report` outputs JSON via `output.Success`:

```json
{
  "ok": true,
  "command": "diff-report",
  "result": {
    "path": "/abs/path/to/diff-report.html",
    "baselineID": "20260401-100000",
    "currentID": "20260404-153012",
    "urlCount": 3,
    "summary": {
      "contrastDelta": 2,
      "touchDelta": 0,
      "typographyDelta": -1,
      "spacingDelta": 0,
      "elementChanges": 5,
      "breakpointsDiffed": 12
    }
  }
}
```

On failure:
```json
{
  "ok": false,
  "command": "diff-report",
  "error": "no baseline set",
  "hint": "run: ks baseline set"
}
```

---

## Data Model Changes

No new persistent data formats are introduced by US-006. The command is read-only:
- Reads snapshots from `snapshot.Store` (US-003 format)
- Reads baselines from `BaselineManager` (US-003 format)
- Reads diff results from `diff.Compare` (US-004 pure function)
- Writes one HTML file (ephemeral output)

Default output path `.kaleidoscope/diff-report.html` is overwritten on each invocation (not timestamped, unlike `ks report`).

---

## File Layout

```
cmd/diff_report.go         — new: RunDiffReport command handler
report/diff_report.go      — new: DiffData, GenerateDiffReport, WriteDiffFile, diffHTMLTemplate
main.go                    — modified: add "diff-report" case + usage entry
```

No new packages. No new dependencies.

---

## Security Considerations

1. **Path traversal:** The `--output` flag accepts an arbitrary path. Use `filepath.Clean` and restrict to writable locations. Do not follow symlinks when writing the output file.
2. **HTML injection:** All user-controlled strings (URL, snapshot IDs, selector names, element names) must be rendered through `html/template` automatic escaping — never via `template.HTML` casts. Only base64 image data URIs use `template.URL`, matching the existing `report` package pattern.
3. **Base64 image size:** Screenshots embedded as base64 can make the HTML file large. No size limit is enforced (matching `ks report` behavior), but the spec notes this as a known trade-off for self-containment.
4. **No network access:** The command reads only from the local filesystem. No URLs are fetched. The generated HTML is fully self-contained with no external script or stylesheet references.
5. **Snapshot data integrity:** Snapshot files are trusted (local disk, written by the same tool). No cryptographic verification is required.

---

## Acceptance Criteria Mapping

| Criterion | Implementation |
|---|---|
| Generates self-contained HTML comparing latest vs baseline | `cmd/diff_report.go` loads both, builds `DiffData`, calls `WriteDiffFile` |
| Side-by-side layout: baseline (left), current (right), diff overlay (center) | `diff-row` CSS grid with three `.diff-col` divs per breakpoint |
| Audit delta tables per URL | `AuditDeltaRow` rendered as table with colored deltas |
| Element change lists with selector, type, details | `ElementChangeRow` table per URL section |
| `--output` flag; default `.kaleidoscope/diff-report.html` | `getFlagValue(args, "--output")` with fallback constant |
| Screenshots base64-embedded | `report.LoadScreenshot` called for each `BreakpointCapture.ScreenshotPath` |
| Graceful failure if no baseline or snapshots | `output.Fail` + `os.Exit(2)` before any report generation |

---

## Dependencies on Prior Stories

| Story | What US-006 needs |
|---|---|
| US-003 (Snapshot System) | `snapshot.Store`, `snapshot.BaselineManager`, `snapshot.Snapshot` struct with screenshot paths and audit summaries |
| US-004 (Diff Engine) | `diff.Compare(baseline, current) (*DiffResult, error)` returning `URLDiff`, `AuditDelta`, `ElementChange`, `PixelDiff` |

US-006 can be developed in parallel with US-003/US-004 against the interfaces defined in this spec, using stub implementations for testing.
