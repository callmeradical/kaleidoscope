package snapshot

import "time"

// ProjectConfig holds the project definition loaded from .ks-project.json.
type ProjectConfig struct {
	Name string   `json:"name"`
	URLs []string `json:"urls"`
}

// AuditSummary holds aggregate counts from an audit run on a single URL.
type AuditSummary struct {
	TotalIssues        int `json:"totalIssues"`
	ContrastViolations int `json:"contrastViolations"`
	TouchViolations    int `json:"touchViolations"`
	TypographyWarnings int `json:"typographyWarnings"`
}

// URLEntry records the capture results for a single URL within a snapshot.
type URLEntry struct {
	URL          string       `json:"url"`
	Dir          string       `json:"dir"`
	Breakpoints  []string     `json:"breakpoints"`
	AuditSummary AuditSummary `json:"auditSummary"`
	AxNodeCount  int          `json:"axNodeCount"`
	Error        string       `json:"error,omitempty"`
}

// Manifest is the root snapshot.json written for every snapshot.
type Manifest struct {
	ID            string        `json:"id"`
	Timestamp     time.Time     `json:"timestamp"`
	CommitHash    string        `json:"commitHash,omitempty"`
	ProjectConfig ProjectConfig `json:"projectConfig"`
	URLs          []URLEntry    `json:"urls"`
}

// Baseline points to the snapshot that serves as the regression baseline.
type Baseline struct {
	SnapshotID string    `json:"snapshotId"`
	SetAt      time.Time `json:"setAt"`
}
