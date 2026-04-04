package report_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/report"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// TestGenerateDiff_ZeroValue verifies the template renders without error for an empty DiffData.
func TestGenerateDiff_ZeroValue(t *testing.T) {
	data := &report.DiffData{}
	var buf bytes.Buffer
	err := report.GenerateDiff(&buf, data)
	if err != nil {
		t.Fatalf("GenerateDiff with zero DiffData returned error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty HTML output, got empty buffer")
	}
}

// TestBuildDiffData_PageCount verifies the output page count matches the input diff page count.
func TestBuildDiffData_PageCount(t *testing.T) {
	baseline := &snapshot.Snapshot{
		ID:        "base-001",
		CreatedAt: time.Now(),
		Pages: []snapshot.PageSnapshot{
			{URL: "https://example.com"},
			{URL: "https://example.com/about"},
		},
	}
	current := &snapshot.Snapshot{
		ID:        "cur-001",
		CreatedAt: time.Now(),
		Pages: []snapshot.PageSnapshot{
			{URL: "https://example.com"},
			{URL: "https://example.com/about"},
		},
	}

	d, err := diff.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}

	data, err := report.BuildDiffData(d, baseline, current)
	if err != nil {
		t.Fatalf("BuildDiffData error: %v", err)
	}

	if len(data.Pages) != len(d.Pages) {
		t.Errorf("page count: got %d, want %d", len(data.Pages), len(d.Pages))
	}
}

// TestBuildDiffData_TrendMapping verifies Delta>0 → "worse", Delta<0 → "better", Delta==0 → "same".
func TestBuildDiffData_TrendMapping(t *testing.T) {
	baseline := &snapshot.Snapshot{
		ID: "base-001",
		Pages: []snapshot.PageSnapshot{
			{
				URL: "https://example.com",
				AuditResult: snapshot.AuditResult{
					ContrastViolations: 1,
					TouchViolations:    3,
					TypographyWarnings: 2,
				},
			},
		},
	}
	current := &snapshot.Snapshot{
		ID: "cur-001",
		Pages: []snapshot.PageSnapshot{
			{
				URL: "https://example.com",
				AuditResult: snapshot.AuditResult{
					ContrastViolations: 2, // worse: +1
					TouchViolations:    1, // better: -2
					TypographyWarnings: 2, // same: 0
				},
			},
		},
	}

	d, _ := diff.Compare(baseline, current)
	data, err := report.BuildDiffData(d, baseline, current)
	if err != nil {
		t.Fatalf("BuildDiffData error: %v", err)
	}

	trendFor := func(category string) string {
		for _, row := range data.Pages[0].AuditDelta {
			if row.Category == category {
				return row.Trend
			}
		}
		return ""
	}

	if got := trendFor("contrast"); got != "worse" {
		t.Errorf("contrast trend: got %q, want %q", got, "worse")
	}
	if got := trendFor("touch"); got != "better" {
		t.Errorf("touch trend: got %q, want %q", got, "better")
	}
	if got := trendFor("typography"); got != "same" {
		t.Errorf("typography trend: got %q, want %q", got, "same")
	}
}
