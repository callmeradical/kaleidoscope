package diff

// CategoryDelta holds counts for one audit category across baseline and snapshot.
type CategoryDelta struct {
	Baseline int `json:"baseline"`
	Snapshot int `json:"snapshot"`
	Delta    int `json:"delta"` // snapshot - baseline; positive = regression
}

// CategoryDeltas holds per-category audit deltas.
type CategoryDeltas struct {
	Contrast     CategoryDelta `json:"contrast"`
	TouchTargets CategoryDelta `json:"touchTargets"`
	Typography   CategoryDelta `json:"typography"`
}

// AuditIssue is a normalized audit issue used in diff output.
type AuditIssue struct {
	Category string `json:"category"`
	Selector string `json:"selector"`
}

// IssueDelta tracks new and resolved issues by selector.
type IssueDelta struct {
	New      []AuditIssue `json:"new"`
	Resolved []AuditIssue `json:"resolved"`
}

// AuditDiff is the result of comparing two audit snapshots.
type AuditDiff struct {
	Categories    CategoryDeltas `json:"categories"`
	Issues        IssueDelta     `json:"issues"`
	HasRegression bool           `json:"hasRegression"`
}

// ElementRect mirrors snapshot.ElementRect, keeping the diff package self-contained.
type ElementRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// RectDelta captures the change in position and size.
type RectDelta struct {
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
	DW float64 `json:"dw"`
	DH float64 `json:"dh"`
}

// ElementChange describes a single element that changed between snapshots.
type ElementChange struct {
	Role     string       `json:"role"`
	Name     string       `json:"name"`
	Baseline *ElementRect `json:"baseline,omitempty"`
	Snapshot *ElementRect `json:"snapshot,omitempty"`
	Delta    *RectDelta   `json:"delta,omitempty"`
}

// ElementDiff is the result of comparing two accessibility trees.
type ElementDiff struct {
	Appeared      []ElementChange `json:"appeared"`
	Disappeared   []ElementChange `json:"disappeared"`
	Moved         []ElementChange `json:"moved"`
	Resized       []ElementChange `json:"resized"`
	HasRegression bool            `json:"hasRegression"`
}

// DiffResult is the top-level output of ks diff.
type DiffResult struct {
	SnapshotID    string      `json:"snapshotId"`
	BaselineID    string      `json:"baselineId"`
	Audit         AuditDiff   `json:"audit"`
	Elements      ElementDiff `json:"elements"`
	HasRegression bool        `json:"hasRegression"`
}
