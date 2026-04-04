package report

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"
	"time"
)

// helper builds a minimal DiffData for use in tests.
func makeTestDiffData(urls []URLDiffSection) *DiffData {
	return &DiffData{
		BaselineID:  "snap-abc",
		CurrentID:   "snap-xyz",
		GeneratedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC),
		URLs:        urls,
	}
}

// TestGenerateDiffReport_EmptyData verifies that rendering an empty DiffData
// (no URLs) succeeds without error and produces valid HTML.
func TestGenerateDiffReport_EmptyData(t *testing.T) {
	data := makeTestDiffData(nil)
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("GenerateDiffReport with empty data returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Errorf("expected HTML doctype in output, got:\n%s", out)
	}
}

// TestGenerateDiffReport_ReportTitle verifies the page title includes
// baseline and current IDs.
func TestGenerateDiffReport_ReportTitle(t *testing.T) {
	data := makeTestDiffData(nil)
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "snap-abc") {
		t.Errorf("expected baselineID 'snap-abc' in output")
	}
	if !strings.Contains(out, "snap-xyz") {
		t.Errorf("expected currentID 'snap-xyz' in output")
	}
}

// TestGenerateDiffReport_SingleURL_BaselineImg verifies that a non-empty
// BaselineURI produces an <img> tag in the baseline column.
func TestGenerateDiffReport_SingleURL_BaselineImg(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{
				{
					Name:        "desktop",
					Width:       1280,
					Height:      720,
					BaselineURI: template.URL("data:image/png;base64,AAAA"),
					CurrentURI:  template.URL("data:image/png;base64,BBBB"),
					DiffPercent: 0,
					HasDiff:     false,
				},
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "data:image/png;base64,AAAA") {
		t.Errorf("expected baseline image URI in output")
	}
	if !strings.Contains(out, "data:image/png;base64,BBBB") {
		t.Errorf("expected current image URI in output")
	}
}

// TestGenerateDiffReport_SideBySideLayout verifies three-column diff-row
// structure (baseline/overlay/current) is present in the output.
func TestGenerateDiffReport_SideBySideLayout(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{
				{
					Name:        "mobile",
					Width:       375,
					Height:      812,
					BaselineURI: template.URL("data:image/png;base64,BASE"),
					CurrentURI:  template.URL("data:image/png;base64,CURR"),
				},
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "diff-row") {
		t.Errorf("expected 'diff-row' CSS class for side-by-side layout")
	}
	if !strings.Contains(out, "diff-col--baseline") {
		t.Errorf("expected 'diff-col--baseline' CSS class")
	}
	if !strings.Contains(out, "diff-col--current") {
		t.Errorf("expected 'diff-col--current' CSS class")
	}
}

// TestGenerateDiffReport_MissingScreenshot_Baseline verifies that a missing
// baseline URI renders a .no-screenshot placeholder instead of a broken <img>.
func TestGenerateDiffReport_MissingScreenshot_Baseline(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{
				{
					Name:        "desktop",
					Width:       1280,
					Height:      720,
					BaselineURI: "", // missing
					CurrentURI:  template.URL("data:image/png;base64,CURR"),
				},
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "no-screenshot") {
		t.Errorf("expected 'no-screenshot' placeholder when baseline URI is empty")
	}
	// Must not render a broken img src for empty URI.
	if strings.Contains(out, `src=""`) {
		t.Errorf("should not render <img src=\"\"> for empty baseline URI")
	}
}

// TestGenerateDiffReport_MissingScreenshot_Current verifies the same for
// the current (right) column.
func TestGenerateDiffReport_MissingScreenshot_Current(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{
				{
					Name:        "desktop",
					Width:       1280,
					Height:      720,
					BaselineURI: template.URL("data:image/png;base64,BASE"),
					CurrentURI:  "", // missing
				},
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "no-screenshot") {
		t.Errorf("expected 'no-screenshot' placeholder when current URI is empty")
	}
}

// TestGenerateDiffReport_DiffOverlay verifies the center overlay column
// appears and includes the diff image when HasDiff is true.
func TestGenerateDiffReport_DiffOverlay(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{
				{
					Name:           "desktop",
					Width:          1280,
					Height:         720,
					BaselineURI:    template.URL("data:image/png;base64,BASE"),
					CurrentURI:     template.URL("data:image/png;base64,CURR"),
					DiffOverlayURI: template.URL("data:image/png;base64,DIFF"),
					DiffPercent:    5.3,
					HasDiff:        true,
				},
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "diff-col--overlay") {
		t.Errorf("expected 'diff-col--overlay' CSS class when HasDiff is true")
	}
	if !strings.Contains(out, "data:image/png;base64,DIFF") {
		t.Errorf("expected diff overlay image in output")
	}
}

// TestGenerateDiffReport_PositiveDelta verifies that a positive delta value
// produces the 'delta-positive' CSS class (regression = bad, shown in red).
func TestGenerateDiffReport_PositiveDelta(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
			AuditDelta: AuditDeltaRow{
				ContrastBefore: 2,
				ContrastAfter:  5,
				ContrastDelta:  3, // positive = regression
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "delta-positive") {
		t.Errorf("expected 'delta-positive' class for positive delta (regression)")
	}
}

// TestGenerateDiffReport_NegativeDelta verifies that a negative delta value
// produces the 'delta-negative' CSS class (improvement = good, shown in green).
func TestGenerateDiffReport_NegativeDelta(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
			AuditDelta: AuditDeltaRow{
				ContrastBefore: 5,
				ContrastAfter:  2,
				ContrastDelta:  -3, // negative = improvement
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "delta-negative") {
		t.Errorf("expected 'delta-negative' class for negative delta (improvement)")
	}
}

// TestGenerateDiffReport_ZeroDelta verifies that a zero delta value
// produces the 'delta-zero' CSS class.
func TestGenerateDiffReport_ZeroDelta(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
			AuditDelta: AuditDeltaRow{
				ContrastBefore: 3,
				ContrastAfter:  3,
				ContrastDelta:  0, // no change
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "delta-zero") {
		t.Errorf("expected 'delta-zero' class for zero delta")
	}
}

// TestGenerateDiffReport_AuditDeltaTable verifies that all four audit
// categories (Contrast, Touch, Typography, Spacing) appear in the output.
func TestGenerateDiffReport_AuditDeltaTable(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
			AuditDelta: AuditDeltaRow{
				ContrastDelta:   1,
				TouchDelta:      -1,
				TypographyDelta: 0,
				SpacingDelta:    2,
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, label := range []string{"Contrast", "Touch", "Typography", "Spacing"} {
		if !strings.Contains(out, label) {
			t.Errorf("expected audit category %q in diff report output", label)
		}
	}
}

// TestGenerateDiffReport_ElementChanges_Visible verifies that the element
// changes table appears when ElementChanges is non-empty.
func TestGenerateDiffReport_ElementChanges_Visible(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
			ElementChanges: []ElementChangeRow{
				{Role: "button", Name: "Submit", Selector: "button#submit", Type: "appeared", Details: "new element"},
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "button#submit") {
		t.Errorf("expected element selector 'button#submit' in element changes table")
	}
	if !strings.Contains(out, "appeared") {
		t.Errorf("expected change type 'appeared' in element changes table")
	}
}

// TestGenerateDiffReport_ElementChanges_Hidden verifies that element change
// table rows do not appear when ElementChanges is empty.
func TestGenerateDiffReport_ElementChanges_Hidden(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL:            "https://example.com",
			Breakpoints:    []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
			ElementChanges: nil,
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// No change-type cells should appear when there are no element changes.
	for _, changeType := range []string{"appeared", "disappeared", "moved", "resized"} {
		if strings.Contains(out, "change-"+changeType) {
			t.Errorf("change type CSS class 'change-%s' should not appear with no element changes", changeType)
		}
	}
}

// TestGenerateDiffReport_ElementChangeTypeClasses verifies each element change
// type gets the corresponding CSS class.
func TestGenerateDiffReport_ElementChangeTypeClasses(t *testing.T) {
	types := []string{"appeared", "disappeared", "moved", "resized"}
	for _, changeType := range types {
		t.Run(changeType, func(t *testing.T) {
			data := makeTestDiffData([]URLDiffSection{
				{
					URL: "https://example.com",
					Breakpoints: []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
					ElementChanges: []ElementChangeRow{
						{Role: "link", Name: "Nav", Selector: "a.nav", Type: changeType, Details: "test"},
					},
				},
			})
			var buf bytes.Buffer
			if err := GenerateDiffReport(&buf, data); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			out := buf.String()
			expectedClass := "change-" + changeType
			if !strings.Contains(out, expectedClass) {
				t.Errorf("expected CSS class %q for change type %q", expectedClass, changeType)
			}
		})
	}
}

// TestGenerateDiffReport_HTMLEscaping verifies that user-controlled strings
// (URL, selector, name, details) are HTML-escaped and cannot inject markup.
func TestGenerateDiffReport_HTMLEscaping(t *testing.T) {
	malicious := `<script>alert('xss')</script>`
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com/path",
			Breakpoints: []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
			ElementChanges: []ElementChangeRow{
				{
					Role:     "button",
					Name:     malicious,
					Selector: malicious,
					Type:     "appeared",
					Details:  malicious,
				},
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// The raw script tag must not appear unescaped in the output.
	if strings.Contains(out, "<script>alert") {
		t.Errorf("XSS: raw <script> tag was not escaped in output")
	}
	// The escaped form should appear instead.
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("expected HTML-escaped form '&lt;script&gt;' in output")
	}
}

// TestGenerateDiffReport_SignedDelta verifies the signedDelta helper formats
// deltas with an explicit sign character.
func TestGenerateDiffReport_SignedDelta(t *testing.T) {
	data := makeTestDiffData([]URLDiffSection{
		{
			URL: "https://example.com",
			Breakpoints: []BreakpointDiffRow{{Name: "desktop", Width: 1280, Height: 720}},
			AuditDelta: AuditDeltaRow{
				ContrastDelta: 2,
				TouchDelta:    -1,
			},
		},
	})
	var buf bytes.Buffer
	if err := GenerateDiffReport(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "+2") {
		t.Errorf("expected signed delta '+2' for positive contrast delta")
	}
	if !strings.Contains(out, "-1") {
		t.Errorf("expected signed delta '-1' for negative touch delta")
	}
}

// TestWriteDiffFile_CreatesFile verifies WriteDiffFile creates the output file
// at the specified path and returns an absolute path.
func TestWriteDiffFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/out/diff-report.html"
	data := makeTestDiffData(nil)

	absPath, err := WriteDiffFile(path, data)
	if err != nil {
		t.Fatalf("WriteDiffFile returned error: %v", err)
	}
	if absPath == "" {
		t.Errorf("expected non-empty absolute path")
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Errorf("output file does not exist at %s", absPath)
	}
}

// TestWriteDiffFile_ContainsHTML verifies the written file is valid HTML.
func TestWriteDiffFile_ContainsHTML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/diff-report.html"
	data := makeTestDiffData(nil)

	absPath, err := WriteDiffFile(path, data)
	if err != nil {
		t.Fatalf("WriteDiffFile returned error: %v", err)
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("cannot read written file: %v", err)
	}
	if !strings.Contains(string(content), "<!DOCTYPE html>") {
		t.Errorf("written file does not contain HTML doctype")
	}
}
