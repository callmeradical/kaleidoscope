package diff

import (
	"fmt"
	"math"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// SnapshotDiff is the result of comparing two snapshots.
type SnapshotDiff struct {
	BaselineID string
	CurrentID  string
	Pages      []PageDiff
}

// PageDiff holds the diff for a single URL across audit, elements, and screenshots.
type PageDiff struct {
	URL             string
	AuditDelta      []CategoryDelta
	ElementChanges  []ElementChange
	BreakpointDiffs []BreakpointDiff
}

// CategoryDelta describes how one audit category changed between snapshots.
type CategoryDelta struct {
	Category string
	Before   int
	After    int
	Delta    int
}

// ElementChange describes a single DOM element change detected via the AX tree.
type ElementChange struct {
	Role     string
	Name     string
	Selector string
	Type     string // "appeared" | "disappeared" | "moved" | "resized"
	Details  string
}

// BreakpointDiff holds screenshot paths and pixel-diff results for one viewport.
type BreakpointDiff struct {
	Name           string
	BaselinePath   string
	CurrentPath    string
	DiffScore      float64
	DiffImageBytes []byte `json:"-"`
}

// Compare produces a SnapshotDiff between a baseline and current snapshot.
func Compare(baseline, current *snapshot.Snapshot) (*SnapshotDiff, error) {
	if baseline == nil {
		return nil, fmt.Errorf("baseline snapshot is nil")
	}
	if current == nil {
		return nil, fmt.Errorf("current snapshot is nil")
	}

	// Build URL-keyed map of baseline pages.
	baseMap := make(map[string]snapshot.PageSnapshot, len(baseline.Pages))
	for _, p := range baseline.Pages {
		baseMap[p.URL] = p
	}

	dir := "" // paths resolved externally via ScreenshotPath
	_ = dir

	var pages []PageDiff
	for _, cur := range current.Pages {
		base, matched := baseMap[cur.URL]

		var pd PageDiff
		pd.URL = cur.URL

		if matched {
			pd.AuditDelta = compareAudit(base.AuditResult, cur.AuditResult)
			pd.ElementChanges = compareElements(base.AXNodes, cur.AXNodes)

			// Build breakpoint diffs by name.
			baseBreakMap := make(map[string]snapshot.BreakpointCapture)
			for _, bp := range base.Breakpoints {
				baseBreakMap[bp.Name] = bp
			}
			for _, bp := range cur.Breakpoints {
				bd := BreakpointDiff{Name: bp.Name}
				if baseBP, ok := baseBreakMap[bp.Name]; ok {
					bd.BaselinePath = snapshot.ScreenshotPath("", baseline.ID, baseBP.ScreenshotFile)
					bd.CurrentPath = snapshot.ScreenshotPath("", current.ID, bp.ScreenshotFile)
				} else {
					bd.CurrentPath = snapshot.ScreenshotPath("", current.ID, bp.ScreenshotFile)
				}
				pd.BreakpointDiffs = append(pd.BreakpointDiffs, bd)
			}
		} else {
			// No baseline page — all audit counts are "new".
			pd.AuditDelta = compareAudit(snapshot.AuditResult{}, cur.AuditResult)
			pd.ElementChanges = compareElements(nil, cur.AXNodes)
		}

		pages = append(pages, pd)
	}

	return &SnapshotDiff{
		BaselineID: baseline.ID,
		CurrentID:  current.ID,
		Pages:      pages,
	}, nil
}

// compareAudit computes per-category deltas between two audit results.
func compareAudit(base, cur snapshot.AuditResult) []CategoryDelta {
	return []CategoryDelta{
		{Category: "contrast", Before: base.ContrastViolations, After: cur.ContrastViolations, Delta: cur.ContrastViolations - base.ContrastViolations},
		{Category: "touch", Before: base.TouchViolations, After: cur.TouchViolations, Delta: cur.TouchViolations - base.TouchViolations},
		{Category: "typography", Before: base.TypographyWarnings, After: cur.TypographyWarnings, Delta: cur.TypographyWarnings - base.TypographyWarnings},
		{Category: "spacing", Before: base.SpacingIssues, After: cur.SpacingIssues, Delta: cur.SpacingIssues - base.SpacingIssues},
	}
}

// compareElements detects appeared, disappeared, moved, and resized nodes.
func compareElements(base, cur []snapshot.AXNodeRecord) []ElementChange {
	type key struct{ role, name string }

	baseMap := make(map[key]snapshot.AXNodeRecord, len(base))
	for _, n := range base {
		baseMap[key{n.Role, n.Name}] = n
	}

	var changes []ElementChange
	seen := make(map[key]bool)

	for _, n := range cur {
		k := key{n.Role, n.Name}
		seen[k] = true
		if bn, ok := baseMap[k]; !ok {
			changes = append(changes, ElementChange{
				Role:     n.Role,
				Name:     n.Name,
				Selector: n.Selector,
				Type:     "appeared",
			})
		} else {
			// Check for bounds changes.
			dx := math.Abs(n.Bounds.X - bn.Bounds.X)
			dy := math.Abs(n.Bounds.Y - bn.Bounds.Y)
			dw := math.Abs(n.Bounds.Width - bn.Bounds.Width)
			dh := math.Abs(n.Bounds.Height - bn.Bounds.Height)
			const threshold = 2.0

			if dw > threshold || dh > threshold {
				changes = append(changes, ElementChange{
					Role:     n.Role,
					Name:     n.Name,
					Selector: n.Selector,
					Type:     "resized",
					Details:  fmt.Sprintf("%.0fx%.0f → %.0fx%.0f", bn.Bounds.Width, bn.Bounds.Height, n.Bounds.Width, n.Bounds.Height),
				})
			} else if dx > threshold || dy > threshold {
				changes = append(changes, ElementChange{
					Role:     n.Role,
					Name:     n.Name,
					Selector: n.Selector,
					Type:     "moved",
					Details:  fmt.Sprintf("(%.0f,%.0f) → (%.0f,%.0f)", bn.Bounds.X, bn.Bounds.Y, n.Bounds.X, n.Bounds.Y),
				})
			}
		}
	}

	// Disappeared nodes.
	for _, bn := range base {
		k := key{bn.Role, bn.Name}
		if !seen[k] {
			changes = append(changes, ElementChange{
				Role:     bn.Role,
				Name:     bn.Name,
				Selector: bn.Selector,
				Type:     "disappeared",
			})
		}
	}

	return changes
}

// CountRegressions returns the total number of audit categories that worsened.
func CountRegressions(d *SnapshotDiff) int {
	total := 0
	for _, p := range d.Pages {
		for _, delta := range p.AuditDelta {
			if delta.Delta > 0 {
				total++
			}
		}
	}
	return total
}
