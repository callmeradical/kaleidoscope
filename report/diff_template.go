package report

var diffHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kaleidoscope Diff Report</title>
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
  code {
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.8rem;
    background: var(--surface2);
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
  }

  /* Page sections */
  .page-section {
    margin-top: 2.5rem;
    padding-top: 1rem;
    border-top: 2px solid var(--border);
  }

  /* Triptych layout */
  .diff-breakpoint { margin-bottom: 2rem; }
  .diff-triptych {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 1rem;
    margin: 1rem 0;
  }
  .diff-pane {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }
  .diff-pane img { width: 100%; height: auto; display: block; }
  .diff-pane figcaption {
    padding: 0.5rem 0.75rem;
    font-size: 0.8rem;
    color: var(--text-muted);
    border-top: 1px solid var(--border);
    text-align: center;
  }
  .diff-overlay figcaption { color: var(--yellow); }
  .diff-identical {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 80px;
    color: var(--text-muted);
    font-style: italic;
    font-size: 0.85rem;
  }
  .no-screenshots {
    color: var(--text-muted);
    font-style: italic;
    padding: 1rem 0;
    font-size: 0.85rem;
  }

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

  /* Delta cell colors */
  .delta-worse { color: var(--red); font-weight: 700; }
  .delta-better { color: var(--green); font-weight: 700; }
  .delta-same { color: var(--text-muted); }

  /* Badges */
  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .trend-better { background: var(--green-dim); color: var(--green); }
  .trend-worse  { background: var(--red-dim);   color: var(--red);   }
  .trend-same   { background: var(--surface2);  color: var(--text-muted); }

  .change-appeared    { background: var(--green-dim);  color: var(--green);  }
  .change-disappeared { background: var(--red-dim);    color: var(--red);    }
  .change-moved       { background: var(--yellow-dim); color: var(--yellow); }
  .change-resized     { background: var(--yellow-dim); color: var(--yellow); }

  .empty-state {
    color: var(--text-muted);
    font-style: italic;
    padding: 1.5rem;
    text-align: center;
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
  Baseline: <code>{{.BaselineID}}</code>{{if .BaselineCommit}} @ <code>{{.BaselineCommit}}</code>{{end}}<br>
  Current: <code>{{.CurrentID}}</code>{{if .CurrentCommit}} @ <code>{{.CurrentCommit}}</code>{{end}}<br>
  Generated {{.GeneratedAt.Format "Jan 2, 2006 at 3:04 PM"}}
</div>

{{range .Pages}}
<section class="page-section">
  <h2>{{.URL}}</h2>

  {{range .Breakpoints}}
  <div class="diff-breakpoint">
    <h3>{{.Name}} · {{.Width}}×{{.Height}}</h3>
    {{if or .BaselineDataURI .CurrentDataURI}}
    <div class="diff-triptych">
      <figure class="diff-pane diff-baseline">
        {{if .BaselineDataURI}}<img src="{{.BaselineDataURI}}" alt="Baseline">{{else}}<div class="diff-identical">No baseline screenshot</div>{{end}}
        <figcaption>Baseline</figcaption>
      </figure>
      <figure class="diff-pane diff-overlay">
        {{if .DiffDataURI}}<img src="{{.DiffDataURI}}" alt="Diff overlay">{{else}}<div class="diff-identical">Identical</div>{{end}}
        <figcaption>Diff ({{.DiffScorePct}} changed)</figcaption>
      </figure>
      <figure class="diff-pane diff-current">
        {{if .CurrentDataURI}}<img src="{{.CurrentDataURI}}" alt="Current">{{else}}<div class="diff-identical">No current screenshot</div>{{end}}
        <figcaption>Current</figcaption>
      </figure>
    </div>
    {{else}}
    <p class="no-screenshots">No screenshots captured for this breakpoint.</p>
    {{end}}
  </div>
  {{end}}

  <h3>Audit Delta</h3>
  {{$hasDeltas := false}}
  {{range .AuditDelta}}{{if ne .Delta 0}}{{$hasDeltas = true}}{{end}}{{end}}
  <div class="table-wrap">
  <table>
    <thead><tr><th>Category</th><th>Before</th><th>After</th><th>Delta</th><th>Trend</th></tr></thead>
    <tbody>
    {{range .AuditDelta}}
    <tr>
      <td>{{.Category}}</td>
      <td>{{.Before}}</td>
      <td>{{.After}}</td>
      <td class="{{if gt .Delta 0}}delta-worse{{else if lt .Delta 0}}delta-better{{else}}delta-same{{end}}">{{if gt .Delta 0}}+{{end}}{{.Delta}}</td>
      <td><span class="badge trend-{{.Trend}}">{{.Trend}}</span></td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>

  <h3>Element Changes</h3>
  {{if .ElementChanges}}
  <div class="table-wrap">
  <table>
    <thead><tr><th>Role</th><th>Name</th><th>Selector</th><th>Type</th><th>Details</th></tr></thead>
    <tbody>
    {{range .ElementChanges}}
    <tr>
      <td>{{.Role}}</td>
      <td>{{.Name}}</td>
      <td><code>{{.Selector}}</code></td>
      <td><span class="badge change-{{.Type}}">{{.Type}}</span></td>
      <td>{{.Details}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{else}}
  <div class="empty-state">No element changes detected.</div>
  {{end}}

</section>
{{end}}

<footer>
  Generated by <strong>kaleidoscope</strong> · AI agent front-end design toolkit
</footer>

</body>
</html>
`
