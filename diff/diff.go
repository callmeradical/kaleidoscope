package diff

import (
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// ElementChange describes a single change to a DOM element between snapshots.
type ElementChange struct {
	Selector string `json:"selector"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	// Type is one of: "appeared", "disappeared", "moved", "resized"
	Type    string `json:"type"`
	Details string `json:"details"`
}

// AuditDelta holds before/after issue counts per audit category.
type AuditDelta struct {
	ContrastBefore   int `json:"contrastBefore"`
	ContrastAfter    int `json:"contrastAfter"`
	TouchBefore      int `json:"touchBefore"`
	TouchAfter       int `json:"touchAfter"`
	TypographyBefore int `json:"typographyBefore"`
	TypographyAfter  int `json:"typographyAfter"`
	SpacingBefore    int `json:"spacingBefore"`
	SpacingAfter     int `json:"spacingAfter"`
}

// BreakpointDiff holds the comparison result for a single breakpoint.
type BreakpointDiff struct {
	Name              string  `json:"name"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	BaselineImagePath string  `json:"baselineImagePath"`
	CurrentImagePath  string  `json:"currentImagePath"`
	// DiffImagePath is the path to the pixel-diff overlay PNG. Empty if images are identical.
	DiffImagePath string  `json:"diffImagePath,omitempty"`
	DiffPercent   float64 `json:"diffPercent"`
}

// URLDiff holds the comparison result for a single URL.
type URLDiff struct {
	URL            string          `json:"url"`
	Breakpoints    []BreakpointDiff `json:"breakpoints"`
	AuditDelta     AuditDelta      `json:"auditDelta"`
	ElementChanges []ElementChange `json:"elementChanges,omitempty"`
}

// SnapshotDiff is the top-level result of comparing two snapshots.
type SnapshotDiff struct {
	BaselineID string    `json:"baselineId"`
	CurrentID  string    `json:"currentId"`
	URLs       []URLDiff `json:"urls"`
}

// Compare computes a diff between a baseline and a current snapshot.
// It matches URLs by identity and breakpoints by name.
func Compare(baseline, current *snapshot.Snapshot) *SnapshotDiff {
	d := &SnapshotDiff{
		BaselineID: baseline.ID,
		CurrentID:  current.ID,
	}

	// Index baseline URLs for fast lookup.
	baselineByURL := make(map[string]*snapshot.URLSnapshot, len(baseline.URLs))
	for i := range baseline.URLs {
		baselineByURL[baseline.URLs[i].URL] = &baseline.URLs[i]
	}

	for _, cur := range current.URLs {
		ud := URLDiff{URL: cur.URL}

		base, ok := baselineByURL[cur.URL]
		if !ok {
			// URL is new — no diff data available.
			d.URLs = append(d.URLs, ud)
			continue
		}

		// Index baseline breakpoints.
		baselineByBP := make(map[string]*snapshot.BreakpointSnapshot, len(base.Breakpoints))
		for i := range base.Breakpoints {
			baselineByBP[base.Breakpoints[i].Name] = &base.Breakpoints[i]
		}

		for _, cbp := range cur.Breakpoints {
			bpd := BreakpointDiff{
				Name:             cbp.Name,
				Width:            cbp.Width,
				Height:           cbp.Height,
				CurrentImagePath: cbp.ImagePath,
			}

			if bbp, ok := baselineByBP[cbp.Name]; ok {
				bpd.BaselineImagePath = bbp.ImagePath
				ud.AuditDelta = computeAuditDelta(bbp.AuditResult, cbp.AuditResult)
				ud.ElementChanges = diffAXNodes(bbp.AXNodes, cbp.AXNodes)
			}

			ud.Breakpoints = append(ud.Breakpoints, bpd)
		}

		d.URLs = append(d.URLs, ud)
	}

	return d
}

// computeAuditDelta calculates before/after counts from two AuditSummary values.
func computeAuditDelta(before, after snapshot.AuditSummary) AuditDelta {
	return AuditDelta{
		ContrastBefore:   before.ContrastViolations,
		ContrastAfter:    after.ContrastViolations,
		TouchBefore:      before.TouchViolations,
		TouchAfter:       after.TouchViolations,
		TypographyBefore: before.TypographyWarnings,
		TypographyAfter:  after.TypographyWarnings,
		SpacingBefore:    before.SpacingIssues,
		SpacingAfter:     after.SpacingIssues,
	}
}

// diffAXNodes compares two accessibility trees and returns element changes.
// Nodes are matched by role+name identity.
func diffAXNodes(before, after []snapshot.AXNode) []ElementChange {
	var changes []ElementChange

	beforeByKey := make(map[string]snapshot.AXNode, len(before))
	for _, n := range before {
		beforeByKey[nodeKey(n)] = n
	}

	afterByKey := make(map[string]snapshot.AXNode, len(after))
	for _, n := range after {
		afterByKey[nodeKey(n)] = n
	}

	// Disappeared nodes.
	for k, n := range beforeByKey {
		if _, ok := afterByKey[k]; !ok {
			changes = append(changes, ElementChange{
				Selector: n.Selector,
				Role:     n.Role,
				Name:     n.Name,
				Type:     "disappeared",
			})
		}
	}

	// Appeared nodes.
	for k, n := range afterByKey {
		if _, ok := beforeByKey[k]; !ok {
			changes = append(changes, ElementChange{
				Selector: n.Selector,
				Role:     n.Role,
				Name:     n.Name,
				Type:     "appeared",
			})
		}
	}

	return changes
}

func nodeKey(n snapshot.AXNode) string {
	return n.Role + "\x00" + n.Name
}
