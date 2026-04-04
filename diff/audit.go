package diff

import "github.com/callmeradical/kaleidoscope/snapshot"

// extractCounts returns per-category violation counts from an audit snapshot.
func extractCounts(audit snapshot.AuditSnapshot) (contrast, touch, typo int) {
	return audit.Summary.ContrastViolations, audit.Summary.TouchViolations, audit.Summary.TypographyWarnings
}

// ComputeAuditDiff compares two audit snapshots and returns a structured diff.
func ComputeAuditDiff(baselineAudit, snapshotAudit snapshot.AuditSnapshot) AuditDiff {
	baseContrast, baseTouch, baseTypo := extractCounts(baselineAudit)
	snapContrast, snapTouch, snapTypo := extractCounts(snapshotAudit)

	categories := CategoryDeltas{
		Contrast: CategoryDelta{
			Baseline: baseContrast,
			Snapshot: snapContrast,
			Delta:    snapContrast - baseContrast,
		},
		TouchTargets: CategoryDelta{
			Baseline: baseTouch,
			Snapshot: snapTouch,
			Delta:    snapTouch - baseTouch,
		},
		Typography: CategoryDelta{
			Baseline: baseTypo,
			Snapshot: snapTypo,
			Delta:    snapTypo - baseTypo,
		},
	}

	// Build selector → category maps for O(1) lookup.
	baseMap := make(map[string]string, len(baselineAudit.Issues))
	for _, issue := range baselineAudit.Issues {
		baseMap[issue.Selector] = issue.Category
	}
	snapMap := make(map[string]string, len(snapshotAudit.Issues))
	for _, issue := range snapshotAudit.Issues {
		snapMap[issue.Selector] = issue.Category
	}

	newIssues := []AuditIssue{}
	for sel, cat := range snapMap {
		if _, found := baseMap[sel]; !found {
			newIssues = append(newIssues, AuditIssue{Category: cat, Selector: sel})
		}
	}

	resolvedIssues := []AuditIssue{}
	for sel, cat := range baseMap {
		if _, found := snapMap[sel]; !found {
			resolvedIssues = append(resolvedIssues, AuditIssue{Category: cat, Selector: sel})
		}
	}

	hasRegression := categories.Contrast.Delta > 0 || categories.TouchTargets.Delta > 0 || categories.Typography.Delta > 0

	return AuditDiff{
		Categories: categories,
		Issues: IssueDelta{
			New:      newIssues,
			Resolved: resolvedIssues,
		},
		HasRegression: hasRegression,
	}
}
