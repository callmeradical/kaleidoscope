package diff_test

import (
	"testing"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func makeSnapshot(id string, pages []snapshot.PageSnapshot) *snapshot.Snapshot {
	return &snapshot.Snapshot{ID: id, Pages: pages}
}

func makePage(url string, audit snapshot.AuditResult, nodes []snapshot.AXNodeRecord) snapshot.PageSnapshot {
	return snapshot.PageSnapshot{URL: url, AuditResult: audit, AXNodes: nodes}
}

// TestCompare_IdenticalSnapshots verifies all deltas are zero for identical snapshots.
func TestCompare_IdenticalSnapshots(t *testing.T) {
	audit := snapshot.AuditResult{
		ContrastViolations: 2,
		TouchViolations:    1,
		TypographyWarnings: 3,
		SpacingIssues:      0,
	}
	page := makePage("https://example.com", audit, nil)
	baseline := makeSnapshot("base-001", []snapshot.PageSnapshot{page})
	current := makeSnapshot("cur-001", []snapshot.PageSnapshot{page})

	d, err := diff.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}
	if len(d.Pages) != 1 {
		t.Fatalf("expected 1 page diff, got %d", len(d.Pages))
	}
	for _, delta := range d.Pages[0].AuditDelta {
		if delta.Delta != 0 {
			t.Errorf("category %q: expected delta 0, got %d", delta.Category, delta.Delta)
		}
	}
	if len(d.Pages[0].ElementChanges) != 0 {
		t.Errorf("expected no element changes for identical snapshots, got %d", len(d.Pages[0].ElementChanges))
	}
}

// TestCompare_ContrastRegression verifies delta=+1 when a contrast violation is added.
func TestCompare_ContrastRegression(t *testing.T) {
	baseAudit := snapshot.AuditResult{ContrastViolations: 1}
	curAudit := snapshot.AuditResult{ContrastViolations: 2}

	baseline := makeSnapshot("base-001", []snapshot.PageSnapshot{makePage("https://example.com", baseAudit, nil)})
	current := makeSnapshot("cur-001", []snapshot.PageSnapshot{makePage("https://example.com", curAudit, nil)})

	d, err := diff.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}

	var contrastDelta int
	found := false
	for _, delta := range d.Pages[0].AuditDelta {
		if delta.Category == "contrast" {
			contrastDelta = delta.Delta
			found = true
		}
	}
	if !found {
		t.Fatal("contrast category not found in audit delta")
	}
	if contrastDelta != 1 {
		t.Errorf("expected contrast delta=1, got %d", contrastDelta)
	}
}

// TestCompareElements_Appeared verifies a node only in current is reported as "appeared".
func TestCompareElements_Appeared(t *testing.T) {
	curNodes := []snapshot.AXNodeRecord{
		{Role: "button", Name: "Submit", Selector: "button.submit"},
	}
	baseline := makeSnapshot("base-001", []snapshot.PageSnapshot{makePage("https://example.com", snapshot.AuditResult{}, nil)})
	current := makeSnapshot("cur-001", []snapshot.PageSnapshot{makePage("https://example.com", snapshot.AuditResult{}, curNodes)})

	d, err := diff.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}
	changes := d.Pages[0].ElementChanges
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "appeared" {
		t.Errorf("expected type 'appeared', got %q", changes[0].Type)
	}
}

// TestCompareElements_Disappeared verifies a node only in baseline is reported as "disappeared".
func TestCompareElements_Disappeared(t *testing.T) {
	baseNodes := []snapshot.AXNodeRecord{
		{Role: "button", Name: "Cancel", Selector: "button.cancel"},
	}
	baseline := makeSnapshot("base-001", []snapshot.PageSnapshot{makePage("https://example.com", snapshot.AuditResult{}, baseNodes)})
	current := makeSnapshot("cur-001", []snapshot.PageSnapshot{makePage("https://example.com", snapshot.AuditResult{}, nil)})

	d, err := diff.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}
	changes := d.Pages[0].ElementChanges
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "disappeared" {
		t.Errorf("expected type 'disappeared', got %q", changes[0].Type)
	}
}

// TestCompareElements_Resized verifies width change >2px is reported as "resized".
func TestCompareElements_Resized(t *testing.T) {
	baseNodes := []snapshot.AXNodeRecord{
		{Role: "link", Name: "Home", Selector: "a.home", Bounds: snapshot.Rect{X: 0, Y: 0, Width: 100, Height: 30}},
	}
	curNodes := []snapshot.AXNodeRecord{
		{Role: "link", Name: "Home", Selector: "a.home", Bounds: snapshot.Rect{X: 0, Y: 0, Width: 150, Height: 30}},
	}
	baseline := makeSnapshot("base-001", []snapshot.PageSnapshot{makePage("https://example.com", snapshot.AuditResult{}, baseNodes)})
	current := makeSnapshot("cur-001", []snapshot.PageSnapshot{makePage("https://example.com", snapshot.AuditResult{}, curNodes)})

	d, err := diff.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}
	changes := d.Pages[0].ElementChanges
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "resized" {
		t.Errorf("expected type 'resized', got %q", changes[0].Type)
	}
}

// TestCompareElements_Moved verifies X/Y-only change >2px is reported as "moved".
func TestCompareElements_Moved(t *testing.T) {
	baseNodes := []snapshot.AXNodeRecord{
		{Role: "link", Name: "About", Selector: "a.about", Bounds: snapshot.Rect{X: 0, Y: 0, Width: 80, Height: 30}},
	}
	curNodes := []snapshot.AXNodeRecord{
		{Role: "link", Name: "About", Selector: "a.about", Bounds: snapshot.Rect{X: 50, Y: 0, Width: 80, Height: 30}},
	}
	baseline := makeSnapshot("base-001", []snapshot.PageSnapshot{makePage("https://example.com", snapshot.AuditResult{}, baseNodes)})
	current := makeSnapshot("cur-001", []snapshot.PageSnapshot{makePage("https://example.com", snapshot.AuditResult{}, curNodes)})

	d, err := diff.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}
	changes := d.Pages[0].ElementChanges
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "moved" {
		t.Errorf("expected type 'moved', got %q", changes[0].Type)
	}
}
