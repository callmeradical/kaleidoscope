package diff

import (
	"testing"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// ---- helpers ----

func bb(x, y, w, h float64) *snapshot.BoundingBox {
	return &snapshot.BoundingBox{X: x, Y: y, Width: w, Height: h}
}

func axNode(role, name string, box *snapshot.BoundingBox) snapshot.AXNode {
	return snapshot.AXNode{Role: role, Name: name, BoundingBox: box}
}

func contrastIssue(selector, msg string) snapshot.AuditIssue {
	return snapshot.AuditIssue{Selector: selector, Message: msg, Category: "contrast"}
}

func touchIssue(selector, msg string) snapshot.AuditIssue {
	return snapshot.AuditIssue{Selector: selector, Message: msg, Category: "touch"}
}

func typographyIssue(selector, msg string) snapshot.AuditIssue {
	return snapshot.AuditIssue{Selector: selector, Message: msg, Category: "typography"}
}

func spacingIssue(selector, msg string) snapshot.AuditIssue {
	return snapshot.AuditIssue{Selector: selector, Message: msg, Category: "spacing"}
}

// ---- ComputeAuditDelta tests ----

// TestComputeAuditDelta_EmptyBoth verifies that comparing two empty AuditData values
// produces zero deltas and no regression.
func TestComputeAuditDelta_EmptyBoth(t *testing.T) {
	result := ComputeAuditDelta(snapshot.AuditData{}, snapshot.AuditData{})

	if result.HasRegression {
		t.Error("expected HasRegression=false for empty audits")
	}
	if len(result.NewIssues) != 0 {
		t.Errorf("expected 0 new issues, got %d", len(result.NewIssues))
	}
	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved issues, got %d", len(result.Resolved))
	}

	for _, cat := range []string{"contrast", "touch", "typography", "spacing"} {
		cd, ok := result.Categories[cat]
		if !ok {
			t.Errorf("expected category %q to be present", cat)
			continue
		}
		if cd.Delta != 0 {
			t.Errorf("category %q: expected delta=0, got %d", cat, cd.Delta)
		}
		if cd.Baseline != 0 || cd.Current != 0 {
			t.Errorf("category %q: expected baseline=0 current=0, got baseline=%d current=%d", cat, cd.Baseline, cd.Current)
		}
	}
}

// TestComputeAuditDelta_NoChange verifies that identical issues in both snapshots
// produce zero deltas and no regression.
func TestComputeAuditDelta_NoChange(t *testing.T) {
	data := snapshot.AuditData{
		ContrastIssues:   []snapshot.AuditIssue{contrastIssue(".btn", "low contrast")},
		TouchIssues:      []snapshot.AuditIssue{touchIssue(".icon", "too small")},
		TypographyIssues: []snapshot.AuditIssue{typographyIssue("h1", "font size")},
		SpacingIssues:    []snapshot.AuditIssue{spacingIssue(".card", "margin inconsistent")},
	}

	result := ComputeAuditDelta(data, data)

	if result.HasRegression {
		t.Error("expected HasRegression=false when snapshot equals baseline")
	}
	if len(result.NewIssues) != 0 {
		t.Errorf("expected 0 new issues, got %d", len(result.NewIssues))
	}
	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved issues, got %d", len(result.Resolved))
	}
	for _, cat := range []string{"contrast", "touch", "typography", "spacing"} {
		cd := result.Categories[cat]
		if cd.Delta != 0 {
			t.Errorf("category %q: expected delta=0, got %d", cat, cd.Delta)
		}
	}
}

// TestComputeAuditDelta_NewIssue verifies that a new contrast issue in the current
// snapshot is detected as a regression.
func TestComputeAuditDelta_NewIssue(t *testing.T) {
	baseline := snapshot.AuditData{}
	current := snapshot.AuditData{
		ContrastIssues: []snapshot.AuditIssue{contrastIssue(".header", "contrast ratio 2.5:1")},
	}

	result := ComputeAuditDelta(baseline, current)

	if !result.HasRegression {
		t.Error("expected HasRegression=true when new contrast issue is present")
	}
	if len(result.NewIssues) != 1 {
		t.Fatalf("expected 1 new issue, got %d", len(result.NewIssues))
	}
	ni := result.NewIssues[0]
	if ni.Status != "new" {
		t.Errorf("expected status='new', got %q", ni.Status)
	}
	if ni.Selector != ".header" {
		t.Errorf("expected selector='.header', got %q", ni.Selector)
	}
	if ni.Category != "contrast" {
		t.Errorf("expected category='contrast', got %q", ni.Category)
	}

	cd := result.Categories["contrast"]
	if cd.Delta != 1 {
		t.Errorf("expected contrast delta=1, got %d", cd.Delta)
	}
	if cd.Baseline != 0 || cd.Current != 1 {
		t.Errorf("expected baseline=0 current=1, got baseline=%d current=%d", cd.Baseline, cd.Current)
	}
}

// TestComputeAuditDelta_ResolvedIssue verifies that an issue present in baseline but
// absent from current is reported as resolved and does not trigger a regression.
func TestComputeAuditDelta_ResolvedIssue(t *testing.T) {
	baseline := snapshot.AuditData{
		SpacingIssues: []snapshot.AuditIssue{spacingIssue(".sidebar", "inconsistent padding")},
	}
	current := snapshot.AuditData{}

	result := ComputeAuditDelta(baseline, current)

	if result.HasRegression {
		t.Error("expected HasRegression=false when an issue was resolved")
	}
	if len(result.Resolved) != 1 {
		t.Fatalf("expected 1 resolved issue, got %d", len(result.Resolved))
	}
	ri := result.Resolved[0]
	if ri.Status != "resolved" {
		t.Errorf("expected status='resolved', got %q", ri.Status)
	}
	if ri.Selector != ".sidebar" {
		t.Errorf("expected selector='.sidebar', got %q", ri.Selector)
	}

	cd := result.Categories["spacing"]
	if cd.Delta != -1 {
		t.Errorf("expected spacing delta=-1, got %d", cd.Delta)
	}
}

// TestComputeAuditDelta_MultiCategory tests mixed changes across all 4 categories.
func TestComputeAuditDelta_MultiCategory(t *testing.T) {
	baseline := snapshot.AuditData{
		ContrastIssues:   []snapshot.AuditIssue{contrastIssue(".btn", "old contrast")},
		TypographyIssues: []snapshot.AuditIssue{typographyIssue("p", "line-height")},
	}
	current := snapshot.AuditData{
		// contrast issue resolved
		TouchIssues:      []snapshot.AuditIssue{touchIssue(".icon", "touch target"), touchIssue(".link", "touch target 2")},
		TypographyIssues: []snapshot.AuditIssue{typographyIssue("p", "line-height")}, // unchanged
		SpacingIssues:    []snapshot.AuditIssue{spacingIssue(".card", "gap")},
	}

	result := ComputeAuditDelta(baseline, current)

	// contrast was resolved (−1), touch is new (+2), spacing is new (+1) → regression
	if !result.HasRegression {
		t.Error("expected HasRegression=true due to new touch and spacing issues")
	}

	contrastDelta := result.Categories["contrast"]
	if contrastDelta.Delta != -1 {
		t.Errorf("expected contrast delta=-1, got %d", contrastDelta.Delta)
	}

	touchDelta := result.Categories["touch"]
	if touchDelta.Delta != 2 {
		t.Errorf("expected touch delta=2, got %d", touchDelta.Delta)
	}

	typoDelta := result.Categories["typography"]
	if typoDelta.Delta != 0 {
		t.Errorf("expected typography delta=0, got %d", typoDelta.Delta)
	}

	spacingDelta := result.Categories["spacing"]
	if spacingDelta.Delta != 1 {
		t.Errorf("expected spacing delta=1, got %d", spacingDelta.Delta)
	}

	// 1 resolved (contrast), 3 new (2 touch + 1 spacing)
	if len(result.Resolved) != 1 {
		t.Errorf("expected 1 resolved issue, got %d", len(result.Resolved))
	}
	if len(result.NewIssues) != 3 {
		t.Errorf("expected 3 new issues, got %d", len(result.NewIssues))
	}
}

// ---- ComputeElementDelta tests ----

// TestComputeElementDelta_Appeared verifies that an element present in current but
// not baseline appears in the Appeared list and does NOT trigger a regression.
func TestComputeElementDelta_Appeared(t *testing.T) {
	baseline := []snapshot.AXNode{}
	current := []snapshot.AXNode{axNode("button", "Submit", bb(0, 0, 100, 40))}

	result := ComputeElementDelta(baseline, current)

	if result.HasRegression {
		t.Error("expected HasRegression=false: appeared elements are informational")
	}
	if len(result.Appeared) != 1 {
		t.Fatalf("expected 1 appeared element, got %d", len(result.Appeared))
	}
	ec := result.Appeared[0]
	if ec.Role != "button" || ec.Name != "Submit" {
		t.Errorf("unexpected appeared element: role=%q name=%q", ec.Role, ec.Name)
	}
	if ec.Before != nil {
		t.Error("expected Before=nil for appeared element")
	}
	if ec.After == nil {
		t.Error("expected After to be set for appeared element")
	}
}

// TestComputeElementDelta_Disappeared verifies that an element in baseline but not
// current is in Disappeared and triggers a regression.
func TestComputeElementDelta_Disappeared(t *testing.T) {
	baseline := []snapshot.AXNode{axNode("link", "Home", bb(10, 20, 80, 30))}
	current := []snapshot.AXNode{}

	result := ComputeElementDelta(baseline, current)

	if !result.HasRegression {
		t.Error("expected HasRegression=true: disappeared elements are regressions")
	}
	if len(result.Disappeared) != 1 {
		t.Fatalf("expected 1 disappeared element, got %d", len(result.Disappeared))
	}
	ec := result.Disappeared[0]
	if ec.Role != "link" || ec.Name != "Home" {
		t.Errorf("unexpected disappeared element: role=%q name=%q", ec.Role, ec.Name)
	}
	if ec.Before == nil {
		t.Error("expected Before to be set for disappeared element")
	}
	if ec.After != nil {
		t.Error("expected After=nil for disappeared element")
	}
}

// TestComputeElementDelta_Moved verifies that an element with a position delta
// beyond PositionThreshold is detected as moved.
func TestComputeElementDelta_Moved(t *testing.T) {
	baseline := []snapshot.AXNode{axNode("button", "OK", bb(100, 200, 80, 40))}
	// Shifted X by 10px (> PositionThreshold=4)
	current := []snapshot.AXNode{axNode("button", "OK", bb(110, 200, 80, 40))}

	result := ComputeElementDelta(baseline, current)

	if !result.HasRegression {
		t.Error("expected HasRegression=true: moved element triggers regression")
	}
	if len(result.Moved) != 1 {
		t.Fatalf("expected 1 moved element, got %d", len(result.Moved))
	}
	ec := result.Moved[0]
	if ec.Delta == nil {
		t.Fatal("expected Delta to be set for moved element")
	}
	if ec.Delta.DX != 10 {
		t.Errorf("expected DX=10, got %v", ec.Delta.DX)
	}
	if ec.Delta.DY != 0 {
		t.Errorf("expected DY=0, got %v", ec.Delta.DY)
	}
}

// TestComputeElementDelta_MovedBelowThreshold verifies that a position delta at or
// below PositionThreshold is NOT reported as moved.
func TestComputeElementDelta_MovedBelowThreshold(t *testing.T) {
	baseline := []snapshot.AXNode{axNode("button", "OK", bb(100, 200, 80, 40))}
	// Shifted X by 2px (≤ PositionThreshold=4)
	current := []snapshot.AXNode{axNode("button", "OK", bb(102, 200, 80, 40))}

	result := ComputeElementDelta(baseline, current)

	if result.HasRegression {
		t.Error("expected HasRegression=false: sub-threshold move is not a regression")
	}
	if len(result.Moved) != 0 {
		t.Errorf("expected 0 moved elements, got %d", len(result.Moved))
	}
}

// TestComputeElementDelta_Resized verifies that an element with a size delta beyond
// SizeThreshold is detected as resized.
func TestComputeElementDelta_Resized(t *testing.T) {
	baseline := []snapshot.AXNode{axNode("img", "Logo", bb(0, 0, 200, 100))}
	// Width increased by 10px (> SizeThreshold=4)
	current := []snapshot.AXNode{axNode("img", "Logo", bb(0, 0, 210, 100))}

	result := ComputeElementDelta(baseline, current)

	if !result.HasRegression {
		t.Error("expected HasRegression=true: resized element triggers regression")
	}
	if len(result.Resized) != 1 {
		t.Fatalf("expected 1 resized element, got %d", len(result.Resized))
	}
	ec := result.Resized[0]
	if ec.Delta == nil {
		t.Fatal("expected Delta to be set for resized element")
	}
	if ec.Delta.DW != 10 {
		t.Errorf("expected DW=10, got %v", ec.Delta.DW)
	}
}

// TestComputeElementDelta_MovedAndResized verifies that an element with both a
// position change and a size change beyond their respective thresholds appears in
// both Moved and Resized.
func TestComputeElementDelta_MovedAndResized(t *testing.T) {
	baseline := []snapshot.AXNode{axNode("section", "Hero", bb(0, 0, 960, 400))}
	// Moved down 50px and shrunk 20px in height
	current := []snapshot.AXNode{axNode("section", "Hero", bb(0, 50, 960, 380))}

	result := ComputeElementDelta(baseline, current)

	if !result.HasRegression {
		t.Error("expected HasRegression=true")
	}
	if len(result.Moved) != 1 {
		t.Errorf("expected 1 moved element, got %d", len(result.Moved))
	}
	if len(result.Resized) != 1 {
		t.Errorf("expected 1 resized element, got %d", len(result.Resized))
	}
	// Verify both entries refer to the same element
	if result.Moved[0].Identity != result.Resized[0].Identity {
		t.Error("expected Moved and Resized entries to share the same identity")
	}
}

// TestComputeElementDelta_NoBoundingBox verifies that matching elements with nil
// BoundingBox do not produce Move or Resize entries.
func TestComputeElementDelta_NoBoundingBox(t *testing.T) {
	baseline := []snapshot.AXNode{axNode("heading", "Welcome", nil)}
	current := []snapshot.AXNode{axNode("heading", "Welcome", nil)}

	result := ComputeElementDelta(baseline, current)

	if result.HasRegression {
		t.Error("expected HasRegression=false when BoundingBox is nil on both sides")
	}
	if len(result.Moved) != 0 {
		t.Errorf("expected 0 moved, got %d", len(result.Moved))
	}
	if len(result.Resized) != 0 {
		t.Errorf("expected 0 resized, got %d", len(result.Resized))
	}
	// Element exists in both — should not appear in Appeared or Disappeared
	if len(result.Appeared) != 0 {
		t.Errorf("expected 0 appeared, got %d", len(result.Appeared))
	}
	if len(result.Disappeared) != 0 {
		t.Errorf("expected 0 disappeared, got %d", len(result.Disappeared))
	}
}

// TestComputeElementDelta_NoBoundingBox_OneNil verifies that when only one side has
// a nil BoundingBox, positional checks are skipped (no Moved or Resized reported).
func TestComputeElementDelta_NoBoundingBox_OneNil(t *testing.T) {
	baseline := []snapshot.AXNode{axNode("button", "Login", nil)}
	current := []snapshot.AXNode{axNode("button", "Login", bb(0, 0, 80, 40))}

	result := ComputeElementDelta(baseline, current)

	if len(result.Moved) != 0 {
		t.Errorf("expected 0 moved when baseline has nil bbox, got %d", len(result.Moved))
	}
	if len(result.Resized) != 0 {
		t.Errorf("expected 0 resized when baseline has nil bbox, got %d", len(result.Resized))
	}
}

// TestComputeElementDelta_SemanticIdentity verifies that elements are matched by
// (role, name) regardless of their order in the input slices.
func TestComputeElementDelta_SemanticIdentity(t *testing.T) {
	baseline := []snapshot.AXNode{
		axNode("button", "Submit", bb(0, 0, 100, 40)),
		axNode("link", "Home", bb(10, 10, 60, 20)),
	}
	// Same elements, reversed order
	current := []snapshot.AXNode{
		axNode("link", "Home", bb(10, 10, 60, 20)),
		axNode("button", "Submit", bb(0, 0, 100, 40)),
	}

	result := ComputeElementDelta(baseline, current)

	if result.HasRegression {
		t.Error("expected HasRegression=false: same elements in different order")
	}
	if len(result.Appeared) != 0 || len(result.Disappeared) != 0 {
		t.Errorf("expected no appeared/disappeared, got appeared=%d disappeared=%d",
			len(result.Appeared), len(result.Disappeared))
	}
	if len(result.Moved) != 0 || len(result.Resized) != 0 {
		t.Errorf("expected no moved/resized, got moved=%d resized=%d",
			len(result.Moved), len(result.Resized))
	}
}

// TestSemanticIDNormalization verifies that role and name values are compared
// case-insensitively and with surrounding whitespace trimmed.
func TestSemanticIDNormalization(t *testing.T) {
	// Baseline has uppercase role and trailing whitespace in name
	baseline := []snapshot.AXNode{axNode("BUTTON", "Submit  ", bb(0, 0, 100, 40))}
	// Current has lowercase role and leading whitespace
	current := []snapshot.AXNode{axNode("button", "  Submit", bb(0, 0, 100, 40))}

	result := ComputeElementDelta(baseline, current)

	if result.HasRegression {
		t.Error("expected HasRegression=false: case/whitespace variants should match")
	}
	if len(result.Appeared) != 0 || len(result.Disappeared) != 0 {
		t.Errorf("expected no appeared/disappeared due to normalization, got appeared=%d disappeared=%d",
			len(result.Appeared), len(result.Disappeared))
	}
}

// TestComputeElementDelta_EmptyNodesSkipped verifies that nodes with both empty
// role and empty name are not indexed and do not produce false positives.
func TestComputeElementDelta_EmptyNodesSkipped(t *testing.T) {
	baseline := []snapshot.AXNode{
		{Role: "", Name: "", BoundingBox: bb(0, 0, 100, 100)},
		axNode("button", "Go", bb(0, 0, 80, 40)),
	}
	current := []snapshot.AXNode{
		axNode("button", "Go", bb(0, 0, 80, 40)),
	}

	result := ComputeElementDelta(baseline, current)

	if result.HasRegression {
		t.Error("expected no regression: empty-role/name nodes should be skipped")
	}
	if len(result.Disappeared) != 0 {
		t.Errorf("expected 0 disappeared, got %d (empty nodes should be ignored)", len(result.Disappeared))
	}
}
