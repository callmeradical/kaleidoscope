package report

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/diff"
)

// --- helpers ---

// writeTempPNG writes a minimal 1×1 PNG file to a temp directory and returns its path.
func writeTempPNG(t *testing.T, dir, name string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating temp PNG %s: %v", name, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding temp PNG %s: %v", name, err)
	}
	return path
}

// minimalDiffData returns a DiffData with no screenshots (empty template.URL) for template tests.
func minimalDiffData() *DiffData {
	return &DiffData{
		BaselineID:       "snap-abc",
		CurrentID:        "snap-xyz",
		GeneratedAt:      time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		TotalRegressions: 0,
		URLs:             nil,
	}
}

// --- buildAuditDeltaRows ---

// TestBuildAuditDeltaRows_FourRows verifies exactly 4 rows are returned.
func TestBuildAuditDeltaRows_FourRows(t *testing.T) {
	rows := buildAuditDeltaRows(diff.AuditDelta{})
	if len(rows) != 4 {
		t.Errorf("expected 4 rows, got %d", len(rows))
	}
}

// TestBuildAuditDeltaRows_Categories verifies category names in order.
func TestBuildAuditDeltaRows_Categories(t *testing.T) {
	rows := buildAuditDeltaRows(diff.AuditDelta{})
	want := []string{"Contrast", "Touch Targets", "Typography", "Spacing"}
	for i, w := range want {
		if rows[i].Category != w {
			t.Errorf("row %d: expected category %q, got %q", i, w, rows[i].Category)
		}
	}
}

// TestBuildAuditDeltaRows_PositiveDelta verifies delta = After - Before for regressions.
func TestBuildAuditDeltaRows_PositiveDelta(t *testing.T) {
	d := diff.AuditDelta{
		ContrastBefore: 2,
		ContrastAfter:  5,
	}
	rows := buildAuditDeltaRows(d)
	if rows[0].Before != 2 {
		t.Errorf("contrast Before: expected 2, got %d", rows[0].Before)
	}
	if rows[0].After != 5 {
		t.Errorf("contrast After: expected 5, got %d", rows[0].After)
	}
	if rows[0].Delta != 3 {
		t.Errorf("contrast Delta: expected 3, got %d", rows[0].Delta)
	}
}

// TestBuildAuditDeltaRows_NegativeDelta verifies delta is negative when issues are resolved.
func TestBuildAuditDeltaRows_NegativeDelta(t *testing.T) {
	d := diff.AuditDelta{
		TouchBefore: 4,
		TouchAfter:  1,
	}
	rows := buildAuditDeltaRows(d)
	// Touch Targets is index 1
	if rows[1].Delta != -3 {
		t.Errorf("touch Delta: expected -3, got %d", rows[1].Delta)
	}
}

// TestBuildAuditDeltaRows_ZeroDelta verifies delta is zero when counts are unchanged.
func TestBuildAuditDeltaRows_ZeroDelta(t *testing.T) {
	d := diff.AuditDelta{
		TypographyBefore: 2,
		TypographyAfter:  2,
	}
	rows := buildAuditDeltaRows(d)
	// Typography is index 2
	if rows[2].Delta != 0 {
		t.Errorf("typography Delta: expected 0, got %d", rows[2].Delta)
	}
}

// --- anyPositiveDelta ---

// TestAnyPositiveDelta_TrueWhenPositive verifies true when at least one delta > 0.
func TestAnyPositiveDelta_TrueWhenPositive(t *testing.T) {
	rows := []AuditDeltaRow{
		{Category: "Contrast", Delta: 0},
		{Category: "Touch Targets", Delta: 2},
	}
	if !anyPositiveDelta(rows) {
		t.Error("expected true when Delta=2, got false")
	}
}

// TestAnyPositiveDelta_FalseWhenAllZero verifies false when all deltas are zero.
func TestAnyPositiveDelta_FalseWhenAllZero(t *testing.T) {
	rows := []AuditDeltaRow{
		{Category: "Contrast", Delta: 0},
		{Category: "Touch Targets", Delta: 0},
	}
	if anyPositiveDelta(rows) {
		t.Error("expected false when all Delta=0, got true")
	}
}

// TestAnyPositiveDelta_FalseWhenNegative verifies false when deltas are negative (resolved).
func TestAnyPositiveDelta_FalseWhenNegative(t *testing.T) {
	rows := []AuditDeltaRow{
		{Category: "Contrast", Delta: -1},
		{Category: "Touch Targets", Delta: -3},
	}
	if anyPositiveDelta(rows) {
		t.Error("expected false when all Delta<0, got true")
	}
}

// TestAnyPositiveDelta_FalseWhenEmpty verifies false for empty slice.
func TestAnyPositiveDelta_FalseWhenEmpty(t *testing.T) {
	if anyPositiveDelta(nil) {
		t.Error("expected false for nil slice, got true")
	}
}

// --- GenerateDiff ---

// TestGenerateDiff_ContainsTitle verifies the generated HTML has the report title.
func TestGenerateDiff_ContainsTitle(t *testing.T) {
	var buf bytes.Buffer
	if err := GenerateDiff(&buf, minimalDiffData()); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Kaleidoscope Diff Report") {
		t.Error("HTML does not contain 'Kaleidoscope Diff Report'")
	}
}

// TestGenerateDiff_ContainsBaselineAndCurrentIDs verifies snapshot IDs appear in the output.
func TestGenerateDiff_ContainsBaselineAndCurrentIDs(t *testing.T) {
	data := minimalDiffData()
	var buf bytes.Buffer
	if err := GenerateDiff(&buf, data); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "snap-abc") {
		t.Error("HTML does not contain baseline ID 'snap-abc'")
	}
	if !strings.Contains(html, "snap-xyz") {
		t.Error("HTML does not contain current ID 'snap-xyz'")
	}
}

// TestGenerateDiff_NoRegressionsBadge verifies badge-pass when TotalRegressions is 0.
func TestGenerateDiff_NoRegressionsBadge(t *testing.T) {
	data := minimalDiffData()
	data.TotalRegressions = 0
	var buf bytes.Buffer
	if err := GenerateDiff(&buf, data); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "badge-pass") {
		t.Error("HTML does not contain badge-pass when no regressions")
	}
}

// TestGenerateDiff_RegressionsBadge verifies badge-fail when TotalRegressions > 0.
func TestGenerateDiff_RegressionsBadge(t *testing.T) {
	data := minimalDiffData()
	data.TotalRegressions = 2
	var buf bytes.Buffer
	if err := GenerateDiff(&buf, data); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "badge-fail") {
		t.Error("HTML does not contain badge-fail when regressions exist")
	}
}

// TestGenerateDiff_FormattedTime verifies the generated-at time is formatted as UTC.
func TestGenerateDiff_FormattedTime(t *testing.T) {
	data := minimalDiffData()
	var buf bytes.Buffer
	if err := GenerateDiff(&buf, data); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "2026-04-03 12:00:00 UTC") {
		t.Errorf("HTML does not contain expected timestamp '2026-04-03 12:00:00 UTC'; got:\n%s", html)
	}
}

// TestGenerateDiff_AuditDeltaTable verifies the audit delta table is rendered for a URL section.
func TestGenerateDiff_AuditDeltaTable(t *testing.T) {
	data := minimalDiffData()
	data.URLs = []URLDiffSection{
		{
			URL: "https://example.com",
			AuditDeltas: []AuditDeltaRow{
				{Category: "Contrast", Before: 1, After: 3, Delta: 2},
			},
		},
	}
	var buf bytes.Buffer
	if err := GenerateDiff(&buf, data); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "https://example.com") {
		t.Error("HTML missing URL 'https://example.com'")
	}
	if !strings.Contains(html, "Contrast") {
		t.Error("HTML missing audit category 'Contrast'")
	}
}

// TestGenerateDiff_ElementChangesTable verifies element changes are rendered.
func TestGenerateDiff_ElementChangesTable(t *testing.T) {
	data := minimalDiffData()
	data.URLs = []URLDiffSection{
		{
			URL: "https://example.com",
			ElementChanges: []ElementChangeRow{
				{Selector: "button.submit", Role: "button", Name: "Submit", Type: "disappeared"},
			},
		},
	}
	var buf bytes.Buffer
	if err := GenerateDiff(&buf, data); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "button.submit") {
		t.Error("HTML missing element selector 'button.submit'")
	}
	if !strings.Contains(html, "disappeared") {
		t.Error("HTML missing element change type 'disappeared'")
	}
}

// TestGenerateDiff_FooterPresent verifies the footer text is present.
func TestGenerateDiff_FooterPresent(t *testing.T) {
	var buf bytes.Buffer
	if err := GenerateDiff(&buf, minimalDiffData()); err != nil {
		t.Fatalf("GenerateDiff returned error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "kaleidoscope") {
		t.Error("HTML missing footer text 'kaleidoscope'")
	}
}

// --- BuildDiffData ---

// TestBuildDiffData_Base64Embedding verifies screenshots are embedded as data URIs.
func TestBuildDiffData_Base64Embedding(t *testing.T) {
	dir := t.TempDir()
	baselinePath := writeTempPNG(t, dir, "baseline.png")
	currentPath := writeTempPNG(t, dir, "current.png")

	sd := &diff.SnapshotDiff{
		BaselineID: "base-001",
		CurrentID:  "cur-002",
		URLs: []diff.URLDiff{
			{
				URL: "https://example.com",
				Breakpoints: []diff.BreakpointDiff{
					{
						Name:              "desktop",
						Width:             1280,
						Height:            720,
						BaselineImagePath: baselinePath,
						CurrentImagePath:  currentPath,
					},
				},
			},
		},
	}

	data, err := BuildDiffData(sd)
	if err != nil {
		t.Fatalf("BuildDiffData returned error: %v", err)
	}

	if len(data.URLs) != 1 {
		t.Fatalf("expected 1 URL section, got %d", len(data.URLs))
	}
	if len(data.URLs[0].ScreenshotSets) != 1 {
		t.Fatalf("expected 1 screenshot set, got %d", len(data.URLs[0].ScreenshotSets))
	}
	set := data.URLs[0].ScreenshotSets[0]
	if !strings.HasPrefix(string(set.BaselineURI), "data:image/png;base64,") {
		t.Errorf("BaselineURI is not a base64 data URI: %s", set.BaselineURI)
	}
	if !strings.HasPrefix(string(set.CurrentURI), "data:image/png;base64,") {
		t.Errorf("CurrentURI is not a base64 data URI: %s", set.CurrentURI)
	}
}

// TestBuildDiffData_DiffURIWhenDiffPath verifies DiffURI is set when DiffImagePath is non-empty.
func TestBuildDiffData_DiffURIWhenDiffPath(t *testing.T) {
	dir := t.TempDir()
	baselinePath := writeTempPNG(t, dir, "baseline.png")
	currentPath := writeTempPNG(t, dir, "current.png")
	diffPath := writeTempPNG(t, dir, "diff.png")

	sd := &diff.SnapshotDiff{
		BaselineID: "base-001",
		CurrentID:  "cur-002",
		URLs: []diff.URLDiff{
			{
				URL: "https://example.com",
				Breakpoints: []diff.BreakpointDiff{
					{
						Name:              "mobile",
						Width:             375,
						Height:            812,
						BaselineImagePath: baselinePath,
						CurrentImagePath:  currentPath,
						DiffImagePath:     diffPath,
						DiffPercent:       1.23,
					},
				},
			},
		},
	}

	data, err := BuildDiffData(sd)
	if err != nil {
		t.Fatalf("BuildDiffData returned error: %v", err)
	}

	set := data.URLs[0].ScreenshotSets[0]
	if !strings.HasPrefix(string(set.DiffURI), "data:image/png;base64,") {
		t.Errorf("DiffURI should be a base64 data URI when DiffImagePath is set, got: %s", set.DiffURI)
	}
	if set.DiffPercent != 1.23 {
		t.Errorf("DiffPercent: expected 1.23, got %f", set.DiffPercent)
	}
}

// TestBuildDiffData_EmptyDiffURIWhenNoDiffPath verifies DiffURI is empty when DiffImagePath is "".
func TestBuildDiffData_EmptyDiffURIWhenNoDiffPath(t *testing.T) {
	dir := t.TempDir()
	baselinePath := writeTempPNG(t, dir, "baseline.png")
	currentPath := writeTempPNG(t, dir, "current.png")

	sd := &diff.SnapshotDiff{
		BaselineID: "base-001",
		CurrentID:  "cur-002",
		URLs: []diff.URLDiff{
			{
				URL: "https://example.com",
				Breakpoints: []diff.BreakpointDiff{
					{
						Name:              "desktop",
						Width:             1280,
						Height:            720,
						BaselineImagePath: baselinePath,
						CurrentImagePath:  currentPath,
						DiffImagePath:     "",
					},
				},
			},
		},
	}

	data, err := BuildDiffData(sd)
	if err != nil {
		t.Fatalf("BuildDiffData returned error: %v", err)
	}
	set := data.URLs[0].ScreenshotSets[0]
	if set.DiffURI != "" {
		t.Errorf("DiffURI should be empty when DiffImagePath is empty, got: %s", set.DiffURI)
	}
}

// TestBuildDiffData_TotalRegressions verifies TotalRegressions is counted correctly.
func TestBuildDiffData_TotalRegressions(t *testing.T) {
	dir := t.TempDir()
	baselinePath := writeTempPNG(t, dir, "baseline.png")
	currentPath := writeTempPNG(t, dir, "current.png")

	makeBP := func() diff.BreakpointDiff {
		return diff.BreakpointDiff{
			Name: "desktop", Width: 1280, Height: 720,
			BaselineImagePath: baselinePath,
			CurrentImagePath:  currentPath,
		}
	}

	sd := &diff.SnapshotDiff{
		BaselineID: "base",
		CurrentID:  "cur",
		URLs: []diff.URLDiff{
			{
				URL:         "https://example.com/a",
				Breakpoints: []diff.BreakpointDiff{makeBP()},
				// Regression: contrast went up
				AuditDelta: diff.AuditDelta{ContrastBefore: 1, ContrastAfter: 3},
			},
			{
				URL:         "https://example.com/b",
				Breakpoints: []diff.BreakpointDiff{makeBP()},
				// No regression: all deltas zero or negative
				AuditDelta: diff.AuditDelta{ContrastBefore: 2, ContrastAfter: 2},
			},
			{
				URL:         "https://example.com/c",
				Breakpoints: []diff.BreakpointDiff{makeBP()},
				// Regression on touch targets
				AuditDelta: diff.AuditDelta{TouchBefore: 0, TouchAfter: 1},
			},
		},
	}

	data, err := BuildDiffData(sd)
	if err != nil {
		t.Fatalf("BuildDiffData returned error: %v", err)
	}
	if data.TotalRegressions != 2 {
		t.Errorf("TotalRegressions: expected 2, got %d", data.TotalRegressions)
	}
}

// TestBuildDiffData_ErrorOnMissingScreenshot verifies an error is returned for missing image files.
func TestBuildDiffData_ErrorOnMissingScreenshot(t *testing.T) {
	sd := &diff.SnapshotDiff{
		BaselineID: "base",
		CurrentID:  "cur",
		URLs: []diff.URLDiff{
			{
				URL: "https://example.com",
				Breakpoints: []diff.BreakpointDiff{
					{
						Name:              "desktop",
						Width:             1280,
						Height:            720,
						BaselineImagePath: "/nonexistent/path/baseline.png",
						CurrentImagePath:  "/nonexistent/path/current.png",
					},
				},
			},
		},
	}

	_, err := BuildDiffData(sd)
	if err == nil {
		t.Error("expected error for missing screenshot files, got nil")
	}
}

// TestBuildDiffData_IDs verifies BaselineID and CurrentID are propagated.
func TestBuildDiffData_IDs(t *testing.T) {
	sd := &diff.SnapshotDiff{
		BaselineID: "baseline-snap",
		CurrentID:  "current-snap",
		URLs:       nil,
	}
	data, err := BuildDiffData(sd)
	if err != nil {
		t.Fatalf("BuildDiffData returned error: %v", err)
	}
	if data.BaselineID != "baseline-snap" {
		t.Errorf("BaselineID: expected 'baseline-snap', got %q", data.BaselineID)
	}
	if data.CurrentID != "current-snap" {
		t.Errorf("CurrentID: expected 'current-snap', got %q", data.CurrentID)
	}
}

// --- WriteDiffFile ---

// TestWriteDiffFile_CreatesFile verifies WriteDiffFile produces a file at the given path.
func TestWriteDiffFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "output", "report.html")

	data := minimalDiffData()
	absPath, err := WriteDiffFile(outPath, data)
	if err != nil {
		t.Fatalf("WriteDiffFile returned error: %v", err)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Errorf("output file does not exist at %s", absPath)
	}
}

// TestWriteDiffFile_ContentIsHTML verifies the output file starts with <!DOCTYPE html>.
func TestWriteDiffFile_ContentIsHTML(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.html")

	data := minimalDiffData()
	absPath, err := WriteDiffFile(outPath, data)
	if err != nil {
		t.Fatalf("WriteDiffFile returned error: %v", err)
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(content)), "<!DOCTYPE html>") {
		t.Errorf("output file does not start with '<!DOCTYPE html>'")
	}
}
