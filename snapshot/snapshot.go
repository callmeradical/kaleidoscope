package snapshot

import "time"

// SnapshotID is the unique identifier for a snapshot, formatted as
// "<timestamp>-<short-commit-hash>" or "<timestamp>" outside git.
type SnapshotID = string

// Manifest is the root snapshot.json written at the root of each snapshot directory.
type Manifest struct {
	ID            SnapshotID  `json:"id"`
	Timestamp     time.Time   `json:"timestamp"`
	CommitHash    string      `json:"commitHash,omitempty"`
	ProjectConfig interface{} `json:"projectConfig"`
	URLs          []URLEntry  `json:"urls"`
	Summary       Summary     `json:"summary"`
}

// URLEntry records capture results for a single URL within a snapshot.
type URLEntry struct {
	URL         string    `json:"url"`
	Dir         string    `json:"dir"`
	TotalIssues int       `json:"totalIssues"`
	AXNodeCount int       `json:"axNodeCount"`
	Breakpoints int       `json:"breakpoints"`
	CapturedAt  time.Time `json:"capturedAt"`
	Reachable   bool      `json:"reachable"`
	Error       string    `json:"error,omitempty"`
}

// Summary aggregates statistics across all URLs in a snapshot.
type Summary struct {
	TotalURLs     int `json:"totalURLs"`
	ReachableURLs int `json:"reachableURLs"`
	TotalIssues   int `json:"totalIssues"`
	TotalAXNodes  int `json:"totalAXNodes"`
}

// BaselineManifest is written to .kaleidoscope/baselines.json to record which
// snapshot serves as the current comparison baseline.
type BaselineManifest struct {
	BaselineID SnapshotID `json:"baselineId"`
	SetAt      time.Time  `json:"setAt"`
	CommitHash string     `json:"commitHash,omitempty"`
}
