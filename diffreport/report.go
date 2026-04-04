package diffreport

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

// BreakpointSection holds all data for one viewport's screenshot trio.
type BreakpointSection struct {
	Name           string
	Width          int
	Height         int
	BaselineDataURI template.URL
	CurrentDataURI  template.URL
	DiffDataURI     template.URL
	DiffPercent    float64
	ChangedPixels  int
	TotalPixels    int
}

// AuditDeltaRow holds one row in the audit delta table.
type AuditDeltaRow struct {
	Category string
	Before   int
	After    int
	Delta    int
	IsWorse  bool // Delta > 0
	IsBetter bool // Delta < 0
}

// ElementChangeRow holds one row in the element changes table.
type ElementChangeRow struct {
	Role     string
	Name     string
	Selector string
	Type     string
	Details  string
}

// URLSection holds all sections for one URL in the diff report.
type URLSection struct {
	URL            string
	Breakpoints    []BreakpointSection
	AuditDeltas    []AuditDeltaRow
	ElementChanges []ElementChangeRow
	HasRegression  bool
}

// Data holds all data needed to render the HTML diff report.
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

// encodeImage reads a PNG file from disk and returns a base64 data URI.
func encodeImage(path string) (template.URL, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading image %s: %w", path, err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return template.URL("data:image/png;base64," + encoded), nil //nolint:gosec // safe: only PNG bytes from our own files
}

// encodeDiffImage base64-encodes in-memory PNG bytes into a data URI.
func encodeDiffImage(pngBytes []byte) template.URL {
	if len(pngBytes) == 0 {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(pngBytes)
	return template.URL("data:image/png;base64," + encoded) //nolint:gosec // safe: only PNG bytes from CompareImages
}

// Build constructs a *Data from a diff result and both snapshots.
func Build(result *diff.Result, baseline, current *snapshot.Snapshot) (*Data, error) {
	baseURLIndex := make(map[string]*snapshot.URLSnapshot, len(baseline.URLs))
	for i := range baseline.URLs {
		baseURLIndex[baseline.URLs[i].URL] = &baseline.URLs[i]
	}
	curURLIndex := make(map[string]*snapshot.URLSnapshot, len(current.URLs))
	for i := range current.URLs {
		curURLIndex[current.URLs[i].URL] = &current.URLs[i]
	}

	data := &Data{
		BaselineID:     result.BaselineID,
		CurrentID:      result.CurrentID,
		BaselineTime:   baseline.CreatedAt,
		CurrentTime:    current.CreatedAt,
		BaselineSHA:    baseline.CommitSHA,
		CurrentSHA:     current.CommitSHA,
		GeneratedAt:    result.GeneratedAt,
		HasRegressions: result.HasRegressions,
	}

	for _, urlDiff := range result.URLs {
		baseSnap := baseURLIndex[urlDiff.URL]
		curSnap := curURLIndex[urlDiff.URL]

		var auditRows []AuditDeltaRow
		for _, d := range urlDiff.AuditDeltas {
			auditRows = append(auditRows, AuditDeltaRow{
				Category: d.Category,
				Before:   d.Before,
				After:    d.After,
				Delta:    d.Delta,
				IsWorse:  d.Delta > 0,
				IsBetter: d.Delta < 0,
			})
		}

		var elemRows []ElementChangeRow
		for _, c := range urlDiff.ElementChanges {
			elemRows = append(elemRows, ElementChangeRow{
				Role:     c.Role,
				Name:     c.Name,
				Selector: c.Selector,
				Type:     c.Type,
				Details:  c.Details,
			})
		}

		// Build baseline breakpoint index.
		var baseBPIndex map[string]*snapshot.BreakpointSnapshot
		if baseSnap != nil {
			baseBPIndex = make(map[string]*snapshot.BreakpointSnapshot, len(baseSnap.Breakpoints))
			for i := range baseSnap.Breakpoints {
				baseBPIndex[baseSnap.Breakpoints[i].Name] = &baseSnap.Breakpoints[i]
			}
		}
		var curBPIndex map[string]*snapshot.BreakpointSnapshot
		if curSnap != nil {
			curBPIndex = make(map[string]*snapshot.BreakpointSnapshot, len(curSnap.Breakpoints))
			for i := range curSnap.Breakpoints {
				curBPIndex[curSnap.Breakpoints[i].Name] = &curSnap.Breakpoints[i]
			}
		}

		var bpSections []BreakpointSection
		for _, bpDiff := range urlDiff.Breakpoints {
			section := BreakpointSection{
				Name:          bpDiff.Name,
				Width:         bpDiff.Width,
				Height:        bpDiff.Height,
				DiffPercent:   bpDiff.DiffPercent,
				ChangedPixels: bpDiff.ChangedPixels,
				TotalPixels:   bpDiff.TotalPixels,
				DiffDataURI:   encodeDiffImage(bpDiff.DiffPNG),
			}

			if baseBPIndex != nil {
				if bp, ok := baseBPIndex[bpDiff.Name]; ok && bp.ScreenshotPath != "" {
					fullPath := fmt.Sprintf(".kaleidoscope/snapshots/%s/%s", result.BaselineID, bp.ScreenshotPath)
					if uri, err := encodeImage(fullPath); err == nil {
						section.BaselineDataURI = uri
					}
				}
			}
			if curBPIndex != nil {
				if bp, ok := curBPIndex[bpDiff.Name]; ok && bp.ScreenshotPath != "" {
					fullPath := fmt.Sprintf(".kaleidoscope/snapshots/%s/%s", result.CurrentID, bp.ScreenshotPath)
					if uri, err := encodeImage(fullPath); err == nil {
						section.CurrentDataURI = uri
					}
				}
			}

			bpSections = append(bpSections, section)
		}

		data.URLs = append(data.URLs, URLSection{
			URL:            urlDiff.URL,
			Breakpoints:    bpSections,
			AuditDeltas:    auditRows,
			ElementChanges: elemRows,
			HasRegression:  urlDiff.HasRegression,
		})
	}

	return data, nil
}

// Generate writes the HTML report to the provided writer.
func Generate(w io.Writer, data *Data) error {
	tmpl, err := template.New("diff-report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parsing diff report template: %w", err)
	}
	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("executing diff report template: %w", err)
	}
	return nil
}

// WriteFile writes the HTML diff report to the given path, creating parent
// directories as needed. Returns the absolute path of the written file.
func WriteFile(path string, data *Data) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0700); err != nil {
		return "", err
	}
	f, err := os.Create(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := Generate(f, data); err != nil {
		return "", err
	}
	return absPath, nil
}
