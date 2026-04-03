package snapshot

import "time"

// Viewport represents the browser viewport dimensions.
type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// BoundingBox represents an element's position and size.
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// AuditIssue represents a single accessibility or UX issue.
type AuditIssue struct {
	Selector string `json:"selector"`
	Message  string `json:"message"`
}

// AuditData groups issues by category.
type AuditData struct {
	Contrast   []AuditIssue `json:"contrast"`
	Touch      []AuditIssue `json:"touch"`
	Typography []AuditIssue `json:"typography"`
	Spacing    []AuditIssue `json:"spacing"`
}

// Element represents an accessible element in the page.
type Element struct {
	Role string      `json:"role"`
	Name string      `json:"name"`
	Box  BoundingBox `json:"box"`
}

// Snapshot is a point-in-time capture of a page's state.
type Snapshot struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"createdAt"`
	URL       string     `json:"url"`
	Viewport  Viewport   `json:"viewport"`
	Audit     AuditData  `json:"audit"`
	Elements  []Element  `json:"elements"`
}
