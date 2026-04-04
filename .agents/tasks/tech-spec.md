# Technical Specification: Snapshot Capture and History (US-002)

## Overview

This spec covers the implementation of `ks snapshot` and `ks history` commands, along with supporting packages for snapshot storage, project config loading, and git integration. This feature depends on US-001 (project config `.ks-project.json`), whose data structures are defined below to the extent needed.

---

## Architecture Overview

```
ks snapshot                        ks history
     │                                  │
     ▼                                  ▼
cmd/snapshot.go                  cmd/history.go
     │                                  │
     ├─ snapshot/manager.go ────────────┘
     │       (persist, list, load)
     │
     ├─ project/config.go
     │       (load .ks-project.json)
     │
     ├─ gitutil/gitutil.go
     │       (short commit hash)
     │
     ├─ browser.WithPage (existing)
     │       - screenshot at 4 breakpoints
     │       - RunAuditInternal (extracted)
     │       - RunAxTreeInternal (extracted)
     │
     └─ output.Success / output.Fail (existing)

File Layout:
.ks-project.json                 ← committed; read by snapshot/history
.kaleidoscope/
  snapshots/
    <id>/                        ← one dir per snapshot
      snapshot.json              ← manifest
      <url-slug>/
        mobile-375x812.png
        tablet-768x1024.png
        desktop-1280x720.png
        wide-1920x1080.png
        audit.json
        ax-tree.json
  baselines.json                 ← committed; written on first snapshot
```

---

## Component Design

### 1. `project` Package — `project/config.go`

Loads and validates `.ks-project.json` from the current working directory.

```go
package project

import (
    "encoding/json"
    "fmt"
    "os"
)

// Config is the .ks-project.json structure.
type Config struct {
    Name    string   `json:"name"`
    BaseURL string   `json:"baseURL"`
    URLs    []string `json:"urls"`
}

// Load reads .ks-project.json from CWD.
func Load() (*Config, error) {
    data, err := os.ReadFile(".ks-project.json")
    if err != nil {
        return nil, fmt.Errorf("cannot read .ks-project.json: %w (run 'ks init' first)", err)
    }
    var c Config
    if err := json.Unmarshal(data, &c); err != nil {
        return nil, fmt.Errorf("invalid .ks-project.json: %w", err)
    }
    if len(c.URLs) == 0 {
        return nil, fmt.Errorf(".ks-project.json has no urls defined")
    }
    return &c, nil
}
```

**Notes:**
- `BaseURL` is optional; if provided, relative URLs in `urls` are resolved against it.
- This package has zero external dependencies.

---

### 2. `gitutil` Package — `gitutil/gitutil.go`

Retrieves the current short commit hash without shelling out to `git` binary (uses `os/exec` as a thin wrapper — acceptable since Chrome is already a subprocess dependency).

```go
package gitutil

import (
    "os/exec"
    "strings"
)

// ShortHash returns the 7-character git commit hash of HEAD,
// or "" if not in a git repo or git is unavailable.
func ShortHash() string {
    out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}
```

---

### 3. `snapshot` Package — `snapshot/manager.go`

Pure-Go package with no Chrome dependency. Handles all persistence logic.

#### 3a. Data Structures

```go
package snapshot

import "time"

// ID format: "<unix-seconds>-<short-hash>" or "<unix-seconds>" outside git.
type ID = string

// Manifest is written as snapshot.json at the root of each snapshot dir.
type Manifest struct {
    ID          string    `json:"id"`
    Timestamp   time.Time `json:"timestamp"`
    CommitHash  string    `json:"commitHash,omitempty"` // empty outside git
    ProjectURLs []string  `json:"projectURLs"`          // snapshot of config.URLs at capture time
    ProjectName string    `json:"projectName,omitempty"`
    BaseURL     string    `json:"baseURL,omitempty"`
    URLSummaries []URLSummary `json:"urlSummaries"`
}

// URLSummary is the per-URL issue counts stored in the manifest for
// quick listing by `ks history`.
type URLSummary struct {
    URL                string `json:"url"`
    Slug               string `json:"slug"`
    ContrastViolations int    `json:"contrastViolations"`
    TouchViolations    int    `json:"touchViolations"`
    TypographyWarnings int    `json:"typographyWarnings"`
    AXActiveNodes      int    `json:"axActiveNodes"`
    AXTotalNodes       int    `json:"axTotalNodes"`
}

// AuditResult is the structure written to <url-slug>/audit.json.
// Mirrors the output of RunAudit but with full detail.
type AuditResult struct {
    URL                string `json:"url"`
    ContrastViolations int    `json:"contrastViolations"`
    TouchViolations    int    `json:"touchViolations"`
    TypographyWarnings int    `json:"typographyWarnings"`
    AXActiveNodes      int    `json:"axActiveNodes"`
    AXTotalNodes       int    `json:"axTotalNodes"`
    TotalIssues        int    `json:"totalIssues"`
}

// AXNode is a single node written to <url-slug>/ax-tree.json.
type AXNode struct {
    NodeID     string            `json:"nodeId"`
    Role       string            `json:"role"`
    Name       string            `json:"name"`
    Children   []string          `json:"children,omitempty"`
    Properties map[string]any    `json:"properties,omitempty"`
}

// BaselinesFile is the structure of .kaleidoscope/baselines.json.
type BaselinesFile struct {
    DefaultBaseline string            `json:"defaultBaseline"` // snapshot ID
    URLBaselines    map[string]string `json:"urlBaselines,omitempty"` // url -> snapshot ID override
}
```

#### 3b. Path Helpers

```go
// SnapshotsDir returns .kaleidoscope/snapshots, creating it if needed.
func SnapshotsDir() (string, error)

// SnapshotPath returns the directory for a specific snapshot ID.
func SnapshotPath(id ID) (string, error)

// URLSlug converts a URL string to a safe filesystem name.
// e.g., "https://example.com/about" -> "example.com_about"
// Uses url.Parse, replaces "/" with "_", strips scheme.
func URLSlug(rawURL string) string

// URLDir returns the path to a URL's subdirectory within a snapshot.
func URLDir(snapshotID ID, slug string) (string, error)
```

#### 3c. Persistence Functions

```go
// NewID generates a snapshot ID from current time and optional git hash.
func NewID(hash string) ID

// WriteManifest serializes and writes snapshot.json into the snapshot dir.
func WriteManifest(id ID, m *Manifest) error

// ReadManifest reads and parses snapshot.json for the given snapshot ID.
func ReadManifest(id ID) (*Manifest, error)

// ListIDs returns all snapshot IDs in reverse chronological order
// (newest first). IDs are directory names under SnapshotsDir().
func ListIDs() ([]ID, error)

// ReadBaselines reads .kaleidoscope/baselines.json.
// Returns nil (no error) if the file does not exist.
func ReadBaselines() (*BaselinesFile, error)

// WriteBaselines writes .kaleidoscope/baselines.json.
func WriteBaselines(b *BaselinesFile) error
```

---

### 4. Internal Audit & AX-Tree Functions

The existing `RunAudit` and `RunAxTree` commands write to stdout via `output.Success`. For snapshot capture, we need reusable functions that return data structures instead.

These are added to `cmd/` as unexported helpers (or alternatively in `analysis/` or a new `capture/` package). To minimize refactor scope and match the rule "reuse existing audit, screenshot, and ax-tree logic internally (no shelling out to CLI)", they are extracted as internal helpers in `cmd/`:

```go
// cmd/internal_audit.go

// auditPage runs the full audit logic against an already-navigated page
// and returns an AuditResult. Does not call output.Success.
func auditPage(page *rod.Page) (snapshot.AuditResult, error)

// axTreePage runs the accessibility tree dump and returns a slice of nodes.
func axTreePage(page *rod.Page) ([]snapshot.AXNode, error)
```

These functions contain the same JS evaluation and analysis loops as `RunAudit` and `RunAxTree`, but return structs instead of calling `output.Success`. The `RunAudit` and `RunAxTree` command handlers are refactored to call these helpers, eliminating code duplication.

---

### 5. `cmd/snapshot.go` — `ks snapshot`

```go
package cmd

func RunSnapshot(args []string) {
    // 1. Load project config
    cfg, err := project.Load()
    // fail with output.Fail("snapshot", err, "...") on error

    // 2. Generate snapshot ID
    hash := gitutil.ShortHash()
    id := snapshot.NewID(hash)

    // 3. Create snapshot root dir
    // .kaleidoscope/snapshots/<id>/

    // 4. For each URL in cfg.URLs:
    //    a. Navigate: browser.WithPage -> page.Navigate(url) + MustWaitLoad
    //    b. For each of 4 breakpoints:
    //       - SetViewport
    //       - MustWaitStable
    //       - page.Screenshot(false, nil)
    //       - Write PNG to <id>/<slug>/mobile-375x812.png, etc.
    //    c. Restore viewport to desktop (1280x720) for audit
    //    d. auditPage(page) -> write <id>/<slug>/audit.json
    //    e. axTreePage(page) -> write <id>/<slug>/ax-tree.json
    //    f. Collect URLSummary

    // 5. Build and write Manifest (snapshot.json)

    // 6. Check baselines; if none exist, auto-promote this snapshot
    //    as the default baseline in .kaleidoscope/baselines.json

    // 7. output.Success("snapshot", {...})
}
```

**Error handling:**
- URL navigation errors: collect per-URL errors, continue with remaining URLs, include errors in output result. Do not abort entire snapshot.
- Screenshot/audit errors: include in per-URL error field, continue.
- If ALL URLs fail, exit with `output.Fail`.

**Output (success):**
```json
{
  "ok": true,
  "command": "snapshot",
  "result": {
    "id": "1743800000-a1b2c3d",
    "path": ".kaleidoscope/snapshots/1743800000-a1b2c3d",
    "timestamp": "2026-04-04T12:00:00Z",
    "commitHash": "a1b2c3d",
    "urlCount": 3,
    "baselinePromoted": true,
    "urls": [
      {
        "url": "https://example.com/",
        "slug": "example.com_",
        "contrastViolations": 2,
        "touchViolations": 0,
        "typographyWarnings": 1,
        "axActiveNodes": 42
      }
    ],
    "errors": []
  }
}
```

---

### 6. `cmd/history.go` — `ks history`

```go
package cmd

func RunHistory(args []string) {
    // 1. List snapshot IDs (newest first)
    ids, err := snapshot.ListIDs()
    // if none: output.Success with empty list

    // 2. For each ID: ReadManifest -> build summary entry
    // 3. Load baselines.json to mark which is baseline

    // 4. output.Success("history", {...})
}
```

**Output (success):**
```json
{
  "ok": true,
  "command": "history",
  "result": {
    "count": 3,
    "snapshots": [
      {
        "id": "1743800000-a1b2c3d",
        "timestamp": "2026-04-04T12:00:00Z",
        "commitHash": "a1b2c3d",
        "isBaseline": false,
        "urlCount": 3,
        "totalContrastViolations": 5,
        "totalTouchViolations": 1,
        "totalTypographyWarnings": 2,
        "totalAXActiveNodes": 126
      }
    ]
  }
}
```

---

## API Definitions (CLI)

### `ks snapshot`

```
Usage: ks snapshot [--local]

Captures full interface state for every URL defined in .ks-project.json.
For each URL: takes screenshots at 4 breakpoints, runs audit, dumps ax-tree.
Persists under .kaleidoscope/snapshots/<id>/.

On first run (no baseline exists), auto-promotes snapshot as baseline.

Flags:
  --local    Use project-local .kaleidoscope/ (default: auto-detected)

Output (JSON):
  ok                  bool
  result.id           string   Snapshot ID
  result.path         string   Absolute path to snapshot directory
  result.timestamp    string   ISO 8601 timestamp
  result.commitHash   string   Short git hash (empty if not in repo)
  result.urlCount     int      Number of URLs captured
  result.baselinePromoted bool  True if this was auto-promoted as baseline
  result.urls         array    Per-URL summary (url, slug, issue counts)
  result.errors       array    Per-URL errors (url, error)
```

### `ks history`

```
Usage: ks history [--limit N]

Lists snapshots in reverse chronological order with summary stats.
Reads manifests from .kaleidoscope/snapshots/.

Flags:
  --limit N   Show only the N most recent snapshots (default: all)

Output (JSON):
  ok                  bool
  result.count        int
  result.snapshots    array
    .id               string
    .timestamp        string
    .commitHash       string
    .isBaseline       bool
    .urlCount         int
    .totalContrastViolations  int
    .totalTouchViolations     int
    .totalTypographyWarnings  int
    .totalAXActiveNodes       int
```

---

## Data Model Changes

### New Files

| Path | Committed? | Description |
|------|-----------|-------------|
| `.ks-project.json` | Yes | Project config (from US-001) |
| `.kaleidoscope/baselines.json` | Yes | Baseline snapshot reference |
| `.kaleidoscope/snapshots/<id>/snapshot.json` | No (gitignored) | Snapshot manifest |
| `.kaleidoscope/snapshots/<id>/<slug>/mobile-375x812.png` | No | Screenshot |
| `.kaleidoscope/snapshots/<id>/<slug>/tablet-768x1024.png` | No | Screenshot |
| `.kaleidoscope/snapshots/<id>/<slug>/desktop-1280x720.png` | No | Screenshot |
| `.kaleidoscope/snapshots/<id>/<slug>/wide-1920x1080.png` | No | Screenshot |
| `.kaleidoscope/snapshots/<id>/<slug>/audit.json` | No | Audit results |
| `.kaleidoscope/snapshots/<id>/<slug>/ax-tree.json` | No | Accessibility tree |

### `.gitignore` Additions

```
.kaleidoscope/snapshots/
.kaleidoscope/state.json
.kaleidoscope/screenshots/
```

(`.kaleidoscope/baselines.json` is NOT gitignored.)

### Snapshot ID Format

```
<unix-epoch-seconds>-<short-git-hash>   # inside a git repo
<unix-epoch-seconds>                    # outside git or no commits
```

Example: `1743800000-a1b2c3d`

Sorting IDs lexicographically gives reverse-chronological order for free since epoch seconds are fixed-width within the current millennium.

### URL Slug Algorithm

```
url.Parse(rawURL)
slug = host + path
slug = strings.ReplaceAll(slug, "/", "_")
slug = regexp.MustCompile(`[^a-zA-Z0-9._-]`).ReplaceAllString(slug, "_")
// Trim trailing underscores
```

Example: `https://example.com/about` → `example.com_about`
Example: `https://example.com/` → `example.com_`

---

## New Go Packages / File Summary

| File | Package | Purpose |
|------|---------|---------|
| `project/config.go` | `project` | Load/validate `.ks-project.json` |
| `gitutil/gitutil.go` | `gitutil` | Get short git commit hash |
| `snapshot/manager.go` | `snapshot` | Data types, path helpers, persistence |
| `cmd/snapshot.go` | `cmd` | `RunSnapshot` implementation |
| `cmd/history.go` | `cmd` | `RunHistory` implementation |
| `cmd/internal_audit.go` | `cmd` | `auditPage`, `axTreePage` helpers |

### `main.go` Changes

Add two cases to the command switch:

```go
case "snapshot":
    cmd.RunSnapshot(cmdArgs)
case "history":
    cmd.RunHistory(cmdArgs)
```

Add entries to the usage string under a new **Project** section:

```
Project:
  snapshot              Capture interface state for all project URLs
  history               List snapshots in reverse chronological order
```

---

## Screenshot Naming Convention

Within each `<url-slug>/` directory, PNGs are named by breakpoint:

```
mobile-375x812.png
tablet-768x1024.png
desktop-1280x720.png
wide-1920x1080.png
```

These names are fixed (not timestamped within the slug dir) because the snapshot ID already provides temporal uniqueness. This makes programmatic access predictable.

---

## Breakpoints (Reused from Existing Code)

| Name | Width | Height |
|------|-------|--------|
| mobile | 375 | 812 |
| tablet | 768 | 1024 |
| desktop | 1280 | 720 |
| wide | 1920 | 1080 |

Viewport is restored to desktop (1280x720) before running audit to ensure consistent analysis results.

---

## Baseline Auto-Promotion Logic

```
Read .kaleidoscope/baselines.json
If file does not exist OR defaultBaseline == "":
    Write baselines.json with defaultBaseline = <new snapshot ID>
    Set result.baselinePromoted = true
Else:
    result.baselinePromoted = false
```

This is atomic at the file level: write is done with `os.WriteFile` which is atomic on Linux for small files.

---

## Security Considerations

1. **URL validation:** Before navigating to each URL, validate with `url.Parse` and ensure the scheme is `http` or `https`. Reject `file://`, `javascript:`, and other schemes to prevent arbitrary local file reads or code execution via the project config.

2. **Path traversal:** The `URLSlug` function must sanitize URL-derived strings used in filesystem paths. Use an allowlist regex (`[a-zA-Z0-9._-]`) and enforce a maximum length (e.g., 200 chars) to prevent excessively long paths.

3. **`.ks-project.json` trust boundary:** This file is committed to the repo and read without user confirmation. Since it controls which URLs are visited, it should be treated as trusted (repo-controlled) configuration. Document that users should not run `ks snapshot` against untrusted repos.

4. **Snapshot file permissions:** Write all files with mode `0644` (no execute bit). Directories with `0755`. No secrets are written to snapshot files.

5. **No code execution from snapshots:** `ks history` only reads manifest JSON files. It does not execute any stored data, embed it in HTML without escaping, or evaluate it.

6. **git subprocess:** `gitutil.ShortHash` runs `git rev-parse --short HEAD`. This is a read-only git command with no user-controlled arguments. The output is sanitized with `strings.TrimSpace` before use in filenames. Max 40 chars enforced on slug construction.

---

## Quality Gates

- `go test ./...` must pass (existing gate).
- New packages (`project`, `gitutil`, `snapshot`) are pure-function modules with no Chrome dependency — they are directly unit-testable without a browser.
- `snapshot.URLSlug` has table-driven unit tests covering edge cases: root path, query strings, unicode, long URLs.
- `snapshot.NewID` tests verify format with and without a git hash.
- `snapshot.ListIDs` tests verify reverse-chronological ordering using a temporary directory.
- `auditPage` and `axTreePage` internal helpers are not directly tested (Chrome dependency), but the audit/ax-tree logic they delegate to is covered by existing integration tests.

---

## Open Questions (Deferred)

- **Deduplication:** Should identical PNGs across snapshots be deduplicated (hard links or content-addressed store)? Deferred to post-MVP; current spec writes independent copies.
- **Arbitrary diff:** Should `ks diff` support comparing two arbitrary snapshot IDs (not just latest vs baseline)? Deferred to US-003 scope.
- **`--limit` for history:** Included as a flag for usability, but implementation is a simple slice truncation.
