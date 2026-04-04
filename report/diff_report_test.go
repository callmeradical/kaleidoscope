package report_test

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/report"
)

func TestGenerateDiff_smoke(t *testing.T) {
	data := &report.DiffData{
		GeneratedAt: time.Now(),
		BaselineID:  "snap-001",
		CurrentID:   "snap-002",
		BaselineAt:  time.Now().Add(-24 * time.Hour),
		CurrentAt:   time.Now(),
		Pages: []report.DiffPage{
			{
				URL: "https://example.com",
				Breakpoints: []report.DiffBreakpoint{
					{
						Name:        "desktop",
						Width:       1280,
						Height:      720,
						BaselineURI: template.URL("data:image/png;base64,abc"),
						CurrentURI:  template.URL("data:image/png;base64,def"),
						OverlayURI:  template.URL("data:image/png;base64,ghi"),
						DiffScore:   0.98,
					},
				},
				AuditDelta: report.AuditDelta{
					Contrast: report.CategoryDelta{Before: 2, After: 1, Delta: -1},
				},
				ElementChanges: []report.ElementChangeRow{
					{Selector: "h1", ChangeType: "resized", Details: "width: 300→400"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := report.GenerateDiff(&buf, data); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}

	output := buf.String()
	checks := []string{
		"Kaleidoscope Diff Report",
		"Baseline",
		"Current",
		"Diff Overlay",
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestGenerateDiff_missingImages(t *testing.T) {
	data := &report.DiffData{
		GeneratedAt: time.Now(),
		BaselineID:  "snap-001",
		CurrentID:   "snap-002",
		Pages: []report.DiffPage{
			{
				URL: "https://example.com",
				Breakpoints: []report.DiffBreakpoint{
					{
						Name:        "mobile",
						Width:       375,
						Height:      812,
						BaselineURI: "",
						CurrentURI:  "",
						OverlayURI:  "",
						DiffScore:   0,
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := report.GenerateDiff(&buf, data); err != nil {
		t.Fatalf("GenerateDiff returned error with missing images: %v", err)
	}

	output := buf.String()
	// Should contain some placeholder for missing screenshots.
	if !strings.Contains(output, "No screenshot") && !strings.Contains(output, "empty-state") {
		t.Error("output should contain placeholder text for missing screenshots")
	}
}

func TestGenerateDiff_deltaColors(t *testing.T) {
	makeDelta := func(delta int) *report.DiffData {
		return &report.DiffData{
			GeneratedAt: time.Now(),
			BaselineID:  "snap-001",
			CurrentID:   "snap-002",
			Pages: []report.DiffPage{
				{
					URL: "https://example.com",
					AuditDelta: report.AuditDelta{
						Contrast: report.CategoryDelta{Before: 1, After: 1 + delta, Delta: delta},
					},
				},
			},
		}
	}

	tests := []struct {
		delta    int
		wantCSS  string
	}{
		{delta: 2, wantCSS: "delta-positive"},
		{delta: -1, wantCSS: "delta-negative"},
		{delta: 0, wantCSS: "delta-zero"},
	}

	for _, tt := range tests {
		t.Run(tt.wantCSS, func(t *testing.T) {
			var buf bytes.Buffer
			if err := report.GenerateDiff(&buf, makeDelta(tt.delta)); err != nil {
				t.Fatalf("GenerateDiff error: %v", err)
			}
			if !strings.Contains(buf.String(), tt.wantCSS) {
				t.Errorf("output missing CSS class %q for delta=%d", tt.wantCSS, tt.delta)
			}
		})
	}
}
