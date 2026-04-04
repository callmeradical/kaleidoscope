package diffreport

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Kaleidoscope Diff Report</title>
<style>
:root {
  --bg: #0f1117;
  --surface: #1a1d27;
  --border: #2a2d3a;
  --text: #e8eaf0;
  --text-muted: #7c8090;
  --red: #ff4444;
  --red-dim: #3d1a1a;
  --green: #44cc88;
  --green-dim: #1a3d2a;
  --yellow: #ffcc44;
  --yellow-dim: #3d3010;
  --blue: #4488ff;
}
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body {
  background: var(--bg);
  color: var(--text);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, monospace;
  font-size: 14px;
  line-height: 1.5;
  padding: 1.5rem;
}
h1, h2, h3 { font-weight: 600; }
h1 { font-size: 1.4rem; margin-bottom: 0.25rem; }
h2 { font-size: 1.1rem; margin: 1.5rem 0 0.75rem; border-bottom: 1px solid var(--border); padding-bottom: 0.4rem; }
h3 { font-size: 0.95rem; color: var(--text-muted); margin: 1rem 0 0.5rem; }
a { color: var(--blue); }
code { font-family: monospace; background: var(--surface); padding: 0.1em 0.3em; border-radius: 3px; font-size: 0.9em; }

/* header */
.report-header {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.25rem 1.5rem;
  margin-bottom: 2rem;
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}
.header-meta { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; flex: 1; }
.meta-block { display: flex; flex-direction: column; }
.meta-label { font-size: 0.7rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
.meta-value { font-size: 0.85rem; font-family: monospace; }
.arrow { font-size: 1.2rem; color: var(--text-muted); }

/* badges */
.badge-regression {
  background: var(--red-dim);
  color: var(--red);
  border: 1px solid var(--red);
  border-radius: 4px;
  padding: 0.2rem 0.6rem;
  font-size: 0.8rem;
  font-weight: 600;
  white-space: nowrap;
}
.badge-ok {
  background: var(--green-dim);
  color: var(--green);
  border: 1px solid var(--green);
  border-radius: 4px;
  padding: 0.2rem 0.6rem;
  font-size: 0.8rem;
  font-weight: 600;
  white-space: nowrap;
}

/* url section */
.url-section {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.25rem 1.5rem;
  margin-bottom: 1.5rem;
}
.url-heading {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  margin-bottom: 1rem;
}

/* breakpoint section */
.bp-section { margin: 1rem 0; }
.bp-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
  flex-wrap: wrap;
}
.bp-name { font-weight: 600; text-transform: capitalize; }
.bp-meta { font-size: 0.8rem; color: var(--text-muted); }

/* screenshot trio */
.screenshot-trio {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 0.75rem;
  margin: 1rem 0;
}
.ss-col {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.ss-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  text-align: center;
}
.ss-col img {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 4px;
  display: block;
}
.no-diff {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--border);
  border-radius: 4px;
  padding: 2rem;
  color: var(--text-muted);
  font-size: 0.85rem;
  min-height: 80px;
}

/* delta cards */
.delta-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 0.75rem;
  margin: 1rem 0;
}
.delta-card {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.75rem 1rem;
}
.delta-card.worse { border-color: var(--red); background: var(--red-dim); }
.delta-card.better { border-color: var(--green); background: var(--green-dim); }
.delta-card.neutral { border-color: var(--border); }
.delta-label { font-size: 0.75rem; color: var(--text-muted); margin-bottom: 0.35rem; text-transform: uppercase; letter-spacing: 0.04em; }
.delta-values { font-size: 0.85rem; margin-bottom: 0.35rem; }
.delta-badge {
  display: inline-block;
  font-weight: 700;
  font-size: 0.85rem;
}
.worse .delta-badge { color: var(--red); }
.better .delta-badge { color: var(--green); }
.neutral .delta-badge { color: var(--text-muted); }

/* tables */
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
  margin: 0.75rem 0;
}
th {
  text-align: left;
  padding: 0.5rem 0.75rem;
  background: var(--bg);
  border-bottom: 1px solid var(--border);
  color: var(--text-muted);
  font-weight: 500;
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
.cell-worse { color: var(--red); font-weight: 600; }
.cell-better { color: var(--green); font-weight: 600; }
.cell-neutral { color: var(--text-muted); }
.type-appeared { color: var(--green); }
.type-disappeared { color: var(--red); }
.type-moved, .type-resized { color: var(--yellow); }

/* footer */
footer {
  text-align: center;
  color: var(--text-muted);
  font-size: 0.75rem;
  margin-top: 2rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}
</style>
</head>
<body>

<div class="report-header">
  <h1>Kaleidoscope Diff Report</h1>
  <div class="header-meta">
    <div class="meta-block">
      <span class="meta-label">Baseline</span>
      <span class="meta-value">{{.BaselineID}}</span>
    </div>
    <div class="meta-block">
      <span class="meta-label">SHA</span>
      <span class="meta-value">{{.BaselineSHA}}</span>
    </div>
    <div class="meta-block">
      <span class="meta-label">Time</span>
      <span class="meta-value">{{.BaselineTime.Format "2006-01-02 15:04:05"}}</span>
    </div>
    <span class="arrow">→</span>
    <div class="meta-block">
      <span class="meta-label">Current</span>
      <span class="meta-value">{{.CurrentID}}</span>
    </div>
    <div class="meta-block">
      <span class="meta-label">SHA</span>
      <span class="meta-value">{{.CurrentSHA}}</span>
    </div>
    <div class="meta-block">
      <span class="meta-label">Time</span>
      <span class="meta-value">{{.CurrentTime.Format "2006-01-02 15:04:05"}}</span>
    </div>
  </div>
  {{if .HasRegressions}}
  <span class="badge-regression">Regressions Detected</span>
  {{else}}
  <span class="badge-ok">No Regressions</span>
  {{end}}
</div>

{{range .URLs}}
<div class="url-section">
  <div class="url-heading">
    <h2>{{.URL}}</h2>
    {{if .HasRegression}}<span class="badge-regression">Regression</span>{{else}}<span class="badge-ok">OK</span>{{end}}
  </div>

  {{if .AuditDeltas}}
  <h3>Audit Deltas</h3>
  <div class="delta-cards">
    {{range .AuditDeltas}}
    <div class="delta-card {{if .IsWorse}}worse{{else if .IsBetter}}better{{else}}neutral{{end}}">
      <div class="delta-label">{{.Category}}</div>
      <div class="delta-values">{{.Before}} → {{.After}}</div>
      <div class="delta-badge">{{if gt .Delta 0}}+{{end}}{{.Delta}}</div>
    </div>
    {{end}}
  </div>
  {{end}}

  {{range .Breakpoints}}
  <div class="bp-section">
    <div class="bp-header">
      <span class="bp-name">{{.Name}}</span>
      <span class="bp-meta">{{.Width}}×{{.Height}}</span>
      <span class="bp-meta">{{printf "%.2f" .DiffPercent}}% changed ({{.ChangedPixels}}/{{.TotalPixels}} px)</span>
    </div>
    <div class="screenshot-trio">
      <div class="ss-col">
        <span class="ss-label">Baseline</span>
        {{if .BaselineDataURI}}<img src="{{.BaselineDataURI}}" alt="Baseline {{.Name}}">{{else}}<div class="no-diff">No baseline screenshot</div>{{end}}
      </div>
      <div class="ss-col">
        <span class="ss-label">Diff Overlay</span>
        {{if .DiffDataURI}}<img src="{{.DiffDataURI}}" alt="Diff {{.Name}}">{{else}}<div class="no-diff">Identical</div>{{end}}
      </div>
      <div class="ss-col">
        <span class="ss-label">Current</span>
        {{if .CurrentDataURI}}<img src="{{.CurrentDataURI}}" alt="Current {{.Name}}">{{else}}<div class="no-diff">No current screenshot</div>{{end}}
      </div>
    </div>
  </div>
  {{end}}

  {{if .AuditDeltas}}
  <h3>Audit Detail</h3>
  <table>
    <thead><tr><th>Category</th><th>Before</th><th>After</th><th>Delta</th></tr></thead>
    <tbody>
    {{range .AuditDeltas}}
    <tr>
      <td>{{.Category}}</td>
      <td>{{.Before}}</td>
      <td>{{.After}}</td>
      <td class="{{if .IsWorse}}cell-worse{{else if .IsBetter}}cell-better{{else}}cell-neutral{{end}}">{{if gt .Delta 0}}+{{end}}{{.Delta}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  {{end}}

  {{if .ElementChanges}}
  <h3>Element Changes</h3>
  <table>
    <thead><tr><th>Role</th><th>Name</th><th>Selector</th><th>Change</th><th>Details</th></tr></thead>
    <tbody>
    {{range .ElementChanges}}
    <tr>
      <td>{{.Role}}</td>
      <td>{{.Name}}</td>
      <td><code>{{.Selector}}</code></td>
      <td class="type-{{.Type}}">{{.Type}}</td>
      <td>{{.Details}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  {{end}}
</div>
{{end}}

<footer>Generated by kaleidoscope &middot; {{.GeneratedAt.Format "2006-01-02 15:04:05 UTC"}}</footer>
</body>
</html>`
