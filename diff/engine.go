package diff

import (
	"math"
	"strings"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// Thresholds controls what counts as a meaningful change.
type Thresholds struct {
	PositionDelta float64
	SizeDelta     float64
}

// DefaultThresholds returns the standard detection thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{PositionDelta: 4.0, SizeDelta: 4.0}
}

// CategoryDelta summarises issue count changes for one audit category.
type CategoryDelta struct {
	Baseline int `json:"baseline"`
	Target   int `json:"target"`
	Delta    int `json:"delta"`
}

// IssueChange represents a single audit issue that appeared or was resolved.
type IssueChange struct {
	Category string `json:"category"`
	Selector string `json:"selector"`
	Message  string `json:"message"`
}

// AuditDiff is the result of comparing audit data between two snapshots.
type AuditDiff struct {
	Categories     map[string]CategoryDelta `json:"categories"`
	NewIssues      []IssueChange            `json:"newIssues"`
	ResolvedIssues []IssueChange            `json:"resolvedIssues"`
}

// Delta2D represents a two-dimensional change value.
type Delta2D struct {
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
}

// ElementChange describes one element change event.
type ElementChange struct {
	Role          string                   `json:"role"`
	Name          string                   `json:"name"`
	BaselineBox   *snapshot.BoundingBox    `json:"baselineBox,omitempty"`
	TargetBox     *snapshot.BoundingBox    `json:"targetBox,omitempty"`
	PositionDelta *Delta2D                 `json:"positionDelta,omitempty"`
	SizeDelta     *Delta2D                 `json:"sizeDelta,omitempty"`
}

// ElementDiff is the result of comparing element trees between two snapshots.
type ElementDiff struct {
	Appeared   []ElementChange `json:"appeared"`
	Disappeared []ElementChange `json:"disappeared"`
	Moved      []ElementChange `json:"moved"`
	Resized    []ElementChange `json:"resized"`
}

// Result is the top-level output of a diff run.
type Result struct {
	SnapshotID     string      `json:"snapshotId"`
	BaselineID     string      `json:"baselineId"`
	HasRegressions bool        `json:"hasRegressions"`
	Audit          AuditDiff   `json:"audit"`
	Elements       ElementDiff `json:"elements"`
}

// categories is the ordered set of audit categories.
var categories = []string{"contrast", "touch", "typography", "spacing"}

// issuesForCategory returns the issue slice for a given category name.
func issuesForCategory(data snapshot.AuditData, cat string) []snapshot.AuditIssue {
	switch cat {
	case "contrast":
		return data.Contrast
	case "touch":
		return data.Touch
	case "typography":
		return data.Typography
	case "spacing":
		return data.Spacing
	}
	return nil
}

// DiffAudit compares audit data between a baseline and target snapshot.
func DiffAudit(baseline, target snapshot.AuditData) AuditDiff {
	result := AuditDiff{
		Categories:     make(map[string]CategoryDelta),
		NewIssues:      []IssueChange{},
		ResolvedIssues: []IssueChange{},
	}

	for _, cat := range categories {
		baseIssues := issuesForCategory(baseline, cat)
		targetIssues := issuesForCategory(target, cat)

		baseMap := make(map[string]snapshot.AuditIssue, len(baseIssues))
		for _, issue := range baseIssues {
			baseMap[issue.Selector] = issue
		}

		targetMap := make(map[string]snapshot.AuditIssue, len(targetIssues))
		for _, issue := range targetIssues {
			targetMap[issue.Selector] = issue
		}

		result.Categories[cat] = CategoryDelta{
			Baseline: len(baseMap),
			Target:   len(targetMap),
			Delta:    len(targetMap) - len(baseMap),
		}

		// New issues: in target but not in baseline
		for sel, issue := range targetMap {
			if _, exists := baseMap[sel]; !exists {
				result.NewIssues = append(result.NewIssues, IssueChange{
					Category: cat,
					Selector: sel,
					Message:  issue.Message,
				})
			}
		}

		// Resolved issues: in baseline but not in target
		for sel, issue := range baseMap {
			if _, exists := targetMap[sel]; !exists {
				result.ResolvedIssues = append(result.ResolvedIssues, IssueChange{
					Category: cat,
					Selector: sel,
					Message:  issue.Message,
				})
			}
		}
	}

	return result
}

// semanticKey returns a normalised identity key for an element.
func semanticKey(e snapshot.Element) string {
	return strings.ToLower(strings.TrimSpace(e.Role)) + ":" + strings.ToLower(strings.TrimSpace(e.Name))
}

// DiffElements compares element trees using semantic identity (role + name).
func DiffElements(baseline, target []snapshot.Element, t Thresholds) ElementDiff {
	result := ElementDiff{
		Appeared:    []ElementChange{},
		Disappeared: []ElementChange{},
		Moved:       []ElementChange{},
		Resized:     []ElementChange{},
	}

	baseMap := make(map[string]snapshot.Element)
	for _, e := range baseline {
		if strings.TrimSpace(e.Name) == "" {
			continue
		}
		baseMap[semanticKey(e)] = e
	}

	targetMap := make(map[string]snapshot.Element)
	for _, e := range target {
		if strings.TrimSpace(e.Name) == "" {
			continue
		}
		targetMap[semanticKey(e)] = e
	}

	// Appeared: in target but not in baseline
	for key, e := range targetMap {
		if _, exists := baseMap[key]; !exists {
			box := e.Box
			result.Appeared = append(result.Appeared, ElementChange{
				Role:      e.Role,
				Name:      e.Name,
				TargetBox: &box,
			})
		}
	}

	// Disappeared: in baseline but not in target
	for key, e := range baseMap {
		if _, exists := targetMap[key]; !exists {
			box := e.Box
			result.Disappeared = append(result.Disappeared, ElementChange{
				Role:        e.Role,
				Name:        e.Name,
				BaselineBox: &box,
			})
		}
	}

	// Moved / Resized: present in both
	for key, bElem := range baseMap {
		tElem, exists := targetMap[key]
		if !exists {
			continue
		}
		bBox := bElem.Box
		tBox := tElem.Box

		dx := tBox.X - bBox.X
		dy := tBox.Y - bBox.Y
		if math.Abs(dx) > t.PositionDelta || math.Abs(dy) > t.PositionDelta {
			bBoxCopy := bBox
			tBoxCopy := tBox
			result.Moved = append(result.Moved, ElementChange{
				Role:          bElem.Role,
				Name:          bElem.Name,
				BaselineBox:   &bBoxCopy,
				TargetBox:     &tBoxCopy,
				PositionDelta: &Delta2D{DX: dx, DY: dy},
			})
		}

		dw := tBox.Width - bBox.Width
		dh := tBox.Height - bBox.Height
		if math.Abs(dw) > t.SizeDelta || math.Abs(dh) > t.SizeDelta {
			bBoxCopy := bBox
			tBoxCopy := tBox
			result.Resized = append(result.Resized, ElementChange{
				Role:        bElem.Role,
				Name:        bElem.Name,
				BaselineBox: &bBoxCopy,
				TargetBox:   &tBoxCopy,
				SizeDelta:   &Delta2D{DX: dw, DY: dh},
			})
		}
	}

	return result
}

// Run computes a full diff between baseline and target snapshots.
func Run(baseline, target *snapshot.Snapshot, t Thresholds) *Result {
	auditDiff := DiffAudit(baseline.Audit, target.Audit)
	elemDiff := DiffElements(baseline.Elements, target.Elements, t)

	hasRegressions := len(auditDiff.NewIssues) > 0 ||
		len(elemDiff.Disappeared) > 0 ||
		len(elemDiff.Moved) > 0 ||
		len(elemDiff.Resized) > 0

	return &Result{
		SnapshotID:     target.ID,
		BaselineID:     baseline.ID,
		HasRegressions: hasRegressions,
		Audit:          auditDiff,
		Elements:       elemDiff,
	}
}
