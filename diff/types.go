// Package diff implements the pure-function diff engine for comparing snapshots.
package diff

// DiffResult is the top-level output of a diff operation.
type DiffResult struct {
	SnapshotID string        `json:"snapshotId"`
	BaselineID string        `json:"baselineId"`
	Regressions bool         `json:"regressions"`
	Audit      AuditDelta    `json:"audit"`
	Elements   ElementDelta  `json:"elements"`
}

// AuditDelta describes audit issue changes between baseline and current.
type AuditDelta struct {
	Categories    map[string]CategoryDelta `json:"categories"`
	NewIssues     []IssueDiff              `json:"newIssues"`
	Resolved      []IssueDiff              `json:"resolved"`
	HasRegression bool                     `json:"hasRegression"`
}

// CategoryDelta holds per-category issue count changes.
// A positive Delta indicates a regression (more issues than baseline).
type CategoryDelta struct {
	Category string `json:"category"`
	Baseline int    `json:"baseline"`
	Current  int    `json:"current"`
	Delta    int    `json:"delta"`
}

// IssueDiff represents a single issue that is new or resolved relative to baseline.
type IssueDiff struct {
	Selector string `json:"selector"`
	Category string `json:"category"`
	Message  string `json:"message"`
	Status   string `json:"status"` // "new" or "resolved"
}

// ElementDelta describes element-level changes between baseline and current snapshots.
type ElementDelta struct {
	Appeared      []ElementChange `json:"appeared"`
	Disappeared   []ElementChange `json:"disappeared"`
	Moved         []ElementChange `json:"moved"`
	Resized       []ElementChange `json:"resized"`
	HasRegression bool            `json:"hasRegression"`
}

// ElementChange describes a single element that changed between snapshots.
type ElementChange struct {
	Role     string         `json:"role"`
	Name     string         `json:"name"`
	Identity string         `json:"identity"`
	Before   *ElementState  `json:"before,omitempty"`
	After    *ElementState  `json:"after,omitempty"`
	Delta    *MoveDelta     `json:"delta,omitempty"`
}

// ElementState holds the position and size of an element.
type ElementState struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// MoveDelta holds the deltas for position and size changes.
type MoveDelta struct {
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
	DW float64 `json:"dw"`
	DH float64 `json:"dh"`
}
