package report

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// DiffData is the view model for the diff report template.
type DiffData struct {
	GeneratedAt    time.Time
	BaselineID     string
	CurrentID      string
	BaselineCommit string
	CurrentCommit  string
	Pages          []DiffPageData
}

// DiffPageData holds the rendered diff for one URL.
type DiffPageData struct {
	URL            string
	Breakpoints    []DiffBreakpointData
	AuditDelta     []DiffCategoryRow
	ElementChanges []DiffElementRow
}

// DiffBreakpointData holds the three image data URIs for one viewport.
type DiffBreakpointData struct {
	Name            string
	Width           int
	Height          int
	BaselineDataURI template.URL
	CurrentDataURI  template.URL
	DiffDataURI     template.URL // empty if identical
	DiffScore       float64
	DiffScorePct    string // pre-formatted, e.g. "4.2%"
}

// DiffCategoryRow is one row in the audit delta table.
type DiffCategoryRow struct {
	Category string
	Before   int
	After    int
	Delta    int
	Trend    string // "better" | "worse" | "same"
}

// DiffElementRow is one row in the element changes table.
type DiffElementRow struct {
	Role     string
	Name     string
	Selector string
	Type     string
	Details  string
}

// loadAndEncode reads a file and returns a base64-encoded PNG data URI.
// Returns an empty template.URL (not an error) if path is empty.
func loadAndEncode(path string) (template.URL, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading image %q: %w", path, err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return template.URL("data:image/png;base64," + encoded), nil
}

// BuildDiffData converts a SnapshotDiff into the view model for the template.
func BuildDiffData(d *diff.SnapshotDiff, baseline, current *snapshot.Snapshot) (*DiffData, error) {
	data := &DiffData{
		GeneratedAt:    time.Now(),
		BaselineID:     baseline.ID,
		CurrentID:      current.ID,
		BaselineCommit: baseline.CommitSHA,
		CurrentCommit:  current.CommitSHA,
	}

	// Build breakpoint metadata lookup from current pages.
	curBPMeta := make(map[string]map[string]snapshot.BreakpointCapture)
	for _, p := range current.Pages {
		m := make(map[string]snapshot.BreakpointCapture)
		for _, bp := range p.Breakpoints {
			m[bp.Name] = bp
		}
		curBPMeta[p.URL] = m
	}
	baseBPMeta := make(map[string]map[string]snapshot.BreakpointCapture)
	for _, p := range baseline.Pages {
		m := make(map[string]snapshot.BreakpointCapture)
		for _, bp := range p.Breakpoints {
			m[bp.Name] = bp
		}
		baseBPMeta[p.URL] = m
	}

	for _, pd := range d.Pages {
		var page DiffPageData
		page.URL = pd.URL

		// Breakpoints.
		for _, bd := range pd.BreakpointDiffs {
			baseURI, _ := loadAndEncode(bd.BaselinePath)
			curURI, _ := loadAndEncode(bd.CurrentPath)

			var diffURI template.URL
			if len(bd.DiffImageBytes) > 0 {
				encoded := base64.StdEncoding.EncodeToString(bd.DiffImageBytes)
				diffURI = template.URL("data:image/png;base64," + encoded)
			}

			pct := fmt.Sprintf("%.1f%%", bd.DiffScore*100)

			// Get width/height from current snapshot metadata.
			var w, h int
			if meta, ok := curBPMeta[pd.URL]; ok {
				if bp, ok := meta[bd.Name]; ok {
					w, h = bp.Width, bp.Height
				}
			}

			page.Breakpoints = append(page.Breakpoints, DiffBreakpointData{
				Name:            bd.Name,
				Width:           w,
				Height:          h,
				BaselineDataURI: baseURI,
				CurrentDataURI:  curURI,
				DiffDataURI:     diffURI,
				DiffScore:       bd.DiffScore,
				DiffScorePct:    pct,
			})
		}

		// Audit delta.
		for _, delta := range pd.AuditDelta {
			trend := "same"
			if delta.Delta > 0 {
				trend = "worse"
			} else if delta.Delta < 0 {
				trend = "better"
			}
			page.AuditDelta = append(page.AuditDelta, DiffCategoryRow{
				Category: delta.Category,
				Before:   delta.Before,
				After:    delta.After,
				Delta:    delta.Delta,
				Trend:    trend,
			})
		}

		// Element changes.
		for _, ec := range pd.ElementChanges {
			page.ElementChanges = append(page.ElementChanges, DiffElementRow{
				Role:     ec.Role,
				Name:     ec.Name,
				Selector: ec.Selector,
				Type:     ec.Type,
				Details:  ec.Details,
			})
		}

		data.Pages = append(data.Pages, page)
	}

	return data, nil
}

// GenerateDiff writes the rendered HTML diff report to w.
func GenerateDiff(w io.Writer, data *DiffData) error {
	tmpl, err := template.New("diff-report").Parse(diffHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parsing diff template: %w", err)
	}
	return tmpl.Execute(w, data)
}

// WriteDiffFile renders the diff report and writes it to a file.
// If outputPath is non-empty, it is used; otherwise a timestamped file under dir is created.
// Returns the absolute path of the written file.
func WriteDiffFile(outputPath, dir string, data *DiffData) (string, error) {
	var path string
	if outputPath != "" {
		path = outputPath
	} else {
		path = filepath.Join(dir, "diff-report.html")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	if err := GenerateDiff(f, data); err != nil {
		os.Remove(path)
		return "", err
	}

	abs, _ := filepath.Abs(path)
	return abs, nil
}
