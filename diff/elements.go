package diff

import (
	"math"
	"sort"
	"strings"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// SemanticKey returns a stable identity key for an element based on role and name.
func SemanticKey(role, name string) string {
	return strings.ToLower(strings.TrimSpace(role)) + ":" + strings.ToLower(strings.TrimSpace(name))
}

// toElementRect converts a snapshot.ElementRect to a diff.ElementRect.
func toElementRect(r *snapshot.ElementRect) *ElementRect {
	if r == nil {
		return nil
	}
	return &ElementRect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}

// abs returns the absolute value of v.
func abs(v float64) float64 {
	return math.Abs(v)
}

// ComputeElementDiff compares two accessibility trees and returns a structured diff.
func ComputeElementDiff(baseline, snap []snapshot.ElementRecord, posThreshold, sizeThreshold float64) ElementDiff {
	baseMap := make(map[string]snapshot.ElementRecord, len(baseline))
	for _, r := range baseline {
		baseMap[SemanticKey(r.Role, r.Name)] = r
	}
	snapMap := make(map[string]snapshot.ElementRecord, len(snap))
	for _, r := range snap {
		snapMap[SemanticKey(r.Role, r.Name)] = r
	}

	appeared := []ElementChange{}
	disappeared := []ElementChange{}
	moved := []ElementChange{}
	resized := []ElementChange{}

	// Appeared: in snapshot but not baseline.
	for key, r := range snapMap {
		if _, found := baseMap[key]; !found {
			appeared = append(appeared, ElementChange{
				Role:     r.Role,
				Name:     r.Name,
				Snapshot: toElementRect(r.Rect),
			})
		}
	}

	// Disappeared: in baseline but not snapshot.
	for key, r := range baseMap {
		if _, found := snapMap[key]; !found {
			disappeared = append(disappeared, ElementChange{
				Role:     r.Role,
				Name:     r.Name,
				Baseline: toElementRect(r.Rect),
			})
		}
	}

	// Moved/Resized: present in both.
	for key, baseRec := range baseMap {
		snapRec, found := snapMap[key]
		if !found {
			continue
		}
		if baseRec.Rect == nil || snapRec.Rect == nil {
			continue
		}
		dx := snapRec.Rect.X - baseRec.Rect.X
		dy := snapRec.Rect.Y - baseRec.Rect.Y
		dw := snapRec.Rect.Width - baseRec.Rect.Width
		dh := snapRec.Rect.Height - baseRec.Rect.Height

		if abs(dx) > posThreshold || abs(dy) > posThreshold {
			moved = append(moved, ElementChange{
				Role:     baseRec.Role,
				Name:     baseRec.Name,
				Baseline: toElementRect(baseRec.Rect),
				Snapshot: toElementRect(snapRec.Rect),
				Delta:    &RectDelta{DX: dx, DY: dy, DW: dw, DH: dh},
			})
		}
		if abs(dw) > sizeThreshold || abs(dh) > sizeThreshold {
			resized = append(resized, ElementChange{
				Role:     baseRec.Role,
				Name:     baseRec.Name,
				Baseline: toElementRect(baseRec.Rect),
				Snapshot: toElementRect(snapRec.Rect),
				Delta:    &RectDelta{DX: dx, DY: dy, DW: dw, DH: dh},
			})
		}
	}

	// Sort all slices for deterministic output.
	sortChanges := func(changes []ElementChange) {
		sort.Slice(changes, func(i, j int) bool {
			ki := SemanticKey(changes[i].Role, changes[i].Name)
			kj := SemanticKey(changes[j].Role, changes[j].Name)
			return ki < kj
		})
	}
	sortChanges(appeared)
	sortChanges(disappeared)
	sortChanges(moved)
	sortChanges(resized)

	hasRegression := len(appeared) > 0 || len(disappeared) > 0 || len(moved) > 0 || len(resized) > 0

	return ElementDiff{
		Appeared:      appeared,
		Disappeared:   disappeared,
		Moved:         moved,
		Resized:       resized,
		HasRegression: hasRegression,
	}
}
