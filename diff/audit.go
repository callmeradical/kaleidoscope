package diff

import (
	"strings"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// auditCategories is the fixed set of categories tracked in audit deltas.
var auditCategories = []string{"contrast", "touch", "typography", "spacing"}

// groupByCategory groups audit issues by their Category field.
func groupByCategory(issues []snapshot.AuditIssue) map[string][]snapshot.AuditIssue {
	m := make(map[string][]snapshot.AuditIssue)
	for _, issue := range issues {
		key := strings.ToLower(strings.TrimSpace(issue.Category))
		m[key] = append(m[key], issue)
	}
	return m
}

// issueKey returns a canonical key used to match issues between baseline and current.
func issueKey(issue snapshot.AuditIssue) string {
	return strings.ToLower(strings.TrimSpace(issue.Category)) + ":" + strings.ToLower(strings.TrimSpace(issue.Selector))
}

// allIssues returns all issues from an AuditData as a flat slice.
func allIssues(data snapshot.AuditData) []snapshot.AuditIssue {
	var all []snapshot.AuditIssue
	all = append(all, data.ContrastIssues...)
	all = append(all, data.TouchIssues...)
	all = append(all, data.TypographyIssues...)
	all = append(all, data.SpacingIssues...)
	return all
}

// ComputeAuditDelta computes the delta between two AuditData snapshots.
// It is a pure function with no Chrome or filesystem dependency.
func ComputeAuditDelta(baseline, current snapshot.AuditData) AuditDelta {
	baselineAll := allIssues(baseline)
	currentAll := allIssues(current)

	baselineByCategory := groupByCategory(baselineAll)
	currentByCategory := groupByCategory(currentAll)

	categories := make(map[string]CategoryDelta, len(auditCategories))
	for _, cat := range auditCategories {
		b := len(baselineByCategory[cat])
		c := len(currentByCategory[cat])
		categories[cat] = CategoryDelta{
			Category: cat,
			Baseline: b,
			Current:  c,
			Delta:    c - b,
		}
	}

	// Build issue key sets.
	baselineKeys := make(map[string]snapshot.AuditIssue, len(baselineAll))
	for _, issue := range baselineAll {
		baselineKeys[issueKey(issue)] = issue
	}
	currentKeys := make(map[string]snapshot.AuditIssue, len(currentAll))
	for _, issue := range currentAll {
		currentKeys[issueKey(issue)] = issue
	}

	// New issues: in current but not in baseline.
	var newIssues []IssueDiff
	for k, issue := range currentKeys {
		if _, exists := baselineKeys[k]; !exists {
			newIssues = append(newIssues, IssueDiff{
				Selector: issue.Selector,
				Category: issue.Category,
				Message:  issue.Message,
				Status:   "new",
			})
		}
	}

	// Resolved issues: in baseline but not in current.
	var resolved []IssueDiff
	for k, issue := range baselineKeys {
		if _, exists := currentKeys[k]; !exists {
			resolved = append(resolved, IssueDiff{
				Selector: issue.Selector,
				Category: issue.Category,
				Message:  issue.Message,
				Status:   "resolved",
			})
		}
	}

	// HasRegression is true if any category has increased issue count.
	hasRegression := false
	for _, cd := range categories {
		if cd.Delta > 0 {
			hasRegression = true
			break
		}
	}

	return AuditDelta{
		Categories:    categories,
		NewIssues:     newIssues,
		Resolved:      resolved,
		HasRegression: hasRegression,
	}
}
