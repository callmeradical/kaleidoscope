package diff

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

// AuditDelta holds before/after counts and their delta for one audit category.
type AuditDelta struct {
	Category string `json:"category"`
	Before   int    `json:"before"`
	After    int    `json:"after"`
	Delta    int    `json:"delta"` // After - Before; positive = regression
}

// ElementChange describes one element-level change between two snapshots.
type ElementChange struct {
	Role     string `json:"role"`
	Name     string `json:"name"`
	Selector string `json:"selector"`
	// Type is one of "appeared", "disappeared", "moved", "resized".
	Type    string `json:"type"`
	Details string `json:"details"`
}

// BreakpointDiff holds screenshot comparison data for one viewport size.
type BreakpointDiff struct {
	Name          string  `json:"name"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	BaselinePath  string  `json:"baselinePath"`
	CurrentPath   string  `json:"currentPath"`
	DiffPNG       []byte  `json:"-"`
	DiffPercent   float64 `json:"diffPercent"`
	ChangedPixels int     `json:"changedPixels"`
	TotalPixels   int     `json:"totalPixels"`
}

// URLDiff holds all diff results for one URL.
type URLDiff struct {
	URL            string          `json:"url"`
	AuditDeltas    []AuditDelta    `json:"auditDeltas"`
	ElementChanges []ElementChange `json:"elementChanges"`
	Breakpoints    []BreakpointDiff `json:"breakpoints"`
	HasRegression  bool            `json:"hasRegression"`
}

// Result is the top-level output of the diff engine.
type Result struct {
	BaselineID     string    `json:"baselineId"`
	CurrentID      string    `json:"currentId"`
	GeneratedAt    time.Time `json:"generatedAt"`
	URLs           []URLDiff `json:"urls"`
	HasRegressions bool      `json:"hasRegressions"`
}

// Compute computes the diff between a baseline and current snapshot.
func Compute(baseline, current *snapshot.Snapshot) (*Result, error) {
	baselineMap := make(map[string]*snapshot.URLSnapshot, len(baseline.URLs))
	for i := range baseline.URLs {
		baselineMap[baseline.URLs[i].URL] = &baseline.URLs[i]
	}

	result := &Result{
		BaselineID:  baseline.ID,
		CurrentID:   current.ID,
		GeneratedAt: time.Now(),
	}

	for i := range current.URLs {
		curURL := &current.URLs[i]
		baseURL, ok := baselineMap[curURL.URL]
		if !ok {
			continue
		}

		deltas := computeAuditDeltas(baseURL.Audit, curURL.Audit)
		changes := computeElementChanges(baseURL.Elements, curURL.Elements)

		bpDiffs, err := computeBreakpointDiffs(baseline.ID, current.ID, baseURL.Breakpoints, curURL.Breakpoints)
		if err != nil {
			return nil, fmt.Errorf("computing breakpoint diffs for %s: %w", curURL.URL, err)
		}

		urlDiff := URLDiff{
			URL:            curURL.URL,
			AuditDeltas:    deltas,
			ElementChanges: changes,
			Breakpoints:    bpDiffs,
			HasRegression:  hasAuditRegression(deltas),
		}
		result.URLs = append(result.URLs, urlDiff)
		if urlDiff.HasRegression {
			result.HasRegressions = true
		}
	}

	return result, nil
}

func computeAuditDeltas(base, cur snapshot.AuditResult) []AuditDelta {
	return []AuditDelta{
		{Category: "Contrast", Before: base.ContrastViolations, After: cur.ContrastViolations, Delta: cur.ContrastViolations - base.ContrastViolations},
		{Category: "Touch", Before: base.TouchViolations, After: cur.TouchViolations, Delta: cur.TouchViolations - base.TouchViolations},
		{Category: "Typography", Before: base.TypographyWarnings, After: cur.TypographyWarnings, Delta: cur.TypographyWarnings - base.TypographyWarnings},
		{Category: "Spacing", Before: base.SpacingIssues, After: cur.SpacingIssues, Delta: cur.SpacingIssues - base.SpacingIssues},
	}
}

func hasAuditRegression(deltas []AuditDelta) bool {
	for _, d := range deltas {
		if d.Delta > 0 {
			return true
		}
	}
	return false
}

func computeElementChanges(baseElems, curElems []snapshot.AXElement) []ElementChange {
	type key = string
	elemKey := func(e snapshot.AXElement) key { return e.Role + "|" + e.Name }

	baseIndex := make(map[key]*snapshot.AXElement, len(baseElems))
	for i := range baseElems {
		k := elemKey(baseElems[i])
		baseIndex[k] = &baseElems[i]
	}
	curIndex := make(map[key]*snapshot.AXElement, len(curElems))
	for i := range curElems {
		k := elemKey(curElems[i])
		curIndex[k] = &curElems[i]
	}

	const threshold = 2.0
	var changes []ElementChange

	for k, bEl := range baseIndex {
		cEl, found := curIndex[k]
		if !found {
			changes = append(changes, ElementChange{
				Role:     bEl.Role,
				Name:     bEl.Name,
				Selector: bEl.Selector,
				Type:     "disappeared",
			})
			continue
		}
		// Check moved
		dx := cEl.X - bEl.X
		dy := cEl.Y - bEl.Y
		if math.Abs(dx) > threshold || math.Abs(dy) > threshold {
			changes = append(changes, ElementChange{
				Role:     bEl.Role,
				Name:     bEl.Name,
				Selector: bEl.Selector,
				Type:     "moved",
				Details:  movementDetails(dx, dy),
			})
			continue
		}
		// Check resized
		dw := cEl.Width - bEl.Width
		dh := cEl.Height - bEl.Height
		if math.Abs(dw) > threshold || math.Abs(dh) > threshold {
			changes = append(changes, ElementChange{
				Role:     bEl.Role,
				Name:     bEl.Name,
				Selector: bEl.Selector,
				Type:     "resized",
				Details:  resizeDetails(dw, dh),
			})
		}
	}

	for k, cEl := range curIndex {
		if _, found := baseIndex[k]; !found {
			changes = append(changes, ElementChange{
				Role:     cEl.Role,
				Name:     cEl.Name,
				Selector: cEl.Selector,
				Type:     "appeared",
			})
		}
	}

	return changes
}

func movementDetails(dx, dy float64) string {
	var parts []string
	if math.Abs(dx) > 2 {
		dir := "right"
		if dx < 0 {
			dir = "left"
		}
		parts = append(parts, fmt.Sprintf("moved %.0fpx %s", math.Abs(dx), dir))
	}
	if math.Abs(dy) > 2 {
		dir := "down"
		if dy < 0 {
			dir = "up"
		}
		parts = append(parts, fmt.Sprintf("%.0fpx %s", math.Abs(dy), dir))
	}
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}

func resizeDetails(dw, dh float64) string {
	return fmt.Sprintf("width %+.0fpx, height %+.0fpx", dw, dh)
}

func computeBreakpointDiffs(baselineID, currentID string, baseBPs, curBPs []snapshot.BreakpointSnapshot) ([]BreakpointDiff, error) {
	baseIndex := make(map[string]*snapshot.BreakpointSnapshot, len(baseBPs))
	for i := range baseBPs {
		baseIndex[baseBPs[i].Name] = &baseBPs[i]
	}

	var diffs []BreakpointDiff
	for i := range curBPs {
		cur := &curBPs[i]
		base, ok := baseIndex[cur.Name]
		if !ok {
			continue
		}

		baselinePath := fmt.Sprintf(".kaleidoscope/snapshots/%s/%s", baselineID, base.ScreenshotPath)
		currentPath := fmt.Sprintf(".kaleidoscope/snapshots/%s/%s", currentID, cur.ScreenshotPath)

		basePNG, err := os.ReadFile(baselinePath)
		if err != nil {
			// Skip if screenshot not available (e.g. in tests).
			diffs = append(diffs, BreakpointDiff{Name: cur.Name, Width: cur.Width, Height: cur.Height, BaselinePath: baselinePath, CurrentPath: currentPath})
			continue
		}
		curPNG, err := os.ReadFile(currentPath)
		if err != nil {
			diffs = append(diffs, BreakpointDiff{Name: cur.Name, Width: cur.Width, Height: cur.Height, BaselinePath: baselinePath, CurrentPath: currentPath})
			continue
		}

		diffPNG, changed, total, err := CompareImages(basePNG, curPNG, 10)
		if err != nil {
			return nil, err
		}
		pct := 0.0
		if total > 0 {
			pct = float64(changed) / float64(total) * 100
		}
		diffs = append(diffs, BreakpointDiff{
			Name:          cur.Name,
			Width:         cur.Width,
			Height:        cur.Height,
			BaselinePath:  baselinePath,
			CurrentPath:   currentPath,
			DiffPNG:       diffPNG,
			DiffPercent:   pct,
			ChangedPixels: changed,
			TotalPixels:   total,
		})
	}
	return diffs, nil
}
