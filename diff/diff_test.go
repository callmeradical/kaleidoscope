package diff

import (
	"testing"
)

// --- Fixture helpers ---

func auditData(contrast, touch, typo int) map[string]any {
	return map[string]any{
		"summary": map[string]any{
			"totalIssues":        float64(contrast + touch + typo),
			"contrastViolations": float64(contrast),
			"touchViolations":    float64(touch),
			"typographyWarnings": float64(typo),
		},
	}
}

func axTreeData(nodes ...map[string]any) map[string]any {
	asAny := make([]any, len(nodes))
	for i, n := range nodes {
		asAny[i] = n
	}
	return map[string]any{
		"nodeCount": float64(len(nodes)),
		"nodes":     asAny,
	}
}

func makeAxNode(role, name string) map[string]any {
	return map[string]any{"role": role, "name": name}
}

func axNodeWithBounds(role, name string, x, y, w, h float64) map[string]any {
	return map[string]any{
		"role": role,
		"name": name,
		"properties": map[string]any{
			"boundingBox": map[string]any{
				"x": x, "y": y, "width": w, "height": h,
			},
		},
	}
}

// --- Tests ---

func TestCompare_NoChange(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(2, 1, 3),
		AxTree: axTreeData(makeAxNode("button", "Submit"), makeAxNode("link", "Home")),
	}
	curr := SnapshotData{
		Audit:  auditData(2, 1, 3),
		AxTree: axTreeData(makeAxNode("button", "Submit"), makeAxNode("link", "Home")),
	}

	result := Compare(base, curr)

	if result.HasRegressions {
		t.Error("expected no regressions")
	}
	if result.Summary.NewAuditIssues != 0 {
		t.Errorf("NewAuditIssues: got %d, want 0", result.Summary.NewAuditIssues)
	}
	if result.Summary.ResolvedAuditIssues != 0 {
		t.Errorf("ResolvedAuditIssues: got %d, want 0", result.Summary.ResolvedAuditIssues)
	}
	if len(result.ElementChanges) != 0 {
		t.Errorf("expected no element changes, got %d", len(result.ElementChanges))
	}
}

func TestCompare_NewAuditIssues(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(1, 0, 2),
		AxTree: axTreeData(),
	}
	curr := SnapshotData{
		Audit:  auditData(3, 1, 2),
		AxTree: axTreeData(),
	}

	result := Compare(base, curr)

	if !result.HasRegressions {
		t.Error("expected regressions")
	}
	if result.Summary.NewAuditIssues != 3 { // +2 contrast, +1 touch
		t.Errorf("NewAuditIssues: got %d, want 3", result.Summary.NewAuditIssues)
	}

	d := result.AuditDeltas["contrast"]
	if d.BaselineCount != 1 || d.CurrentCount != 3 || d.Delta != 2 {
		t.Errorf("contrast delta: got %+v", d)
	}

	d = result.AuditDeltas["touchTargets"]
	if d.BaselineCount != 0 || d.CurrentCount != 1 || d.Delta != 1 {
		t.Errorf("touchTargets delta: got %+v", d)
	}

	d = result.AuditDeltas["typography"]
	if d.Delta != 0 {
		t.Errorf("typography should be unchanged, delta=%d", d.Delta)
	}
}

func TestCompare_ResolvedAuditIssues(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(5, 3, 4),
		AxTree: axTreeData(),
	}
	curr := SnapshotData{
		Audit:  auditData(2, 0, 4),
		AxTree: axTreeData(),
	}

	result := Compare(base, curr)

	if result.HasRegressions {
		t.Error("resolved-only changes should not be regressions")
	}
	if result.Summary.ResolvedAuditIssues != 6 { // -3 contrast, -3 touch
		t.Errorf("ResolvedAuditIssues: got %d, want 6", result.Summary.ResolvedAuditIssues)
	}
	if result.Summary.NewAuditIssues != 0 {
		t.Errorf("NewAuditIssues: got %d, want 0", result.Summary.NewAuditIssues)
	}
}

func TestCompare_ElementAppeared(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(makeAxNode("button", "Submit")),
	}
	curr := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(makeAxNode("button", "Submit"), makeAxNode("link", "Help")),
	}

	result := Compare(base, curr)

	if result.Summary.ElementsAppeared != 1 {
		t.Errorf("ElementsAppeared: got %d, want 1", result.Summary.ElementsAppeared)
	}

	found := false
	for _, ec := range result.ElementChanges {
		if ec.Kind == "appeared" && ec.Role == "link" && ec.Name == "Help" {
			found = true
		}
	}
	if !found {
		t.Error("expected appeared element link:Help")
	}
}

func TestCompare_ElementDisappeared(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(makeAxNode("button", "Submit"), makeAxNode("link", "Help")),
	}
	curr := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(makeAxNode("button", "Submit")),
	}

	result := Compare(base, curr)

	if !result.HasRegressions {
		t.Error("disappeared element should be a regression")
	}
	if result.Summary.ElementsDisappeared != 1 {
		t.Errorf("ElementsDisappeared: got %d, want 1", result.Summary.ElementsDisappeared)
	}

	found := false
	for _, ec := range result.ElementChanges {
		if ec.Kind == "disappeared" && ec.Role == "link" && ec.Name == "Help" {
			found = true
		}
	}
	if !found {
		t.Error("expected disappeared element link:Help")
	}
}

func TestCompare_ElementMoved(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("button", "Submit", 10, 20, 100, 40)),
	}
	curr := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("button", "Submit", 10, 80, 100, 40)),
	}

	result := Compare(base, curr)

	if result.Summary.ElementsMoved != 1 {
		t.Errorf("ElementsMoved: got %d, want 1", result.Summary.ElementsMoved)
	}

	found := false
	for _, ec := range result.ElementChanges {
		if ec.Kind == "moved" && ec.Name == "Submit" {
			found = true
			if ec.DeltaY != 60 {
				t.Errorf("DeltaY: got %f, want 60", ec.DeltaY)
			}
			if ec.DeltaX != 0 {
				t.Errorf("DeltaX: got %f, want 0", ec.DeltaX)
			}
		}
	}
	if !found {
		t.Error("expected moved element button:Submit")
	}
}

func TestCompare_ElementResized(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("button", "Submit", 10, 20, 100, 40)),
	}
	curr := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("button", "Submit", 10, 20, 200, 60)),
	}

	result := Compare(base, curr)

	if result.Summary.ElementsResized != 1 {
		t.Errorf("ElementsResized: got %d, want 1", result.Summary.ElementsResized)
	}

	found := false
	for _, ec := range result.ElementChanges {
		if ec.Kind == "resized" && ec.Name == "Submit" {
			found = true
			if ec.DeltaW != 100 {
				t.Errorf("DeltaW: got %f, want 100", ec.DeltaW)
			}
			if ec.DeltaH != 20 {
				t.Errorf("DeltaH: got %f, want 20", ec.DeltaH)
			}
		}
	}
	if !found {
		t.Error("expected resized element button:Submit")
	}
}

func TestCompare_ElementMovedAndResized(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("heading", "Title", 0, 0, 200, 30)),
	}
	curr := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("heading", "Title", 50, 10, 300, 50)),
	}

	result := Compare(base, curr)

	if result.Summary.ElementsMoved != 1 {
		t.Errorf("ElementsMoved: got %d, want 1", result.Summary.ElementsMoved)
	}
	if result.Summary.ElementsResized != 1 {
		t.Errorf("ElementsResized: got %d, want 1", result.Summary.ElementsResized)
	}
}

func TestCompare_BelowMoveThreshold(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("button", "OK", 10, 20, 100, 40)),
	}
	curr := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("button", "OK", 12, 23, 100, 40)),
	}

	result := Compare(base, curr)

	if result.Summary.ElementsMoved != 0 {
		t.Errorf("movement below threshold should be ignored, got %d moved", result.Summary.ElementsMoved)
	}
}

func TestCompare_BelowResizeThreshold(t *testing.T) {
	base := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("button", "OK", 10, 20, 100, 40)),
	}
	curr := SnapshotData{
		Audit:  auditData(0, 0, 0),
		AxTree: axTreeData(axNodeWithBounds("button", "OK", 10, 20, 103, 42)),
	}

	result := Compare(base, curr)

	if result.Summary.ElementsResized != 0 {
		t.Errorf("resize below threshold should be ignored, got %d resized", result.Summary.ElementsResized)
	}
}

func TestCompare_NilAuditAndAxTree(t *testing.T) {
	base := SnapshotData{}
	curr := SnapshotData{}

	result := Compare(base, curr)

	if result.HasRegressions {
		t.Error("nil data should not produce regressions")
	}
	if result.Summary.NewAuditIssues != 0 {
		t.Errorf("NewAuditIssues: got %d, want 0", result.Summary.NewAuditIssues)
	}
}

func TestCompare_MixedChanges(t *testing.T) {
	base := SnapshotData{
		Audit: auditData(1, 2, 0),
		AxTree: axTreeData(
			axNodeWithBounds("button", "Submit", 10, 20, 100, 40),
			makeAxNode("link", "OldLink"),
		),
	}
	curr := SnapshotData{
		Audit: auditData(3, 0, 1),
		AxTree: axTreeData(
			axNodeWithBounds("button", "Submit", 10, 100, 100, 40),
			makeAxNode("link", "NewLink"),
		),
	}

	result := Compare(base, curr)

	if !result.HasRegressions {
		t.Error("expected regressions")
	}
	// +2 contrast, +1 typo = 3 new
	if result.Summary.NewAuditIssues != 3 {
		t.Errorf("NewAuditIssues: got %d, want 3", result.Summary.NewAuditIssues)
	}
	// -2 touch = 2 resolved
	if result.Summary.ResolvedAuditIssues != 2 {
		t.Errorf("ResolvedAuditIssues: got %d, want 2", result.Summary.ResolvedAuditIssues)
	}
	if result.Summary.ElementsMoved != 1 {
		t.Errorf("ElementsMoved: got %d, want 1", result.Summary.ElementsMoved)
	}
	if result.Summary.ElementsAppeared != 1 {
		t.Errorf("ElementsAppeared: got %d, want 1", result.Summary.ElementsAppeared)
	}
	if result.Summary.ElementsDisappeared != 1 {
		t.Errorf("ElementsDisappeared: got %d, want 1", result.Summary.ElementsDisappeared)
	}
}

func TestExtractAuditCounts_FallbackFormat(t *testing.T) {
	audit := map[string]any{
		"contrast": map[string]any{
			"violations": float64(5),
		},
		"touchTargets": map[string]any{
			"violations": float64(2),
		},
		"typography": map[string]any{
			"warnings": float64(3),
		},
	}

	counts := extractAuditCounts(audit)

	if counts["contrast"] != 5 {
		t.Errorf("contrast: got %d, want 5", counts["contrast"])
	}
	if counts["touchTargets"] != 2 {
		t.Errorf("touchTargets: got %d, want 2", counts["touchTargets"])
	}
	if counts["typography"] != 3 {
		t.Errorf("typography: got %d, want 3", counts["typography"])
	}
}

func TestCompare_ExitCodeLogic(t *testing.T) {
	// Only resolved = no regressions (exit 0)
	r1 := Compare(
		SnapshotData{Audit: auditData(5, 0, 0), AxTree: axTreeData()},
		SnapshotData{Audit: auditData(0, 0, 0), AxTree: axTreeData()},
	)
	if r1.HasRegressions {
		t.Error("resolved-only should not be a regression")
	}

	// New audit issues = regression (exit 1)
	r2 := Compare(
		SnapshotData{Audit: auditData(0, 0, 0), AxTree: axTreeData()},
		SnapshotData{Audit: auditData(1, 0, 0), AxTree: axTreeData()},
	)
	if !r2.HasRegressions {
		t.Error("new audit issue should be a regression")
	}

	// Disappeared element = regression (exit 1)
	r3 := Compare(
		SnapshotData{Audit: auditData(0, 0, 0), AxTree: axTreeData(makeAxNode("button", "X"))},
		SnapshotData{Audit: auditData(0, 0, 0), AxTree: axTreeData()},
	)
	if !r3.HasRegressions {
		t.Error("disappeared element should be a regression")
	}

	// Appeared element only = no regression (exit 0)
	r4 := Compare(
		SnapshotData{Audit: auditData(0, 0, 0), AxTree: axTreeData()},
		SnapshotData{Audit: auditData(0, 0, 0), AxTree: axTreeData(makeAxNode("button", "X"))},
	)
	if r4.HasRegressions {
		t.Error("appeared-only should not be a regression")
	}

	// Moved element only = no regression (exit 0)
	r5 := Compare(
		SnapshotData{Audit: auditData(0, 0, 0), AxTree: axTreeData(axNodeWithBounds("button", "X", 0, 0, 50, 50))},
		SnapshotData{Audit: auditData(0, 0, 0), AxTree: axTreeData(axNodeWithBounds("button", "X", 100, 100, 50, 50))},
	)
	if r5.HasRegressions {
		t.Error("moved-only should not be a regression")
	}
}
