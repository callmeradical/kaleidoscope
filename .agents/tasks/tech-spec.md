# Tech Spec: US-006 — Side-by-Side HTML Diff Report

## Overview

`ks diff-report [snapshot-id] [--output path]` generates a self-contained HTML file
comparing a baseline snapshot against the latest (or specified) snapshot. The report
embeds base64-encoded screenshots, pixel-diff overlays, per-URL audit delta tables, and
element change lists. It requires no external dependencies — pure Go, no Chrome needed.

US-006 depends on:
- **US-003** — snapshot capture system (stores audit + screenshots + ax-tree per run)
- **US-004** — diff engine (pure-function comparison of two snapshots)

This spec covers all three layers (snapshot model, diff engine, diff report) because
US-006 cannot be implemented without agreeing on the interfaces they expose.

---

## 1. Architecture Overview

```
cmd/diff_report.go          CLI entry point: ks diff-report
    |
    +--> snapshot.Load()    Reads two snapshots from disk
    |
    +--> diff.Compare()     Pure-function diff (no Chrome)
    |
    +--> diff.PixelDiff()   Generates overlay images in-process
    |
    +--> report.GenerateDiff()   Renders HTML using new template
    |
    +--> output.Success()   Emits JSON result
```

**New packages / files:**

| Path | Purpose |
|---|---|
| `snapshot/snapshot.go` | Snapshot data model, load/save, index |
| `snapshot/store.go` | File I/O for snapshot directory layout |
| `diff/diff.go` | Pure-function audit + element diff engine |
| `diff/pixel.go` | Pure-Go pixel diff using `image/png` |
| `report/diff_report.go` | Diff-specific `Data` struct + `GenerateDiff()` |
| `report/diff_template.go` | Separate HTML template constant for diff report |
| `cmd/diff_report.go` | `RunDiffReport(args []string)` |
| `main.go` (edit) | Route `diff-report` to `cmd.RunDiffReport` |

---

## 2. Data Model

### 2.1 Snapshot (`snapshot/snapshot.go`)

```go
// Snapshot is the root record written at the end of `ks snapshot`.
type Snapshot struct {
    ID          string             `json:"id"`           // e.g. "20260404-153012-abc1234"
    CreatedAt   time.Time          `json:"createdAt"`
    CommitSHA   string             `json:"commitSha,omitempty"`
    CommitMsg   string             `json:"commitMsg,omitempty"`
    ProjectFile string             `json:"projectFile"`  // path to .ks-project.json used
    Pages       []PageSnapshot     `json:"pages"`
}

// PageSnapshot holds all captured data for one URL.
type PageSnapshot struct {
    URL         string             `json:"url"`
    Title       string             `json:"title"`
    Breakpoints []BreakpointCapture `json:"breakpoints"`
    AuditResult AuditResult        `json:"audit"`
    AXNodes     []AXNodeRecord     `json:"axNodes"`
}

// BreakpointCapture links a breakpoint label to its screenshot file (relative to snapshot dir).
type BreakpointCapture struct {
    Name        string `json:"name"`          // "mobile", "tablet", "desktop", "wide"
    Width       int    `json:"width"`
    Height      int    `json:"height"`
    ScreenshotFile string `json:"screenshotFile"` // relative path within snapshot dir
}

// AuditResult mirrors the per-URL summary already produced by `ks audit`.
type AuditResult struct {
    ContrastViolations int              `json:"contrastViolations"`
    TouchViolations    int              `json:"touchViolations"`
    TypographyWarnings int              `json:"typographyWarnings"`
    SpacingIssues      int              `json:"spacingIssues"`
    ContrastIssues     []ContrastRecord `json:"contrastIssues"`
    TouchIssues        []TouchRecord    `json:"touchIssues"`
    TypographyIssues   []TypoRecord     `json:"typographyIssues"`
    SpacingIssues_     []SpacingRecord  `json:"spacingIssueList"`
}

// AXNodeRecord is the semantic identity used for element-level diffing.
type AXNodeRecord struct {
    Role     string `json:"role"`
    Name     string `json:"name"`
    Selector string `json:"selector,omitempty"`
    Bounds   Rect   `json:"bounds,omitempty"`
    Ignored  bool   `json:"ignored"`
}

type Rect struct {
    X, Y, Width, Height float64
}
```

**Concrete sub-types** (`ContrastRecord`, `TouchRecord`, `TypoRecord`, `SpacingRecord`) mirror
the existing `report.ContrastIssue` etc. but live in the `snapshot` package so the diff
engine has no dependency on the report package.

### 2.2 Snapshot Storage Layout

```
.kaleidoscope/
  snapshots/
    <snapshot-id>/
      snapshot.json          # Snapshot struct (no screenshot bytes)
      mobile-<url-hash>.png
      tablet-<url-hash>.png
      desktop-<url-hash>.png
      wide-<url-hash>.png
  baselines.json             # committed to repo
```

`baselines.json`:
```json
{
  "default": "20260404-153012-abc1234"
}
```

The key `"default"` is used when no project-specific key is set. Future multi-URL
projects can use the URL or project name as key. Only the `"default"` key is required
for US-006.

### 2.3 Diff Result (`diff/diff.go`)

```go
// SnapshotDiff is the output of Compare(); pure data, no I/O.
type SnapshotDiff struct {
    BaselineID  string       `json:"baselineId"`
    CurrentID   string       `json:"currentId"`
    Pages       []PageDiff   `json:"pages"`
}

// PageDiff holds all diff data for one URL.
type PageDiff struct {
    URL             string           `json:"url"`
    AuditDelta      []CategoryDelta  `json:"auditDelta"`
    ElementChanges  []ElementChange  `json:"elementChanges"`
    BreakpointDiffs []BreakpointDiff `json:"breakpointDiffs"`
}

// CategoryDelta describes before/after/delta for one audit category.
type CategoryDelta struct {
    Category string `json:"category"` // "contrast", "touch", "typography", "spacing"
    Before   int    `json:"before"`
    After    int    `json:"after"`
    Delta    int    `json:"delta"` // positive = regression, negative = improvement
}

// ElementChange describes a single DOM element that appeared, disappeared, moved, or resized.
type ElementChange struct {
    Role     string `json:"role"`
    Name     string `json:"name"`
    Selector string `json:"selector,omitempty"`
    Type     string `json:"type"` // "appeared" | "disappeared" | "moved" | "resized"
    Details  string `json:"details"` // human-readable: "moved 12px right", "resized 320x40 -> 320x48"
}

// BreakpointDiff links a breakpoint to the two screenshot paths and the pixel diff result.
type BreakpointDiff struct {
    Name           string  `json:"name"`
    BaselinePath   string  `json:"baselinePath"`   // absolute path on disk
    CurrentPath    string  `json:"currentPath"`    // absolute path on disk
    DiffScore      float64 `json:"diffScore"`      // 0.0 = identical, 1.0 = completely different
    DiffImageBytes []byte  `json:"-"`              // in-memory PNG; not serialised to JSON
}
```

---

## 3. Component Design

### 3.1 Snapshot Package (`snapshot/`)

**`snapshot.Load(dir, id string) (*Snapshot, error)`**
- Reads `<dir>/snapshots/<id>/snapshot.json`
- Returns error if directory or file not found

**`snapshot.Latest(dir string) (*Snapshot, error)`**
- Lists `<dir>/snapshots/`, sorts by `CreatedAt` descending, returns first

**`snapshot.LoadBaseline(dir string) (*Snapshot, error)`**
- Reads `<dir>/baselines.json`, extracts `"default"` ID, calls `Load`

**`snapshot.ScreenshotPath(dir, snapshotID, filename string) string`**
- Returns absolute path: `<dir>/snapshots/<snapshotID>/<filename>`

### 3.2 Diff Engine (`diff/diff.go`)

**`diff.Compare(baseline, current *snapshot.Snapshot) (*SnapshotDiff, error)`**
- Iterates pages by URL; matches pages between snapshots by URL string equality
- For each matched page, calls:
  - `compareAudit(base, cur snapshot.AuditResult) []CategoryDelta`
  - `compareElements(base, cur []snapshot.AXNodeRecord) []ElementChange`

**`compareAudit`** — pure arithmetic:
- Produces four `CategoryDelta` rows (contrast, touch, typography, spacing)
- Delta = After - Before

**`compareElements`** — identity-based matching:
- Match nodes by `(Role, Name)` tuple
- Unmatched in baseline → `"disappeared"`
- Unmatched in current → `"appeared"`
- Matched with changed `Bounds` (threshold: >2px) → `"moved"` or `"resized"`
  - `"resized"` if Width or Height changed; `"moved"` if only X/Y changed

### 3.3 Pixel Diff Engine (`diff/pixel.go`)

**`diff.PixelDiff(baselinePath, currentPath string) (score float64, overlayPNG []byte, err error)`**

Algorithm (pure Go, `image` + `image/png`):
1. Decode both PNGs via `image/png`
2. Resize smaller image to match larger dimensions (nearest-neighbour, in-place)
3. Walk every pixel: compute Euclidean distance in RGB space
4. Pixels with distance > threshold (default 10/255) are "different"
5. `score = differentPixels / totalPixels`
6. Build overlay image:
   - Copy current image as base
   - Overlay different pixels with a semi-transparent red (RGBA `255, 0, 0, 128`)
7. Encode overlay to PNG bytes, return

### 3.4 Diff Report (`report/diff_report.go` + `report/diff_template.go`)

**New data type:**

```go
// DiffData holds everything the diff HTML template needs.
type DiffData struct {
    GeneratedAt    time.Time
    BaselineID     string
    CurrentID      string
    BaselineCommit string
    CurrentCommit  string
    Pages          []DiffPageData
}

// DiffPageData is the per-URL view model.
type DiffPageData struct {
    URL             string
    Breakpoints     []DiffBreakpointData
    AuditDelta      []DiffCategoryRow
    ElementChanges  []DiffElementRow
}

// DiffBreakpointData holds the three base64 images for one breakpoint.
type DiffBreakpointData struct {
    Name            string
    Width           int
    Height          int
    BaselineDataURI template.URL // base64 PNG
    CurrentDataURI  template.URL // base64 PNG
    DiffDataURI     template.URL // base64 PNG overlay; empty string if images are identical
    DiffScore       float64      // 0-1; displayed as percentage
}

// DiffCategoryRow is one row of the audit delta table.
type DiffCategoryRow struct {
    Category string
    Before   int
    After    int
    Delta    int
    Trend    string // "better" | "worse" | "same"
}

// DiffElementRow is one row of the element change list.
type DiffElementRow struct {
    Role     string
    Name     string
    Selector string
    Type     string // "appeared" | "disappeared" | "moved" | "resized"
    Details  string
}
```

**`report.BuildDiffData(d *diff.SnapshotDiff, baseline, current *snapshot.Snapshot) (*DiffData, error)`**
- Assembles the view model from the raw diff result
- For each breakpoint diff, calls `report.LoadScreenshotBytes(path)` and base64-encodes
- Encodes `DiffImageBytes` overlay as base64 data URI
- Returns `*DiffData`

**`report.GenerateDiff(w io.Writer, data *DiffData) error`**
- Parses and executes `diffHTMLTemplate` constant (separate from existing `htmlTemplate`)

**`report.WriteDiffFile(dir string, data *DiffData) (string, error)`**
- Mirror of existing `WriteFile`; filename pattern: `diff-report-<unix-ms>.html`

### 3.5 CLI Command (`cmd/diff_report.go`)

```go
func RunDiffReport(args []string) {
    // 1. Parse flags
    snapshotID := positionalArg(args, 0)  // optional
    outputPath := getFlagValue(args, "--output")

    // 2. Resolve state dir
    dir, err := browser.StateDir()

    // 3. Load snapshots (no browser required)
    baseline, err := snapshot.LoadBaseline(dir)
    // → output.Fail and os.Exit(2) if not found

    var current *snapshot.Snapshot
    if snapshotID != "" {
        current, err = snapshot.Load(dir, snapshotID)
    } else {
        current, err = snapshot.Latest(dir)
    }
    // → output.Fail and os.Exit(2) if not found

    // 4. Run diff engine
    d, err := diff.Compare(baseline, current)

    // 5. Run pixel diff for each breakpoint
    for pi := range d.Pages {
        for bi := range d.Pages[pi].BreakpointDiffs {
            bd := &d.Pages[pi].BreakpointDiffs[bi]
            bd.DiffScore, bd.DiffImageBytes, err = diff.PixelDiff(bd.BaselinePath, bd.CurrentPath)
            // non-fatal: log warning, continue with empty overlay
        }
    }

    // 6. Build view model
    data, err := report.BuildDiffData(d, baseline, current)

    // 7. Write HTML
    reportPath := writeDiffReport(outputPath, dir, data)

    // 8. Emit result
    output.Success("diff-report", map[string]any{
        "path":        reportPath,
        "baselineId":  baseline.ID,
        "currentId":   current.ID,
        "pages":       len(d.Pages),
        "regressions": countRegressions(d),
    })
}
```

Error conditions producing `output.Fail` + `os.Exit(2)`:
- No `baselines.json` found or `"default"` key missing
- No snapshots directory or no snapshot files present
- Specified `snapshot-id` directory not found

---

## 4. HTML Template Design (`report/diff_template.go`)

The template reuses the same CSS variables and dark-theme design as `htmlTemplate`.
New layout elements:

### 4.1 Per-URL Section Structure

```
<section class="page-section">
  <h2>[URL]</h2>

  <!-- Per-breakpoint row -->
  <div class="diff-breakpoint">
    <h3>desktop · 1280×720</h3>
    <div class="diff-triptych">
      <figure class="diff-pane diff-baseline">
        <img src="data:image/png;base64,...">
        <figcaption>Baseline</figcaption>
      </figure>
      <figure class="diff-pane diff-overlay">
        <img src="data:image/png;base64,...">  <!-- pixel diff overlay -->
        <figcaption>Diff (4.2% changed)</figcaption>
      </figure>
      <figure class="diff-pane diff-current">
        <img src="data:image/png;base64,...">
        <figcaption>Current</figcaption>
      </figure>
    </div>
  </div>

  <!-- Audit delta table (below breakpoints) -->
  <h3>Audit Delta</h3>
  <table class="audit-delta">...</table>

  <!-- Element changes -->
  <h3>Element Changes</h3>
  <table class="element-changes">...</table>
</section>
```

### 4.2 Audit Delta Table Columns

| Category | Before | After | Delta | Trend |
|---|---|---|---|---|
| contrast | 3 | 5 | +2 | worse (red badge) |
| touch | 1 | 0 | -1 | better (green badge) |

Delta cell color: red if positive, green if negative, muted if zero.

### 4.3 Element Changes Table Columns

| Role | Name | Type | Details |
|---|---|---|---|
| button | "Submit" | appeared | — |
| heading | "Welcome" | moved | 12px down |
| img | "Logo" | resized | 120×40 → 180×40 |

Type badge colors: `appeared` = green, `disappeared` = red, `moved`/`resized` = yellow.

### 4.4 Empty States

- No breakpoint images found for a page: show muted "No screenshots captured"
- Empty element changes: show "No element changes detected"
- Empty audit delta with all-zero deltas: show "No audit changes detected"

### 4.5 Report Header

```html
<h1>Kaleidoscope Diff Report</h1>
<div class="meta">
  Baseline: <code>{{.BaselineID}}</code>{{if .BaselineCommit}} ({{.BaselineCommit}}){{end}}<br>
  Current:  <code>{{.CurrentID}}</code>{{if .CurrentCommit}} ({{.CurrentCommit}}){{end}}<br>
  Generated {{.GeneratedAt.Format "Jan 2, 2006 at 3:04 PM"}}
</div>
```

---

## 5. API / CLI Definition

### Command Signature

```
ks diff-report [snapshot-id] [--output <path>]
```

| Argument | Required | Description |
|---|---|---|
| `snapshot-id` | No | Compare baseline vs this snapshot. Defaults to latest snapshot. |
| `--output <path>` | No | Output file path. Default: `.kaleidoscope/diff-report.html` |

### Output (JSON)

```json
{
  "ok": true,
  "command": "diff-report",
  "result": {
    "path": "/abs/path/to/diff-report.html",
    "baselineId": "20260401-090000-deadbeef",
    "currentId":  "20260404-153012-abc1234",
    "pages": 2,
    "regressions": 3
  }
}
```

`regressions` = count of `CategoryDelta` entries where `Delta > 0` across all pages.

### Error Cases

```json
{"ok":false,"command":"diff-report","error":"no baseline set","hint":"Run: ks baseline set"}
{"ok":false,"command":"diff-report","error":"no snapshots found","hint":"Run: ks snapshot to capture one"}
{"ok":false,"command":"diff-report","error":"snapshot abc123 not found","hint":"Run: ks snapshot list to see available IDs"}
```

### `main.go` addition

```go
case "diff-report":
    cmd.RunDiffReport(cmdArgs)
```

---

## 6. File Layout Summary

```
snapshot/
  snapshot.go      # Snapshot, PageSnapshot, BreakpointCapture, AuditResult, AXNodeRecord, Rect
  store.go         # Load, Latest, LoadBaseline, ScreenshotPath, index helpers

diff/
  diff.go          # SnapshotDiff, PageDiff, CategoryDelta, ElementChange, BreakpointDiff; Compare()
  pixel.go         # PixelDiff() — pure image/png pixel comparison + overlay generation

report/
  diff_report.go   # DiffData, DiffPageData, DiffBreakpointData, DiffCategoryRow, DiffElementRow
                   # BuildDiffData(), GenerateDiff(), WriteDiffFile()
  diff_template.go # diffHTMLTemplate string constant

cmd/
  diff_report.go   # RunDiffReport(args []string)

main.go            # add "diff-report" case
```

No changes to existing files except `main.go` (one `case` added).

---

## 7. Security Considerations

**HTML injection in report data**
- All user-controlled strings (URLs, element names, selectors, commit messages) are
  inserted via `{{.Field}}` in `html/template`, which auto-escapes HTML entities.
- `DataURI` fields must use `template.URL` (as the existing report already does) to
  suppress spurious escaping of the `data:` scheme. No other `template.URL` casts
  should be added.

**File path traversal**
- `snapshot-id` is a positional argument that becomes part of a file path.
- `snapshot.Load` must validate that the resolved path is under the snapshots directory
  before opening (use `filepath.Clean` + `strings.HasPrefix` check).

**Local-only data**
- Snapshots and reports are written only to the local `.kaleidoscope/` directory.
- No network calls are made by the diff-report command.

**PNG decoding**
- `image/png` decoding is safe for local files written by `ks snapshot`.
- No user-supplied remote images are decoded, so SSRF/zip-bomb risk is nil.

---

## 8. Quality Gates

- `go test ./...` must pass (existing gate)
- New packages (`snapshot`, `diff`) must have unit tests covering:
  - `Compare` with identical snapshots → all deltas zero, no element changes
  - `Compare` with a new contrast violation → delta +1
  - `compareElements` with an appeared node → one `"appeared"` change
  - `PixelDiff` with identical images → score 0.0
  - `PixelDiff` with fully red image vs fully blue → score ~1.0
  - `snapshot.Load` with a path traversal id (`../etc`) → error returned
- `report.GenerateDiff` must execute without template errors on a zero-value `DiffData`
- `RunDiffReport` with missing baseline must exit with code 2 and `ok: false` JSON

---

## 9. Open Questions (from PRD)

1. **Deduplication**: identical screenshots across snapshots could be hard-linked rather
   than copied. Out of scope for US-006 but the `BreakpointCapture.ScreenshotFile` field
   is a relative path, so deduplication could be added transparently in US-003 without
   changing the US-006 data model.

2. **Arbitrary snapshot comparison**: `ks diff-report <id-a> <id-b>` would require
   accepting two positional args. The current design uses `[snapshot-id]` as "current"
   and always compares to baseline. Supporting two arbitrary IDs is a backwards-compatible
   extension (add second positional + `--baseline` flag).
