# Tech Spec: US-006 — Side-by-Side HTML Diff Report

## Overview

Implements `ks diff-report [snapshot-id] [--output path]`, which generates a self-contained HTML
report comparing a baseline snapshot against a current (or specified) snapshot. The report shows
screenshots side-by-side with a pixel diff overlay, per-URL audit delta tables, and element change
lists. All images are base64-embedded for portability.

This feature depends on:
- **US-003** — Snapshot system (`snapshot` package): stores audit results + screenshots per run
- **US-004** — Diff engine (`diff` package): pure-function comparator that produces structured
  deltas from two snapshots

---

## Architecture Overview

```
main.go
  └─ "diff-report" → cmd.RunDiffReport(args)
                          │
                          ├─ snapshot.LoadBaseline()       [US-003]
                          ├─ snapshot.LoadLatest() / Load(id)   [US-003]
                          ├─ diff.Compare(baseline, current)     [US-004]
                          ├─ pixeldiff.Overlay(img1, img2)       [new – diff/pixel.go]
                          └─ diffreport.Generate(w, data)        [new – report/diff_report.go]
```

No Chrome/browser dependency. The diff report is generated entirely from stored snapshot data.

---

## Assumed Interfaces from US-003 and US-004

These types and functions are defined by their respective stories. US-006 consumes them as-is.

### snapshot package (US-003)

```go
// Snapshot is a point-in-time capture of one or more URLs.
type Snapshot struct {
    ID        string         `json:"id"`         // e.g. "20240404-143022-abc1"
    CreatedAt time.Time      `json:"createdAt"`
    CommitHash string        `json:"commitHash,omitempty"`
    Pages     []PageSnapshot `json:"pages"`
}

// PageSnapshot holds all audit data and screenshot paths for one URL.
type PageSnapshot struct {
    URL         string               `json:"url"`
    Screenshots []BreakpointShot     `json:"screenshots"`
    AuditResult AuditResult          `json:"auditResult"`
    AXNodes     []AXNode             `json:"axNodes"`
}

// BreakpointShot records a single viewport screenshot (path relative to snapshot dir).
type BreakpointShot struct {
    Breakpoint string `json:"breakpoint"` // "mobile" | "tablet" | "desktop" | "wide"
    Width      int    `json:"width"`
    Height     int    `json:"height"`
    Path       string `json:"path"` // absolute or relative path to PNG
}

// AuditResult mirrors the per-category counts and detailed issue lists.
type AuditResult struct {
    ContrastViolations int             `json:"contrastViolations"`
    TouchViolations    int             `json:"touchViolations"`
    TypographyWarnings int             `json:"typographyWarnings"`
    SpacingIssues      int             `json:"spacingIssues"`
    // Detailed lists omitted here (used by diff engine, not report directly)
}

// AXNode is a simplified accessibility node (role + name for semantic identity).
type AXNode struct {
    NodeID   string `json:"nodeId"`
    Role     string `json:"role"`
    Name     string `json:"name"`
    Children []string `json:"children"`
}

// Baseline references the snapshot currently marked as regression baseline.
type Baseline struct {
    SnapshotID string    `json:"snapshotId"`
    MarkedAt   time.Time `json:"markedAt"`
}

// LoadLatest returns the most recently created snapshot from .kaleidoscope/snapshots/.
func LoadLatest() (*Snapshot, error)

// Load returns the snapshot with the given ID.
func Load(id string) (*Snapshot, error)

// LoadBaseline reads .kaleidoscope/baselines.json and returns the baseline snapshot.
func LoadBaseline() (*Snapshot, error)
```

### diff package (US-004)

```go
// Result is the output of the diff engine.
type Result struct {
    BaselineID string     `json:"baselineId"`
    CurrentID  string     `json:"currentId"`
    Pages      []PageDiff `json:"pages"`
}

// PageDiff holds all changes detected for a single URL.
type PageDiff struct {
    URL            string          `json:"url"`
    AuditDelta     AuditDelta      `json:"auditDelta"`
    ElementChanges []ElementChange `json:"elementChanges"`
}

// AuditDelta is per-category before/after/delta counts.
type AuditDelta struct {
    Contrast   CategoryDelta `json:"contrast"`
    Touch      CategoryDelta `json:"touch"`
    Typography CategoryDelta `json:"typography"`
    Spacing    CategoryDelta `json:"spacing"`
}

type CategoryDelta struct {
    Before int `json:"before"`
    After  int `json:"after"`
    Delta  int `json:"delta"` // After - Before; positive = regression
}

// ElementChange describes one semantic element that appeared, disappeared, moved, or resized.
type ElementChange struct {
    Selector   string            `json:"selector"`   // CSS selector or AX identity "role[name]"
    ChangeType string            `json:"changeType"` // "appeared" | "disappeared" | "moved" | "resized"
    Details    map[string]any    `json:"details"`    // e.g. {from: {x,y,w,h}, to: {x,y,w,h}}
}

// Compare is a pure function: two snapshots in, a Result out.
func Compare(baseline, current *snapshot.Snapshot) (*Result, error)
```

---

## New Components

### 1. `diff/pixel.go` — Pixel Diff Overlay Generator

**Purpose:** Compare two PNG images pixel-by-pixel and produce a highlighted difference image.
Uses only Go stdlib `image`, `image/color`, `image/png`, `image/draw`.

**API:**

```go
package pixeldiff

import "image"

// Options configures the pixel diff.
type Options struct {
    // Threshold is the per-channel difference (0–255) below which pixels are considered identical.
    // Default: 10
    Threshold uint8
    // HighlightColor is the RGBA color used to mark differing pixels. Default: red (255,0,0,255).
    HighlightColor color.RGBA
    // DimOpacity controls how much identical pixels are dimmed (0.0 = no dim, 1.0 = black).
    // Default: 0.5
    DimOpacity float64
}

// Overlay compares img1 and img2 and returns a diff image of the same dimensions as img1.
// Pixels that differ beyond Threshold are highlighted. Identical pixels are dimmed.
// If the images have different dimensions, img2 is treated as blank for the differing region.
func Overlay(img1, img2 image.Image, opts *Options) image.Image

// Score returns the proportion of pixels that are identical (0.0–1.0).
// Used to populate DiffScore in the report.
func Score(img1, img2 image.Image, threshold uint8) float64
```

**Algorithm:**
1. Determine output bounds: `max(img1.Bounds(), img2.Bounds())`
2. For each pixel (x, y):
   - Read RGBA from both images (treat out-of-bounds as (0,0,0,0))
   - Compute per-channel absolute difference
   - If max channel delta > Threshold → paint `HighlightColor`
   - Else → paint dimmed version of img1 pixel at `DimOpacity`
3. Encode output as PNG into a `bytes.Buffer`, return the image

### 2. `report/diff_report.go` — Diff Report Data Model and HTML Template

**Purpose:** Holds the `DiffData` struct and the embedded HTML template for the diff report.
Mirrors the pattern of `report/report.go` but with a 3-column layout and delta tables.

**Data Model:**

```go
package report

// DiffData holds all data required to render the HTML diff report.
type DiffData struct {
    GeneratedAt  time.Time
    BaselineID   string
    CurrentID    string
    BaselineAt   time.Time
    CurrentAt    time.Time

    Pages []DiffPage
}

// DiffPage is the data for one URL across all breakpoints.
type DiffPage struct {
    URL         string
    Breakpoints []DiffBreakpoint

    // Audit delta table data
    AuditDelta AuditDelta // re-exported from diff package or duplicated struct

    // Element change list
    ElementChanges []ElementChangeRow
}

// DiffBreakpoint holds screenshot data for one viewport comparison.
type DiffBreakpoint struct {
    Name     string       // "mobile" | "tablet" | "desktop" | "wide"
    Width    int
    Height   int

    BaselineURI template.URL // base64 data URI or empty if no baseline shot
    CurrentURI  template.URL // base64 data URI or empty if no current shot
    OverlayURI  template.URL // base64 data URI of pixel diff image; empty if either image missing
    DiffScore   float64      // 0.0–1.0 similarity; 1.0 = identical
}

// AuditDelta mirrors diff.AuditDelta (to avoid import cycle with report package).
type AuditDelta struct {
    Contrast   CategoryDelta
    Touch      CategoryDelta
    Typography CategoryDelta
    Spacing    CategoryDelta
}

type CategoryDelta struct {
    Before int
    After  int
    Delta  int
}

// ElementChangeRow is a flattened row for the HTML change list table.
type ElementChangeRow struct {
    Selector   string
    ChangeType string
    Details    string // human-readable summary of the Details map
}
```

**Functions:**

```go
// GenerateDiff writes the diff HTML report to w.
func GenerateDiff(w io.Writer, data *DiffData) error

// WriteDiffFile writes the diff report to path, creating parent directories as needed.
// Returns the absolute path of the written file.
func WriteDiffFile(path string, data *DiffData) (string, error)
```

### 3. `cmd/diff_report.go` — Command Implementation

**File:** `/workspace/cmd/diff_report.go`

**Signature:** `func RunDiffReport(args []string)`

**Logic:**

```
1. Parse args:
   - snapshotID = getArg(args)           // optional positional; if empty → load latest
   - outputPath = getFlagValue(args, "--output")  // defaults to .kaleidoscope/diff-report.html

2. Load baseline snapshot:
   baseline, err := snapshot.LoadBaseline()
   → if err: output.Fail with "no baseline set; run: ks snapshot baseline <id>"

3. Load current snapshot:
   if snapshotID != "":
       current, err = snapshot.Load(snapshotID)
   else:
       current, err = snapshot.LoadLatest()
   → if err: output.Fail with "no snapshots found; run: ks snapshot"

4. Guard: if baseline.ID == current.ID → output.Fail "baseline and current are the same snapshot"

5. Compute diff:
   diffResult, err := diff.Compare(baseline, current)

6. Build DiffData:
   For each PageDiff in diffResult.Pages:
     a. Find matching PageSnapshot in baseline.Pages and current.Pages by URL
     b. For each breakpoint name in ["mobile","tablet","desktop","wide"]:
        - Load baseline PNG at breakpoint (if exists) → decode image.Image
        - Load current PNG at breakpoint (if exists) → decode image.Image
        - If both exist: compute Overlay + Score
        - Encode overlay PNG → base64 data URI
        - Encode baseline/current PNGs → base64 data URIs (report.LoadScreenshot helper)
        - Append DiffBreakpoint
     c. Map diff.AuditDelta → report.AuditDelta
     d. Map diff.ElementChanges → []report.ElementChangeRow (Details map → human string)

7. Determine output path:
   if outputPath == "": use stateDir + "/diff-report.html"
   Ensure parent dir exists

8. WriteDiffFile(outputPath, &diffData)

9. output.Success("diff-report", map[string]any{
       "path": absPath,
       "baselineId": baseline.ID,
       "currentId": current.ID,
       "pages": len(diffData.Pages),
   })
```

### 4. Main Router Update

Add to `main.go` switch:

```go
case "diff-report":
    cmd.RunDiffReport(cmdArgs)
```

Add to usage string under "UX Evaluation":

```
  diff-report [snapshot-id]  Side-by-side HTML diff vs baseline
```

Add to `cmd/usage.go`:

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

## HTML Template Design

The diff report template extends the existing dark-theme CSS variables from `report/report.go`.

### Layout Structure

```
<h1>Kaleidoscope Diff Report</h1>
<meta: baseline-id | current-id | generated-at>

For each Page:
  <h2>URL</h2>

  For each Breakpoint:
    <h3>mobile — 375×812</h3>
    <div class="diff-row">
      <div class="diff-col">
        <label>Baseline</label>
        <img src="{{baselineURI}}">
      </div>
      <div class="diff-col">
        <label>Diff Overlay  (similarity: 97.3%)</label>
        <img src="{{overlayURI}}">
      </div>
      <div class="diff-col">
        <label>Current</label>
        <img src="{{currentURI}}">
      </div>
    </div>

  <h3>Audit Delta</h3>
  <table> Contrast | Touch | Typography | Spacing — Before / After / Delta (±N) </table>

  <h3>Element Changes</h3>
  <table> Selector | Type | Details </table>
```

### CSS Additions (inline in template)

```css
.diff-row {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 0.75rem;
    margin: 1rem 0;
}
.diff-col {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
}
.diff-col img {
    width: 100%;
    height: auto;
    display: block;
}
.diff-col-label {
    padding: 0.4rem 0.75rem;
    font-size: 0.75rem;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border);
}
.diff-score-good  { color: var(--green); }
.diff-score-warn  { color: var(--yellow); }
.diff-score-bad   { color: var(--red); }

.delta-positive { color: var(--red); }   /* regression: more issues */
.delta-negative { color: var(--green); } /* improvement: fewer issues */
.delta-zero     { color: var(--text-muted); }
```

### Diff Score Thresholds

| DiffScore | CSS class | Meaning |
|-----------|-----------|---------|
| ≥ 0.99 | `diff-score-good` | Visually identical |
| 0.95–0.99 | `diff-score-warn` | Minor differences |
| < 0.95 | `diff-score-bad` | Significant visual change |

### Delta Badge Convention

- Delta > 0 (more issues) → red badge `+N`
- Delta < 0 (fewer issues) → green badge `−N`
- Delta == 0 → muted `0`

### Missing Screenshot Handling

If a breakpoint image is missing on one side, display a placeholder card:

```html
<div class="diff-col empty-state">No screenshot at this breakpoint</div>
```

OverlayURI is empty → omit the center column entirely if either image is missing.

---

## Data Model Changes

No persistent data model changes. The command reads existing snapshot/baseline files and writes a
single output HTML file. The default output path is:

```
.kaleidoscope/diff-report.html   (when --local state exists)
~/.kaleidoscope/diff-report.html  (fallback)
```

This file is overwritten on each run (not timestamped). Add to `.gitignore`:

```
.kaleidoscope/diff-report.html
```

---

## File Layout

```
/workspace/
├── cmd/
│   └── diff_report.go          # NEW: RunDiffReport()
├── diff/
│   └── pixel.go                # NEW: Overlay(), Score()
├── report/
│   ├── report.go               # UNCHANGED
│   └── diff_report.go          # NEW: DiffData, GenerateDiff(), WriteDiffFile()
└── main.go                     # MODIFIED: add "diff-report" case
```

---

## Security Considerations

1. **Path traversal:** The `--output` flag value must be treated as a user-supplied path.
   Use `filepath.Clean` before creating the file. Do not allow writes outside the workspace.

2. **base64 embedding:** Screenshots are read from local disk, not fetched from the network.
   No SSRF risk. Existing `report.LoadScreenshot` pattern is safe.

3. **HTML template injection:** All user-supplied strings (URL, snapshot ID, selector, change
   details) are passed via `html/template`, which auto-escapes. Do not use `template.HTML()`
   for any user-controlled strings.

4. **Image decoding:** PNG files are decoded with `image/png` from local snapshot paths.
   Malformed PNGs return an error; the command fails gracefully rather than panicking.

5. **No network calls:** The command is entirely local. No HTTP requests are made.

---

## Error Handling Matrix

| Condition | Behavior |
|-----------|----------|
| No `.kaleidoscope/baselines.json` | `output.Fail` with hint to run `ks snapshot baseline` |
| Baseline snapshot directory missing | `output.Fail` with hint |
| No snapshots in `.kaleidoscope/snapshots/` | `output.Fail` with hint to run `ks snapshot` |
| Specified snapshot ID not found | `output.Fail` with "snapshot not found: <id>" |
| Baseline == current | `output.Fail` with "no diff: baseline and current are the same snapshot" |
| Missing screenshot for a breakpoint | Skip overlay for that breakpoint; log in report |
| Malformed PNG | Skip that breakpoint's overlay; continue |
| Cannot create output file | `output.Fail` with underlying OS error |

---

## Testing

Quality gate: `go test ./...`

### Unit Tests

| File | Test |
|------|------|
| `diff/pixel_test.go` | `TestOverlay_identical` — two equal images → score 1.0, all pixels dimmed |
| `diff/pixel_test.go` | `TestOverlay_fullyDifferent` — solid red vs solid blue → score 0.0, all pixels highlighted |
| `diff/pixel_test.go` | `TestOverlay_differentSizes` — img2 taller → extra rows highlighted |
| `diff/pixel_test.go` | `TestScore_threshold` — pixel delta at boundary of Threshold |
| `report/diff_report_test.go` | `TestGenerateDiff_smoke` — builds minimal DiffData, calls GenerateDiff, checks non-empty HTML output without error |
| `report/diff_report_test.go` | `TestGenerateDiff_missingImages` — OverlayURI="" → renders placeholder, no template error |

### Integration Behavior (manual)

1. `ks snapshot` → capture snapshot A
2. Modify page
3. `ks snapshot` → capture snapshot B
4. `ks snapshot baseline <A-id>`
5. `ks diff-report` → produces `.kaleidoscope/diff-report.html`
6. Open in browser: verify 3-column layout, delta tables, element change list

---

## Open Questions (deferred)

- Should diff-report accept two explicit snapshot IDs (`ks diff-report <id-a> <id-b>`) for
  arbitrary comparison? (PRD open question; not in scope for US-006.)
- Should the overlay image be stored to disk as a cached artifact, or always regenerated?
  Current spec: regenerated on every run (no caching), simplest approach.
