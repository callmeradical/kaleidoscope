// Package diff provides a pure-function engine that compares two snapshots
// and reports regressions as structured data.
package diff

import "math"

// MoveThreshold is the minimum position delta (in pixels) to consider an
// element as "moved".
const MoveThreshold = 5.0

// ResizeThreshold is the minimum size delta (in pixels) to consider an
// element as "resized".
const ResizeThreshold = 5.0

// SnapshotData holds the per-path data extracted from a snapshot for diffing.
type SnapshotData struct {
	Audit  map[string]any `json:"audit"`
	AxTree map[string]any `json:"axTree"`
}

// DiffResult is the top-level output of a comparison.
type DiffResult struct {
	HasRegressions  bool                   `json:"hasRegressions"`
	AuditDeltas     map[string]*AuditDelta `json:"auditDeltas"`
	ElementChanges  []ElementChange        `json:"elementChanges"`
	ScreenshotDiffs []ScreenshotDiffResult `json:"screenshotDiffs,omitempty"`
	Summary         Summary                `json:"summary"`
}

// AuditDelta reports the change in a single audit category.
type AuditDelta struct {
	Category      string `json:"category"`
	BaselineCount int    `json:"baselineCount"`
	CurrentCount  int    `json:"currentCount"`
	Delta         int    `json:"delta"` // positive = regression
}

// ElementChange describes a single element-level change.
type ElementChange struct {
	Kind     string  `json:"kind"` // "appeared", "disappeared", "moved", "resized"
	Role     string  `json:"role"`
	Name     string  `json:"name"`
	Detail   string  `json:"detail,omitempty"`
	DeltaX   float64 `json:"deltaX,omitempty"`
	DeltaY   float64 `json:"deltaY,omitempty"`
	DeltaW   float64 `json:"deltaW,omitempty"`
	DeltaH   float64 `json:"deltaH,omitempty"`
}

// Summary gives aggregate counts.
type Summary struct {
	NewAuditIssues      int `json:"newAuditIssues"`
	ResolvedAuditIssues int `json:"resolvedAuditIssues"`
	ElementsAppeared    int `json:"elementsAppeared"`
	ElementsDisappeared int `json:"elementsDisappeared"`
	ElementsMoved       int `json:"elementsMoved"`
	ElementsResized     int `json:"elementsResized"`
}

// axNode is the normalized representation of an accessibility tree node.
type axNode struct {
	Role string
	Name string
	// Bounding box (from properties, if present)
	X, Y, W, H float64
	HasBounds   bool
}

// Compare takes baseline and current snapshot data and returns a structured diff.
// This is a pure function with no side effects.
func Compare(baseline, current SnapshotData) *DiffResult {
	result := &DiffResult{
		AuditDeltas: make(map[string]*AuditDelta),
	}

	// --- Audit deltas ---
	baseAudit := extractAuditCounts(baseline.Audit)
	currAudit := extractAuditCounts(current.Audit)

	categories := []string{"contrast", "touchTargets", "typography"}
	for _, cat := range categories {
		bCount := baseAudit[cat]
		cCount := currAudit[cat]
		delta := &AuditDelta{
			Category:      cat,
			BaselineCount: bCount,
			CurrentCount:  cCount,
			Delta:         cCount - bCount,
		}
		result.AuditDeltas[cat] = delta
		if delta.Delta > 0 {
			result.Summary.NewAuditIssues += delta.Delta
		} else if delta.Delta < 0 {
			result.Summary.ResolvedAuditIssues += -delta.Delta
		}
	}

	// --- Element changes via ax-tree ---
	baseNodes := extractAxNodes(baseline.AxTree)
	currNodes := extractAxNodes(current.AxTree)

	baseMap := indexByIdentity(baseNodes)
	currMap := indexByIdentity(currNodes)

	// Appeared: in current but not baseline
	for key, cNode := range currMap {
		if _, ok := baseMap[key]; !ok {
			result.ElementChanges = append(result.ElementChanges, ElementChange{
				Kind: "appeared",
				Role: cNode.Role,
				Name: cNode.Name,
			})
			result.Summary.ElementsAppeared++
		}
	}

	// Disappeared: in baseline but not current
	for key, bNode := range baseMap {
		if _, ok := currMap[key]; !ok {
			result.ElementChanges = append(result.ElementChanges, ElementChange{
				Kind: "disappeared",
				Role: bNode.Role,
				Name: bNode.Name,
			})
			result.Summary.ElementsDisappeared++
		}
	}

	// Moved / Resized: in both, compare bounds
	for key, bNode := range baseMap {
		cNode, ok := currMap[key]
		if !ok {
			continue
		}
		if !bNode.HasBounds || !cNode.HasBounds {
			continue
		}

		dx := cNode.X - bNode.X
		dy := cNode.Y - bNode.Y
		dw := cNode.W - bNode.W
		dh := cNode.H - bNode.H

		if math.Abs(dx) >= MoveThreshold || math.Abs(dy) >= MoveThreshold {
			result.ElementChanges = append(result.ElementChanges, ElementChange{
				Kind:   "moved",
				Role:   bNode.Role,
				Name:   bNode.Name,
				DeltaX: dx,
				DeltaY: dy,
			})
			result.Summary.ElementsMoved++
		}

		if math.Abs(dw) >= ResizeThreshold || math.Abs(dh) >= ResizeThreshold {
			result.ElementChanges = append(result.ElementChanges, ElementChange{
				Kind:   "resized",
				Role:   bNode.Role,
				Name:   bNode.Name,
				DeltaW: dw,
				DeltaH: dh,
			})
			result.Summary.ElementsResized++
		}
	}

	// Determine if regressions exist
	result.HasRegressions = result.Summary.NewAuditIssues > 0 ||
		result.Summary.ElementsDisappeared > 0

	return result
}

// extractAuditCounts pulls per-category issue counts from the audit payload.
// Expects the structure produced by captureAudit in cmd/snapshot.go.
func extractAuditCounts(audit map[string]any) map[string]int {
	counts := map[string]int{
		"contrast":     0,
		"touchTargets": 0,
		"typography":   0,
	}
	if audit == nil {
		return counts
	}

	// Try summary block first (canonical format)
	if summary, ok := audit["summary"].(map[string]any); ok {
		if v, ok := toInt(summary["contrastViolations"]); ok {
			counts["contrast"] = v
		}
		if v, ok := toInt(summary["touchViolations"]); ok {
			counts["touchTargets"] = v
		}
		if v, ok := toInt(summary["typographyWarnings"]); ok {
			counts["typography"] = v
		}
		return counts
	}

	// Fallback: per-category blocks
	if c, ok := audit["contrast"].(map[string]any); ok {
		if v, ok := toInt(c["violations"]); ok {
			counts["contrast"] = v
		}
	}
	if tt, ok := audit["touchTargets"].(map[string]any); ok {
		if v, ok := toInt(tt["violations"]); ok {
			counts["touchTargets"] = v
		}
	}
	if ty, ok := audit["typography"].(map[string]any); ok {
		if v, ok := toInt(ty["warnings"]); ok {
			counts["typography"] = v
		}
	}

	return counts
}

// extractAxNodes parses the ax-tree payload into a slice of axNode.
func extractAxNodes(axTree map[string]any) []axNode {
	if axTree == nil {
		return nil
	}
	nodesRaw, ok := axTree["nodes"].([]any)
	if !ok {
		return nil
	}

	var nodes []axNode
	for _, raw := range nodesRaw {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		n := axNode{
			Role: toString(m["role"]),
			Name: toString(m["name"]),
		}
		// Extract bounding box from properties if present
		if props, ok := m["properties"].(map[string]any); ok {
			if bb, ok := props["boundingBox"].(map[string]any); ok {
				n.X, _ = toFloat(bb["x"])
				n.Y, _ = toFloat(bb["y"])
				n.W, _ = toFloat(bb["width"])
				n.H, _ = toFloat(bb["height"])
				n.HasBounds = true
			}
		}
		nodes = append(nodes, n)
	}
	return nodes
}

// indexByIdentity creates a map keyed by "role:name" for matching elements.
func indexByIdentity(nodes []axNode) map[string]axNode {
	m := make(map[string]axNode, len(nodes))
	for _, n := range nodes {
		key := n.Role + ":" + n.Name
		// If duplicate identity, keep the last one (reasonable for flat trees)
		m[key] = n
	}
	return m
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
