package diff

import (
	"math"
	"strings"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

const (
	// PositionThreshold is the minimum pixel delta in X or Y that counts as a move.
	PositionThreshold = 4.0
	// SizeThreshold is the minimum pixel delta in Width or Height that counts as a resize.
	SizeThreshold = 4.0
)

// semanticID returns a canonical identity string for an AXNode based on role and name.
func semanticID(node snapshot.AXNode) string {
	return strings.ToLower(strings.TrimSpace(node.Role)) + ":" + strings.ToLower(strings.TrimSpace(node.Name))
}

// buildNodeMap indexes AXNodes by semantic identity, skipping nodes with empty role and name.
func buildNodeMap(nodes []snapshot.AXNode) map[string]snapshot.AXNode {
	m := make(map[string]snapshot.AXNode, len(nodes))
	for _, node := range nodes {
		if node.Role == "" && node.Name == "" {
			continue
		}
		m[semanticID(node)] = node
	}
	return m
}

// toElementState converts a BoundingBox pointer to an ElementState pointer.
// Returns nil if bb is nil.
func toElementState(bb *snapshot.BoundingBox) *ElementState {
	if bb == nil {
		return nil
	}
	return &ElementState{X: bb.X, Y: bb.Y, Width: bb.Width, Height: bb.Height}
}

// ComputeElementDelta computes element-level changes between two accessibility trees.
// It is a pure function with no Chrome or filesystem dependency.
func ComputeElementDelta(baseline, current []snapshot.AXNode) ElementDelta {
	baselineMap := buildNodeMap(baseline)
	currentMap := buildNodeMap(current)

	var appeared, disappeared, moved, resized []ElementChange

	// Step 3: Elements in current but not baseline → Appeared.
	for id, node := range currentMap {
		if _, exists := baselineMap[id]; !exists {
			appeared = append(appeared, ElementChange{
				Role:     node.Role,
				Name:     node.Name,
				Identity: id,
				Before:   nil,
				After:    toElementState(node.BoundingBox),
			})
		}
	}

	// Step 4: Elements in baseline but not current → Disappeared.
	for id, node := range baselineMap {
		if _, exists := currentMap[id]; !exists {
			disappeared = append(disappeared, ElementChange{
				Role:     node.Role,
				Name:     node.Name,
				Identity: id,
				Before:   toElementState(node.BoundingBox),
				After:    nil,
			})
		}
	}

	// Step 5: Elements in both — check for moves and resizes.
	for id, bNode := range baselineMap {
		cNode, exists := currentMap[id]
		if !exists {
			continue
		}
		if bNode.BoundingBox == nil || cNode.BoundingBox == nil {
			continue
		}

		dx := cNode.BoundingBox.X - bNode.BoundingBox.X
		dy := cNode.BoundingBox.Y - bNode.BoundingBox.Y
		dw := cNode.BoundingBox.Width - bNode.BoundingBox.Width
		dh := cNode.BoundingBox.Height - bNode.BoundingBox.Height

		delta := &MoveDelta{DX: dx, DY: dy, DW: dw, DH: dh}
		before := toElementState(bNode.BoundingBox)
		after := toElementState(cNode.BoundingBox)

		if math.Abs(dx) > PositionThreshold || math.Abs(dy) > PositionThreshold {
			moved = append(moved, ElementChange{
				Role:     cNode.Role,
				Name:     cNode.Name,
				Identity: id,
				Before:   before,
				After:    after,
				Delta:    delta,
			})
		}

		if math.Abs(dw) > SizeThreshold || math.Abs(dh) > SizeThreshold {
			resized = append(resized, ElementChange{
				Role:     cNode.Role,
				Name:     cNode.Name,
				Identity: id,
				Before:   before,
				After:    after,
				Delta:    delta,
			})
		}
	}

	// Step 6: HasRegression — Appeared is informational only.
	hasRegression := len(disappeared) > 0 || len(moved) > 0 || len(resized) > 0

	return ElementDelta{
		Appeared:      appeared,
		Disappeared:   disappeared,
		Moved:         moved,
		Resized:       resized,
		HasRegression: hasRegression,
	}
}
