package snapshot

import "time"

// Rect represents a 2D bounding box.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// AXElement represents an accessibility tree node captured in a snapshot.
type AXElement struct {
	Role     string `json:"role"`
	Name     string `json:"name"`
	Selector string `json:"selector,omitempty"`
	Bounds   *Rect  `json:"bounds,omitempty"`
}

// AuditSummary holds aggregate counts per audit category.
type AuditSummary struct {
	ContrastViolations int `json:"contrast_violations"`
	TouchViolations    int `json:"touch_violations"`
	TypographyWarnings int `json:"typography_warnings"`
	SpacingIssues      int `json:"spacing_issues"`
}

// BreakpointCapture holds screenshot data for one viewport.
type BreakpointCapture struct {
	Name           string `json:"name"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	ScreenshotPath string `json:"screenshot_path"`
}

// URLSnapshot holds all capture data for one URL.
type URLSnapshot struct {
	URL         string             `json:"url"`
	Breakpoints []BreakpointCapture `json:"breakpoints"`
	AuditResult AuditSummary       `json:"audit_result"`
	AXElements  []AXElement        `json:"ax_elements"`
}

// Snapshot is the top-level structure for one capture run.
type Snapshot struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	CommitSHA string        `json:"commit_sha,omitempty"`
	URLs      []URLSnapshot `json:"urls"`
}

// Store provides access to the snapshot store.
type Store interface {
	Latest() (*Snapshot, error)
	LoadByID(id string) (*Snapshot, error)
}

// BaselineManager provides access to the active baseline configuration.
type BaselineManager interface {
	ActiveBaselineID() (string, error)
}
