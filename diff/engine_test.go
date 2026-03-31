package diff

import (
	"testing"
)

func TestDiffAudit_NoChange(t *testing.T) {
	audit := map[string]any{
		"summary": map[string]any{
			"contrastViolations": float64(2),
			"touchViolations":    float64(1),
			"typographyWarnings": float64(0),
		},
	}
	deltas := DiffAudit(audit, audit)
	for _, d := range deltas {
		if d.Delta != 0 {
			t.Errorf("expected delta 0 for %s, got %d", d.Category, d.Delta)
		}
	}
}

func TestDiffAudit_Regression(t *testing.T) {
	baseline := map[string]any{
		"summary": map[string]any{
			"contrastViolations": float64(2),
			"touchViolations":    float64(0),
			"typographyWarnings": float64(0),
		},
	}
	current := map[string]any{
		"summary": map[string]any{
			"contrastViolations": float64(5),
			"touchViolations":    float64(0),
			"typographyWarnings": float64(1),
		},
	}
	deltas := DiffAudit(baseline, current)
	found := false
	for _, d := range deltas {
		if d.Category == "contrastViolations" {
			if d.Before != 2 || d.After != 5 || d.Delta != 3 {
				t.Errorf("unexpected contrastViolations delta: %+v", d)
			}
			found = true
		}
	}
	if !found {
		t.Error("contrastViolations delta not found")
	}
	if !HasRegressions(deltas, nil) {
		t.Error("expected regressions to be detected")
	}
}

func TestDiffAudit_Resolved(t *testing.T) {
	baseline := map[string]any{
		"summary": map[string]any{
			"contrastViolations": float64(3),
			"touchViolations":    float64(0),
			"typographyWarnings": float64(0),
		},
	}
	current := map[string]any{
		"summary": map[string]any{
			"contrastViolations": float64(1),
			"touchViolations":    float64(0),
			"typographyWarnings": float64(0),
		},
	}
	deltas := DiffAudit(baseline, current)
	if HasRegressions(deltas, nil) {
		t.Error("expected no regressions when issues are resolved")
	}
}

func TestDiffAxTree_Appeared(t *testing.T) {
	baseline := map[string]any{
		"nodes": []any{
			map[string]any{"nodeId": "1", "role": "button", "name": "Submit"},
		},
	}
	current := map[string]any{
		"nodes": []any{
			map[string]any{"nodeId": "1", "role": "button", "name": "Submit"},
			map[string]any{"nodeId": "2", "role": "link", "name": "Learn more"},
		},
	}
	changes := DiffAxTree(baseline, current)
	found := false
	for _, c := range changes {
		if c.Role == "link" && c.Name == "Learn more" && c.Change == "appeared" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'appeared' change for new link element")
	}
}

func TestDiffAxTree_Disappeared(t *testing.T) {
	baseline := map[string]any{
		"nodes": []any{
			map[string]any{"nodeId": "1", "role": "button", "name": "Submit"},
			map[string]any{"nodeId": "2", "role": "link", "name": "Old link"},
		},
	}
	current := map[string]any{
		"nodes": []any{
			map[string]any{"nodeId": "1", "role": "button", "name": "Submit"},
		},
	}
	changes := DiffAxTree(baseline, current)
	found := false
	for _, c := range changes {
		if c.Role == "link" && c.Name == "Old link" && c.Change == "disappeared" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'disappeared' change for removed link")
	}
	if !HasRegressions(nil, changes) {
		t.Error("expected regressions when element disappears")
	}
}

func TestDiffAxTree_NoChange(t *testing.T) {
	tree := map[string]any{
		"nodes": []any{
			map[string]any{"nodeId": "1", "role": "button", "name": "Submit"},
		},
	}
	changes := DiffAxTree(tree, tree)
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d", len(changes))
	}
}
