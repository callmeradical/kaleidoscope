package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/callmeradical/kaleidoscope/browser"
	diffpkg "github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunDiffReport(args []string) {
	snapshotID := getArg(args)
	outputPath := getFlagValue(args, "--output")

	proj, err := snapshot.LoadProject()
	if err != nil {
		output.Fail("diff-report", err, "Run 'ks init' to create a project first")
		os.Exit(2)
	}

	stateDir, err := browser.StateDir()
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	if snapshotID == "" {
		latest, err := snapshot.LatestSnapshot(stateDir)
		if err != nil || latest == nil {
			output.Fail("diff-report", fmt.Errorf("no snapshots found"), "Run 'ks snapshot' first")
			os.Exit(2)
		}
		snapshotID = latest.ID
	}

	baselines, err := snapshot.LoadBaselines(stateDir)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}
	if len(baselines) == 0 {
		output.Fail("diff-report", fmt.Errorf("no baseline found"), "Run 'ks accept' to set a baseline")
		os.Exit(2)
	}

	if outputPath == "" {
		outputPath = filepath.Join(stateDir, "diff-report.html")
	}

	type BreakpointDiff struct {
		Name       string
		Width      int
		Height     int
		Baseline64 string
		Current64  string
		Diff64     string
		SimPct     float64
	}

	type URLSection struct {
		Path           string
		AuditDeltas    []diffpkg.AuditDelta
		ElementChanges []diffpkg.ElementChange
		Breakpoints    []BreakpointDiff
		HasRegressions bool
	}

	var sections []URLSection

	for _, urlPath := range proj.Paths {
		baselineID, ok := baselines[urlPath]
		if !ok {
			continue
		}

		baseAudit, _ := snapshot.ReadAuditJSON(stateDir, baselineID, urlPath)
		currAudit, _ := snapshot.ReadAuditJSON(stateDir, snapshotID, urlPath)
		var auditDeltas []diffpkg.AuditDelta
		if baseAudit != nil && currAudit != nil {
			auditDeltas = diffpkg.DiffAudit(baseAudit, currAudit)
		}

		baseAX, _ := snapshot.ReadAxTreeJSON(stateDir, baselineID, urlPath)
		currAX, _ := snapshot.ReadAxTreeJSON(stateDir, snapshotID, urlPath)
		var elemChanges []diffpkg.ElementChange
		if baseAX != nil && currAX != nil {
			elemChanges = diffpkg.DiffAxTree(baseAX, currAX)
		}

		var bpDiffs []BreakpointDiff
		for _, bp := range proj.Breakpoints {
			basePNG := snapshot.ScreenshotPath(stateDir, baselineID, urlPath, bp.Name, bp.Width, bp.Height)
			currPNG := snapshot.ScreenshotPath(stateDir, snapshotID, urlPath, bp.Name, bp.Width, bp.Height)

			bpDiff := BreakpointDiff{
				Name:   bp.Name,
				Width:  bp.Width,
				Height: bp.Height,
			}

			snapPath := snapshot.SnapshotPath(stateDir, snapshotID)
			diffPNG := filepath.Join(snapPath, snapshot.URLToPath(urlPath), fmt.Sprintf("diff-%s-%dx%d.png", bp.Name, bp.Width, bp.Height))

			if _, err := os.Stat(basePNG); err == nil {
				bpDiff.Baseline64 = loadBase64PNG(basePNG)
			}
			if _, err := os.Stat(currPNG); err == nil {
				bpDiff.Current64 = loadBase64PNG(currPNG)
			}

			if bpDiff.Baseline64 != "" && bpDiff.Current64 != "" {
				if result, err := diffpkg.DiffScreenshots(basePNG, currPNG, diffPNG); err == nil {
					bpDiff.SimPct = result.Similarity * 100
					if _, err := os.Stat(diffPNG); err == nil {
						bpDiff.Diff64 = loadBase64PNG(diffPNG)
					}
				}
			}

			bpDiffs = append(bpDiffs, bpDiff)
		}

		sections = append(sections, URLSection{
			Path:           urlPath,
			AuditDeltas:    auditDeltas,
			ElementChanges: elemChanges,
			Breakpoints:    bpDiffs,
			HasRegressions: diffpkg.HasRegressions(auditDeltas, elemChanges),
		})
	}

	tmpl, err := template.New("diff-report").Parse(diffReportTemplate)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	data := map[string]any{
		"BaselineID": getAnyBaselineID(baselines),
		"CurrentID":  snapshotID,
		"Project":    proj.Name,
		"Sections":   sections,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	output.Success("diff-report", map[string]any{
		"path":     outputPath,
		"baseline": getAnyBaselineID(baselines),
		"current":  snapshotID,
		"urls":     len(sections),
	})
}

func loadBase64PNG(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

var diffReportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Kaleidoscope Diff Report — {{.Project}}</title>
<style>
body{font-family:system-ui,sans-serif;background:#0d1117;color:#c9d1d9;margin:0;padding:24px}
h1,h2,h3{color:#e6edf3}
.header{border-bottom:1px solid #30363d;padding-bottom:16px;margin-bottom:24px}
.meta{color:#8b949e;font-size:14px}
.url-section{border:1px solid #30363d;border-radius:6px;margin-bottom:24px;overflow:hidden}
.url-header{background:#161b22;padding:12px 16px;font-size:14px;font-weight:600;display:flex;align-items:center;gap:8px}
.badge{display:inline-block;padding:2px 8px;border-radius:12px;font-size:12px;font-weight:600}
.badge-ok{background:#1a3a1a;color:#3fb950}
.badge-fail{background:#3a1a1a;color:#f85149}
.section-body{padding:16px}
.bp-grid{display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;margin-bottom:16px}
.bp-card{background:#161b22;border:1px solid #30363d;border-radius:4px;padding:8px}
.bp-label{font-size:12px;color:#8b949e;margin-bottom:4px}
.bp-images{display:flex;gap:4px}
.bp-images img{width:100%;max-width:220px;height:auto;border-radius:2px;border:1px solid #21262d}
.sim-score{font-size:11px;color:#8b949e;margin-top:4px}
table{width:100%;border-collapse:collapse;font-size:13px;margin-bottom:12px}
th{background:#161b22;color:#8b949e;text-align:left;padding:6px 8px;border-bottom:1px solid #30363d}
td{padding:6px 8px;border-bottom:1px solid #21262d}
.delta-pos{color:#f85149}
.delta-neg{color:#3fb950}
.delta-zero{color:#8b949e}
.no-data{color:#8b949e;font-style:italic;font-size:13px}
</style>
</head>
<body>
<div class="header">
<h1>Kaleidoscope Diff Report</h1>
<div class="meta">
  <strong>Project:</strong> {{.Project}}<br>
  <strong>Baseline:</strong> {{.BaselineID}}<br>
  <strong>Current:</strong> {{.CurrentID}}
</div>
</div>

{{range .Sections}}
<div class="url-section">
  <div class="url-header">
    {{.Path}}
    {{if .HasRegressions}}<span class="badge badge-fail">regressions</span>{{else}}<span class="badge badge-ok">ok</span>{{end}}
  </div>
  <div class="section-body">

    {{if .Breakpoints}}
    <h3>Screenshots</h3>
    <div class="bp-grid">
      {{range .Breakpoints}}
      <div class="bp-card">
        <div class="bp-label">{{.Name}} ({{.Width}}×{{.Height}})</div>
        <div class="bp-images">
          {{if .Baseline64}}<img src="{{.Baseline64}}" title="Baseline">{{end}}
          {{if .Diff64}}<img src="{{.Diff64}}" title="Diff">{{end}}
          {{if .Current64}}<img src="{{.Current64}}" title="Current">{{end}}
        </div>
        {{if .SimPct}}<div class="sim-score">Similarity: {{printf "%.1f" .SimPct}}%</div>{{end}}
      </div>
      {{end}}
    </div>
    {{end}}

    <h3>Audit Deltas</h3>
    {{if .AuditDeltas}}
    <table>
      <tr><th>Category</th><th>Before</th><th>After</th><th>Delta</th></tr>
      {{range .AuditDeltas}}
      <tr>
        <td>{{.Category}}</td>
        <td>{{.Before}}</td>
        <td>{{.After}}</td>
        <td class="{{if gt .Delta 0}}delta-pos{{else if lt .Delta 0}}delta-neg{{else}}delta-zero{{end}}">
          {{if gt .Delta 0}}+{{end}}{{.Delta}}
        </td>
      </tr>
      {{end}}
    </table>
    {{else}}<p class="no-data">No audit data available.</p>{{end}}

    <h3>Element Changes</h3>
    {{if .ElementChanges}}
    <table>
      <tr><th>Role</th><th>Name</th><th>Change</th></tr>
      {{range .ElementChanges}}
      <tr>
        <td>{{.Role}}</td>
        <td>{{.Name}}</td>
        <td class="{{if eq .Change "disappeared"}}delta-pos{{else}}delta-neg{{end}}">{{.Change}}</td>
      </tr>
      {{end}}
    </table>
    {{else}}<p class="no-data">No element changes detected.</p>{{end}}

  </div>
</div>
{{end}}

<footer style="color:#8b949e;font-size:12px;margin-top:32px;border-top:1px solid #30363d;padding-top:16px">
Generated by kaleidoscope (ks)
</footer>
</body>
</html>`
