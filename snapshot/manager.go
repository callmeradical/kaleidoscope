package snapshot

import "time"

// ID is the string identifier for a snapshot (e.g. "1712345678-abc1234").
type ID = string

// Manifest is the root snapshot.json written inside each snapshot directory.
type Manifest struct {
	ID           string       `json:"id"`
	Timestamp    time.Time    `json:"timestamp"`
	CommitHash   string       `json:"commitHash,omitempty"`
	ProjectURLs  []string     `json:"projectURLs"`
	ProjectName  string       `json:"projectName,omitempty"`
	BaseURL      string       `json:"baseURL,omitempty"`
	URLSummaries []URLSummary `json:"urlSummaries"`
}

// URLSummary holds per-URL audit summary data stored in the manifest.
type URLSummary struct {
	URL                 string `json:"url"`
	Slug                string `json:"slug"`
	ContrastViolations  int    `json:"contrastViolations"`
	TouchViolations     int    `json:"touchViolations"`
	TypographyWarnings  int    `json:"typographyWarnings"`
	AXActiveNodes       int    `json:"axActiveNodes"`
	AXTotalNodes        int    `json:"axTotalNodes"`
}

// AuditResult holds the per-URL audit output returned by the internal audit helper.
type AuditResult struct {
	URL                 string
	ContrastViolations  int
	TouchViolations     int
	TypographyWarnings  int
	AXActiveNodes       int
	AXTotalNodes        int
	TotalIssues         int
}

// AXNode represents a single node from the accessibility tree.
type AXNode struct {
	NodeID     string            `json:"nodeId"`
	Role       string            `json:"role"`
	Name       string            `json:"name"`
	Children   []string          `json:"children,omitempty"`
	Properties map[string]any    `json:"properties,omitempty"`
}

// BaselinesFile is the structure of .kaleidoscope/baselines.json.
type BaselinesFile struct {
	DefaultBaseline string            `json:"defaultBaseline"`
	URLBaselines    map[string]string `json:"urlBaselines,omitempty"`
}

// SnapshotsDir returns the path to the snapshots directory, creating it if needed.
func SnapshotsDir() (string, error) {
	panic("not implemented")
}

// SnapshotPath returns the path to a specific snapshot directory, creating it if needed.
func SnapshotPath(id ID) (string, error) {
	panic("not implemented")
}

// URLSlug converts a raw URL into a filesystem-safe slug using host+path.
// Characters outside [a-zA-Z0-9._-] are replaced with underscores.
// The result is capped at 200 characters.
func URLSlug(rawURL string) string {
	panic("not implemented")
}

// URLDir returns the per-URL subdirectory inside a snapshot, creating it if needed.
func URLDir(snapshotID ID, slug string) (string, error) {
	panic("not implemented")
}

// NewID generates a snapshot ID.
// Format: "<unix_seconds>-<hash>" when hash is non-empty, "<unix_seconds>" otherwise.
func NewID(hash string) ID {
	panic("not implemented")
}

// WriteManifest serialises m to snapshot.json inside the snapshot directory for id.
func WriteManifest(id ID, m *Manifest) error {
	panic("not implemented")
}

// ReadManifest reads and deserialises snapshot.json from the snapshot directory for id.
func ReadManifest(id ID) (*Manifest, error) {
	panic("not implemented")
}

// ListIDs returns snapshot IDs in descending order (newest first).
func ListIDs() ([]ID, error) {
	panic("not implemented")
}

// ReadBaselines reads .kaleidoscope/baselines.json.
// Returns nil, nil when the file does not exist.
func ReadBaselines() (*BaselinesFile, error) {
	panic("not implemented")
}

// WriteBaselines writes b to .kaleidoscope/baselines.json.
func WriteBaselines(b *BaselinesFile) error {
	panic("not implemented")
}
