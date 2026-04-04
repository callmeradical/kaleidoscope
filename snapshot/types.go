package snapshot

import "time"

// Snapshot captures a point-in-time state of a page including audit results,
// accessibility tree nodes, and screenshot data.
type Snapshot struct {
	ID         string       `json:"id"`
	CreatedAt  time.Time    `json:"created_at"`
	CommitSHA  string       `json:"commit_sha"`
	URL        string       `json:"url"`
	Audit      AuditSummary `json:"audit"`
	AXNodes    []AXNode     `json:"ax_nodes"`
	Screenshot string       `json:"screenshot"` // base64-encoded PNG or file path
}

// AuditSummary holds the structured audit results for a snapshot.
type AuditSummary struct {
	TotalIssues        int          `json:"total_issues"`
	ContrastViolations []AuditIssue `json:"contrast_violations"`
	TouchViolations    []AuditIssue `json:"touch_violations"`
	TypographyWarnings []AuditIssue `json:"typography_warnings"`
}

// AuditIssue represents a single audit finding identified by selector and detail.
type AuditIssue struct {
	Selector string `json:"selector"`
	Detail   string `json:"detail"`
}

// AXNode represents a node in the accessibility tree with its semantic identity
// and bounding box.
type AXNode struct {
	Role   string  `json:"role"`
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
