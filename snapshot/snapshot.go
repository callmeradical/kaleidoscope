// Package snapshot defines the types used to store and retrieve audit snapshots.
// Full implementation is provided by US-001 and US-002.
package snapshot

import "fmt"

// BoundingBox holds the position and size of a DOM element.
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// AXNode represents a single node in the accessibility tree.
type AXNode struct {
	Role        string       `json:"role"`
	Name        string       `json:"name"`
	BoundingBox *BoundingBox `json:"boundingBox,omitempty"`
}

// AuditIssue represents a single issue found during an audit.
type AuditIssue struct {
	Selector string `json:"selector"`
	Message  string `json:"message"`
	Category string `json:"category"`
}

// AuditData holds all issues collected during a full UX audit.
type AuditData struct {
	ContrastIssues   []AuditIssue `json:"contrastIssues"`
	TouchIssues      []AuditIssue `json:"touchIssues"`
	TypographyIssues []AuditIssue `json:"typographyIssues"`
	SpacingIssues    []AuditIssue `json:"spacingIssues"`
}

// Snapshot is the top-level record stored per capture.
type Snapshot struct {
	ID        string    `json:"id"`
	AuditData AuditData `json:"auditData"`
	AXNodes   []AXNode  `json:"axNodes"`
}

// Load retrieves a snapshot by ID. Full implementation pending US-001.
func Load(id string) (*Snapshot, error) {
	return nil, fmt.Errorf("not implemented: snapshot.Load")
}

// LoadLatest retrieves the most recent snapshot. Full implementation pending US-001.
func LoadLatest() (*Snapshot, error) {
	return nil, fmt.Errorf("not implemented: snapshot.LoadLatest")
}

// LoadBaseline retrieves the snapshot designated as the project baseline. Full implementation pending US-002.
func LoadBaseline() (*Snapshot, error) {
	return nil, fmt.Errorf("not implemented: snapshot.LoadBaseline")
}
