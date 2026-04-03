package report

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/callmeradical/kaleidoscope/diff"
)

// DiffScreenshotSet holds the three images (baseline, current, diff overlay) for one breakpoint.
type DiffScreenshotSet struct {
	Breakpoint  string
	Width       int
	Height      int
	BaselineURI template.URL
	CurrentURI  template.URL
	DiffURI     template.URL
	DiffPercent float64
}

// AuditDeltaRow holds a single row of the per-category audit delta table.
type AuditDeltaRow struct {
	Category string
	Before   int
	After    int
	Delta    int
}

// ElementChangeRow holds a single row of the element change list.
type ElementChangeRow struct {
	Selector string
	Role     string
	Name     string
	Type     string
	Details  string
}

// URLDiffSection holds all diff data for a single URL.
type URLDiffSection struct {
	URL            string
	ScreenshotSets []DiffScreenshotSet
	AuditDeltas    []AuditDeltaRow
	ElementChanges []ElementChangeRow
	HasRegressions bool
}

// DiffData holds all data needed to render the HTML diff report.
type DiffData struct {
	BaselineID       string
	CurrentID        string
	GeneratedAt      time.Time
	URLs             []URLDiffSection
	TotalRegressions int
}

// buildAuditDeltaRows converts an AuditDelta into exactly 4 display rows.
func buildAuditDeltaRows(d diff.AuditDelta) []AuditDeltaRow {
	return []AuditDeltaRow{
		{Category: "Contrast", Before: d.ContrastBefore, After: d.ContrastAfter, Delta: d.ContrastAfter - d.ContrastBefore},
		{Category: "Touch Targets", Before: d.TouchBefore, After: d.TouchAfter, Delta: d.TouchAfter - d.TouchBefore},
		{Category: "Typography", Before: d.TypographyBefore, After: d.TypographyAfter, Delta: d.TypographyAfter - d.TypographyBefore},
		{Category: "Spacing", Before: d.SpacingBefore, After: d.SpacingAfter, Delta: d.SpacingAfter - d.SpacingBefore},
	}
}

// buildElementChangeRows maps diff.ElementChange slice to display rows.
func buildElementChangeRows(changes []diff.ElementChange) []ElementChangeRow {
	rows := make([]ElementChangeRow, len(changes))
	for i, c := range changes {
		rows[i] = ElementChangeRow{
			Selector: c.Selector,
			Role:     c.Role,
			Name:     c.Name,
			Type:     c.Type,
			Details:  c.Details,
		}
	}
	return rows
}

// anyPositiveDelta returns true if any audit delta row has a positive (regression) delta.
func anyPositiveDelta(rows []AuditDeltaRow) bool {
	for _, r := range rows {
		if r.Delta > 0 {
			return true
		}
	}
	return false
}

// BuildDiffData assembles a DiffData from a SnapshotDiff, base64-embedding all screenshots.
func BuildDiffData(d *diff.SnapshotDiff) (*DiffData, error) {
	data := DiffData{
		BaselineID:  d.BaselineID,
		CurrentID:   d.CurrentID,
		GeneratedAt: time.Now(),
	}

	for _, u := range d.URLs {
		section := URLDiffSection{URL: u.URL}

		for _, b := range u.Breakpoints {
			baseURI, err := LoadScreenshot(b.BaselineImagePath)
			if err != nil {
				return nil, fmt.Errorf("loading baseline screenshot %s: %w", b.BaselineImagePath, err)
			}
			currURI, err := LoadScreenshot(b.CurrentImagePath)
			if err != nil {
				return nil, fmt.Errorf("loading current screenshot %s: %w", b.CurrentImagePath, err)
			}

			var diffURI template.URL
			if b.DiffImagePath != "" {
				diffURI, err = LoadScreenshot(b.DiffImagePath)
				if err != nil {
					return nil, fmt.Errorf("loading diff image %s: %w", b.DiffImagePath, err)
				}
			}

			section.ScreenshotSets = append(section.ScreenshotSets, DiffScreenshotSet{
				Breakpoint:  b.Name,
				Width:       b.Width,
				Height:      b.Height,
				BaselineURI: baseURI,
				CurrentURI:  currURI,
				DiffURI:     diffURI,
				DiffPercent: b.DiffPercent,
			})
		}

		section.AuditDeltas = buildAuditDeltaRows(u.AuditDelta)
		section.ElementChanges = buildElementChangeRows(u.ElementChanges)
		section.HasRegressions = anyPositiveDelta(section.AuditDeltas)
		data.URLs = append(data.URLs, section)
	}

	for _, s := range data.URLs {
		if s.HasRegressions {
			data.TotalRegressions++
		}
	}

	return &data, nil
}

// GenerateDiff executes the diff HTML template against data, writing to w.
func GenerateDiff(w io.Writer, data *DiffData) error {
	funcMap := template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.UTC().Format("2006-01-02 15:04:05 UTC")
		},
	}
	tmpl, err := template.New("diff-report").Funcs(funcMap).Parse(diffHtmlTemplate)
	if err != nil {
		return fmt.Errorf("parsing diff template: %w", err)
	}
	return tmpl.Execute(w, data)
}

// WriteDiffFile writes the diff report to the given path and returns the absolute path.
func WriteDiffFile(path string, data *DiffData) (string, error) {
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}
	f, err := os.Create(cleanPath)
	if err != nil {
		return "", fmt.Errorf("creating report file: %w", err)
	}
	defer f.Close()
	if err := GenerateDiff(f, data); err != nil {
		return "", fmt.Errorf("generating diff report: %w", err)
	}
	abs, err := filepath.Abs(cleanPath)
	if err != nil {
		return cleanPath, nil
	}
	return abs, nil
}

var diffHtmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kaleidoscope Diff Report — baseline {{.BaselineID}} → {{.CurrentID}}</title>
<style>
  :root {
    --bg: #0f1117;
    --surface: #1a1d27;
    --surface2: #242836;
    --border: #2e3345;
    --text: #e2e4e9;
    --text-muted: #8b8fa3;
    --accent: #6c72ff;
    --accent-dim: rgba(108,114,255,0.15);
    --green: #34d399;
    --green-dim: rgba(52,211,153,0.15);
    --red: #f87171;
    --red-dim: rgba(248,113,113,0.15);
    --yellow: #fbbf24;
    --yellow-dim: rgba(251,191,36,0.15);
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: var(--bg);
    color: var(--text);
    line-height: 1.6;
    padding: 2rem;
    max-width: 1400px;
    margin: 0 auto;
  }
  h1 { font-size: 1.75rem; font-weight: 700; margin-bottom: 0.25rem; }
  h2 {
    font-size: 1.15rem;
    font-weight: 600;
    margin: 2.5rem 0 1rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid var(--border);
  }
  h3 { font-size: 0.95rem; font-weight: 600; margin: 1.25rem 0 0.5rem; }
  .meta { color: var(--text-muted); font-size: 0.85rem; margin-bottom: 2rem; }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    margin: 0.75rem 0;
  }
  th {
    text-align: left;
    padding: 0.5rem 0.75rem;
    border-bottom: 2px solid var(--border);
    color: var(--text-muted);
    font-weight: 600;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  td {
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid var(--border);
    vertical-align: top;
  }
  tr:last-child td { border-bottom: none; }
  .table-wrap {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }
  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .badge-pass { background: var(--green-dim); color: var(--green); }
  .badge-fail { background: var(--red-dim); color: var(--red); }
  .badge-warn { background: var(--yellow-dim); color: var(--yellow); }
  code {
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.8rem;
    background: var(--surface2);
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
  }
  .url-section {
    margin: 2rem 0;
    padding: 1.5rem;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
  }
  .diff-grid {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 1rem;
    margin: 1rem 0;
  }
  .diff-col {
    background: var(--surface2);
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
    font-size: 0.78rem;
    color: var(--text-muted);
    border-top: 1px solid var(--border);
    text-align: center;
  }
  .diff-identical {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 120px;
    color: var(--text-muted);
    font-size: 0.85rem;
    font-style: italic;
  }
  footer {
    margin-top: 3rem;
    padding-top: 1.5rem;
    border-top: 1px solid var(--border);
    color: var(--text-muted);
    font-size: 0.75rem;
    text-align: center;
  }
</style>
</head>
<body>

<h1>Kaleidoscope Diff Report</h1>
<div class="meta">
  Baseline <code>{{.BaselineID}}</code> → Current <code>{{.CurrentID}}</code><br>
  Generated {{.GeneratedAt | formatTime}} ·
  {{if .TotalRegressions}}
    <span class="badge badge-fail">{{.TotalRegressions}} URL(s) with regressions</span>
  {{else}}
    <span class="badge badge-pass">No regressions</span>
  {{end}}
</div>

{{range .URLs}}
<div class="url-section">
  <h2>{{.URL}}{{if .HasRegressions}} <span class="badge badge-fail">regression</span>{{end}}</h2>

  {{range .ScreenshotSets}}
  <h3>{{.Breakpoint}} — {{.Width}}×{{.Height}}</h3>
  <div class="diff-grid">
    <div class="diff-col">
      {{if .BaselineURI}}<img src="{{.BaselineURI}}" alt="Baseline at {{.Breakpoint}}">{{end}}
      <div class="diff-col-label">Baseline</div>
    </div>
    <div class="diff-col">
      {{if .DiffURI}}
        <img src="{{.DiffURI}}" alt="Diff overlay at {{.Breakpoint}}">
        <div class="diff-col-label">Diff <span class="badge badge-warn">{{printf "%.2f" .DiffPercent}}% changed</span></div>
      {{else}}
        <div class="diff-identical">Identical</div>
        <div class="diff-col-label">Diff</div>
      {{end}}
    </div>
    <div class="diff-col">
      {{if .CurrentURI}}<img src="{{.CurrentURI}}" alt="Current at {{.Breakpoint}}">{{end}}
      <div class="diff-col-label">Current</div>
    </div>
  </div>
  {{end}}

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
        {{if gt .Delta 0}}<span class="badge badge-fail">+{{.Delta}} regression</span>
        {{else if lt .Delta 0}}<span class="badge badge-pass">{{.Delta}} resolved</span>
        {{else}}—{{end}}
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>

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
        {{else}}<span class="badge badge-warn">{{.Type}}</span>{{end}}
      </td>
      <td>{{.Details}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{end}}
</div>
{{end}}

<footer>
  Generated by <strong>kaleidoscope</strong> · AI agent front-end design toolkit
</footer>

</body>
</html>
`
