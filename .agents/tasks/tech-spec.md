# Tech Spec: US-002 — Snapshot Capture and History

## Overview

This spec covers the implementation of `ks snapshot` and `ks history` commands, along with the supporting `project` and `snapshot` packages. The feature gives AI agents and developers a chronological record of interface state (screenshots, audits, accessibility trees) keyed to git commits.

---

## Architecture Overview

```
.ks-project.json          ← committed; lists URLs to capture
.kaleidoscope/
  snapshots/
    <snapshot-id>/        ← one dir per run
      snapshot.json       ← manifest
      <url-slug>/         ← one dir per URL
        mobile-375x812.png
        tablet-768x1024.png
        desktop-1280x720.png
        wide-1920x1080.png
        audit.json
        ax-tree.json
  baselines.json          ← committed; points to baseline snapshot ID
```

New Go packages:

```
project/     ← load/save .ks-project.json
snapshot/    ← manifest types, storage, baseline management
cmd/snapshot.go
cmd/history.go
```

Internal refactors (no new public API surface):

```
cmd/audit_core.go    ← pure audit logic extracted from cmd/audit.go
cmd/axtree_core.go   ← pure ax-tree logic extracted from cmd/axtree.go
```

The `snapshot` command drives the browser directly via `browser.WithPage`; it does not shell out to `ks audit` or `ks ax-tree`.

---

## Component Design

### 1. `project` Package (`project/project.go`)

Manages `.ks-project.json` in the working directory.

```go
package project

type Config struct {
    Version     int      `json:"version"`
    URLs        []string `json:"urls"`
    // Breakpoints is optional; defaults to all four standard breakpoints.
    Breakpoints []string `json:"breakpoints,omitempty"`
}

// Load reads .ks-project.json from the current working directory.
// Returns ErrNotFound (sentinel) if the file does not exist.
func Load() (*Config, error)

// Save writes cfg to .ks-project.json in the current working directory.
func Save(cfg *Config) error
```

Validation rules enforced by `Load`:
- `version` must be 1.
- `urls` must be non-empty.
- Each URL must parse without error (`url.Parse`).

### 2. `snapshot` Package (`snapshot/snapshot.go`)

Manages the on-disk snapshot store and baseline pointer.

#### Types

```go
package snapshot

type Manifest struct {
    ID        string        `json:"id"`
    Timestamp time.Time     `json:"timestamp"`
    CommitHash string       `json:"commitHash,omitempty"` // empty outside git
    Project   project.Config `json:"project"`
    URLs      []URLEntry    `json:"urls"`
}

type URLEntry struct {
    URL          string         `json:"url"`
    Slug         string         `json:"slug"`         // dir name inside snapshot dir
    Breakpoints  []BreakpointEntry `json:"breakpoints"`
    AuditSummary AuditSummary   `json:"auditSummary"`
    Error        string         `json:"error,omitempty"` // set if URL was unreachable
}

type BreakpointEntry struct {
    Name   string `json:"name"`
    Width  int    `json:"width"`
    Height int    `json:"height"`
    File   string `json:"file"` // relative path within snapshot dir
}

type AuditSummary struct {
    TotalIssues        int `json:"totalIssues"`
    ContrastViolations int `json:"contrastViolations"`
    TouchViolations    int `json:"touchViolations"`
    TypographyWarnings int `json:"typographyWarnings"`
}

type Baselines struct {
    SnapshotID string `json:"snapshotId"`
}

type ListEntry struct {
    ID         string       `json:"id"`
    Timestamp  time.Time    `json:"timestamp"`
    CommitHash string       `json:"commitHash,omitempty"`
    URLCount   int          `json:"urlCount"`
    Summary    AuditSummary `json:"summary"` // aggregate across all URLs
    IsBaseline bool         `json:"isBaseline"`
}
```

#### Functions

```go
// SnapshotRoot returns .kaleidoscope/snapshots/ (relative to CWD),
// creating it if necessary.
func SnapshotRoot() (string, error)

// NewID builds a snapshot ID from the current time and short git hash.
// Format: 20060102T150405Z-<hash> or 20060102T150405Z if not in git.
func NewID() string

// Store writes manifest and all associated files into SnapshotRoot()/<manifest.ID>/.
// The manifest is serialised as snapshot.json at the root of that directory.
func Store(manifest *Manifest) error

// List returns all snapshots in reverse chronological order.
func List() ([]ListEntry, error)

// LoadBaselines reads .kaleidoscope/baselines.json.
// Returns nil, nil if the file does not exist.
func LoadBaselines() (*Baselines, error)

// SaveBaselines writes .kaleidoscope/baselines.json.
func SaveBaselines(b *Baselines) error
```

#### Slug generation (`slugify`)

Internal helper. Converts a URL to a filesystem-safe directory name:

1. Parse the URL.
2. Combine `host` + `path`, replacing `/` with `-`.
3. Strip leading/trailing dashes.
4. Truncate to 80 characters.

Examples:
- `http://localhost:3000` → `localhost-3000`
- `http://localhost:3000/about/team` → `localhost-3000-about-team`

Collisions within a single snapshot are resolved by appending `-2`, `-3`, etc.

#### Short commit hash (`gitShortHash`)

Internal helper using `os/exec`:

```go
func gitShortHash() string {
    out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}
```

---

### 3. Internal Audit/AX-Tree Extraction

To avoid shelling out, the core logic currently in `cmd/audit.go` and `cmd/axtree.go` is extracted into unexported functions that accept `*rod.Page` and return structured data.

#### `cmd/audit_core.go`

```go
// runAuditOnPage runs the full audit against an already-navigated page.
// Returns the audit summary and nil error on success.
func runAuditOnPage(page *rod.Page) (snapshot.AuditSummary, error)
```

The existing `RunAudit` is updated to call `runAuditOnPage` and wrap the result with `output.Success`.

#### `cmd/axtree_core.go`

```go
// AXNode is a minimal accessibility node for snapshot storage.
type AXNode struct {
    NodeID   string            `json:"nodeId"`
    Role     string            `json:"role"`
    Name     string            `json:"name"`
    Children []string          `json:"children,omitempty"`
    Properties map[string]any  `json:"properties,omitempty"`
}

// runAxTreeOnPage returns the simplified accessibility tree for a page.
func runAxTreeOnPage(page *rod.Page) ([]AXNode, error)
```

The existing `RunAxTree` is updated to call `runAxTreeOnPage`.

---

### 4. `cmd/snapshot.go` — `ks snapshot`

```
ks snapshot [--full-page]
```

**Flags:**
- `--full-page` — capture full-page screenshots (passed through to the breakpoints loop).

**Algorithm:**

```
1. Load .ks-project.json → fail with clear error if not found.
2. Generate snapshot ID via snapshot.NewID().
3. Create snapshot directory: <SnapshotRoot>/<id>/.
4. Ensure browser is running (browser.WithPage probe); fail with hint if not.
5. For each URL in project.Config.URLs:
   a. Navigate: page.Navigate(url). On error, record URLEntry{Error: ...} and continue.
   b. page.MustWaitStable().
   c. For each of the 4 standard breakpoints:
      i.  SetViewport.
      ii. MustWaitStable.
      iii.Screenshot → write to <snapshot-dir>/<slug>/<name>-<W>x<H>.png.
   d. Restore viewport.
   e. Run runAuditOnPage → populate AuditSummary.
   f. Run runAxTreeOnPage → serialize to <snapshot-dir>/<slug>/ax-tree.json.
   g. Write audit summary to <snapshot-dir>/<slug>/audit.json.
   h. Append URLEntry to manifest.
6. Write snapshot.json manifest.
7. If no baselines.json exists, call snapshot.SaveBaselines({SnapshotID: id}).
8. output.Success("snapshot", result).
```

**Output (`result` payload):**

```json
{
  "id": "20260404T120000Z-abc1234",
  "snapshotDir": ".kaleidoscope/snapshots/20260404T120000Z-abc1234",
  "timestamp": "2026-04-04T12:00:00Z",
  "commitHash": "abc1234",
  "autoPromotedBaseline": true,
  "urls": [
    {
      "url": "http://localhost:3000",
      "slug": "localhost-3000",
      "auditSummary": { "totalIssues": 5, "contrastViolations": 2, "touchViolations": 1, "typographyWarnings": 2 },
      "error": ""
    }
  ]
}
```

**Error handling:**
- Missing `.ks-project.json`: `output.Fail` with hint `"Create .ks-project.json with a list of URLs. See 'ks snapshot --help'."` and exit 2.
- Browser not running: `output.Fail` with existing hint `"Is the browser running? Run: ks start"` and exit 2.
- Individual URL unreachable: record `error` field in `URLEntry`, log to stderr, continue. Final result still succeeds (OK: true) but includes the per-URL error.

---

### 5. `cmd/history.go` — `ks history`

```
ks history [--limit N]
```

**Flags:**
- `--limit N` — show only N most recent entries (default: unlimited).

**Algorithm:**
1. Call `snapshot.List()`.
2. Load `snapshot.LoadBaselines()` to mark the baseline entry.
3. Apply `--limit`.
4. `output.Success("history", result)`.

**Output:**

```json
{
  "snapshots": [
    {
      "id": "20260404T120000Z-abc1234",
      "timestamp": "2026-04-04T12:00:00Z",
      "commitHash": "abc1234",
      "urlCount": 3,
      "summary": { "totalIssues": 12, ... },
      "isBaseline": true
    }
  ]
}
```

---

### 6. `main.go` Updates

Add two new `case` entries in the `switch`:

```go
case "snapshot":
    cmd.RunSnapshot(cmdArgs)
case "history":
    cmd.RunHistory(cmdArgs)
```

Add to the `usage` string under a new **Snapshots** section:

```
Snapshots:
  snapshot [--full-page]  Capture all project URLs; auto-promote first as baseline
  history [--limit N]     List snapshots in reverse chronological order
```

---

## API Definitions

No HTTP API. All interaction is via the CLI and JSON stdout.

### `ks snapshot` JSON output

| Field | Type | Description |
|---|---|---|
| `ok` | bool | Always `true` on success |
| `command` | string | `"snapshot"` |
| `result.id` | string | Snapshot ID |
| `result.snapshotDir` | string | Relative path to snapshot directory |
| `result.timestamp` | RFC3339 string | Capture time |
| `result.commitHash` | string | Short git hash, or empty |
| `result.autoPromotedBaseline` | bool | `true` if this became the baseline |
| `result.urls[]` | array | Per-URL capture results (see above) |

### `ks history` JSON output

| Field | Type | Description |
|---|---|---|
| `ok` | bool | Always `true` |
| `command` | string | `"history"` |
| `result.snapshots[]` | array | Entries in reverse chronological order |
| `result.snapshots[].isBaseline` | bool | Whether this is the current baseline |

---

## Data Model Changes

### New files committed to repo

**`.ks-project.json`** (project root)

```json
{
  "version": 1,
  "urls": [
    "http://localhost:3000",
    "http://localhost:3000/about"
  ]
}
```

**`.kaleidoscope/baselines.json`**

```json
{
  "snapshotId": "20260404T120000Z-abc1234"
}
```

### New gitignored paths

```
.kaleidoscope/snapshots/
```

This entry must be added to `.gitignore` (or documented; the feature itself does not modify `.gitignore` automatically to avoid interfering with repos that manage ignore rules differently).

### On-disk layout

```
.kaleidoscope/
  snapshots/
    20260404T120000Z-abc1234/
      snapshot.json
      localhost-3000/
        mobile-375x812.png
        tablet-768x1024.png
        desktop-1280x720.png
        wide-1920x1080.png
        audit.json        ← AuditSummary struct as JSON
        ax-tree.json      ← []AXNode as JSON
      localhost-3000-about/
        ... (same structure)
```

**`snapshot.json`** schema (stored at snapshot root):

```json
{
  "id": "20260404T120000Z-abc1234",
  "timestamp": "2026-04-04T12:00:00Z",
  "commitHash": "abc1234",
  "project": { "version": 1, "urls": ["..."] },
  "urls": [
    {
      "url": "http://localhost:3000",
      "slug": "localhost-3000",
      "breakpoints": [
        { "name": "mobile", "width": 375, "height": 812, "file": "localhost-3000/mobile-375x812.png" }
      ],
      "auditSummary": { "totalIssues": 5, "contrastViolations": 2, "touchViolations": 1, "typographyWarnings": 2 },
      "error": ""
    }
  ]
}
```

---

## Security Considerations

- **No remote I/O.** All snapshot data is written to the local filesystem. No network calls beyond the existing Chrome browser automation.
- **URL validation.** Project URLs are parsed with `url.Parse` before any navigation attempt. Invalid URLs are rejected at config load time, not at capture time.
- **No shell injection.** The `gitShortHash` helper passes arguments as a string slice to `exec.Command`, not via a shell. No user-supplied data is interpolated into the command.
- **File path safety.** Slug generation replaces all URL components with alphanumeric characters and dashes only. The slugified value is never used in a shell context. `filepath.Join` is used for all path construction to prevent directory traversal.
- **Permissions.** All created directories use `0755`; all written files use `0644`, consistent with existing patterns in the codebase.
- **Sensitive URLs.** `.ks-project.json` may contain internal URLs. Users are responsible for ensuring this file is not committed to public repositories when those URLs are sensitive. The tool does not enforce or warn about this.

---

## Implementation Checklist

- [ ] `project/project.go` — Config type, Load, Save, validation
- [ ] `snapshot/snapshot.go` — Manifest types, NewID, slugify, gitShortHash, Store, List, LoadBaselines, SaveBaselines
- [ ] `cmd/audit_core.go` — extract `runAuditOnPage` from `cmd/audit.go`
- [ ] `cmd/axtree_core.go` — extract `runAxTreeOnPage` from `cmd/axtree.go`, define `AXNode`
- [ ] `cmd/audit.go` — update to delegate to `runAuditOnPage`
- [ ] `cmd/axtree.go` — update to delegate to `runAxTreeOnPage`
- [ ] `cmd/snapshot.go` — `RunSnapshot` implementation
- [ ] `cmd/history.go` — `RunHistory` implementation
- [ ] `main.go` — add `snapshot` and `history` cases + usage string
- [ ] `cmd/util.go` — add `--limit` to flag-value parser set
- [ ] `go test ./...` passes
