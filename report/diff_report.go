package report

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"
)

// DiffData holds all data needed to render the HTML diff report.
type DiffData struct {
	GeneratedAt time.Time
	BaselineID  string
	CurrentID   string
	BaselineAt  time.Time
	CurrentAt   time.Time
	Pages       []DiffPage
}

// DiffPage holds diff information for a single URL.
type DiffPage struct {
	URL            string
	Breakpoints    []DiffBreakpoint
	AuditDelta     AuditDelta
	ElementChanges []ElementChangeRow
}

// DiffBreakpoint holds the three images and diff score for one breakpoint.
type DiffBreakpoint struct {
	Name        string
	Width       int
	Height      int
	BaselineURI template.URL
	CurrentURI  template.URL
	OverlayURI  template.URL
	DiffScore   float64
}

// AuditDelta contains per-category before/after/delta counts.
type AuditDelta struct {
	Contrast   CategoryDelta
	Touch      CategoryDelta
	Typography CategoryDelta
	Spacing    CategoryDelta
}

// CategoryDelta holds before/after/delta counts for one audit category.
type CategoryDelta struct {
	Before int
	After  int
	Delta  int
}

// ElementChangeRow represents a single element-level change.
type ElementChangeRow struct {
	Selector   string
	ChangeType string
	Details    string
}

var diffFuncMap = template.FuncMap{
	"deltaClass": func(n int) string {
		if n > 0 {
			return "delta-positive"
		}
		if n < 0 {
			return "delta-negative"
		}
		return "delta-zero"
	},
	"deltaSign": func(n int) string {
		if n > 0 {
			return fmt.Sprintf("+%d", n)
		}
		if n < 0 {
			return fmt.Sprintf("−%d", -n)
		}
		return "0"
	},
	"scoreClass": func(f float64) string {
		if f >= 0.99 {
			return "diff-score-good"
		}
		if f >= 0.95 {
			return "diff-score-warn"
		}
		return "diff-score-bad"
	},
	"pct": func(f float64) string {
		return fmt.Sprintf("%.1f%%", f*100)
	},
}

const diffReportTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Kaleidoscope Diff Report</title>
  <style>
    :root {
      --bg: #0d1117;
      --surface: #161b22;
      --border: #30363d;
      --text: #c9d1d9;
      --muted: #8b949e;
      --green: #3fb950;
      --red: #f85149;
      --yellow: #d29922;
      --blue: #58a6ff;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; font-size: 14px; padding: 24px; }
    h1 { font-size: 24px; margin-bottom: 8px; color: var(--blue); }
    h2 { font-size: 18px; margin: 32px 0 12px; border-bottom: 1px solid var(--border); padding-bottom: 8px; }
    h3 { font-size: 14px; margin: 20px 0 8px; color: var(--muted); }
    .meta { color: var(--muted); font-size: 12px; margin-bottom: 24px; }
    .meta span { margin-right: 16px; }
    .diff-row { display: flex; gap: 12px; margin: 12px 0; }
    .diff-col { flex: 1; border: 1px solid var(--border); border-radius: 6px; overflow: hidden; background: var(--surface); }
    .diff-col-label { padding: 6px 10px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); border-bottom: 1px solid var(--border); }
    .diff-col img { width: 100%; display: block; }
    .empty-state { padding: 24px; text-align: center; color: var(--muted); font-size: 12px; }
    .diff-score-good { color: var(--green); }
    .diff-score-warn { color: var(--yellow); }
    .diff-score-bad { color: var(--red); }
    table { width: 100%; border-collapse: collapse; margin: 8px 0 16px; }
    th, td { padding: 6px 10px; text-align: left; border-bottom: 1px solid var(--border); font-size: 12px; }
    th { color: var(--muted); font-weight: 600; }
    .delta-positive { color: var(--red); font-weight: 600; }
    .delta-negative { color: var(--green); font-weight: 600; }
    .delta-zero { color: var(--muted); }
  </style>
</head>
<body>
  <h1>Kaleidoscope Diff Report</h1>
  <div class="meta">
    <span>Baseline: <strong>{{.BaselineID}}</strong></span>
    <span>Current: <strong>{{.CurrentID}}</strong></span>
    <span>Generated: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</span>
  </div>

  {{range .Pages}}
  <h2>{{.URL}}</h2>

  {{range .Breakpoints}}
  <h3>{{.Name}} — {{.Width}}×{{.Height}}</h3>
  <div class="diff-row">
    <div class="diff-col">
      <div class="diff-col-label">Baseline</div>
      {{if .BaselineURI}}<img src="{{.BaselineURI}}" alt="Baseline screenshot">
      {{else}}<div class="empty-state">No screenshot at this breakpoint</div>{{end}}
    </div>
    <div class="diff-col">
      <div class="diff-col-label">Diff Overlay (similarity: <span class="{{scoreClass .DiffScore}}">{{pct .DiffScore}}</span>)</div>
      {{if .OverlayURI}}<img src="{{.OverlayURI}}" alt="Diff overlay">
      {{else}}<div class="empty-state">No screenshot at this breakpoint</div>{{end}}
    </div>
    <div class="diff-col">
      <div class="diff-col-label">Current</div>
      {{if .CurrentURI}}<img src="{{.CurrentURI}}" alt="Current screenshot">
      {{else}}<div class="empty-state">No screenshot at this breakpoint</div>{{end}}
    </div>
  </div>
  {{end}}

  <h3>Audit Delta</h3>
  <table>
    <thead><tr><th>Category</th><th>Before</th><th>After</th><th>Delta</th></tr></thead>
    <tbody>
      <tr>
        <td>Contrast</td>
        <td>{{.AuditDelta.Contrast.Before}}</td>
        <td>{{.AuditDelta.Contrast.After}}</td>
        <td><span class="{{deltaClass .AuditDelta.Contrast.Delta}}">{{deltaSign .AuditDelta.Contrast.Delta}}</span></td>
      </tr>
      <tr>
        <td>Touch</td>
        <td>{{.AuditDelta.Touch.Before}}</td>
        <td>{{.AuditDelta.Touch.After}}</td>
        <td><span class="{{deltaClass .AuditDelta.Touch.Delta}}">{{deltaSign .AuditDelta.Touch.Delta}}</span></td>
      </tr>
      <tr>
        <td>Typography</td>
        <td>{{.AuditDelta.Typography.Before}}</td>
        <td>{{.AuditDelta.Typography.After}}</td>
        <td><span class="{{deltaClass .AuditDelta.Typography.Delta}}">{{deltaSign .AuditDelta.Typography.Delta}}</span></td>
      </tr>
      <tr>
        <td>Spacing</td>
        <td>{{.AuditDelta.Spacing.Before}}</td>
        <td>{{.AuditDelta.Spacing.After}}</td>
        <td><span class="{{deltaClass .AuditDelta.Spacing.Delta}}">{{deltaSign .AuditDelta.Spacing.Delta}}</span></td>
      </tr>
    </tbody>
  </table>

  <h3>Element Changes</h3>
  {{if .ElementChanges}}
  <table>
    <thead><tr><th>Selector</th><th>Type</th><th>Details</th></tr></thead>
    <tbody>
      {{range .ElementChanges}}
      <tr>
        <td>{{.Selector}}</td>
        <td>{{.ChangeType}}</td>
        <td>{{.Details}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{else}}
  <p style="color:var(--muted);font-size:12px;">No element changes detected.</p>
  {{end}}

  {{end}}
</body>
</html>
`

// GenerateDiff writes the diff HTML report to the given writer.
func GenerateDiff(w io.Writer, data *DiffData) error {
	tmpl, err := template.New("diff-report").Funcs(diffFuncMap).Parse(diffReportTmpl)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

// WriteDiffFile generates the diff report and writes it to a file.
// Returns the absolute path of the written file.
func WriteDiffFile(path string, data *DiffData) (string, error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := GenerateDiff(f, data); err != nil {
		os.Remove(path)
		return "", err
	}
	return filepath.Abs(path)
}
