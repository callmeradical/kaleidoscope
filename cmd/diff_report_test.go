package cmd

import (
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/report"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// makeSnap is a test helper that constructs a Snapshot with the given ID and URLs.
func makeSnap(id string, urls []snapshot.URLSnapshot) *snapshot.Snapshot {
	return &snapshot.Snapshot{
		ID:        id,
		CreatedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC),
		URLs:      urls,
	}
}

// TestBuildDiffData_URLCount verifies that buildDiffData produces one
// URLDiffSection per URL present in the snapshots.
func TestBuildDiffData_URLCount(t *testing.T) {
	baseline := makeSnap("base-001", []snapshot.URLSnapshot{
		{URL: "https://example.com/page1", Breakpoints: []snapshot.BreakpointCapture{{Name: "desktop", Width: 1280, Height: 720}}},
		{URL: "https://example.com/page2", Breakpoints: []snapshot.BreakpointCapture{{Name: "desktop", Width: 1280, Height: 720}}},
	})
	current := makeSnap("snap-002", []snapshot.URLSnapshot{
		{URL: "https://example.com/page1", Breakpoints: []snapshot.BreakpointCapture{{Name: "desktop", Width: 1280, Height: 720}}},
		{URL: "https://example.com/page2", Breakpoints: []snapshot.BreakpointCapture{{Name: "desktop", Width: 1280, Height: 720}}},
	})
	result := &diff.DiffResult{}

	data := buildDiffData(baseline, current, result)

	if len(data.URLs) != 2 {
		t.Errorf("expected 2 URLDiffSections, got %d", len(data.URLs))
	}
}

// TestBuildDiffData_SnapshotIDs verifies BaselineID and CurrentID are set.
func TestBuildDiffData_SnapshotIDs(t *testing.T) {
	baseline := makeSnap("base-001", nil)
	current := makeSnap("snap-002", nil)
	result := &diff.DiffResult{}

	data := buildDiffData(baseline, current, result)

	if data.BaselineID != "base-001" {
		t.Errorf("expected BaselineID 'base-001', got %q", data.BaselineID)
	}
	if data.CurrentID != "snap-002" {
		t.Errorf("expected CurrentID 'snap-002', got %q", data.CurrentID)
	}
}

// TestBuildDiffData_AuditDeltaMapping verifies that AuditDelta fields from
// diff.URLDiff are correctly mapped to report.AuditDeltaRow.
func TestBuildDiffData_AuditDeltaMapping(t *testing.T) {
	baseline := makeSnap("base", []snapshot.URLSnapshot{
		{
			URL: "https://example.com",
			AuditResult: snapshot.AuditSummary{
				ContrastViolations: 3,
				TouchViolations:    1,
				TypographyWarnings: 2,
				SpacingIssues:      4,
			},
		},
	})
	current := makeSnap("curr", []snapshot.URLSnapshot{
		{URL: "https://example.com"},
	})
	result := &diff.DiffResult{
		URLs: []diff.URLDiff{
			{
				URL: "https://example.com",
				AuditDelta: diff.AuditDelta{
					ContrastBefore:   3,
					ContrastAfter:    5,
					ContrastDelta:    2,
					TouchBefore:      1,
					TouchAfter:       0,
					TouchDelta:       -1,
					TypographyBefore: 2,
					TypographyAfter:  2,
					TypographyDelta:  0,
					SpacingBefore:    4,
					SpacingAfter:     6,
					SpacingDelta:     2,
				},
			},
		},
	}

	data := buildDiffData(baseline, current, result)

	if len(data.URLs) == 0 {
		t.Fatal("expected at least one URLDiffSection")
	}
	delta := data.URLs[0].AuditDelta

	tests := []struct {
		name string
		got  int
		want int
	}{
		{"ContrastBefore", delta.ContrastBefore, 3},
		{"ContrastAfter", delta.ContrastAfter, 5},
		{"ContrastDelta", delta.ContrastDelta, 2},
		{"TouchBefore", delta.TouchBefore, 1},
		{"TouchAfter", delta.TouchAfter, 0},
		{"TouchDelta", delta.TouchDelta, -1},
		{"TypographyBefore", delta.TypographyBefore, 2},
		{"TypographyAfter", delta.TypographyAfter, 2},
		{"TypographyDelta", delta.TypographyDelta, 0},
		{"SpacingBefore", delta.SpacingBefore, 4},
		{"SpacingAfter", delta.SpacingAfter, 6},
		{"SpacingDelta", delta.SpacingDelta, 2},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("AuditDeltaRow.%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

// TestBuildDiffData_ElementChangeMapping verifies that ElementChange entries
// from diff.URLDiff are correctly converted to report.ElementChangeRow.
func TestBuildDiffData_ElementChangeMapping(t *testing.T) {
	baseline := makeSnap("base", []snapshot.URLSnapshot{{URL: "https://example.com"}})
	current := makeSnap("curr", []snapshot.URLSnapshot{{URL: "https://example.com"}})
	result := &diff.DiffResult{
		URLs: []diff.URLDiff{
			{
				URL: "https://example.com",
				ElementChanges: []diff.ElementChange{
					{Role: "button", Name: "Submit", Selector: "button#submit", Type: "appeared", Details: "new interactive element"},
					{Role: "heading", Name: "Title", Selector: "h1.title", Type: "resized", Details: "width changed from 200 to 300"},
				},
			},
		},
	}

	data := buildDiffData(baseline, current, result)

	if len(data.URLs) == 0 {
		t.Fatal("expected at least one URLDiffSection")
	}
	changes := data.URLs[0].ElementChanges

	if len(changes) != 2 {
		t.Fatalf("expected 2 ElementChangeRows, got %d", len(changes))
	}

	tests := []struct {
		idx      int
		field    string
		got      string
		want     string
	}{
		{0, "Role", changes[0].Role, "button"},
		{0, "Name", changes[0].Name, "Submit"},
		{0, "Selector", changes[0].Selector, "button#submit"},
		{0, "Type", changes[0].Type, "appeared"},
		{0, "Details", changes[0].Details, "new interactive element"},
		{1, "Role", changes[1].Role, "heading"},
		{1, "Type", changes[1].Type, "resized"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("ElementChangeRow[%d].%s = %q, want %q", tt.idx, tt.field, tt.got, tt.want)
		}
	}
}

// TestBuildDiffData_MissingCurrentBreakpoint verifies that when a breakpoint
// exists in baseline but not in current, CurrentURI is empty and BaselineURI
// is non-empty (if the screenshot file exists).
func TestBuildDiffData_MissingCurrentBreakpoint(t *testing.T) {
	baseline := makeSnap("base", []snapshot.URLSnapshot{
		{
			URL: "https://example.com",
			Breakpoints: []snapshot.BreakpointCapture{
				{Name: "desktop", Width: 1280, Height: 720, ScreenshotPath: ""},
			},
		},
	})
	current := makeSnap("curr", []snapshot.URLSnapshot{
		{
			URL:         "https://example.com",
			Breakpoints: nil, // no breakpoints in current
		},
	})
	result := &diff.DiffResult{}

	data := buildDiffData(baseline, current, result)

	if len(data.URLs) == 0 {
		t.Fatal("expected at least one URLDiffSection")
	}
	if len(data.URLs[0].Breakpoints) == 0 {
		t.Fatal("expected at least one BreakpointDiffRow for baseline breakpoint")
	}
	bp := data.URLs[0].Breakpoints[0]
	// CurrentURI must be empty because current has no matching breakpoint.
	if bp.CurrentURI != "" {
		t.Errorf("expected empty CurrentURI when breakpoint missing in current, got %q", bp.CurrentURI)
	}
	// Name and dimensions must be carried from baseline.
	if bp.Name != "desktop" {
		t.Errorf("expected breakpoint Name 'desktop', got %q", bp.Name)
	}
	if bp.Width != 1280 {
		t.Errorf("expected breakpoint Width 1280, got %d", bp.Width)
	}
}

// TestBuildDiffData_PixelDiff_HasDiff verifies that when a URLDiff has a
// PixelDiff with DiffPercent > 0, the BreakpointDiffRow has HasDiff = true
// and DiffPercent is set.
func TestBuildDiffData_PixelDiff_HasDiff(t *testing.T) {
	baseline := makeSnap("base", []snapshot.URLSnapshot{
		{
			URL: "https://example.com",
			Breakpoints: []snapshot.BreakpointCapture{
				{Name: "desktop", Width: 1280, Height: 720},
			},
		},
	})
	current := makeSnap("curr", []snapshot.URLSnapshot{
		{
			URL: "https://example.com",
			Breakpoints: []snapshot.BreakpointCapture{
				{Name: "desktop", Width: 1280, Height: 720},
			},
		},
	})
	result := &diff.DiffResult{
		URLs: []diff.URLDiff{
			{
				URL: "https://example.com",
				PixelDiff: &diff.PixelDiff{
					DiffPath:      "/tmp/diff.png",
					DiffPercent:   8.5,
					ChangedPixels: 17000,
					TotalPixels:   200000,
				},
			},
		},
	}

	data := buildDiffData(baseline, current, result)

	if len(data.URLs) == 0 {
		t.Fatal("expected at least one URLDiffSection")
	}
	if len(data.URLs[0].Breakpoints) == 0 {
		t.Fatal("expected at least one BreakpointDiffRow")
	}
	bp := data.URLs[0].Breakpoints[0]
	if !bp.HasDiff {
		t.Errorf("expected HasDiff = true when PixelDiff.DiffPercent > 0")
	}
	if bp.DiffPercent != 8.5 {
		t.Errorf("expected DiffPercent = 8.5, got %f", bp.DiffPercent)
	}
}

// TestBuildDiffData_PixelDiff_NoDiff verifies that when DiffPercent == 0,
// HasDiff is false.
func TestBuildDiffData_PixelDiff_NoDiff(t *testing.T) {
	baseline := makeSnap("base", []snapshot.URLSnapshot{
		{
			URL: "https://example.com",
			Breakpoints: []snapshot.BreakpointCapture{
				{Name: "desktop", Width: 1280, Height: 720},
			},
		},
	})
	current := makeSnap("curr", []snapshot.URLSnapshot{
		{
			URL: "https://example.com",
			Breakpoints: []snapshot.BreakpointCapture{
				{Name: "desktop", Width: 1280, Height: 720},
			},
		},
	})
	result := &diff.DiffResult{
		URLs: []diff.URLDiff{
			{
				URL: "https://example.com",
				PixelDiff: &diff.PixelDiff{
					DiffPercent:   0.0,
					ChangedPixels: 0,
					TotalPixels:   200000,
				},
			},
		},
	}

	data := buildDiffData(baseline, current, result)

	if len(data.URLs) == 0 || len(data.URLs[0].Breakpoints) == 0 {
		t.Fatal("expected URL and breakpoint sections")
	}
	bp := data.URLs[0].Breakpoints[0]
	if bp.HasDiff {
		t.Errorf("expected HasDiff = false when DiffPercent == 0")
	}
}

// TestBuildDiffData_NilPixelDiff verifies that buildDiffData does not panic
// when URLDiff.PixelDiff is nil.
func TestBuildDiffData_NilPixelDiff(t *testing.T) {
	baseline := makeSnap("base", []snapshot.URLSnapshot{
		{
			URL: "https://example.com",
			Breakpoints: []snapshot.BreakpointCapture{
				{Name: "desktop", Width: 1280, Height: 720},
			},
		},
	})
	current := makeSnap("curr", []snapshot.URLSnapshot{
		{
			URL: "https://example.com",
			Breakpoints: []snapshot.BreakpointCapture{
				{Name: "desktop", Width: 1280, Height: 720},
			},
		},
	})
	result := &diff.DiffResult{
		URLs: []diff.URLDiff{
			{
				URL:       "https://example.com",
				PixelDiff: nil, // explicitly nil
			},
		},
	}

	// Must not panic.
	data := buildDiffData(baseline, current, result)
	if len(data.URLs) == 0 || len(data.URLs[0].Breakpoints) == 0 {
		t.Fatal("expected URL and breakpoint sections even with nil PixelDiff")
	}
	bp := data.URLs[0].Breakpoints[0]
	if bp.HasDiff {
		t.Errorf("expected HasDiff = false when PixelDiff is nil")
	}
}

// TestBuildDiffData_GeneratedAt verifies that GeneratedAt is populated.
func TestBuildDiffData_GeneratedAt(t *testing.T) {
	baseline := makeSnap("base", nil)
	current := makeSnap("curr", nil)
	result := &diff.DiffResult{}

	data := buildDiffData(baseline, current, result)

	if data.GeneratedAt.IsZero() {
		t.Errorf("expected non-zero GeneratedAt timestamp")
	}
}

// TestBuildDiffData_URLOrderFromBaseline verifies the URL order follows
// the baseline snapshot ordering.
func TestBuildDiffData_URLOrderFromBaseline(t *testing.T) {
	urls := []string{
		"https://example.com/alpha",
		"https://example.com/beta",
		"https://example.com/gamma",
	}
	var baseURLs, currURLs []snapshot.URLSnapshot
	for _, u := range urls {
		baseURLs = append(baseURLs, snapshot.URLSnapshot{URL: u})
		currURLs = append(currURLs, snapshot.URLSnapshot{URL: u})
	}

	baseline := makeSnap("base", baseURLs)
	current := makeSnap("curr", currURLs)
	result := &diff.DiffResult{}

	data := buildDiffData(baseline, current, result)

	if len(data.URLs) != 3 {
		t.Fatalf("expected 3 URLDiffSections, got %d", len(data.URLs))
	}
	for i, want := range urls {
		if data.URLs[i].URL != want {
			t.Errorf("URLs[%d] = %q, want %q", i, data.URLs[i].URL, want)
		}
	}
}

// TestBuildDiffData_BreakpointCount verifies that all breakpoints from the
// baseline URL are represented in the output.
func TestBuildDiffData_BreakpointCount(t *testing.T) {
	bps := []snapshot.BreakpointCapture{
		{Name: "mobile", Width: 375, Height: 812},
		{Name: "tablet", Width: 768, Height: 1024},
		{Name: "desktop", Width: 1280, Height: 720},
	}
	baseline := makeSnap("base", []snapshot.URLSnapshot{
		{URL: "https://example.com", Breakpoints: bps},
	})
	current := makeSnap("curr", []snapshot.URLSnapshot{
		{URL: "https://example.com", Breakpoints: bps},
	})
	result := &diff.DiffResult{}

	data := buildDiffData(baseline, current, result)

	if len(data.URLs) == 0 {
		t.Fatal("expected at least one URLDiffSection")
	}
	if len(data.URLs[0].Breakpoints) != 3 {
		t.Errorf("expected 3 BreakpointDiffRows, got %d", len(data.URLs[0].Breakpoints))
	}
}

// Ensure the report.DiffData type is accessible (compile-time check).
var _ *report.DiffData = nil
