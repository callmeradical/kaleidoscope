package diff

import "github.com/callmeradical/kaleidoscope/snapshot"

// AuditDelta holds before/after/delta counts for each audit category.
type AuditDelta struct {
	ContrastBefore   int `json:"contrast_before"`
	ContrastAfter    int `json:"contrast_after"`
	ContrastDelta    int `json:"contrast_delta"`
	TouchBefore      int `json:"touch_before"`
	TouchAfter       int `json:"touch_after"`
	TouchDelta       int `json:"touch_delta"`
	TypographyBefore int `json:"typography_before"`
	TypographyAfter  int `json:"typography_after"`
	TypographyDelta  int `json:"typography_delta"`
	SpacingBefore    int `json:"spacing_before"`
	SpacingAfter     int `json:"spacing_after"`
	SpacingDelta     int `json:"spacing_delta"`
}

// ElementChange describes a single DOM element change between snapshots.
type ElementChange struct {
	Role     string `json:"role"`
	Name     string `json:"name"`
	Selector string `json:"selector"`
	// Type is one of: "appeared", "disappeared", "moved", "resized"
	Type    string `json:"type"`
	Details string `json:"details"`
}

// PixelDiff holds pixel-level diff statistics for one screenshot pair.
type PixelDiff struct {
	DiffPath      string  `json:"diff_path"`
	DiffPercent   float64 `json:"diff_percent"`
	ChangedPixels int     `json:"changed_pixels"`
	TotalPixels   int     `json:"total_pixels"`
}

// URLDiff holds all diff data for a single URL.
type URLDiff struct {
	URL            string          `json:"url"`
	AuditDelta     AuditDelta      `json:"audit_delta"`
	ElementChanges []ElementChange `json:"element_changes"`
	PixelDiff      *PixelDiff      `json:"pixel_diff,omitempty"`
}

// DiffResult is the top-level result of comparing two snapshots.
type DiffResult struct {
	URLs []URLDiff `json:"urls"`
}

// Compare computes the diff between a baseline and current snapshot.
// Stub implementation: returns an empty DiffResult with no error.
// The real implementation is delivered in US-004.
func Compare(baseline, current *snapshot.Snapshot) (*DiffResult, error) {
	return &DiffResult{}, nil
}
