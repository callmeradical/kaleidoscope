package diff

import (
	"testing"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

func auditSnap(contrast, touch, typo int, issues ...snapshot.AuditIssueRecord) snapshot.AuditSnapshot {
	return snapshot.AuditSnapshot{
		Summary: snapshot.AuditSummary{
			ContrastViolations: contrast,
			TouchViolations:    touch,
			TypographyWarnings: typo,
			TotalIssues:        contrast + touch + typo,
		},
		Issues: issues,
	}
}

func TestAuditDiff_Identical(t *testing.T) {
	base := auditSnap(2, 1, 3)
	snap := auditSnap(2, 1, 3)
	result := ComputeAuditDiff(base, snap)

	if result.HasRegression {
		t.Error("expected no regression for identical audits")
	}
	if result.Categories.Contrast.Delta != 0 {
		t.Errorf("contrast delta: got %d, want 0", result.Categories.Contrast.Delta)
	}
	if result.Categories.TouchTargets.Delta != 0 {
		t.Errorf("touch delta: got %d, want 0", result.Categories.TouchTargets.Delta)
	}
	if result.Categories.Typography.Delta != 0 {
		t.Errorf("typo delta: got %d, want 0", result.Categories.Typography.Delta)
	}
	if len(result.Issues.New) != 0 {
		t.Errorf("new issues: got %d, want 0", len(result.Issues.New))
	}
	if len(result.Issues.Resolved) != 0 {
		t.Errorf("resolved issues: got %d, want 0", len(result.Issues.Resolved))
	}
}

func TestAuditDiff_MoreContrastViolations(t *testing.T) {
	base := auditSnap(1, 0, 0)
	snap := auditSnap(3, 0, 0)
	result := ComputeAuditDiff(base, snap)

	if !result.HasRegression {
		t.Error("expected regression when contrast violations increased")
	}
	if result.Categories.Contrast.Delta != 2 {
		t.Errorf("contrast delta: got %d, want 2", result.Categories.Contrast.Delta)
	}
}

func TestAuditDiff_TouchViolationsResolved(t *testing.T) {
	base := auditSnap(0, 3, 0)
	snap := auditSnap(0, 0, 0)
	result := ComputeAuditDiff(base, snap)

	if result.HasRegression {
		t.Error("expected no regression when touch violations resolved")
	}
	if result.Categories.TouchTargets.Delta != -3 {
		t.Errorf("touch delta: got %d, want -3", result.Categories.TouchTargets.Delta)
	}
}

func TestAuditDiff_PerIssueTracking(t *testing.T) {
	base := auditSnap(1, 0, 0,
		snapshot.AuditIssueRecord{Category: "contrast", Selector: "p"},
	)
	snap := auditSnap(1, 0, 0,
		snapshot.AuditIssueRecord{Category: "contrast", Selector: "h1"},
	)
	result := ComputeAuditDiff(base, snap)

	// Counts are same so no category regression, but issue set changed.
	if result.Categories.Contrast.Delta != 0 {
		t.Errorf("contrast delta: got %d, want 0", result.Categories.Contrast.Delta)
	}

	var foundNew, foundResolved bool
	for _, issue := range result.Issues.New {
		if issue.Category == "contrast" && issue.Selector == "h1" {
			foundNew = true
		}
	}
	for _, issue := range result.Issues.Resolved {
		if issue.Category == "contrast" && issue.Selector == "p" {
			foundResolved = true
		}
	}
	if !foundNew {
		t.Error("expected new issue {contrast, h1}")
	}
	if !foundResolved {
		t.Error("expected resolved issue {contrast, p}")
	}
}

func TestAuditDiff_MoreTypographyWarnings(t *testing.T) {
	base := auditSnap(0, 0, 1)
	snap := auditSnap(0, 0, 4)
	result := ComputeAuditDiff(base, snap)

	if !result.HasRegression {
		t.Error("expected regression when typography warnings increased")
	}
	if result.Categories.Typography.Delta != 3 {
		t.Errorf("typography delta: got %d, want 3", result.Categories.Typography.Delta)
	}
}
