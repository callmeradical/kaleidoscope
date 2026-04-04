package snapshot

import "time"

// AuditSummary holds aggregate counts from an audit run.
type AuditSummary struct {
	ContrastViolations int `json:"contrastViolations"`
	TouchViolations    int `json:"touchViolations"`
	TypographyWarnings int `json:"typographyWarnings"`
	TotalIssues        int `json:"totalIssues"`
}

// AuditIssueRecord represents a single audit issue keyed by selector.
type AuditIssueRecord struct {
	Category string `json:"category"` // "contrast" | "touch" | "typography"
	Selector string `json:"selector"`
}

// AuditSnapshot holds the full audit data for a snapshot.
type AuditSnapshot struct {
	Summary AuditSummary       `json:"summary"`
	Issues  []AuditIssueRecord `json:"issues"`
}

// ElementRect represents the bounding box of an element.
type ElementRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ElementRecord represents an accessible element captured in a snapshot.
type ElementRecord struct {
	Role string       `json:"role"`
	Name string       `json:"name"`
	Rect *ElementRect `json:"rect,omitempty"`
}

// Snapshot is the top-level record stored per capture.
type Snapshot struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"createdAt"`
	URL       string        `json:"url"`
	AuditData AuditSnapshot `json:"audit"`
	Elements  []ElementRecord `json:"elements"`
}
