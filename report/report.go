package report

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Screenshot holds a breakpoint screenshot with its metadata.
type Screenshot struct {
	Breakpoint string
	Width      int
	Height     int
	Path       string
	DataURI    template.URL // base64-encoded data URI for embedding
}

// ContrastIssue represents a single contrast violation.
type ContrastIssue struct {
	Selector   string
	Text       string
	Ratio      float64
	Foreground string
	Background string
	IsLarge    bool
	AA         bool
	AAA        bool
}

// TouchIssue represents a single touch target violation.
type TouchIssue struct {
	Tag       string
	Width     float64
	Height    float64
	Violation string
}

// TypographyIssue represents a single typography warning.
type TypographyIssue struct {
	Tag        string
	FontSize   float64
	LineHeight float64
	FontFamily string
	Warning    string
}

// SpacingIssue represents a spacing inconsistency.
type SpacingIssue struct {
	Container string
	Index     int
	Gap       float64
	Expected  float64
}

// Data holds all data needed to render the HTML report.
type Data struct {
	URL         string
	Title       string
	GeneratedAt time.Time
	Viewport    string

	Screenshots []Screenshot

	// Audit summary
	TotalIssues        int
	ContrastViolations int
	TouchViolations    int
	TypographyWarnings int
	SpacingIssues      int

	// Accessibility
	AXTotalNodes  int
	AXActiveNodes int

	// Detailed findings
	ContrastIssues   []ContrastIssue
	TouchIssues      []TouchIssue
	TypographyIssues []TypographyIssue
	SpacingIssueList []SpacingIssue
}

// LoadScreenshot reads a PNG file and returns a base64 data URI.
func LoadScreenshot(path string) (template.URL, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading screenshot %s: %w", path, err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return template.URL("data:image/png;base64," + encoded), nil
}

// Generate writes the HTML report to the given writer.
func Generate(w io.Writer, data *Data) error {
	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}
	return tmpl.Execute(w, data)
}

// WriteFile generates the report and writes it to a file.
// Returns the absolute path of the written file.
func WriteFile(dir string, data *Data) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("report-%d.html", time.Now().UnixMilli())
	path := filepath.Join(dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := Generate(f, data); err != nil {
		os.Remove(path)
		return "", err
	}

	abs, _ := filepath.Abs(path)
	return abs, nil
}

var htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kaleidoscope Report — {{.URL}}</title>
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
    max-width: 1200px;
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
  .meta a { color: var(--accent); text-decoration: none; }

  /* Summary cards */
  .summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 0.75rem;
    margin: 1.5rem 0;
  }
  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1rem 1.25rem;
  }
  .card-label { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); }
  .card-value { font-size: 1.75rem; font-weight: 700; margin-top: 0.25rem; }
  .card-value.pass { color: var(--green); }
  .card-value.warn { color: var(--yellow); }
  .card-value.fail { color: var(--red); }

  /* Screenshots */
  .screenshots {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 1rem;
    margin: 1rem 0;
  }
  .screenshot-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }
  .screenshot-card img {
    width: 100%;
    height: auto;
    display: block;
  }
  .screenshot-label {
    padding: 0.5rem 0.75rem;
    font-size: 0.8rem;
    color: var(--text-muted);
    border-top: 1px solid var(--border);
  }
  .screenshot-label strong { color: var(--text); }

  /* Tables */
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

  /* Badges */
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

  /* Color swatch */
  .swatch {
    display: inline-block;
    width: 14px;
    height: 14px;
    border-radius: 3px;
    border: 1px solid var(--border);
    vertical-align: middle;
    margin-right: 4px;
  }

  .empty-state {
    color: var(--text-muted);
    font-style: italic;
    padding: 1.5rem;
    text-align: center;
  }

  code {
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.8rem;
    background: var(--surface2);
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
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

<h1>Kaleidoscope Report</h1>
<div class="meta">
  <a href="{{.URL}}">{{.URL}}</a>{{if .Title}} — {{.Title}}{{end}}<br>
  Generated {{.GeneratedAt.Format "Jan 2, 2006 at 3:04 PM"}} · Viewport {{.Viewport}}
</div>

<!-- Summary -->
<h2>Summary</h2>
<div class="summary">
  <div class="card">
    <div class="card-label">Total Issues</div>
    <div class="card-value {{if eq .TotalIssues 0}}pass{{else if le .TotalIssues 5}}warn{{else}}fail{{end}}">{{.TotalIssues}}</div>
  </div>
  <div class="card">
    <div class="card-label">Contrast</div>
    <div class="card-value {{if eq .ContrastViolations 0}}pass{{else}}fail{{end}}">{{.ContrastViolations}}</div>
  </div>
  <div class="card">
    <div class="card-label">Touch Targets</div>
    <div class="card-value {{if eq .TouchViolations 0}}pass{{else}}fail{{end}}">{{.TouchViolations}}</div>
  </div>
  <div class="card">
    <div class="card-label">Typography</div>
    <div class="card-value {{if eq .TypographyWarnings 0}}pass{{else}}warn{{end}}">{{.TypographyWarnings}}</div>
  </div>
  <div class="card">
    <div class="card-label">Spacing</div>
    <div class="card-value {{if eq .SpacingIssues 0}}pass{{else}}warn{{end}}">{{.SpacingIssues}}</div>
  </div>
  <div class="card">
    <div class="card-label">AX Nodes</div>
    <div class="card-value pass">{{.AXActiveNodes}}</div>
  </div>
</div>

<!-- Screenshots -->
<h2>Screenshots</h2>
<div class="screenshots">
{{range .Screenshots}}
  <div class="screenshot-card">
    <img src="{{.DataURI}}" alt="Screenshot at {{.Breakpoint}}">
    <div class="screenshot-label"><strong>{{.Breakpoint}}</strong> · {{.Width}}×{{.Height}}</div>
  </div>
{{end}}
</div>

<!-- Contrast -->
<h2>Contrast {{if .ContrastIssues}}<span class="badge badge-fail">{{len .ContrastIssues}} violations</span>{{else}}<span class="badge badge-pass">Pass</span>{{end}}</h2>
{{if .ContrastIssues}}
<div class="table-wrap">
<table>
  <thead>
    <tr><th>Element</th><th>Text</th><th>Ratio</th><th>FG</th><th>BG</th><th>AA</th><th>AAA</th></tr>
  </thead>
  <tbody>
  {{range .ContrastIssues}}
    <tr>
      <td><code>{{.Selector}}</code></td>
      <td>{{.Text}}</td>
      <td>{{printf "%.1f" .Ratio}}:1</td>
      <td><span class="swatch" style="background:{{.Foreground}}"></span>{{.Foreground}}</td>
      <td><span class="swatch" style="background:{{.Background}}"></span>{{.Background}}</td>
      <td>{{if .AA}}<span class="badge badge-pass">Pass</span>{{else}}<span class="badge badge-fail">Fail</span>{{end}}</td>
      <td>{{if .AAA}}<span class="badge badge-pass">Pass</span>{{else}}<span class="badge badge-fail">Fail</span>{{end}}</td>
    </tr>
  {{end}}
  </tbody>
</table>
</div>
{{else}}
<div class="card empty-state">All text elements meet WCAG AA contrast requirements.</div>
{{end}}

<!-- Touch Targets -->
<h2>Touch Targets {{if .TouchIssues}}<span class="badge badge-fail">{{len .TouchIssues}} violations</span>{{else}}<span class="badge badge-pass">Pass</span>{{end}}</h2>
{{if .TouchIssues}}
<div class="table-wrap">
<table>
  <thead>
    <tr><th>Element</th><th>Width</th><th>Height</th><th>Issue</th></tr>
  </thead>
  <tbody>
  {{range .TouchIssues}}
    <tr>
      <td><code>{{.Tag}}</code></td>
      <td>{{printf "%.0f" .Width}}px</td>
      <td>{{printf "%.0f" .Height}}px</td>
      <td>{{.Violation}}</td>
    </tr>
  {{end}}
  </tbody>
</table>
</div>
{{else}}
<div class="card empty-state">All interactive elements meet the 48×48px minimum touch target size.</div>
{{end}}

<!-- Typography -->
<h2>Typography {{if .TypographyIssues}}<span class="badge badge-warn">{{len .TypographyIssues}} warnings</span>{{else}}<span class="badge badge-pass">Pass</span>{{end}}</h2>
{{if .TypographyIssues}}
<div class="table-wrap">
<table>
  <thead>
    <tr><th>Element</th><th>Font Size</th><th>Line Height</th><th>Font Family</th><th>Warning</th></tr>
  </thead>
  <tbody>
  {{range .TypographyIssues}}
    <tr>
      <td><code>{{.Tag}}</code></td>
      <td>{{printf "%.0f" .FontSize}}px</td>
      <td>{{printf "%.1f" .LineHeight}}px</td>
      <td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{.FontFamily}}</td>
      <td>{{.Warning}}</td>
    </tr>
  {{end}}
  </tbody>
</table>
</div>
{{else}}
<div class="card empty-state">All text elements meet typography best practices.</div>
{{end}}

<!-- Spacing -->
<h2>Spacing {{if .SpacingIssueList}}<span class="badge badge-warn">{{len .SpacingIssueList}} inconsistencies</span>{{else}}<span class="badge badge-pass">Consistent</span>{{end}}</h2>
{{if .SpacingIssueList}}
<div class="table-wrap">
<table>
  <thead>
    <tr><th>Container</th><th>Gap Index</th><th>Actual</th><th>Expected</th></tr>
  </thead>
  <tbody>
  {{range .SpacingIssueList}}
    <tr>
      <td><code>{{.Container}}</code></td>
      <td>{{.Index}}</td>
      <td>{{printf "%.0f" .Gap}}px</td>
      <td>{{printf "%.0f" .Expected}}px</td>
    </tr>
  {{end}}
  </tbody>
</table>
</div>
{{else}}
<div class="card empty-state">Spacing is consistent across all measured element groups.</div>
{{end}}

<footer>
  Generated by <strong>kaleidoscope</strong> · AI agent front-end design toolkit
</footer>

</body>
</html>
`
