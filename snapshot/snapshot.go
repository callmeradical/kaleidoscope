package snapshot

import "time"

// Snapshot is the top-level record captured during a ks snapshot run.
type Snapshot struct {
	ID          string        `json:"id"`
	CreatedAt   time.Time     `json:"createdAt"`
	CommitSHA   string        `json:"commitSHA"`
	CommitMsg   string        `json:"commitMsg"`
	ProjectFile string        `json:"projectFile"`
	Pages       []PageSnapshot `json:"pages"`
}

// PageSnapshot holds all captures for a single URL.
type PageSnapshot struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Breakpoints []BreakpointCapture `json:"breakpoints"`
	AuditResult AuditResult       `json:"auditResult"`
	AXNodes     []AXNodeRecord    `json:"axNodes"`
}

// BreakpointCapture records a screenshot at a specific viewport width.
type BreakpointCapture struct {
	Name           string `json:"name"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	ScreenshotFile string `json:"screenshotFile"`
}

// AuditResult holds the aggregated audit counts and issue lists for a page.
type AuditResult struct {
	ContrastViolations int `json:"contrastViolations"`
	TouchViolations    int `json:"touchViolations"`
	TypographyWarnings int `json:"typographyWarnings"`
	SpacingIssues      int `json:"spacingIssues"`

	ContrastIssues   []ContrastRecord `json:"contrastIssues"`
	TouchIssues      []TouchRecord    `json:"touchIssues"`
	TypographyIssues []TypoRecord     `json:"typographyIssues"`
	SpacingIssueList []SpacingRecord  `json:"spacingIssueList"`
}

// ContrastRecord mirrors a single contrast violation.
type ContrastRecord struct {
	Selector   string  `json:"selector"`
	Text       string  `json:"text"`
	Ratio      float64 `json:"ratio"`
	Foreground string  `json:"foreground"`
	Background string  `json:"background"`
}

// TouchRecord mirrors a single touch target violation.
type TouchRecord struct {
	Tag       string  `json:"tag"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Violation string  `json:"violation"`
}

// TypoRecord mirrors a single typography warning.
type TypoRecord struct {
	Tag        string  `json:"tag"`
	FontSize   float64 `json:"fontSize"`
	LineHeight float64 `json:"lineHeight"`
	Warning    string  `json:"warning"`
}

// SpacingRecord mirrors a single spacing inconsistency.
type SpacingRecord struct {
	Container string  `json:"container"`
	Index     int     `json:"index"`
	Gap       float64 `json:"gap"`
	Expected  float64 `json:"expected"`
}

// AXNodeRecord represents one node in the accessibility tree.
type AXNodeRecord struct {
	Role     string `json:"role"`
	Name     string `json:"name"`
	Selector string `json:"selector"`
	Bounds   Rect   `json:"bounds"`
	Ignored  bool   `json:"ignored"`
}

// Rect describes an element's bounding box.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
