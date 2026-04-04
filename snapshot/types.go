package snapshot

import "time"

// Snapshot holds metadata for a captured snapshot.
type Snapshot struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	URLPath        string    `json:"urlPath"`
	CreatedAt      time.Time `json:"createdAt"`
	ScreenshotPath string    `json:"screenshotPath,omitempty"`
	AuditResult    any       `json:"auditResult,omitempty"`
	AXTree         any       `json:"axTree,omitempty"`
}

// Baselines maps URL path → Snapshot ID for the accepted baseline per URL.
type Baselines map[string]string
