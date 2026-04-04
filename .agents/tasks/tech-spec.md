# Tech Spec: US-006 — Side-by-Side HTML Diff Report

## Overview

Implements `ks diff-report [snapshot-id] [--output path]`, a self-contained HTML report comparing
a baseline snapshot to a current (or specified) snapshot. The report displays baseline/current
screenshots side-by-side with a pixel diff overlay per breakpoint, plus per-category audit delta
tables and element change lists.

**Story dependencies:** US-003 (snapshot system) and US-004 (diff engine) must be implemented
first. This spec defines the interfaces those stories must expose for US-006 to consume.

---

## Architecture Overview

```
main.go
  └── "diff-report" → cmd.RunDiffReport(args)

cmd/diff_report.go          ← new: command handler (no Chrome dependency)
  ├── snapshot.Load(id)     ← US-003: reads snapshot from disk
  ├── snapshot.LoadBaseline ← US-003: reads baselines.json
  ├── diff.Compute(a, b)    ← US-004: pure-function diff engine
  └── diffreport.Generate() ← new: HTML template engine

snapshot/                   ← new package (US-003)
  ├── snapshot.go           ← data types + Load/Save/List
  └── baseline.go           ← baselines.json read/write

diff/                       ← new package (US-004)
  ├── diff.go               ← Compute() pure function
  └── pixeldiff.go          ← pure Go image comparison

diffreport/                 ← new package (US-006)
  ├── report.go             ← Generate(w, data), WriteFile()
  └── template.go           ← htmlTemplate string const
```

Storage layout on disk (`.kaleidoscope/` is gitignored; `baselines.json` is committed):

```
.kaleidoscope/
  snapshots/
    <snapshot-id>/
      snapshot.json
      screenshots/
        <url-slug>-<breakpoint>-<WxH>.png
  diff-report.html          ← default output path
baselines.json              ← maps URL → snapshot-id (committed)
```

---

## Detailed Component Design

### 1. `snapshot` Package (US-003 interface contract)

**File:** `snapshot/snapshot.go`

```go
package snapshot

import "time"

// Snapshot is the top-level record saved per `ks snapshot` run.
type Snapshot struct {
    ID        string      `json:"id"`         // "<YYYYMMDD-HHmmss>-<shortSHA>"
    CreatedAt time.Time   `json:"createdAt"`
    CommitSHA string      `json:"commitSha,omitempty"`
    URLs      []URLSnapshot `json:"urls"`
}

// URLSnapshot holds all data captured for one URL in one snapshot run.
type URLSnapshot struct {
    URL         string               `json:"url"`
    Breakpoints []BreakpointSnapshot `json:"breakpoints"`
    Audit       AuditResult          `json:"audit"`
    Elements    []AXElement          `json:"elements"`
}

// BreakpointSnapshot records one viewport capture.
type BreakpointSnapshot struct {
    Name           string `json:"name"`
    Width          int    `json:"width"`
    Height         int    `json:"height"`
    ScreenshotPath string `json:"screenshotPath"` // relative to snapshot dir
}

// AuditResult is the serializable form of a full UX/a11y audit.
type AuditResult struct {
    ContrastViolations int              `json:"contrastViolations"`
    TouchViolations    int              `json:"touchViolations"`
    TypographyWarnings int              `json:"typographyWarnings"`
    SpacingIssues      int              `json:"spacingIssues"`
    ContrastIssueList  []ContrastEntry  `json:"contrastIssues,omitempty"`
    TouchIssueList     []TouchEntry     `json:"touchIssues,omitempty"`
    TypographyIssueList []TypoEntry     `json:"typographyIssues,omitempty"`
    SpacingIssueList   []SpacingEntry   `json:"spacingIssueList,omitempty"`
    AXTotalNodes       int              `json:"axTotalNodes"`
    AXActiveNodes      int              `json:"axActiveNodes"`
}

type ContrastEntry  struct { Selector, Text, Foreground, Background string; Ratio float64; AA, AAA bool }
type TouchEntry     struct { Tag string; Width, Height float64; Violation string }
type TypoEntry      struct { Tag string; FontSize, LineHeight float64; FontFamily, Warning string }
type SpacingEntry   struct { Container string; Index int; Gap, Expected float64 }

// AXElement is one node from the accessibility tree (for element diffing).
type AXElement struct {
    Role     string  `json:"role"`
    Name     string  `json:"name"`
    Selector string  `json:"selector"`
    X        float64 `json:"x"`
    Y        float64 `json:"y"`
    Width    float64 `json:"width"`
    Height   float64 `json:"height"`
}
```

**File:** `snapshot/store.go` — storage helpers

```go
// SnapshotDir returns the absolute path to .kaleidoscope/snapshots/<id>/.
func SnapshotDir(id string) (string, error)

// Load reads and deserializes snapshot.json from a snapshot directory.
func Load(id string) (*Snapshot, error)

// Latest returns the most recent snapshot by creation time.
// Returns error "no snapshots found" if directory is empty.
func Latest() (*Snapshot, error)

// List returns all snapshot IDs sorted newest-first.
func List() ([]string, error)

// Save writes snapshot.json into the snapshot directory.
func Save(s *Snapshot) error
```

**File:** `snapshot/baseline.go` — baseline management

```go
// Baselines maps URL strings to snapshot IDs.
type Baselines map[string]string

// LoadBaselines reads baselines.json from the repo root (next to .ks-project.json).
// Returns empty Baselines (not error) if the file does not exist.
func LoadBaselines() (Baselines, error)

// SaveBaselines writes baselines.json atomically.
func SaveBaselines(b Baselines) error

// BaselineFor returns the snapshot ID set as baseline for a given URL,
// and a bool indicating whether one is set.
func (b Baselines) BaselineFor(url string) (string, bool)
```

---

### 2. `diff` Package (US-004 interface contract)

**File:** `diff/diff.go`

```go
package diff

import "time"

// AuditDelta summarises the change in one audit category between two snapshots.
type AuditDelta struct {
    Category string `json:"category"` // "contrast" | "touch" | "typography" | "spacing"
    Before   int    `json:"before"`
    After    int    `json:"after"`
    Delta    int    `json:"delta"` // After - Before; positive = regression
}

// ElementChange describes one accessibility-tree element that changed.
type ElementChange struct {
    Role     string `json:"role"`
    Name     string `json:"name"`
    Selector string `json:"selector"`
    Type     string `json:"type"`    // "appeared" | "disappeared" | "moved" | "resized"
    Details  string `json:"details"` // human-readable: e.g. "moved 14px right, 3px down"
}

// BreakpointDiff holds the pixel-level comparison for one viewport.
type BreakpointDiff struct {
    Name          string  `json:"name"`
    Width         int     `json:"width"`
    Height        int     `json:"height"`
    BaselinePath  string  `json:"baselinePath"`
    CurrentPath   string  `json:"currentPath"`
    DiffPNG       []byte  `json:"-"` // in-memory pixel diff image; not serialised
    DiffPercent   float64 `json:"diffPercent"`   // 0–100
    ChangedPixels int     `json:"changedPixels"`
    TotalPixels   int     `json:"totalPixels"`
}

// URLDiff is the complete diff for one URL.
type URLDiff struct {
    URL            string           `json:"url"`
    AuditDeltas    []AuditDelta     `json:"auditDeltas"`
    ElementChanges []ElementChange  `json:"elementChanges"`
    Breakpoints    []BreakpointDiff `json:"breakpoints"`
    HasRegression  bool             `json:"hasRegression"` // any audit delta > 0
}

// Result is the top-level output of Compute().
type Result struct {
    BaselineID     string    `json:"baselineId"`
    CurrentID      string    `json:"currentId"`
    GeneratedAt    time.Time `json:"generatedAt"`
    URLs           []URLDiff `json:"urls"`
    HasRegressions bool      `json:"hasRegressions"`
}

// Compute is a pure function: it takes two snapshots and returns a diff.
// No Chrome, no filesystem side effects other than reading screenshot files.
func Compute(baseline, current *snapshot.Snapshot) (*Result, error)
```

**File:** `diff/pixeldiff.go`

```go
// CompareImages decodes two PNG byte slices, compares them pixel-by-pixel,
// and returns a diff image (changed pixels highlighted in red on a faded
// background) plus statistics. Uses only stdlib image/color/image/png.
//
// If dimensions differ, the smaller image is padded with black to match.
// Similarity threshold: pixels with total RGB channel delta < threshold
// (default 10) are considered unchanged.
func CompareImages(baselinePNG, currentPNG []byte, threshold uint8) (diffPNG []byte, changed, total int, err error)
```

Algorithm (pure Go, no cgo, no external deps):
1. `image.Decode` both PNGs via `image/png`
2. Determine max bounds
3. Iterate every pixel: compute per-channel absolute delta, sum
4. If sum > threshold×3: mark pixel red (255,0,0,255) in diff image; else copy baseline pixel at 30% opacity
5. Encode diff image as PNG via `image/png`

---

### 3. `diffreport` Package (US-006 — new)

**File:** `diffreport/report.go`

```go
package diffreport

// BreakpointSection holds pre-encoded images for one breakpoint row in the report.
type BreakpointSection struct {
    Name           string
    Width          int
    Height         int
    BaselineDataURI template.URL // base64 PNG
    CurrentDataURI  template.URL // base64 PNG
    DiffDataURI     template.URL // base64 PNG (empty string if images identical)
    DiffPercent     float64
    ChangedPixels   int
    TotalPixels     int
}

// AuditDeltaRow is one row in the audit delta table.
type AuditDeltaRow struct {
    Category string
    Before   int
    After    int
    Delta    int
    IsWorse  bool // Delta > 0
    IsBetter bool // Delta < 0
}

// ElementChangeRow is one row in the element changes table.
type ElementChangeRow struct {
    Role     string
    Name     string
    Selector string
    Type     string
    Details  string
}

// URLSection groups all report content for one URL.
type URLSection struct {
    URL            string
    Breakpoints    []BreakpointSection
    AuditDeltas    []AuditDeltaRow
    ElementChanges []ElementChangeRow
    HasRegression  bool
}

// Data is the top-level template context.
type Data struct {
    BaselineID     string
    CurrentID      string
    BaselineTime   time.Time
    CurrentTime    time.Time
    BaselineSHA    string
    CurrentSHA     string
    GeneratedAt    time.Time
    URLs           []URLSection
    HasRegressions bool
}

// Generate writes the self-contained HTML report to w.
func Generate(w io.Writer, data *Data) error

// Build converts a diff.Result and the two source snapshots into Data,
// base64-encoding all screenshots and diff images in memory.
func Build(result *diff.Result, baseline, current *snapshot.Snapshot) (*Data, error)

// WriteFile writes the report to path, creating parent directories as needed.
// Returns the absolute path of the written file.
func WriteFile(path string, data *Data) (string, error)
```

---

### 4. `cmd/diff_report.go` — Command Handler

```go
package cmd

// RunDiffReport implements `ks diff-report [snapshot-id] [--output path]`.
//
// No browser/Chrome dependency. Reads from disk only.
func RunDiffReport(args []string) {
    snapshotID  := getArg(args)                                      // optional positional
    outputPath  := getFlagValue(args, "--output")
    if outputPath == "" {
        outputPath = ".kaleidoscope/diff-report.html"
    }

    // 1. Load baselines
    baselines, err := snapshot.LoadBaselines()
    // → output.Fail("diff-report", err, "Run `ks snapshot --set-baseline` first") on error

    // 2. Load current snapshot
    var current *snapshot.Snapshot
    if snapshotID != "" {
        current, err = snapshot.Load(snapshotID)
    } else {
        current, err = snapshot.Latest()
    }
    // → output.Fail("diff-report", err, "No snapshots found. Run `ks snapshot` first") on error

    // 3. Find baseline snapshot for each URL in current snapshot
    //    (baselines is a map; look up per URL or use a global "default" baseline)
    //    Collect all required baseline snapshot IDs, load unique ones.
    // → output.Fail("diff-report", ..., "No baseline set. Run `ks snapshot --set-baseline <id>`") if missing

    // 4. Compute diff
    result, err := diff.Compute(baseline, current)
    // → output.Fail("diff-report", err, "diff computation failed") on error

    // 5. Build HTML data model
    data, err := diffreport.Build(result, baseline, current)

    // 6. Write file
    path, err := diffreport.WriteFile(outputPath, data)

    // 7. Emit JSON result
    output.Success("diff-report", map[string]any{
        "path":           path,
        "baselineId":     result.BaselineID,
        "currentId":      result.CurrentID,
        "hasRegressions": result.HasRegressions,
        "urlCount":       len(result.URLs),
    })
}
```

**Registration in `main.go`:**

```go
case "diff-report":
    cmd.RunDiffReport(cmdArgs)
```

Add to the usage string under "UX Evaluation":
```
diff-report [id] [--output path]   Side-by-side HTML diff vs baseline
```

---

### 5. HTML Template Design (`diffreport/template.go`)

The template reuses the existing Kaleidoscope dark-theme CSS variables from `report/report.go`
(copy the `:root` block and base styles; do not import across packages — each HTML template is
self-contained).

**Page structure:**

```
<header>
  Kaleidoscope Diff Report
  Baseline: <id> (<commit> · <timestamp>)  →  Current: <id> (<commit> · <timestamp>)
  [regression badge if HasRegressions]
</header>

{{range .URLs}}
<section class="url-section">
  <h2>{{.URL}} [regression badge if .HasRegression]</h2>

  <!-- Audit delta summary cards (4 cards: Contrast / Touch / Typography / Spacing) -->
  <div class="delta-cards">
    {{range .AuditDeltas}}
    <div class="delta-card {{if .IsWorse}}worse{{else if .IsBetter}}better{{else}}neutral{{end}}">
      <div class="delta-label">{{.Category}}</div>
      <div class="delta-values">{{.Before}} → {{.After}}</div>
      <div class="delta-badge">{{if gt .Delta 0}}+{{end}}{{.Delta}}</div>
    </div>
    {{end}}
  </div>

  <!-- Per-breakpoint screenshot trio -->
  {{range .Breakpoints}}
  <div class="bp-section">
    <h3>{{.Name}} ({{.Width}}×{{.Height}}) — {{printf "%.2f" .DiffPercent}}% changed</h3>
    <div class="screenshot-trio">
      <div class="ss-col">
        <div class="ss-label">Baseline</div>
        <img src="{{.BaselineDataURI}}" alt="Baseline {{.Name}}">
      </div>
      <div class="ss-col">
        <div class="ss-label">Diff Overlay</div>
        {{if .DiffDataURI}}
        <img src="{{.DiffDataURI}}" alt="Pixel diff {{.Name}}">
        {{else}}
        <div class="no-diff">Identical</div>
        {{end}}
      </div>
      <div class="ss-col">
        <div class="ss-label">Current</div>
        <img src="{{.CurrentDataURI}}" alt="Current {{.Name}}">
      </div>
    </div>
  </div>
  {{end}}

  <!-- Audit delta tables (one table per category with issues) -->
  <!-- Element change list -->
</section>
{{end}}

<footer>Generated by kaleidoscope · {{.GeneratedAt.Format ...}}</footer>
```

**CSS additions** (on top of shared base styles):

```css
.screenshot-trio {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 0.75rem;
    margin: 1rem 0;
}
.ss-col { display: flex; flex-direction: column; gap: 0.5rem; }
.ss-label { font-size: 0.75rem; text-transform: uppercase; color: var(--text-muted); }
.ss-col img { width: 100%; height: auto; border-radius: 6px; border: 1px solid var(--border); }
.no-diff {
    display: flex; align-items: center; justify-content: center;
    background: var(--green-dim); color: var(--green);
    border-radius: 6px; padding: 2rem; font-size: 0.85rem;
}

.delta-cards { display: grid; grid-template-columns: repeat(4, 1fr); gap: 0.75rem; margin: 1rem 0; }
.delta-card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 0.75rem 1rem; }
.delta-card.worse  { border-color: var(--red);    background: var(--red-dim); }
.delta-card.better { border-color: var(--green);  background: var(--green-dim); }
.delta-label  { font-size: 0.7rem; text-transform: uppercase; color: var(--text-muted); }
.delta-values { font-size: 0.9rem; margin: 0.25rem 0; }
.delta-badge  { font-size: 1.25rem; font-weight: 700; }
.delta-card.worse  .delta-badge { color: var(--red); }
.delta-card.better .delta-badge { color: var(--green); }
.delta-card.neutral .delta-badge { color: var(--text-muted); }

.url-section { margin: 3rem 0; padding-top: 1.5rem; border-top: 2px solid var(--border); }
.bp-section  { margin: 1.5rem 0; }
```

---

## API Definitions

### CLI

```
ks diff-report [snapshot-id] [--output <path>]
```

| Argument      | Type     | Default                          | Description                                     |
|---------------|----------|----------------------------------|-------------------------------------------------|
| snapshot-id   | string   | latest snapshot                  | ID of the snapshot to compare against baseline  |
| --output      | string   | `.kaleidoscope/diff-report.html` | Output path for the generated HTML file         |

### JSON Output (stdout, `output.Result` convention)

**Success:**
```json
{
  "ok": true,
  "command": "diff-report",
  "result": {
    "path": "/abs/path/to/diff-report.html",
    "baselineId": "20240401-120000-a1b2c3d",
    "currentId":  "20240404-153012-e4f5a6b",
    "hasRegressions": true,
    "urlCount": 2
  }
}
```

**Failure (no baseline):**
```json
{
  "ok": false,
  "command": "diff-report",
  "error": "no baseline set for any URL",
  "hint": "Run `ks snapshot --set-baseline <id>` to set a baseline"
}
```

**Failure (no snapshots):**
```json
{
  "ok": false,
  "command": "diff-report",
  "error": "no snapshots found",
  "hint": "Run `ks snapshot` to capture the current state"
}
```

---

## Data Model Changes

### New files (committed to repo)

**`baselines.json`** — maps URL → snapshot ID. Global baseline, one per URL.

```json
{
  "https://example.com": "20240401-120000-a1b2c3d",
  "https://example.com/about": "20240401-120000-a1b2c3d"
}
```

### New files (gitignored, under `.kaleidoscope/`)

**`.kaleidoscope/snapshots/<id>/snapshot.json`:**

```json
{
  "id": "20240404-153012-e4f5a6b",
  "createdAt": "2024-04-04T15:30:12Z",
  "commitSha": "e4f5a6b",
  "urls": [
    {
      "url": "https://example.com",
      "breakpoints": [
        { "name": "mobile",  "width": 375,  "height": 812,  "screenshotPath": "screenshots/example-com-mobile-375x812.png" },
        { "name": "desktop", "width": 1280, "height": 720,  "screenshotPath": "screenshots/example-com-desktop-1280x720.png" }
      ],
      "audit": {
        "contrastViolations": 2,
        "touchViolations": 0,
        "typographyWarnings": 1,
        "spacingIssues": 0,
        "contrastIssues": [...],
        "axTotalNodes": 120,
        "axActiveNodes": 98
      },
      "elements": [
        { "role": "button", "name": "Submit", "selector": "button#submit", "x": 100, "y": 200, "width": 80, "height": 40 }
      ]
    }
  ]
}
```

**`.kaleidoscope/snapshots/<id>/screenshots/<url-slug>-<bp>-<WxH>.png`**

URL slug: replace `://`, `/`, `.` with `-`, truncate to 40 chars.

**`.kaleidoscope/diff-report.html`** — default diff report output (overwritten on each run).

### `.gitignore` additions

```
.kaleidoscope/
```

---

## Security Considerations

1. **Path traversal via snapshot-id:** The snapshot-id argument is used to construct a filesystem path (`.kaleidoscope/snapshots/<id>/`). Validate that the resolved absolute path starts with the expected snapshot root directory before opening any files. Reject IDs containing `..`, `/`, or null bytes.

2. **HTML injection in report data:** All dynamic text fields (URL, selector strings, element names, diff details) rendered into the HTML template must be passed through Go's `html/template` package, which auto-escapes by default. Never use `text/template` or `template.HTML()` for user-derived strings. The only `template.URL` values used are data URIs constructed internally (base64-encoded PNGs) — not from user input.

3. **Image size limits for pixel diff:** Large screenshots (e.g. full-page 1920×10000) decoded into `image.RGBA` buffers consume significant memory (≈76 MB per image). Set a maximum decoded dimension of 4096×8192 pixels; return an error for oversized images rather than OOM-crashing. This is validated in `diff.CompareImages()` after decode.

4. **Snapshot directory permissions:** `snapshot.Save()` creates directories with mode `0700` and files with mode `0600` since screenshots may capture authenticated page states.

5. **No shell execution:** The diff engine and report generator have no Chrome dependency and must not shell out to any external process. This eliminates an entire class of command injection risk.

---

## Implementation Checklist

The following must all be in place before US-006 passes acceptance criteria:

**US-003 (prerequisite):**
- [ ] `snapshot/` package with `Snapshot`, `URLSnapshot`, `AuditResult`, `AXElement` types
- [ ] `snapshot.Save`, `snapshot.Load`, `snapshot.Latest`, `snapshot.List`
- [ ] `snapshot.LoadBaselines`, `snapshot.SaveBaselines`
- [ ] `cmd/snapshot.go`: `ks snapshot` (capture) and `ks snapshot --set-baseline <id>`
- [ ] `main.go`: route `"snapshot"` → `cmd.RunSnapshot`

**US-004 (prerequisite):**
- [ ] `diff/` package with `Result`, `URLDiff`, `AuditDelta`, `ElementChange`, `BreakpointDiff`
- [ ] `diff.Compute(baseline, current *snapshot.Snapshot) (*Result, error)`
- [ ] `diff.CompareImages(a, b []byte, threshold uint8) ([]byte, int, int, error)` — pure Go
- [ ] Element matching by `role+name` composite key; report moved/resized/appeared/disappeared

**US-006 (this story):**
- [ ] `diffreport/` package with `Data`, `URLSection`, `BreakpointSection`, `AuditDeltaRow`, `ElementChangeRow`
- [ ] `diffreport.Build(result, baseline, current)` — loads and base64-encodes all images
- [ ] `diffreport.Generate(w, data)` — renders HTML template
- [ ] `diffreport.WriteFile(path, data)` — writes with MkdirAll
- [ ] `cmd/diff_report.go`: `RunDiffReport` with path-traversal validation
- [ ] `main.go`: route `"diff-report"` → `cmd.RunDiffReport`, update usage string
- [ ] Graceful error for missing baseline (clear hint message)
- [ ] Graceful error for no snapshots (clear hint message)
- [ ] Report uses `--output` flag; defaults to `.kaleidoscope/diff-report.html`
- [ ] All tests pass: `go test ./...`
