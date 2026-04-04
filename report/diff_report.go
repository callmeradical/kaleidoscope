package report

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ElementChangeRow is one row in the element changes table of the diff report.
type ElementChangeRow struct {
	Role     string
	Name     string
	Selector string
	// Type is one of: "appeared", "disappeared", "moved", "resized"
	Type    string
	Details string
}

// AuditDeltaRow holds before/after/delta values for display in the diff report.
type AuditDeltaRow struct {
	ContrastBefore   int
	ContrastAfter    int
	ContrastDelta    int
	TouchBefore      int
	TouchAfter       int
	TouchDelta       int
	TypographyBefore int
	TypographyAfter  int
	TypographyDelta  int
	SpacingBefore    int
	SpacingAfter     int
	SpacingDelta     int
}

// BreakpointDiffRow holds display data for one breakpoint comparison.
type BreakpointDiffRow struct {
	Name           string
	Width          int
	Height         int
	BaselineURI    template.URL
	CurrentURI     template.URL
	DiffOverlayURI template.URL
	DiffPercent    float64
	HasDiff        bool
}

// URLDiffSection holds all display data for one URL in the diff report.
type URLDiffSection struct {
	URL            string
	Breakpoints    []BreakpointDiffRow
	AuditDelta     AuditDeltaRow
	ElementChanges []ElementChangeRow
}

// DiffData holds all data needed to render the HTML diff report.
type DiffData struct {
	BaselineID  string
	CurrentID   string
	GeneratedAt time.Time
	URLs        []URLDiffSection
}

var diffFuncMap = template.FuncMap{
	// deltaClass returns a CSS class name based on the sign of delta.
	// Positive delta = regression (bad) → delta-positive (red).
	// Negative delta = improvement (good) → delta-negative (green).
	"deltaClass": func(delta int) string {
		if delta > 0 {
			return "delta-positive"
		} else if delta < 0 {
			return "delta-negative"
		}
		return "delta-zero"
	},
	// signedDelta formats an integer delta with an explicit sign.
	// Returns template.HTML to prevent html/template from escaping the '+' character.
	"signedDelta": func(delta int) template.HTML {
		if delta > 0 {
			return template.HTML(fmt.Sprintf("+%d", delta))
		} else if delta < 0 {
			return template.HTML(fmt.Sprintf("%d", delta))
		}
		return "0"
	},
}

// GenerateDiffReport writes the HTML diff report to w.
func GenerateDiffReport(w io.Writer, data *DiffData) error {
	tmpl, err := template.New("diff-report").Funcs(diffFuncMap).Parse(diffHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parsing diff template: %w", err)
	}
	return tmpl.Execute(w, data)
}

// WriteDiffFile generates the diff report and writes it to the given path.
// The parent directory is created if it does not exist.
// Returns the absolute path of the written file.
func WriteDiffFile(path string, data *DiffData) (string, error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := GenerateDiffReport(f, data); err != nil {
		_ = f.Close()
		os.Remove(path)
		return "", err
	}

	abs, _ := filepath.Abs(path)
	return abs, nil
}

// diffHTMLTemplate is the self-contained HTML template for the diff report.
var diffHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Diff Report — {{.BaselineID}} vs {{.CurrentID}}</title>
<style>
:root {
  --bg: #0f1117;
  --surface: #1a1d27;
  --border: #2a2d3a;
  --text: #e2e8f0;
  --text-muted: #64748b;
  --accent: #6366f1;
  --green: #22c55e;
  --red: #ef4444;
  --yellow: #eab308;
}
*, *::before, *::after { box-sizing: border-box; }
body { background: var(--bg); color: var(--text); font-family: system-ui, sans-serif; margin: 0; padding: 1.5rem; line-height: 1.5; }
h1 { font-size: 1.5rem; margin-bottom: 0.25rem; }
h2 { font-size: 1.15rem; margin: 2rem 0 0.5rem; border-bottom: 1px solid var(--border); padding-bottom: 0.25rem; }
h3 { font-size: 1rem; margin: 1.25rem 0 0.5rem; color: var(--text-muted); }
p { margin: 0.25rem 0; }
table { width: 100%; border-collapse: collapse; margin: 0.75rem 0; font-size: 0.9rem; }
th, td { text-align: left; padding: 0.4rem 0.6rem; border: 1px solid var(--border); }
th { background: var(--surface); color: var(--text-muted); font-weight: 600; }

.diff-row { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 0.75rem; margin: 1rem 0; }
.diff-col { background: var(--surface); border: 1px solid var(--border); border-radius: 6px; padding: 0.75rem; }
.diff-col img { width: 100%; height: auto; display: block; border-radius: 4px; }
.diff-label { font-size: 0.8rem; color: var(--text-muted); margin-bottom: 0.5rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; }
.diff-col--overlay .diff-label { color: var(--yellow); }

.delta-positive { color: var(--red); font-weight: 600; }
.delta-negative { color: var(--green); font-weight: 600; }
.delta-zero { color: var(--text-muted); }

.no-screenshot { display: flex; align-items: center; justify-content: center; min-height: 120px; color: var(--text-muted); font-style: italic; font-size: 0.85rem; border: 1px dashed var(--border); border-radius: 4px; }

.meta { color: var(--text-muted); font-size: 0.85rem; margin-bottom: 1.5rem; }
</style>
</head>
<body>
<h1>Diff Report</h1>
<p class="meta">
  Baseline: <strong>{{.BaselineID}}</strong> &rarr;
  Current: <strong>{{.CurrentID}}</strong> &mdash;
  Generated: {{.GeneratedAt.Format "2006-01-02 15:04:05 UTC"}}
</p>

{{range .URLs}}
<h2>{{.URL}}</h2>

{{range .Breakpoints}}
<h3>{{.Name}} &mdash; {{.Width}}x{{.Height}}</h3>
<div class="diff-row">
  <div class="diff-col diff-col--baseline">
    <div class="diff-label">Baseline ({{$.BaselineID}})</div>
    {{if .BaselineURI}}<img src="{{.BaselineURI}}" alt="Baseline screenshot">{{else}}<div class="no-screenshot">No baseline screenshot</div>{{end}}
  </div>
  {{if .HasDiff}}
  <div class="diff-col diff-col--overlay">
    <div class="diff-label">Diff ({{printf "%.1f" .DiffPercent}}% changed)</div>
    {{if .DiffOverlayURI}}<img src="{{.DiffOverlayURI}}" alt="Diff overlay">{{else}}<div class="no-screenshot">No visual diff overlay</div>{{end}}
  </div>
  {{end}}
  <div class="diff-col diff-col--current">
    <div class="diff-label">Current ({{$.CurrentID}})</div>
    {{if .CurrentURI}}<img src="{{.CurrentURI}}" alt="Current screenshot">{{else}}<div class="no-screenshot">No current screenshot</div>{{end}}
  </div>
</div>
{{end}}

<table>
  <thead>
    <tr><th>Category</th><th>Before</th><th>After</th><th>Delta</th></tr>
  </thead>
  <tbody>
    <tr>
      <td>Contrast</td>
      <td>{{.AuditDelta.ContrastBefore}}</td>
      <td>{{.AuditDelta.ContrastAfter}}</td>
      <td class="{{deltaClass .AuditDelta.ContrastDelta}}">{{signedDelta .AuditDelta.ContrastDelta}}</td>
    </tr>
    <tr>
      <td>Touch</td>
      <td>{{.AuditDelta.TouchBefore}}</td>
      <td>{{.AuditDelta.TouchAfter}}</td>
      <td class="{{deltaClass .AuditDelta.TouchDelta}}">{{signedDelta .AuditDelta.TouchDelta}}</td>
    </tr>
    <tr>
      <td>Typography</td>
      <td>{{.AuditDelta.TypographyBefore}}</td>
      <td>{{.AuditDelta.TypographyAfter}}</td>
      <td class="{{deltaClass .AuditDelta.TypographyDelta}}">{{signedDelta .AuditDelta.TypographyDelta}}</td>
    </tr>
    <tr>
      <td>Spacing</td>
      <td>{{.AuditDelta.SpacingBefore}}</td>
      <td>{{.AuditDelta.SpacingAfter}}</td>
      <td class="{{deltaClass .AuditDelta.SpacingDelta}}">{{signedDelta .AuditDelta.SpacingDelta}}</td>
    </tr>
  </tbody>
</table>

{{if .ElementChanges}}
<style>
.change-appeared { color: var(--green); font-weight: 600; }
.change-disappeared { color: var(--red); font-weight: 600; }
.change-moved { color: var(--yellow); font-weight: 600; }
.change-resized { color: var(--yellow); font-weight: 600; }
</style>
<table>
  <thead>
    <tr><th>Role</th><th>Name</th><th>Selector</th><th>Change Type</th><th>Details</th></tr>
  </thead>
  <tbody>
    {{range .ElementChanges}}
    <tr>
      <td>{{.Role}}</td>
      <td>{{.Name}}</td>
      <td>{{.Selector}}</td>
      <td class="change-{{.Type}}">{{.Type}}</td>
      <td>{{.Details}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
{{end}}

{{end}}
</body>
</html>
`
