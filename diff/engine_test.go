package diff

import (
	"testing"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// ---- Helpers ----

func makeAuditData(contrast, touch, typography, spacing []snapshot.AuditIssue) snapshot.AuditData {
	return snapshot.AuditData{
		Contrast:   contrast,
		Touch:      touch,
		Typography: typography,
		Spacing:    spacing,
	}
}

func issue(selector, message string) snapshot.AuditIssue {
	return snapshot.AuditIssue{Selector: selector, Message: message}
}

func elem(role, name string, x, y, w, h float64) snapshot.Element {
	return snapshot.Element{
		Role: role,
		Name: name,
		Box:  snapshot.BoundingBox{X: x, Y: y, Width: w, Height: h},
	}
}

// ---- Audit diff tests ----

func TestDiffAudit_NoChange(t *testing.T) {
	issues := []snapshot.AuditIssue{issue("h1", "low contrast")}
	data := makeAuditData(issues, nil, nil, nil)
	result := DiffAudit(data, data)

	if len(result.NewIssues) != 0 {
		t.Errorf("expected no new issues, got %d", len(result.NewIssues))
	}
	if len(result.ResolvedIssues) != 0 {
		t.Errorf("expected no resolved issues, got %d", len(result.ResolvedIssues))
	}
	for _, cat := range []string{"contrast", "touch", "typography", "spacing"} {
		delta := result.Categories[cat]
		if delta.Delta != 0 {
			t.Errorf("category %q: expected delta=0, got %d", cat, delta.Delta)
		}
	}
}

func TestDiffAudit_NewIssue(t *testing.T) {
	base := makeAuditData([]snapshot.AuditIssue{issue("h1", "low contrast")}, nil, nil, nil)
	target := makeAuditData([]snapshot.AuditIssue{
		issue("h1", "low contrast"),
		issue("p", "very low contrast"),
	}, nil, nil, nil)

	result := DiffAudit(base, target)

	if len(result.NewIssues) != 1 {
		t.Fatalf("expected 1 new issue, got %d", len(result.NewIssues))
	}
	ni := result.NewIssues[0]
	if ni.Category != "contrast" {
		t.Errorf("expected category=contrast, got %q", ni.Category)
	}
	if ni.Selector != "p" {
		t.Errorf("expected selector=p, got %q", ni.Selector)
	}
	if ni.Message != "very low contrast" {
		t.Errorf("expected message=very low contrast, got %q", ni.Message)
	}
}

func TestDiffAudit_ResolvedIssue(t *testing.T) {
	base := makeAuditData(nil, []snapshot.AuditIssue{issue("button", "too small")}, nil, nil)
	target := makeAuditData(nil, nil, nil, nil)

	result := DiffAudit(base, target)

	if len(result.ResolvedIssues) != 1 {
		t.Fatalf("expected 1 resolved issue, got %d", len(result.ResolvedIssues))
	}
	ri := result.ResolvedIssues[0]
	if ri.Category != "touch" {
		t.Errorf("expected category=touch, got %q", ri.Category)
	}
	if ri.Selector != "button" {
		t.Errorf("expected selector=button, got %q", ri.Selector)
	}
}

func TestDiffAudit_AllCategories(t *testing.T) {
	base := makeAuditData(
		[]snapshot.AuditIssue{issue("h1", "c1")},
		[]snapshot.AuditIssue{issue("btn", "t1")},
		[]snapshot.AuditIssue{issue("p", "ty1"), issue("span", "ty2")},
		[]snapshot.AuditIssue{},
	)
	target := makeAuditData(
		[]snapshot.AuditIssue{issue("h1", "c1"), issue("h2", "c2")}, // contrast +1
		[]snapshot.AuditIssue{},                                        // touch -1
		[]snapshot.AuditIssue{issue("p", "ty1"), issue("span", "ty2")}, // typography 0
		[]snapshot.AuditIssue{issue("div", "s1")},                       // spacing +1
	)

	result := DiffAudit(base, target)

	if d := result.Categories["contrast"].Delta; d != 1 {
		t.Errorf("contrast delta: want 1, got %d", d)
	}
	if d := result.Categories["touch"].Delta; d != -1 {
		t.Errorf("touch delta: want -1, got %d", d)
	}
	if d := result.Categories["typography"].Delta; d != 0 {
		t.Errorf("typography delta: want 0, got %d", d)
	}
	if d := result.Categories["spacing"].Delta; d != 1 {
		t.Errorf("spacing delta: want 1, got %d", d)
	}
}

func TestDiffAudit_SelectorMatching(t *testing.T) {
	// Same message, different selectors → two separate new issues
	base := makeAuditData(nil, nil, nil, nil)
	target := makeAuditData([]snapshot.AuditIssue{
		issue("h1", "low contrast"),
		issue("h2", "low contrast"),
	}, nil, nil, nil)

	result := DiffAudit(base, target)

	if len(result.NewIssues) != 2 {
		t.Errorf("expected 2 new issues (different selectors), got %d", len(result.NewIssues))
	}
}

func TestDiffAudit_MessageNotUsedForMatching(t *testing.T) {
	// Same selector, different message → treated as the same issue (no new/resolved)
	base := makeAuditData([]snapshot.AuditIssue{issue("h1", "old message")}, nil, nil, nil)
	target := makeAuditData([]snapshot.AuditIssue{issue("h1", "new message")}, nil, nil, nil)

	result := DiffAudit(base, target)

	if len(result.NewIssues) != 0 {
		t.Errorf("expected no new issues (same selector), got %d", len(result.NewIssues))
	}
	if len(result.ResolvedIssues) != 0 {
		t.Errorf("expected no resolved issues (same selector), got %d", len(result.ResolvedIssues))
	}
}

// ---- Element diff tests ----

func TestDiffElements_Appeared(t *testing.T) {
	base := []snapshot.Element{}
	target := []snapshot.Element{elem("button", "Submit", 10, 10, 100, 40)}
	thresholds := DefaultThresholds()

	result := DiffElements(base, target, thresholds)

	if len(result.Appeared) != 1 {
		t.Fatalf("expected 1 appeared, got %d", len(result.Appeared))
	}
	e := result.Appeared[0]
	if e.TargetBox == nil {
		t.Error("expected TargetBox to be set")
	}
	if e.BaselineBox != nil {
		t.Error("expected BaselineBox to be nil")
	}
}

func TestDiffElements_Disappeared(t *testing.T) {
	base := []snapshot.Element{elem("link", "Home", 0, 0, 80, 30)}
	target := []snapshot.Element{}
	thresholds := DefaultThresholds()

	result := DiffElements(base, target, thresholds)

	if len(result.Disappeared) != 1 {
		t.Fatalf("expected 1 disappeared, got %d", len(result.Disappeared))
	}
	e := result.Disappeared[0]
	if e.BaselineBox == nil {
		t.Error("expected BaselineBox to be set")
	}
	if e.TargetBox != nil {
		t.Error("expected TargetBox to be nil")
	}
}

func TestDiffElements_Moved(t *testing.T) {
	base := []snapshot.Element{elem("button", "Submit", 10, 10, 100, 40)}
	target := []snapshot.Element{elem("button", "Submit", 20, 20, 100, 40)} // moved 10px
	thresholds := DefaultThresholds()

	result := DiffElements(base, target, thresholds)

	if len(result.Moved) != 1 {
		t.Fatalf("expected 1 moved, got %d", len(result.Moved))
	}
	m := result.Moved[0]
	if m.PositionDelta == nil {
		t.Fatal("expected PositionDelta to be set")
	}
	if m.PositionDelta.DX != 10 {
		t.Errorf("expected DX=10, got %f", m.PositionDelta.DX)
	}
	if m.PositionDelta.DY != 10 {
		t.Errorf("expected DY=10, got %f", m.PositionDelta.DY)
	}
}

func TestDiffElements_NotMoved(t *testing.T) {
	// Movement within threshold (≤4px) should not be reported
	base := []snapshot.Element{elem("button", "Submit", 10, 10, 100, 40)}
	target := []snapshot.Element{elem("button", "Submit", 13, 13, 100, 40)} // 3px, within threshold
	thresholds := DefaultThresholds()

	result := DiffElements(base, target, thresholds)

	if len(result.Moved) != 0 {
		t.Errorf("expected no moved elements (within threshold), got %d", len(result.Moved))
	}
}

func TestDiffElements_Resized(t *testing.T) {
	base := []snapshot.Element{elem("img", "Logo", 0, 0, 200, 100)}
	target := []snapshot.Element{elem("img", "Logo", 0, 0, 210, 100)} // width +10
	thresholds := DefaultThresholds()

	result := DiffElements(base, target, thresholds)

	if len(result.Resized) != 1 {
		t.Fatalf("expected 1 resized, got %d", len(result.Resized))
	}
	r := result.Resized[0]
	if r.SizeDelta == nil {
		t.Fatal("expected SizeDelta to be set")
	}
	if r.SizeDelta.DX != 10 {
		t.Errorf("expected DX=10, got %f", r.SizeDelta.DX)
	}
}

func TestDiffElements_EmptyNameSkipped(t *testing.T) {
	base := []snapshot.Element{
		elem("button", "", 0, 0, 100, 40),         // empty name — should be skipped
		elem("button", "   ", 0, 0, 100, 40),       // whitespace-only — should be skipped
		elem("button", "Real Button", 0, 0, 100, 40), // valid
	}
	target := []snapshot.Element{}
	thresholds := DefaultThresholds()

	result := DiffElements(base, target, thresholds)

	// Only "Real Button" should have been indexed, so only 1 disappeared
	if len(result.Disappeared) != 1 {
		t.Errorf("expected 1 disappeared (unnamed skipped), got %d", len(result.Disappeared))
	}
}

func TestDiffElements_SemanticKey(t *testing.T) {
	// Same role+name but different positions → should be Moved, not Appeared/Disappeared
	base := []snapshot.Element{elem("button", "Submit", 0, 0, 100, 40)}
	target := []snapshot.Element{elem("button", "Submit", 50, 0, 100, 40)} // moved 50px
	thresholds := DefaultThresholds()

	result := DiffElements(base, target, thresholds)

	if len(result.Appeared) != 0 {
		t.Errorf("expected no appeared, got %d", len(result.Appeared))
	}
	if len(result.Disappeared) != 0 {
		t.Errorf("expected no disappeared, got %d", len(result.Disappeared))
	}
	if len(result.Moved) != 1 {
		t.Errorf("expected 1 moved (matched by semantic key), got %d", len(result.Moved))
	}
}

// ---- HasRegressions tests ----

func emptySnapshot(id string) *snapshot.Snapshot {
	return &snapshot.Snapshot{ID: id}
}

func snapshotWithAuditIssue(id, selector string) *snapshot.Snapshot {
	return &snapshot.Snapshot{
		ID: id,
		Audit: snapshot.AuditData{
			Contrast: []snapshot.AuditIssue{{Selector: selector, Message: "low contrast"}},
		},
	}
}

func TestHasRegressions_NewAuditIssue(t *testing.T) {
	base := emptySnapshot("base")
	target := snapshotWithAuditIssue("target", "h1")
	result := Run(base, target, DefaultThresholds())
	if !result.HasRegressions {
		t.Error("expected HasRegressions=true when new audit issue present")
	}
}

func TestHasRegressions_Disappeared(t *testing.T) {
	base := &snapshot.Snapshot{
		ID:       "base",
		Elements: []snapshot.Element{elem("nav", "Main Nav", 0, 0, 800, 60)},
	}
	target := emptySnapshot("target")
	result := Run(base, target, DefaultThresholds())
	if !result.HasRegressions {
		t.Error("expected HasRegressions=true when element disappeared")
	}
}

func TestHasRegressions_Moved(t *testing.T) {
	base := &snapshot.Snapshot{
		ID:       "base",
		Elements: []snapshot.Element{elem("button", "Submit", 0, 0, 100, 40)},
	}
	target := &snapshot.Snapshot{
		ID:       "target",
		Elements: []snapshot.Element{elem("button", "Submit", 100, 0, 100, 40)},
	}
	result := Run(base, target, DefaultThresholds())
	if !result.HasRegressions {
		t.Error("expected HasRegressions=true when element moved beyond threshold")
	}
}

func TestHasRegressions_Resized(t *testing.T) {
	base := &snapshot.Snapshot{
		ID:       "base",
		Elements: []snapshot.Element{elem("img", "Logo", 0, 0, 200, 100)},
	}
	target := &snapshot.Snapshot{
		ID:       "target",
		Elements: []snapshot.Element{elem("img", "Logo", 0, 0, 100, 100)},
	}
	result := Run(base, target, DefaultThresholds())
	if !result.HasRegressions {
		t.Error("expected HasRegressions=true when element resized beyond threshold")
	}
}

func TestHasRegressions_AppearedOnly(t *testing.T) {
	// Only new elements appeared — not a regression
	base := emptySnapshot("base")
	target := &snapshot.Snapshot{
		ID:       "target",
		Elements: []snapshot.Element{elem("button", "New Button", 0, 0, 100, 40)},
	}
	result := Run(base, target, DefaultThresholds())
	if result.HasRegressions {
		t.Error("expected HasRegressions=false when only new elements appeared")
	}
}

func TestHasRegressions_ResolvedOnly(t *testing.T) {
	// Issues resolved and nothing new — not a regression
	base := snapshotWithAuditIssue("base", "h1")
	target := emptySnapshot("target")
	result := Run(base, target, DefaultThresholds())
	if result.HasRegressions {
		t.Error("expected HasRegressions=false when issues only resolved (nothing new)")
	}
}
