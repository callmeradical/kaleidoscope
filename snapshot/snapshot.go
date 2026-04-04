package snapshot

import "time"

// Snapshot represents a point-in-time capture of a project's interface state.
type Snapshot struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"createdAt"`
	CommitSHA string        `json:"commitSHA"`
	URLs      []URLSnapshot `json:"urls"`
}

// URLSnapshot holds all data captured for a single URL in one snapshot run.
type URLSnapshot struct {
	URL         string              `json:"url"`
	Breakpoints []BreakpointSnapshot `json:"breakpoints"`
	Audit       AuditResult         `json:"audit"`
	Elements    []AXElement         `json:"elements"`
}

// BreakpointSnapshot holds screenshot metadata for one viewport size.
type BreakpointSnapshot struct {
	Name           string `json:"name"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	ScreenshotPath string `json:"screenshotPath"` // relative to snapshot dir
}

// AuditResult holds aggregate counts and issue lists from one URL audit.
type AuditResult struct {
	ContrastViolations  int `json:"contrastViolations"`
	TouchViolations     int `json:"touchViolations"`
	TypographyWarnings  int `json:"typographyWarnings"`
	SpacingIssues       int `json:"spacingIssues"`
	AXTotalNodes        int `json:"axTotalNodes"`
	AXActiveNodes       int `json:"axActiveNodes"`

	ContrastIssueList  []ContrastEntry  `json:"contrastIssueList,omitempty"`
	TouchIssueList     []TouchEntry     `json:"touchIssueList,omitempty"`
	TypographyIssueList []TypoEntry     `json:"typographyIssueList,omitempty"`
	SpacingIssueList   []SpacingEntry   `json:"spacingIssueList,omitempty"`
}

// ContrastEntry is one contrast violation recorded in a snapshot.
type ContrastEntry struct {
	Selector   string  `json:"selector"`
	Text       string  `json:"text"`
	Ratio      float64 `json:"ratio"`
	Foreground string  `json:"foreground"`
	Background string  `json:"background"`
}

// TouchEntry is one touch target violation recorded in a snapshot.
type TouchEntry struct {
	Tag       string  `json:"tag"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Violation string  `json:"violation"`
}

// TypoEntry is one typography warning recorded in a snapshot.
type TypoEntry struct {
	Tag        string  `json:"tag"`
	FontSize   float64 `json:"fontSize"`
	LineHeight float64 `json:"lineHeight"`
	FontFamily string  `json:"fontFamily"`
	Warning    string  `json:"warning"`
}

// SpacingEntry is one spacing inconsistency recorded in a snapshot.
type SpacingEntry struct {
	Container string  `json:"container"`
	Index     int     `json:"index"`
	Gap       float64 `json:"gap"`
	Expected  float64 `json:"expected"`
}

// AXElement is one accessibility tree node recorded in a snapshot.
type AXElement struct {
	Role     string  `json:"role"`
	Name     string  `json:"name"`
	Selector string  `json:"selector"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}
