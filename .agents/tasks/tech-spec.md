# Tech Spec: Snapshot Capture and History (US-002)

## Overview

This spec covers `ks snapshot` and `ks history` — two new CLI commands that enable chronological tracking of interface state across commits. A snapshot captures screenshots at 4 breakpoints, an accessibility tree dump, and a full audit result for every URL defined in `.ks-project.json`. On first run, the snapshot is auto-promoted as the baseline in `.kaleidoscope/baselines.json`.

**Depends on:** US-001 (project config — `.ks-project.json` must already exist)

---

## Architecture Overview

```
main.go                          # add "snapshot" and "history" cases
cmd/
  snapshot.go                    # RunSnapshot — iterates URLs, captures, writes snapshot
  history.go                     # RunHistory — lists snapshots in reverse-chron order
  project.go                     # ReadProjectConfig / ProjectConfig struct (if not from US-001)
snapshot/
  snapshot.go                    # data types: Manifest, URLEntry, BaselineManifest
  store.go                       # disk I/O: create dirs, write/read manifests, list snapshots
  git.go                         # short commit hash helper (graceful fallback)
  urlkey.go                      # sanitize URL → safe directory name
```

No new external dependencies. All I/O uses `os`, `encoding/json`, `path/filepath`. Screenshot capture reuses `breakpoints` logic directly (via internal function calls, not shelling out).

---

## Component Design

### 1. Project Config (`cmd/project.go`)

> If US-001 already defines this struct and file, reuse it directly.

**File:** `.ks-project.json` (committed to repo, in CWD)

```go
type ProjectConfig struct {
    Version     int      `json:"version"`
    URLs        []string `json:"urls"`
}
```

**Function:**
```go
// ReadProjectConfig reads .ks-project.json from CWD.
// Returns error with a clear message if the file is missing or malformed.
func ReadProjectConfig() (*ProjectConfig, error)
```

---

### 2. Snapshot Data Types (`snapshot/snapshot.go`)

```go
// SnapshotID format: "20060102T150405Z-abc1234" or "20060102T150405Z" (no git).
type SnapshotID = string

// Manifest is the root snapshot.json written at the snapshot root.
type Manifest struct {
    ID          SnapshotID    `json:"id"`
    Timestamp   time.Time     `json:"timestamp"`
    CommitHash  string        `json:"commitHash,omitempty"` // short hash; empty if not a git repo
    ProjectConfig interface{} `json:"projectConfig"`        // verbatim copy of .ks-project.json at capture time
    URLs        []URLEntry    `json:"urls"`
    Summary     Summary       `json:"summary"`
}

// URLEntry is the per-URL section of snapshot.json (not the full audit/ax-tree data,
// just enough for history display and quick summary).
type URLEntry struct {
    URL             string `json:"url"`
    Dir             string `json:"dir"`           // relative path inside snapshot dir
    TotalIssues     int    `json:"totalIssues"`
    AXNodeCount     int    `json:"axNodeCount"`
    Breakpoints     int    `json:"breakpoints"`   // always 4
    CapturedAt      time.Time `json:"capturedAt"`
    Reachable       bool   `json:"reachable"`
    Error           string `json:"error,omitempty"` // if Reachable=false
}

// Summary is the aggregate across all URLs, for `ks history` display.
type Summary struct {
    TotalURLs        int `json:"totalURLs"`
    ReachableURLs    int `json:"reachableURLs"`
    TotalIssues      int `json:"totalIssues"`
    TotalAXNodes     int `json:"totalAXNodes"`
}

// BaselineManifest is .kaleidoscope/baselines.json (committed to repo).
type BaselineManifest struct {
    BaselineID  SnapshotID `json:"baselineId"`
    SetAt       time.Time  `json:"setAt"`
    CommitHash  string     `json:"commitHash,omitempty"`
}
```

---

### 3. Snapshot Store (`snapshot/store.go`)

```go
// SnapshotsDir returns .kaleidoscope/snapshots/ in CWD, creating it if needed.
func SnapshotsDir() (string, error)

// SnapshotPath returns the absolute path for a given snapshot ID.
func SnapshotPath(id SnapshotID) (string, error)

// URLDir returns and creates the per-URL subdirectory inside a snapshot.
// e.g. "https://example.com/about" → "<snapshot-dir>/example.com-about"
func URLDir(snapshotPath, rawURL string) (string, error)

// WriteManifest marshals and writes snapshot.json into the snapshot root dir.
func WriteManifest(snapshotPath string, m *Manifest) error

// ReadManifest reads snapshot.json from a snapshot directory.
func ReadManifest(snapshotPath string) (*Manifest, error)

// ListSnapshotIDs returns all snapshot IDs found in .kaleidoscope/snapshots/,
// sorted newest-first (lexicographic on the ISO timestamp prefix).
func ListSnapshotIDs() ([]SnapshotID, error)

// ReadBaselineManifest reads .kaleidoscope/baselines.json.
// Returns nil, nil if the file does not exist.
func ReadBaselineManifest() (*BaselineManifest, error)

// WriteBaselineManifest writes .kaleidoscope/baselines.json.
func WriteBaselineManifest(b *BaselineManifest) error
```

---

### 4. URL Key Sanitization (`snapshot/urlkey.go`)

Converts a raw URL to a safe directory name used inside the snapshot:

```go
// URLToKey sanitizes a URL into a filesystem-safe directory name.
// "https://example.com/about?foo=bar" → "example.com-about"
// Rules:
//   - strip scheme
//   - replace "/" with "-"
//   - strip query string and fragment
//   - collapse consecutive "-" → single "-"
//   - strip leading/trailing "-"
//   - truncate to 128 chars
func URLToKey(rawURL string) string
```

---

### 5. Git Hash Helper (`snapshot/git.go`)

```go
// ShortCommitHash returns the 7-char short hash of HEAD.
// Returns "" (no error) if CWD is not a git repo or git is unavailable.
func ShortCommitHash() string
```

Implementation: run `git rev-parse --short HEAD` via `os/exec`, capture stdout, trim whitespace. On any error (not a repo, no git binary), return `""`.

---

### 6. `ks snapshot` Command (`cmd/snapshot.go`)

```go
func RunSnapshot(args []string) {
    // 1. Load project config (.ks-project.json in CWD)
    // 2. Build snapshot ID (timestamp + short commit hash)
    // 3. Create snapshot root dir: .kaleidoscope/snapshots/<id>/
    // 4. For each URL in config.URLs:
    //    a. Create URL subdirectory via snapshot.URLDir()
    //    b. Navigate browser to URL (reuse browser.WithPage + page.Navigate)
    //    c. Capture 4 breakpoints — call internal captureBreakpoints() that
    //       saves PNGs to the URL dir and returns []BreakpointResult
    //    d. Run audit — call internal runAudit() returning the audit summary struct
    //    e. Run ax-tree — call internal runAxTree() returning nodes slice
    //    f. Write audit.json and ax-tree.json into URL dir
    //    g. Record URLEntry (reachable=true)
    //    If any step fails (navigate, screenshot), record URLEntry with
    //    reachable=false and error message; continue to next URL
    // 5. Assemble Manifest, write snapshot.json
    // 6. If .kaleidoscope/baselines.json does not exist, auto-promote:
    //    write BaselineManifest pointing to this snapshot ID
    // 7. output.Success("snapshot", result)
}
```

**Internal helpers (unexported, in snapshot.go or a snapshot_helpers.go):**

```go
// captureBreakpoints navigates to url on page, captures 4 PNGs into destDir,
// returns list of {breakpoint, width, height, path}.
func captureBreakpoints(page *rod.Page, destDir string) ([]map[string]any, error)

// runAuditOnPage runs the same audit logic as cmd/audit.go but returns
// the result as a map[string]any (not printing it).
func runAuditOnPage(page *rod.Page) (map[string]any, error)

// runAxTreeOnPage runs the ax-tree dump and returns the nodes slice.
func runAxTreeOnPage(page *rod.Page) ([]map[string]any, error)
```

These helpers share logic with `RunAudit` / `RunAxTree` / `RunBreakpoints`. To avoid code duplication, extract reusable core from each existing command into an internal helper in the same package that both the original command and the snapshot command can call. Specifically:

- Move the breakpoint capture loop from `RunBreakpoints` into `captureBreakpointsToDir(page, dir string) ([]BreakpointResult, error)` in `cmd/breakpoints.go`, and have `RunBreakpoints` call it.
- Move the audit data-gathering from `RunAudit` into `gatherAuditData(page, selector string) (map[string]any, error)` in `cmd/audit.go`, and have `RunAudit` call it.
- Move the ax-tree data-gathering from `RunAxTree` into `gatherAxTree(page) (map[string]any, error)` in `cmd/axtree.go`, and have `RunAxTree` call it.

**Output on success:**
```json
{
  "ok": true,
  "command": "snapshot",
  "result": {
    "id": "20260404T120000Z-abc1234",
    "path": ".kaleidoscope/snapshots/20260404T120000Z-abc1234",
    "urls": 3,
    "reachable": 3,
    "totalIssues": 12,
    "promotedToBaseline": true
  }
}
```

**Failure modes:**
- Missing `.ks-project.json`: `output.Fail` with hint `"Run: echo '{\"version\":1,\"urls\":[\"https://...\"]' > .ks-project.json"`
- No URLs in config: `output.Fail` with hint `"Add at least one URL to .ks-project.json"`
- Browser not running: `output.Fail` with hint `"Is the browser running? Run: ks start"`
- Individual URL unreachable: log to URLEntry.Error, continue; do NOT abort entire snapshot

**Exit codes:** 0 on success (even if some URLs were unreachable), 2 on fatal errors.

---

### 7. `ks history` Command (`cmd/history.go`)

```go
func RunHistory(args []string) {
    // 1. List snapshot IDs from .kaleidoscope/snapshots/ (newest first)
    // 2. Read manifest for each snapshot
    // 3. Read baselines.json to mark which snapshot is the current baseline
    // 4. output.Success("history", result)
}
```

**Output on success:**
```json
{
  "ok": true,
  "command": "history",
  "result": {
    "snapshots": [
      {
        "id": "20260404T120000Z-abc1234",
        "timestamp": "2026-04-04T12:00:00Z",
        "commitHash": "abc1234",
        "isBaseline": true,
        "urls": 3,
        "reachableUrls": 3,
        "totalIssues": 12,
        "totalAXNodes": 148
      }
    ],
    "baselineId": "20260404T120000Z-abc1234"
  }
}
```

If `.kaleidoscope/snapshots/` does not exist or is empty, return `{"snapshots": [], "baselineId": ""}` — do NOT error.

---

## API Definitions

### `ks snapshot [--local]`

| Flag | Description |
|------|-------------|
| `--local` | Use project-local `.kaleidoscope/` (already supported by `browser.StateDir`) |

No positional arguments. URLs come from `.ks-project.json`.

### `ks history`

No flags or arguments. Always reads from project-local `.kaleidoscope/snapshots/`.

---

## Data Model

### Directory Layout

```
<project-root>/
├── .ks-project.json                        # committed; defines URLs to snapshot
└── .kaleidoscope/
    ├── baselines.json                      # committed; points to baseline snapshot ID
    └── snapshots/                          # gitignored
        └── 20260404T120000Z-abc1234/
            ├── snapshot.json               # manifest
            ├── example.com/               # sanitized from "https://example.com"
            │   ├── mobile.png             # 375×812
            │   ├── tablet.png             # 768×1024
            │   ├── desktop.png            # 1280×720
            │   ├── wide.png               # 1920×1080
            │   ├── audit.json             # full audit result map
            │   └── ax-tree.json           # full nodes slice
            └── example.com-about/         # sanitized from "https://example.com/about"
                ├── mobile.png
                ├── tablet.png
                ├── desktop.png
                ├── wide.png
                ├── audit.json
                └── ax-tree.json
```

### `.ks-project.json` (minimal, defined by US-001)

```json
{
  "version": 1,
  "urls": [
    "https://example.com",
    "https://example.com/about"
  ]
}
```

### `snapshot.json` (root manifest)

```json
{
  "id": "20260404T120000Z-abc1234",
  "timestamp": "2026-04-04T12:00:00Z",
  "commitHash": "abc1234",
  "projectConfig": { "version": 1, "urls": ["https://example.com"] },
  "urls": [
    {
      "url": "https://example.com",
      "dir": "example.com",
      "totalIssues": 5,
      "axNodeCount": 72,
      "breakpoints": 4,
      "capturedAt": "2026-04-04T12:00:01Z",
      "reachable": true
    }
  ],
  "summary": {
    "totalURLs": 1,
    "reachableURLs": 1,
    "totalIssues": 5,
    "totalAXNodes": 72
  }
}
```

### `.kaleidoscope/baselines.json`

```json
{
  "baselineId": "20260404T120000Z-abc1234",
  "setAt": "2026-04-04T12:00:05Z",
  "commitHash": "abc1234"
}
```

### `audit.json` (per-URL, verbatim audit result)

The full map returned by `gatherAuditData()` — same shape as current `RunAudit` output's `result` field:
```json
{
  "selector": "",
  "summary": { "totalIssues": 5, "contrastViolations": 2, "touchViolations": 1, "typographyWarnings": 2 },
  "accessibility": { "totalNodes": 80, "ignoredNodes": 8, "activeNodes": 72 },
  "contrast": { "violations": 2 },
  "touchTargets": { "total": 10, "violations": 1 },
  "typography": { "warnings": 2 }
}
```

### `ax-tree.json` (per-URL, verbatim ax-tree result)

The nodes slice from `gatherAxTree()` — same shape as current `RunAxTree` output's `result` field:
```json
{
  "nodeCount": 72,
  "nodes": [...]
}
```

---

## `main.go` Changes

Add two new cases to the switch statement:

```go
case "snapshot":
    cmd.RunSnapshot(cmdArgs)
case "history":
    cmd.RunHistory(cmdArgs)
```

Add entries to the usage string:

```
Snapshot & History:
  snapshot               Capture full interface state for all project URLs
  history                List snapshots in reverse chronological order
```

---

## `.gitignore` Changes

Add to project `.gitignore` (or document as a required addition):

```
.kaleidoscope/snapshots/
.kaleidoscope/state.json
.kaleidoscope/screenshots/
```

`.kaleidoscope/baselines.json` is intentionally **not** gitignored.

---

## Security Considerations

1. **No remote I/O** — all snapshot data is local disk only. No network calls beyond navigating the user-specified URLs.
2. **URL validation** — `ks snapshot` should reject URLs that use non-http(s) schemes (file://, javascript:, etc.) to prevent unexpected browser behavior. Return a clear error per URL.
3. **Directory traversal** — `URLToKey()` must strip `..` components and any leading `/` after sanitization. The resulting key is always a flat name with no path separators.
4. **File permissions** — snapshot directories and files are written with mode `0755` (dirs) and `0644` (files), matching the existing codebase convention.
5. **`exec.Command` for git** — the only shell-out is `git rev-parse --short HEAD`. This is a fixed command with no user-controlled arguments. No injection risk.
6. **Audit/ax-tree run in-process** — no shelling out to `ks audit` or `ks ax-tree`; internal helpers are called directly, eliminating any command-injection surface.

---

## Refactoring Required in Existing Commands

To enable code reuse without duplication, three existing commands need small internal restructuring (the public `Run*` functions remain unchanged in behavior):

### `cmd/breakpoints.go`
Extract the core capture loop:
```go
// captureBreakpointsToDir saves 4 breakpoint PNGs to destDir and returns metadata.
// Called by both RunBreakpoints and RunSnapshot.
func captureBreakpointsToDir(page *rod.Page, destDir string) ([]map[string]any, error)
```
`RunBreakpoints` becomes a thin wrapper that calls this and then calls `browser.ScreenshotDir()` for its default destDir.

### `cmd/audit.go`
Extract the data-gathering portion:
```go
// gatherAuditData runs all audit checks on the current page and returns the result map.
// Does NOT call output.Success or os.Exit.
func gatherAuditData(page *rod.Page, selector string) (map[string]any, error)
```

### `cmd/axtree.go`
Extract the data-gathering portion:
```go
// gatherAxTreeData dumps the full ax-tree from the current page and returns the result map.
// Does NOT call output.Success or os.Exit.
func gatherAxTreeData(page *rod.Page) (map[string]any, error)
```

---

## Testing

Quality gate: `go test ./...`

New test files:

| File | Tests |
|------|-------|
| `snapshot/urlkey_test.go` | URLToKey edge cases: root, deep path, query string, `..` traversal, special chars, long URL truncation |
| `snapshot/git_test.go` | ShortCommitHash returns non-empty string inside a git repo (if test env has git), returns `""` in a temp non-git dir |
| `snapshot/store_test.go` | SnapshotPath, WriteManifest/ReadManifest round-trip, ListSnapshotIDs order, baseline read/write round-trip |

No integration tests (require a running browser) — consistent with the existing test surface.

---

## Implementation Order

1. `snapshot/urlkey.go` + test — no dependencies
2. `snapshot/git.go` + test — no dependencies
3. `snapshot/snapshot.go` — data types only
4. `snapshot/store.go` + test — depends on types
5. Refactor `cmd/breakpoints.go`, `cmd/audit.go`, `cmd/axtree.go` — extract internal helpers
6. `cmd/project.go` — ProjectConfig (coordinate with US-001 owner to avoid duplication)
7. `cmd/snapshot.go` — depends on all above
8. `cmd/history.go` — depends on store
9. `main.go` — wire up new commands
10. `go test ./...`
