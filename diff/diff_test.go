package diff_test

import (
	"testing"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// --- Helpers ---

func auditWith(contrast, touch, typography []snapshot.AuditIssue) snapshot.AuditSummary {
	return snapshot.AuditSummary{
		TotalIssues:        len(contrast) + len(touch) + len(typography),
		ContrastViolations: contrast,
		TouchViolations:    touch,
		TypographyWarnings: typography,
	}
}

func issue(selector, detail string) snapshot.AuditIssue {
	return snapshot.AuditIssue{Selector: selector, Detail: detail}
}

func node(role, name string, x, y, w, h float64) snapshot.AXNode {
	return snapshot.AXNode{Role: role, Name: name, X: x, Y: y, Width: w, Height: h}
}

func snapshotWith(id string, audit snapshot.AuditSummary, nodes []snapshot.AXNode) *snapshot.Snapshot {
	return &snapshot.Snapshot{ID: id, Audit: audit, AXNodes: nodes}
}

// --- Audit diff tests ---

// TestCompareAudit_NoChange: identical audits produce all-zero deltas and empty issue slices.
func TestCompareAudit_NoChange(t *testing.T) {
	a := auditWith(
		[]snapshot.AuditIssue{issue("h1", "low contrast")},
		[]snapshot.AuditIssue{},
		[]snapshot.AuditIssue{},
	)
	result := diff.CompareAudit(a, a)

	if result.ContrastDelta != 0 {
		t.Errorf("ContrastDelta: got %d, want 0", result.ContrastDelta)
	}
	if result.TouchDelta != 0 {
		t.Errorf("TouchDelta: got %d, want 0", result.TouchDelta)
	}
	if result.TypographyDelta != 0 {
		t.Errorf("TypographyDelta: got %d, want 0", result.TypographyDelta)
	}
	if result.TotalDelta != 0 {
		t.Errorf("TotalDelta: got %d, want 0", result.TotalDelta)
	}
	if len(result.NewIssues) != 0 {
		t.Errorf("NewIssues: got %d, want 0", len(result.NewIssues))
	}
	if len(result.ResolvedIssues) != 0 {
		t.Errorf("ResolvedIssues: got %d, want 0", len(result.ResolvedIssues))
	}
}

// TestCompareAudit_NewIssues: target has extra contrast violations not in baseline.
func TestCompareAudit_NewIssues(t *testing.T) {
	baseline := auditWith(
		[]snapshot.AuditIssue{issue("h1", "old contrast issue")},
		nil, nil,
	)
	target := auditWith(
		[]snapshot.AuditIssue{
			issue("h1", "old contrast issue"),
			issue("p.text", "new contrast issue"),
		},
		nil, nil,
	)

	result := diff.CompareAudit(baseline, target)

	if result.ContrastDelta != 1 {
		t.Errorf("ContrastDelta: got %d, want 1", result.ContrastDelta)
	}
	if len(result.NewIssues) != 1 {
		t.Fatalf("NewIssues: got %d, want 1", len(result.NewIssues))
	}
	if result.NewIssues[0].Category != "contrast" {
		t.Errorf("NewIssues[0].Category: got %q, want %q", result.NewIssues[0].Category, "contrast")
	}
	if result.NewIssues[0].Selector != "p.text" {
		t.Errorf("NewIssues[0].Selector: got %q, want %q", result.NewIssues[0].Selector, "p.text")
	}
	if len(result.ResolvedIssues) != 0 {
		t.Errorf("ResolvedIssues: got %d, want 0", len(result.ResolvedIssues))
	}
}

// TestCompareAudit_ResolvedIssues: baseline has issues absent from target.
func TestCompareAudit_ResolvedIssues(t *testing.T) {
	baseline := auditWith(
		[]snapshot.AuditIssue{issue("a.link", "contrast violation")},
		nil, nil,
	)
	target := auditWith(nil, nil, nil)

	result := diff.CompareAudit(baseline, target)

	if result.ContrastDelta != -1 {
		t.Errorf("ContrastDelta: got %d, want -1", result.ContrastDelta)
	}
	if len(result.ResolvedIssues) != 1 {
		t.Fatalf("ResolvedIssues: got %d, want 1", len(result.ResolvedIssues))
	}
	if result.ResolvedIssues[0].Selector != "a.link" {
		t.Errorf("ResolvedIssues[0].Selector: got %q, want %q", result.ResolvedIssues[0].Selector, "a.link")
	}
	if len(result.NewIssues) != 0 {
		t.Errorf("NewIssues: got %d, want 0", len(result.NewIssues))
	}
}

// TestCompareAudit_SelectorMatching: same selector+category = same issue (no change);
// different selectors = distinct issues.
func TestCompareAudit_SelectorMatching(t *testing.T) {
	shared := issue("button", "low contrast")
	extra := issue("span.caption", "low contrast")

	baseline := auditWith([]snapshot.AuditIssue{shared}, nil, nil)
	target := auditWith([]snapshot.AuditIssue{shared, extra}, nil, nil)

	result := diff.CompareAudit(baseline, target)

	// shared is in both → not new, not resolved
	// extra is only in target → new
	if len(result.NewIssues) != 1 {
		t.Fatalf("NewIssues: got %d, want 1", len(result.NewIssues))
	}
	if result.NewIssues[0].Selector != extra.Selector {
		t.Errorf("NewIssues[0].Selector: got %q, want %q", result.NewIssues[0].Selector, extra.Selector)
	}
	if len(result.ResolvedIssues) != 0 {
		t.Errorf("ResolvedIssues: got %d, want 0", len(result.ResolvedIssues))
	}
}

// TestCompareAudit_DetailIgnored: same selector+category, different Detail text = treated as same issue.
func TestCompareAudit_DetailIgnored(t *testing.T) {
	baseline := auditWith(
		[]snapshot.AuditIssue{issue("h2", "contrast ratio 2.5:1 (required 4.5:1)")},
		nil, nil,
	)
	// Same selector, but detail text changed (e.g. ratio improved but still failing)
	target := auditWith(
		[]snapshot.AuditIssue{issue("h2", "contrast ratio 3.0:1 (required 4.5:1)")},
		nil, nil,
	)

	result := diff.CompareAudit(baseline, target)

	// selector identity wins — same issue, no regression
	if len(result.NewIssues) != 0 {
		t.Errorf("NewIssues: got %d, want 0 (detail changes should not create new issue)", len(result.NewIssues))
	}
	if len(result.ResolvedIssues) != 0 {
		t.Errorf("ResolvedIssues: got %d, want 0", len(result.ResolvedIssues))
	}
}

// --- Element diff tests ---

// TestCompareElements_Appeared: node in target not in baseline → Appeared with Baseline=nil.
func TestCompareElements_Appeared(t *testing.T) {
	baseline := []snapshot.AXNode{}
	target := []snapshot.AXNode{node("button", "Submit", 10, 20, 80, 40)}

	result := diff.CompareElements(baseline, target)

	if len(result.Appeared) != 1 {
		t.Fatalf("Appeared: got %d, want 1", len(result.Appeared))
	}
	c := result.Appeared[0]
	if c.Role != "button" || c.Name != "Submit" {
		t.Errorf("Appeared[0]: got {%q, %q}, want {button, Submit}", c.Role, c.Name)
	}
	if c.Baseline != nil {
		t.Errorf("Appeared[0].Baseline: got non-nil, want nil")
	}
	if c.Target == nil {
		t.Errorf("Appeared[0].Target: got nil, want non-nil")
	}
	if len(result.Disappeared) != 0 {
		t.Errorf("Disappeared: got %d, want 0", len(result.Disappeared))
	}
}

// TestCompareElements_Disappeared: node in baseline not in target → Disappeared with Target=nil.
func TestCompareElements_Disappeared(t *testing.T) {
	baseline := []snapshot.AXNode{node("link", "Home", 0, 0, 60, 20)}
	target := []snapshot.AXNode{}

	result := diff.CompareElements(baseline, target)

	if len(result.Disappeared) != 1 {
		t.Fatalf("Disappeared: got %d, want 1", len(result.Disappeared))
	}
	c := result.Disappeared[0]
	if c.Role != "link" || c.Name != "Home" {
		t.Errorf("Disappeared[0]: got {%q, %q}, want {link, Home}", c.Role, c.Name)
	}
	if c.Target != nil {
		t.Errorf("Disappeared[0].Target: got non-nil, want nil")
	}
	if c.Baseline == nil {
		t.Errorf("Disappeared[0].Baseline: got nil, want non-nil")
	}
	if len(result.Appeared) != 0 {
		t.Errorf("Appeared: got %d, want 0", len(result.Appeared))
	}
}

// TestCompareElements_Moved: position delta > PositionThreshold → node in Moved with correct Delta.
func TestCompareElements_Moved(t *testing.T) {
	baseline := []snapshot.AXNode{node("button", "Login", 100, 200, 80, 40)}
	target := []snapshot.AXNode{node("button", "Login", 110, 200, 80, 40)} // DX=10 > threshold

	result := diff.CompareElements(baseline, target)

	if len(result.Moved) != 1 {
		t.Fatalf("Moved: got %d, want 1", len(result.Moved))
	}
	c := result.Moved[0]
	if c.Delta == nil {
		t.Fatal("Moved[0].Delta: got nil, want non-nil")
	}
	if c.Delta.DX != 10 {
		t.Errorf("Moved[0].Delta.DX: got %v, want 10", c.Delta.DX)
	}
	if c.Delta.DY != 0 {
		t.Errorf("Moved[0].Delta.DY: got %v, want 0", c.Delta.DY)
	}
}

// TestCompareElements_MovedBelowThreshold: position delta <= PositionThreshold → not in Moved.
func TestCompareElements_MovedBelowThreshold(t *testing.T) {
	baseline := []snapshot.AXNode{node("button", "Login", 100, 200, 80, 40)}
	// DX=2 <= PositionThreshold(4), DY=0
	target := []snapshot.AXNode{node("button", "Login", 102, 200, 80, 40)}

	result := diff.CompareElements(baseline, target)

	if len(result.Moved) != 0 {
		t.Errorf("Moved: got %d, want 0 (delta below threshold)", len(result.Moved))
	}
}

// TestCompareElements_Resized: size delta > SizeThreshold → node in Resized with correct Delta.
func TestCompareElements_Resized(t *testing.T) {
	baseline := []snapshot.AXNode{node("image", "Logo", 0, 0, 100, 50)}
	target := []snapshot.AXNode{node("image", "Logo", 0, 0, 200, 50)} // DWidth=100 > threshold

	result := diff.CompareElements(baseline, target)

	if len(result.Resized) != 1 {
		t.Fatalf("Resized: got %d, want 1", len(result.Resized))
	}
	c := result.Resized[0]
	if c.Delta == nil {
		t.Fatal("Resized[0].Delta: got nil, want non-nil")
	}
	if c.Delta.DWidth != 100 {
		t.Errorf("Resized[0].Delta.DWidth: got %v, want 100", c.Delta.DWidth)
	}
}

// TestCompareElements_EmptyNameExcluded: nodes with empty/whitespace-only Name are ignored.
func TestCompareElements_EmptyNameExcluded(t *testing.T) {
	baseline := []snapshot.AXNode{
		node("div", "", 0, 0, 100, 100),
		node("div", "   ", 0, 0, 100, 100),
	}
	target := []snapshot.AXNode{
		node("section", "", 50, 50, 200, 200),
	}

	result := diff.CompareElements(baseline, target)

	if len(result.Appeared) != 0 {
		t.Errorf("Appeared: got %d, want 0 (empty-name nodes should be excluded)", len(result.Appeared))
	}
	if len(result.Disappeared) != 0 {
		t.Errorf("Disappeared: got %d, want 0 (empty-name nodes should be excluded)", len(result.Disappeared))
	}
}

// TestCompareElements_MovedAndResized: one node both moves and resizes → in both Moved and Resized.
func TestCompareElements_MovedAndResized(t *testing.T) {
	baseline := []snapshot.AXNode{node("heading", "Welcome", 0, 0, 300, 60)}
	// Move by 10px (> threshold) AND resize width by 100px (> threshold)
	target := []snapshot.AXNode{node("heading", "Welcome", 10, 0, 400, 60)}

	result := diff.CompareElements(baseline, target)

	if len(result.Moved) != 1 {
		t.Errorf("Moved: got %d, want 1", len(result.Moved))
	}
	if len(result.Resized) != 1 {
		t.Errorf("Resized: got %d, want 1", len(result.Resized))
	}
}

// --- Regression flag tests ---

// TestRegressionFlag_FalseWhenClean: identical snapshots → Regressions=false.
func TestRegressionFlag_FalseWhenClean(t *testing.T) {
	audit := auditWith(
		[]snapshot.AuditIssue{issue("h1", "low contrast")},
		nil, nil,
	)
	nodes := []snapshot.AXNode{node("button", "Sign in", 10, 10, 80, 40)}
	s := snapshotWith("snap-001", audit, nodes)

	result := diff.Compare(s, s)

	if result.Regressions {
		t.Error("Regressions: got true, want false for identical snapshots")
	}
}

// TestRegressionFlag_TrueOnNewAuditIssue: new audit issue → Regressions=true.
func TestRegressionFlag_TrueOnNewAuditIssue(t *testing.T) {
	baselineAudit := auditWith(nil, nil, nil)
	targetAudit := auditWith(
		[]snapshot.AuditIssue{issue("p", "new contrast violation")},
		nil, nil,
	)
	baseline := snapshotWith("baseline", baselineAudit, nil)
	target := snapshotWith("target", targetAudit, nil)

	result := diff.Compare(baseline, target)

	if !result.Regressions {
		t.Error("Regressions: got false, want true when new audit issue exists")
	}
}

// TestRegressionFlag_TrueOnElementChange: appeared element → Regressions=true.
func TestRegressionFlag_TrueOnElementChange(t *testing.T) {
	baseline := snapshotWith("baseline", snapshot.AuditSummary{}, nil)
	target := snapshotWith("target", snapshot.AuditSummary{},
		[]snapshot.AXNode{node("button", "Subscribe", 0, 0, 120, 40)},
	)

	result := diff.Compare(baseline, target)

	if !result.Regressions {
		t.Error("Regressions: got false, want true when element appeared")
	}
}

// TestCompareAudit_EmptySlicesNotNil: NewIssues and ResolvedIssues must not be nil (JSON: []).
func TestCompareAudit_EmptySlicesNotNil(t *testing.T) {
	a := auditWith(nil, nil, nil)
	result := diff.CompareAudit(a, a)

	if result.NewIssues == nil {
		t.Error("NewIssues: got nil, want empty slice")
	}
	if result.ResolvedIssues == nil {
		t.Error("ResolvedIssues: got nil, want empty slice")
	}
}

// TestCompareElements_EmptySlicesNotNil: all slices must not be nil (JSON: []).
func TestCompareElements_EmptySlicesNotNil(t *testing.T) {
	result := diff.CompareElements(nil, nil)

	if result.Appeared == nil {
		t.Error("Appeared: got nil, want empty slice")
	}
	if result.Disappeared == nil {
		t.Error("Disappeared: got nil, want empty slice")
	}
	if result.Moved == nil {
		t.Error("Moved: got nil, want empty slice")
	}
	if result.Resized == nil {
		t.Error("Resized: got nil, want empty slice")
	}
}

// TestCompareAudit_MultiCategory: changes across contrast, touch, and typography all tracked.
func TestCompareAudit_MultiCategory(t *testing.T) {
	baseline := auditWith(nil, nil, nil)
	target := auditWith(
		[]snapshot.AuditIssue{issue("h1", "contrast")},
		[]snapshot.AuditIssue{issue("button", "too small")},
		[]snapshot.AuditIssue{issue("p", "line-height")},
	)

	result := diff.CompareAudit(baseline, target)

	if result.ContrastDelta != 1 {
		t.Errorf("ContrastDelta: got %d, want 1", result.ContrastDelta)
	}
	if result.TouchDelta != 1 {
		t.Errorf("TouchDelta: got %d, want 1", result.TouchDelta)
	}
	if result.TypographyDelta != 1 {
		t.Errorf("TypographyDelta: got %d, want 1", result.TypographyDelta)
	}
	if result.TotalDelta != 3 {
		t.Errorf("TotalDelta: got %d, want 3", result.TotalDelta)
	}
	if len(result.NewIssues) != 3 {
		t.Errorf("NewIssues: got %d, want 3", len(result.NewIssues))
	}
}
