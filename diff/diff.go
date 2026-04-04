// Package diff provides a pure-function diff engine for comparing Kaleidoscope
// snapshots. It has no browser or filesystem dependency.
package diff

import (
	"math"
	"strings"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

const (
	// PositionThreshold is the minimum pixel delta (absolute) in X or Y that
	// classifies an element as "moved".
	PositionThreshold = 4.0
	// SizeThreshold is the minimum pixel delta (absolute) in width or height
	// that classifies an element as "resized".
	SizeThreshold = 4.0
)

// DiffResult is the top-level output of the diff engine.
type DiffResult struct {
	SnapshotID  string      `json:"snapshot_id"`
	BaselineID  string      `json:"baseline_id"`
	Regressions bool        `json:"regressions"`
	Audit       AuditDiff   `json:"audit"`
	Elements    ElementDiff `json:"elements"`
}

// AuditDiff describes changes between two audit summaries.
type AuditDiff struct {
	ContrastDelta    int          `json:"contrast_delta"`
	TouchDelta       int          `json:"touch_delta"`
	TypographyDelta  int          `json:"typography_delta"`
	TotalDelta       int          `json:"total_delta"`
	NewIssues        []IssueDelta `json:"new_issues"`
	ResolvedIssues   []IssueDelta `json:"resolved_issues"`
}

// IssueDelta records a single audit issue that has appeared or been resolved.
type IssueDelta struct {
	Category string `json:"category"`
	Selector string `json:"selector"`
	Detail   string `json:"detail"`
}

// ElementDiff describes structural changes between two accessibility trees.
type ElementDiff struct {
	Appeared    []ElementChange `json:"appeared"`
	Disappeared []ElementChange `json:"disappeared"`
	Moved       []ElementChange `json:"moved"`
	Resized     []ElementChange `json:"resized"`
}

// ElementChange records a change to a single semantic element.
type ElementChange struct {
	Role     string  `json:"role"`
	Name     string  `json:"name"`
	Baseline *Rect   `json:"baseline,omitempty"`
	Target   *Rect   `json:"target,omitempty"`
	Delta    *Delta  `json:"delta,omitempty"`
}

// Rect holds the bounding box of an element.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Delta describes the difference in position or size between two rects.
type Delta struct {
	DX      float64 `json:"dx,omitempty"`
	DY      float64 `json:"dy,omitempty"`
	DWidth  float64 `json:"d_width,omitempty"`
	DHeight float64 `json:"d_height,omitempty"`
}

// Compare is the top-level diff function. It compares baseline against target
// and returns a DiffResult describing all changes.
func Compare(baseline, target *snapshot.Snapshot) DiffResult {
	auditDiff := CompareAudit(baseline.Audit, target.Audit)
	elemDiff := CompareElements(baseline.AXNodes, target.AXNodes)

	regressions := len(auditDiff.NewIssues) > 0 ||
		len(elemDiff.Appeared)+len(elemDiff.Disappeared)+len(elemDiff.Moved)+len(elemDiff.Resized) > 0

	return DiffResult{
		SnapshotID:  target.ID,
		BaselineID:  baseline.ID,
		Regressions: regressions,
		Audit:       auditDiff,
		Elements:    elemDiff,
	}
}

// CompareAudit compares two AuditSummary values and returns an AuditDiff.
func CompareAudit(baseline, target snapshot.AuditSummary) AuditDiff {
	baselineIndex := buildIssueIndex(baseline)
	targetIndex := buildIssueIndex(target)

	newIssues := []IssueDelta{}
	for key, delta := range targetIndex {
		if _, ok := baselineIndex[key]; !ok {
			newIssues = append(newIssues, delta)
		}
	}

	resolvedIssues := []IssueDelta{}
	for key, delta := range baselineIndex {
		if _, ok := targetIndex[key]; !ok {
			resolvedIssues = append(resolvedIssues, delta)
		}
	}

	contrastDelta := len(target.ContrastViolations) - len(baseline.ContrastViolations)
	touchDelta := len(target.TouchViolations) - len(baseline.TouchViolations)
	typographyDelta := len(target.TypographyWarnings) - len(baseline.TypographyWarnings)

	return AuditDiff{
		ContrastDelta:   contrastDelta,
		TouchDelta:      touchDelta,
		TypographyDelta: typographyDelta,
		TotalDelta:      contrastDelta + touchDelta + typographyDelta,
		NewIssues:       newIssues,
		ResolvedIssues:  resolvedIssues,
	}
}

// buildIssueIndex flattens an AuditSummary into a map keyed by "<category>:<selector>".
func buildIssueIndex(a snapshot.AuditSummary) map[string]IssueDelta {
	m := make(map[string]IssueDelta)
	add := func(category string, issues []snapshot.AuditIssue) {
		for _, issue := range issues {
			key := category + ":" + issue.Selector
			m[key] = IssueDelta{Category: category, Selector: issue.Selector, Detail: issue.Detail}
		}
	}
	add("contrast", a.ContrastViolations)
	add("touch", a.TouchViolations)
	add("typography", a.TypographyWarnings)
	return m
}

// semanticKey produces a stable identity key from role + name.
func semanticKey(role, name string) string {
	return strings.ToLower(strings.TrimSpace(role)) + "|" + strings.ToLower(strings.TrimSpace(name))
}

// nodeToRect converts an AXNode to a Rect.
func nodeToRect(n snapshot.AXNode) *Rect {
	return &Rect{X: n.X, Y: n.Y, Width: n.Width, Height: n.Height}
}

// CompareElements compares two accessibility trees and returns an ElementDiff.
// Matching uses semantic identity (role + name); nodes with empty name are skipped.
func CompareElements(baseline, target []snapshot.AXNode) ElementDiff {
	baselineIndex := make(map[string]snapshot.AXNode)
	for _, n := range baseline {
		if strings.TrimSpace(n.Name) == "" {
			continue
		}
		key := semanticKey(n.Role, n.Name)
		baselineIndex[key] = n
	}

	targetIndex := make(map[string]snapshot.AXNode)
	for _, n := range target {
		if strings.TrimSpace(n.Name) == "" {
			continue
		}
		key := semanticKey(n.Role, n.Name)
		targetIndex[key] = n
	}

	appeared := []ElementChange{}
	disappeared := []ElementChange{}
	moved := []ElementChange{}
	resized := []ElementChange{}

	for key, tn := range targetIndex {
		if _, ok := baselineIndex[key]; !ok {
			appeared = append(appeared, ElementChange{
				Role:   tn.Role,
				Name:   tn.Name,
				Target: nodeToRect(tn),
			})
		}
	}

	for key, bn := range baselineIndex {
		if _, ok := targetIndex[key]; !ok {
			disappeared = append(disappeared, ElementChange{
				Role:     bn.Role,
				Name:     bn.Name,
				Baseline: nodeToRect(bn),
			})
		}
	}

	for key, bn := range baselineIndex {
		tn, ok := targetIndex[key]
		if !ok {
			continue
		}
		dx := tn.X - bn.X
		dy := tn.Y - bn.Y
		if math.Abs(dx) > PositionThreshold || math.Abs(dy) > PositionThreshold {
			moved = append(moved, ElementChange{
				Role:     bn.Role,
				Name:     bn.Name,
				Baseline: nodeToRect(bn),
				Target:   nodeToRect(tn),
				Delta:    &Delta{DX: dx, DY: dy},
			})
		}

		dw := tn.Width - bn.Width
		dh := tn.Height - bn.Height
		if math.Abs(dw) > SizeThreshold || math.Abs(dh) > SizeThreshold {
			resized = append(resized, ElementChange{
				Role:     bn.Role,
				Name:     bn.Name,
				Baseline: nodeToRect(bn),
				Target:   nodeToRect(tn),
				Delta:    &Delta{DWidth: dw, DHeight: dh},
			})
		}
	}

	return ElementDiff{
		Appeared:    appeared,
		Disappeared: disappeared,
		Moved:       moved,
		Resized:     resized,
	}
}
