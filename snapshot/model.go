package snapshot

import "time"

// AuditSummary holds aggregated counts from an accessibility/quality audit.
type AuditSummary struct {
	TotalIssues        int `json:"totalIssues"`
	ContrastViolations int `json:"contrastViolations"`
	TouchViolations    int `json:"touchViolations"`
	TypographyWarnings int `json:"typographyWarnings"`
}

// URLEntry records capture results for a single URL within a snapshot.
type URLEntry struct {
	URL         string       `json:"url"`
	Dir         string       `json:"dir"`
	AuditSummary AuditSummary `json:"auditSummary"`
	AxNodeCount int          `json:"axNodeCount"`
	Screenshots []string     `json:"screenshots"`
	Error       string       `json:"error,omitempty"`
}

// ProjectConfig is a snapshot-in-time copy of the project configuration.
type ProjectConfig struct {
	Name        string   `json:"name"`
	URLs        []string `json:"urls"`
	Breakpoints []string `json:"breakpoints"`
}

// Manifest is the root snapshot.json written to each snapshot directory.
type Manifest struct {
	ID            string        `json:"id"`
	CommitHash    string        `json:"commitHash,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
	ProjectConfig ProjectConfig `json:"projectConfig"`
	URLs          []URLEntry    `json:"urls"`
}
