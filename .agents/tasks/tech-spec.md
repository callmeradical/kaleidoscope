# Tech Spec: US-006 — Side-by-Side HTML Diff Report

**Story:** As a developer, I want a visual HTML report showing baseline vs current screenshots side-by-side with diff overlays and audit delta tables, so I can quickly review what changed.

**Command:** `ks diff-report [snapshot-id] [--output path]`

**Dependencies:** US-003 (snapshot system), US-004 (diff engine)

---

## 1. Architecture Overview

`diff-report` is a pure read-and-render command. It does not launch Chrome or perform any live capture. Its data flow:

```
ks diff-report [snapshot-id] [--output path]
        │
        ▼
cmd/diff_report.go  ←─ parses flags, orchestrates
        │
        ├─── snapshot.Load(id or "latest")  →  *snapshot.Snapshot   (US-003)
        ├─── snapshot.LoadBaseline()        →  *snapshot.Snapshot   (US-003)
        ├─── diff.Compare(baseline, current) →  *diff.SnapshotDiff   (US-004)
        ├─── report.BuildDiffData(diff, ...)→  *report.DiffData
        │       └── base64-encodes all referenced PNG files
        └─── report.WriteDiffFile(dir, data) →  path string
                │
                └─── output.Success("diff-report", result)
```

All new code follows the existing conventions:
- JSON output via `output.Success` / `output.Fail`
- Exit codes: `os.Exit(0)` success, `os.Exit(2)` failure
- Self-contained HTML with base64-embedded screenshots (matching `ks report`)
- Pure Go — no external binaries, no ImageMagick

---

## 2. Assumed Data Structures from US-003 (Snapshot Package)

US-006 reads but never writes snapshot data. The spec assumes the following types exist in a `snapshot` package after US-003 is delivered:

```go
package snapshot

type Snapshot struct {
    ID         string        `json:"id"`
    CapturedAt time.Time     `json:"capturedAt"`
    CommitHash string        `json:"commitHash,omitempty"`
    URLs       []URLSnapshot `json:"urls"`
}

type URLSnapshot struct {
    URL         string               `json:"url"`
    Title       string               `json:"title"`
    Breakpoints []BreakpointSnapshot `json:"breakpoints"`
    Audit       AuditSummary         `json:"audit"`
    AXNodes     []AXNode             `json:"axNodes,omitempty"`
}

type BreakpointSnapshot struct {
    Name           string `json:"name"`
    Width          int    `json:"width"`
    Height         int    `json:"height"`
    ScreenshotPath string `json:"screenshotPath"` // absolute or relative to snapshot dir
}

type AuditSummary struct {
    ContrastViolations int `json:"contrastViolations"`
    TouchViolations    int `json:"touchViolations"`
    TypographyWarnings int `json:"typographyWarnings"`
    SpacingIssues      int `json:"spacingIssues"`
}

type AXNode struct {
    Role     string `json:"role"`
    Name     string `json:"name"`
    Selector string `json:"selector,omitempty"`
}

// Storage layout (from US-003):
//   .kaleidoscope/snapshots/<id>/snapshot.json
//   .kaleidoscope/snapshots/<id>/<url-slug>/<breakpoint>.png
//   .kaleidoscope/baselines.json  →  {"<url>": "<snapshot-id>"}

func Load(stateDir, id string) (*Snapshot, error)       // loads by ID; "latest" resolves to newest
func LoadBaseline(stateDir string) (map[string]string, error) // url → snapshot-id map
```

If US-003 delivers different field names, `cmd/diff_report.go` is the only file that needs updating.

---

## 3. Assumed Data Structures from US-004 (Diff Package)

US-006 consumes but never produces diff data. The spec assumes the following types exist in a `diff` package after US-004 is delivered:

```go
package diff

type SnapshotDiff struct {
    BaselineID string    `json:"baselineId"`
    CurrentID  string    `json:"currentId"`
    URLs       []URLDiff `json:"urls"`
}

type URLDiff struct {
    URL            string           `json:"url"`
    Breakpoints    []BreakpointDiff `json:"breakpoints"`
    AuditDelta     AuditDelta       `json:"auditDelta"`
    ElementChanges []ElementChange  `json:"elementChanges"`
}

type BreakpointDiff struct {
    Name              string  `json:"name"`
    Width             int     `json:"width"`
    Height            int     `json:"height"`
    BaselineImagePath string  `json:"baselineImagePath"`
    CurrentImagePath  string  `json:"currentImagePath"`
    DiffImagePath     string  `json:"diffImagePath,omitempty"` // empty if images identical
    DiffPercent       float64 `json:"diffPercent"`             // 0.0–100.0
}

type AuditDelta struct {
    ContrastBefore     int `json:"contrastBefore"`
    ContrastAfter      int `json:"contrastAfter"`
    TouchBefore        int `json:"touchBefore"`
    TouchAfter         int `json:"touchAfter"`
    TypographyBefore   int `json:"typographyBefore"`
    TypographyAfter    int `json:"typographyAfter"`
    SpacingBefore      int `json:"spacingBefore"`
    SpacingAfter       int `json:"spacingAfter"`
}

type ElementChange struct {
    Selector string `json:"selector"`
    Role     string `json:"role"`
    Name     string `json:"name"`
    // Type is one of: "appeared", "disappeared", "moved", "resized"
    Type     string `json:"type"`
    Details  string `json:"details"` // human-readable, e.g. "moved 12px right"
}

// Pure function — no Chrome, no I/O
func Compare(baseline, current *snapshot.Snapshot) *SnapshotDiff
```

---

## 4. New Files

### 4.1 `cmd/diff_report.go`

```go
package cmd

// RunDiffReport implements: ks diff-report [snapshot-id] [--output path]
func RunDiffReport(args []string)
```

**Logic:**

1. Parse args:
   - `snapshotID` = first non-flag arg (empty → "latest")
   - `outputPath` = `getFlagValue(args, "--output")` (empty → `""`, defaulting to `.kaleidoscope/diff-report.html`)

2. Resolve state dir via `browser.StateDir()`.

3. Load current snapshot:
   ```go
   current, err := snapshot.Load(stateDir, snapshotID)
   // err if no snapshots exist → output.Fail with hint "Run 'ks snapshot' first"
   ```

4. Load baseline map:
   ```go
   baselines, err := snapshot.LoadBaseline(stateDir)
   // err if baselines.json missing → output.Fail with hint "Run 'ks baseline set' first"
   ```

5. Determine baseline snapshot ID for each URL (or a global baseline if the map has a single entry). If no URL in `current` has a baseline entry → `output.Fail`.

6. Load baseline snapshot:
   ```go
   baselineSnap, err := snapshot.Load(stateDir, baselineID)
   ```

7. Compute diff:
   ```go
   snapshotDiff := diff.Compare(baselineSnap, current)
   ```

8. Build report data:
   ```go
   data, err := report.BuildDiffData(snapshotDiff)
   // encodes all PNGs to base64 data URIs
   ```

9. Determine output path:
   ```go
   if outputPath == "" {
       outputPath = filepath.Join(stateDir, "diff-report.html")
   }
   ```

10. Write report:
    ```go
    path, err := report.WriteDiffFile(outputPath, data)
    ```

11. Return:
    ```go
    output.Success("diff-report", map[string]any{
        "path":       path,
        "baselineId": snapshotDiff.BaselineID,
        "currentId":  snapshotDiff.CurrentID,
        "urlCount":   len(snapshotDiff.URLs),
    })
    ```

---

### 4.2 `report/diff_report.go`

New file extending the `report` package. Contains types, builder, renderer, and HTML template for diff reports.

#### Types

```go
package report

// DiffScreenshotSet holds base64-encoded images for one breakpoint comparison.
type DiffScreenshotSet struct {
    Breakpoint  string
    Width       int
    Height      int
    BaselineURI template.URL  // data:image/png;base64,...
    CurrentURI  template.URL  // data:image/png;base64,...
    DiffURI     template.URL  // empty string if images identical
    DiffPercent float64
}

// AuditDeltaRow is one row in the audit delta table.
type AuditDeltaRow struct {
    Category string // "Contrast", "Touch Targets", "Typography", "Spacing"
    Before   int
    After    int
    Delta    int    // After - Before; negative = improvement, positive = regression
}

// ElementChangeRow is one row in the element change list.
type ElementChangeRow struct {
    Selector string
    Role     string
    Name     string
    Type     string // appeared | disappeared | moved | resized
    Details  string
}

// URLDiffSection holds all diff data for one URL.
type URLDiffSection struct {
    URL            string
    ScreenshotSets []DiffScreenshotSet
    AuditDeltas    []AuditDeltaRow
    ElementChanges []ElementChangeRow
    HasRegressions bool // true if any AuditDeltaRow.Delta > 0
}

// DiffData is the top-level template model for a diff report.
type DiffData struct {
    BaselineID      string
    CurrentID       string
    GeneratedAt     time.Time
    URLs            []URLDiffSection
    TotalRegressions int   // count of URL sections with HasRegressions=true
}
```

#### Functions

```go
// BuildDiffData converts a diff.SnapshotDiff into a DiffData suitable for rendering.
// It reads and base64-encodes all referenced screenshot files.
func BuildDiffData(d *diff.SnapshotDiff) (*DiffData, error)

// GenerateDiff writes the HTML diff report to w.
func GenerateDiff(w io.Writer, data *DiffData) error

// WriteDiffFile writes the HTML diff report to the given file path.
// The directory is created if it does not exist.
// Returns the absolute path of the written file.
func WriteDiffFile(path string, data *DiffData) (string, error)
```

**`BuildDiffData` detail:**

```
for each URLDiff in d.URLs:
    section := URLDiffSection{URL: u.URL}
    for each BreakpointDiff b in u.Breakpoints:
        baseURI  = loadScreenshot(b.BaselineImagePath)  // reuse existing LoadScreenshot()
        currURI  = loadScreenshot(b.CurrentImagePath)
        diffURI  = ""
        if b.DiffImagePath != "":
            diffURI = loadScreenshot(b.DiffImagePath)
        section.ScreenshotSets = append(..., DiffScreenshotSet{...})

    section.AuditDeltas = buildAuditDeltaRows(u.AuditDelta)
    section.ElementChanges = buildElementChangeRows(u.ElementChanges)
    section.HasRegressions = anyPositiveDelta(section.AuditDeltas)
    data.URLs = append(data.URLs, section)
```

**`buildAuditDeltaRows`** produces exactly 4 rows:
| Category | Before | After | Delta |
|---|---|---|---|
| Contrast | d.ContrastBefore | d.ContrastAfter | After-Before |
| Touch Targets | … | … | … |
| Typography | … | … | … |
| Spacing | … | … | … |

---

## 5. HTML Template Design (`diffHtmlTemplate`)

The template is embedded in `report/diff_report.go` as a string constant, following the exact same approach as `htmlTemplate` in `report/report.go`.

### Layout Structure

```html
<!DOCTYPE html>
<html>
<head>
  <title>Kaleidoscope Diff Report — baseline {{.BaselineID}} → {{.CurrentID}}</title>
  <style>/* dark theme, reusing same CSS variables */</style>
</head>
<body>

<h1>Kaleidoscope Diff Report</h1>
<div class="meta">
  Baseline: <code>{{.BaselineID}}</code> →
  Current:  <code>{{.CurrentID}}</code><br>
  Generated {{.GeneratedAt | formatTime}}
  {{if .TotalRegressions}}
    · <span class="badge badge-fail">{{.TotalRegressions}} URL(s) with regressions</span>
  {{else}}
    · <span class="badge badge-pass">No regressions detected</span>
  {{end}}
</div>

{{range .URLs}}
<section class="url-section">
  <h2>{{.URL}}
    {{if .HasRegressions}}<span class="badge badge-fail">Regression</span>{{end}}
  </h2>

  {{range .ScreenshotSets}}
  <h3>{{.Breakpoint}} · {{.Width}}×{{.Height}}</h3>
  <div class="diff-grid">
    <div class="diff-col">
      <div class="diff-col-label">Baseline</div>
      <img src="{{.BaselineURI}}" alt="Baseline {{.Breakpoint}}">
    </div>
    <div class="diff-col">
      <div class="diff-col-label">
        Diff overlay
        {{if gt .DiffPercent 0.0}}
          <span class="badge badge-warn">{{printf "%.1f" .DiffPercent}}% changed</span>
        {{else}}
          <span class="badge badge-pass">Identical</span>
        {{end}}
      </div>
      {{if .DiffURI}}
        <img src="{{.DiffURI}}" alt="Pixel diff {{.Breakpoint}}">
      {{else}}
        <div class="diff-identical">No pixel differences</div>
      {{end}}
    </div>
    <div class="diff-col">
      <div class="diff-col-label">Current</div>
      <img src="{{.CurrentURI}}" alt="Current {{.Breakpoint}}">
    </div>
  </div>
  {{end}}

  <!-- Audit delta table -->
  <h3>Audit Delta</h3>
  <div class="table-wrap">
  <table>
    <thead><tr><th>Category</th><th>Before</th><th>After</th><th>Delta</th></tr></thead>
    <tbody>
    {{range .AuditDeltas}}
    <tr>
      <td>{{.Category}}</td>
      <td>{{.Before}}</td>
      <td>{{.After}}</td>
      <td>
        {{if gt .Delta 0}}
          <span class="badge badge-fail">+{{.Delta}} regression</span>
        {{else if lt .Delta 0}}
          <span class="badge badge-pass">{{.Delta}} resolved</span>
        {{else}}
          <span class="badge">—</span>
        {{end}}
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>

  <!-- Element changes -->
  {{if .ElementChanges}}
  <h3>Element Changes</h3>
  <div class="table-wrap">
  <table>
    <thead><tr><th>Selector</th><th>Role</th><th>Name</th><th>Change</th><th>Details</th></tr></thead>
    <tbody>
    {{range .ElementChanges}}
    <tr>
      <td><code>{{.Selector}}</code></td>
      <td>{{.Role}}</td>
      <td>{{.Name}}</td>
      <td>
        {{if eq .Type "appeared"}}<span class="badge badge-pass">appeared</span>
        {{else if eq .Type "disappeared"}}<span class="badge badge-fail">disappeared</span>
        {{else if eq .Type "moved"}}<span class="badge badge-warn">moved</span>
        {{else if eq .Type "resized"}}<span class="badge badge-warn">resized</span>
        {{else}}<span class="badge">{{.Type}}</span>{{end}}
      </td>
      <td>{{.Details}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{end}}

</section>
{{end}}

<footer>Generated by <strong>kaleidoscope</strong> · AI agent front-end design toolkit</footer>
</body>
</html>
```

### CSS Additions (appended to existing variables)

```css
/* Diff-specific layout */
.url-section {
  margin: 2.5rem 0;
  padding-top: 1rem;
  border-top: 2px solid var(--border);
}

.diff-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 0.75rem;
  margin: 0.75rem 0 1.5rem;
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
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.diff-identical {
  padding: 2rem;
  text-align: center;
  color: var(--text-muted);
  font-style: italic;
}
```

---

## 6. Changes to Existing Files

### `main.go`

Add one case to the `switch` block:

```go
case "diff-report":
    cmd.RunDiffReport(cmdArgs)
```

Also add `diff-report` to the `usage` string under `UX Evaluation:`:

```
  diff-report [snap-id]   Side-by-side HTML diff report vs baseline
```

### `cmd/usage.go`

Add an entry to `CommandUsage`:

```go
"diff-report": `ks diff-report [snapshot-id] [--output path]

Generate a self-contained HTML report comparing a snapshot against its baseline.
Shows side-by-side screenshots (baseline / diff overlay / current) per URL and
breakpoint, with audit delta tables and element change lists.

Arguments:
  snapshot-id   ID of snapshot to compare (default: latest)

Options:
  --output path   Output file path (default: .kaleidoscope/diff-report.html)

Examples:
  ks diff-report
  ks diff-report snap-20240403-142301
  ks diff-report --output /tmp/my-report.html`,
```

---

## 7. File Summary

| File | Type | Description |
|---|---|---|
| `cmd/diff_report.go` | **New** | `RunDiffReport` CLI handler |
| `report/diff_report.go` | **New** | `DiffData` types, `BuildDiffData`, `GenerateDiff`, `WriteDiffFile`, HTML template |
| `main.go` | **Modified** | Add `case "diff-report"` routing + usage string entry |
| `cmd/usage.go` | **Modified** | Add `"diff-report"` usage documentation |

---

## 8. Error Handling

| Condition | Behavior |
|---|---|
| No snapshots exist | `output.Fail("diff-report", "no snapshots found", "Run 'ks snapshot' first")` |
| No baseline set | `output.Fail("diff-report", "no baseline configured", "Run 'ks baseline set' first")` |
| Snapshot ID not found | `output.Fail("diff-report", "snapshot '<id>' not found", "Run 'ks snapshot list' to see available snapshots")` |
| Screenshot file missing | `output.Fail(...)` with the underlying file path in the error message |
| Output directory not writable | `output.Fail(...)` propagating the `os.MkdirAll` error |

---

## 9. Security Considerations

| Risk | Mitigation |
|---|---|
| **Path traversal via snapshot ID** | Validate snapshot ID with `regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`).MatchString(id)` before using in file path construction; reject invalid IDs |
| **Path traversal via `--output`** | Accept arbitrary paths (by design for power users), but use `filepath.Clean` and ensure no directory traversal reaches unintended locations; document that this is user-controlled |
| **XSS in HTML report** | All template data rendered through `html/template` (not `text/template`), which auto-escapes HTML entities; data URIs for images are constructed from local files only |
| **Arbitrary file read** | Screenshot paths come from snapshot metadata written by `ks snapshot` (trusted local process); no user-controlled path input in image loading code path |
| **File permissions** | Report file written with default `0644` (no execute bit); created via `os.Create` which uses umask |
| **Template injection** | Snapshot IDs and URLs rendered in HTML as text nodes via `html/template` auto-escaping, not injected into raw attributes without quoting |

---

## 10. Acceptance Criteria Mapping

| AC | Implementation |
|---|---|
| `ks diff-report` generates a self-contained HTML file | `report.WriteDiffFile` encodes all images as base64 data URIs |
| Side-by-side layout per URL per breakpoint | `diff-grid` CSS grid with 3 columns; baseline / diff / current |
| Audit delta tables below each URL section | `AuditDeltaRow` slice in `URLDiffSection`, rendered as `<table>` |
| Element change lists with selector, type, details | `ElementChangeRow` slice in `URLDiffSection`, rendered as `<table>` |
| `--output` controls path; default `.kaleidoscope/diff-report.html` | Parsed in `RunDiffReport`; fallback to `filepath.Join(stateDir, "diff-report.html")` |
| Screenshots are base64-embedded | `BuildDiffData` calls `report.LoadScreenshot` for each image path |
| Graceful failure if no baseline or no snapshots | Explicit error checks with `output.Fail` and actionable hints |

---

## 11. Open Questions (deferred to implementation)

1. **Snapshot ID resolution** — if a `baselines.json` maps per-URL (url → snapshot-id) rather than globally, `RunDiffReport` must join across multiple baseline snapshots. The spec above assumes a single baseline snapshot ID for simplicity; implementation should verify the US-003 schema.

2. **Missing DiffImagePath** — if US-004's diff engine writes a pixel diff PNG only when differences exceed a threshold, `DiffURI` will be empty and the center column shows a "no differences" placeholder. Confirm threshold behavior with US-004.

3. **Snapshot deduplication (open question from PRD)** — does not affect US-006 report rendering; screenshot paths from metadata are used as-is regardless of deduplication.
