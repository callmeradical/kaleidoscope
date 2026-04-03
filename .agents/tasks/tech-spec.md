# Tech Spec: US-002 — Snapshot Capture and History

**Story**: As an AI agent, I want to automatically capture full interface state for every URL in a project, so that I can compare state before and after code changes.

**Depends on**: US-001 (project config, `.ks-project.json`)

---

## Architecture Overview

US-002 adds two new CLI commands (`ks snapshot`, `ks history`) and the supporting infrastructure for snapshot persistence, baseline tracking, and internal capture helpers. The design follows existing kaleidoscope conventions throughout.

### Key Design Principles

1. **No shelling out** — reuse existing audit, ax-tree, and breakpoints logic via internal Go function calls, not `exec.Command("ks", ...)`.
2. **Pure JSON output** — all commands use `output.Success` / `output.Fail` exactly like existing commands.
3. **Layered packages** — `project/` for config I/O, `snapshot/` for persistence, `cmd/` for CLI orchestration.
4. **Internal capture helpers** — thin, data-returning wrappers extracted alongside the existing `Run*` functions so both the original commands and `RunSnapshot` share logic without duplication.

### High-Level Data Flow

```
ks snapshot
  └── Load .ks-project.json            (project.LoadConfig)
  └── Resolve snapshot ID              (timestamp + git short-hash)
  └── Create snapshot directory        (snapshot.CreateDir)
  └── For each URL in project:
      ├── Navigate browser             (page.Navigate + WaitLoad)
      ├── Capture 4 breakpoint PNGs    (captureBreakpointsData)
      ├── Run audit                    (captureAuditData)
      └── Capture ax-tree              (captureAxTreeData)
      └── Write URL subdirectory files
  └── Write root snapshot.json
  └── Auto-promote baseline if needed  (snapshot.EnsureBaseline)
  └── output.Success("snapshot", ...)

ks history
  └── List .kaleidoscope/snapshots/    (snapshot.List)
  └── Read snapshot.json from each
  └── Sort descending by timestamp
  └── output.Success("history", ...)
```

---

## Directory Layout (On-Disk)

```
.ks-project.json                        ← committed to repo (US-001)
.kaleidoscope/                          ← gitignored
  snapshots/
    1743720000000-abc1234/              ← snapshot ID
      snapshot.json                     ← root manifest
      root/                             ← URL "/" (sanitized path)
        mobile.png
        tablet.png
        desktop.png
        wide.png
        audit.json
        ax-tree.json
      about/                            ← URL "/about"
        mobile.png
        ...
.kaleidoscope/baselines.json            ← committed to repo
```

### URL Path → Directory Name Rules

| URL path           | Directory name  |
|--------------------|-----------------|
| `/`                | `root`          |
| `/about`           | `about`         |
| `/products/items`  | `products_items`|
| `?query=x`         | query portion stripped before slugification |

Characters that are not `[a-zA-Z0-9_-]` are replaced with `_`. Leading/trailing underscores are trimmed. Empty result falls back to `root`.

---

## New Go Files

| File                      | Package    | Purpose                                       |
|---------------------------|------------|-----------------------------------------------|
| `project/config.go`       | `project`  | `ProjectConfig` struct; `LoadConfig`, `FindConfig` |
| `snapshot/model.go`       | `snapshot` | `Manifest`, `URLEntry`, `AuditSummary` structs |
| `snapshot/store.go`       | `snapshot` | `CreateDir`, `List`, `ReadManifest`, `WriteManifest`, `SnapshotsDir` |
| `snapshot/baseline.go`    | `snapshot` | `Baseline` struct; `ReadBaseline`, `WriteBaseline`, `EnsureBaseline` |
| `cmd/snapshot.go`         | `cmd`      | `RunSnapshot` — orchestrates full capture loop |
| `cmd/history.go`          | `cmd`      | `RunHistory` — lists snapshots                |
| `cmd/capture_helpers.go`  | `cmd`      | Internal `captureAuditData`, `captureAxTreeData`, `captureBreakpointsData` |

**Modified files**:
- `main.go` — add `"snapshot"` and `"history"` cases to the switch
- `browser/state.go` — add `SnapshotsDir()` helper

---

## Component Design

### `project/config.go`

```go
package project

import (
    "encoding/json"
    "os"
)

// ProjectConfig mirrors .ks-project.json (written by US-001).
type ProjectConfig struct {
    Name        string   `json:"name"`
    URLs        []string `json:"urls"`
    Breakpoints []string `json:"breakpoints,omitempty"` // optional override; default: all 4
}

// FindConfig searches CWD and parent directories for .ks-project.json.
// Returns (nil, nil) if not found (not an error — caller decides).
func FindConfig() (*ProjectConfig, error)

// LoadConfig reads .ks-project.json from the given path.
func LoadConfig(path string) (*ProjectConfig, error)
```

`FindConfig` walks up from `os.Getwd()` stopping at filesystem root or when a `.git` directory is found (same boundary git uses). If `.ks-project.json` is not found, returns `nil, nil`.

### `snapshot/model.go`

```go
package snapshot

import "time"

// Manifest is the root snapshot.json written in each snapshot directory.
type Manifest struct {
    ID            string        `json:"id"`
    Timestamp     time.Time     `json:"timestamp"`
    CommitHash    string        `json:"commitHash,omitempty"` // empty outside git
    ProjectConfig ProjectConfig `json:"projectConfig"`
    URLs          []URLEntry    `json:"urls"`
}

// ProjectConfig mirrors the project config at capture time (snapshot in time).
type ProjectConfig struct {
    Name        string   `json:"name"`
    URLs        []string `json:"urls"`
    Breakpoints []string `json:"breakpoints,omitempty"`
}

// URLEntry records capture results for one URL.
type URLEntry struct {
    URL          string       `json:"url"`
    Dir          string       `json:"dir"`          // directory name within snapshot
    AuditSummary AuditSummary `json:"auditSummary"`
    AxNodeCount  int          `json:"axNodeCount"`
    Screenshots  []string     `json:"screenshots"`  // filenames, e.g. ["mobile.png", ...]
    Error        string       `json:"error,omitempty"` // non-empty if this URL failed
}

// AuditSummary is a condensed copy of the audit summary for quick history display.
type AuditSummary struct {
    TotalIssues        int `json:"totalIssues"`
    ContrastViolations int `json:"contrastViolations"`
    TouchViolations    int `json:"touchViolations"`
    TypographyWarnings int `json:"typographyWarnings"`
}
```

### `snapshot/store.go`

```go
package snapshot

// SnapshotsDir returns (and creates) .kaleidoscope/snapshots/ relative to stateDir.
func SnapshotsDir() (string, error)

// GenerateID returns "<unix-ms>-<shortHash>" or "<unix-ms>" if not in a git repo.
func GenerateID() string

// CreateDir creates the snapshot directory and returns its path.
// e.g., .kaleidoscope/snapshots/1743720000000-abc1234/
func CreateDir(id string) (string, error)

// URLDir returns the sanitized directory name for a URL path.
func URLDir(rawURL string) string

// WriteManifest serialises m to <snapshotDir>/snapshot.json.
func WriteManifest(snapshotDir string, m *Manifest) error

// ReadManifest reads <snapshotDir>/snapshot.json.
func ReadManifest(snapshotDir string) (*Manifest, error)

// List returns all snapshot manifests found under SnapshotsDir(),
// sorted by Timestamp descending.
func List() ([]*Manifest, error)
```

**`GenerateID` implementation detail**: calls `exec.Command("git", "rev-parse", "--short", "HEAD")` with a short timeout. If the command fails or returns non-zero, uses timestamp-only format. Uses `time.Now().UnixMilli()` for the timestamp component.

### `snapshot/baseline.go`

```go
package snapshot

import "time"

// Baseline is the content of .kaleidoscope/baselines.json.
type Baseline struct {
    SnapshotID  string    `json:"snapshotId"`
    PromotedAt  time.Time `json:"promotedAt"`
    PromotedBy  string    `json:"promotedBy"` // "auto" or "user"
}

// BaselinePath returns the path to baselines.json (alongside state dir, committed).
func BaselinePath() (string, error)

// ReadBaseline reads baselines.json; returns (nil, nil) if not found.
func ReadBaseline() (*Baseline, error)

// WriteBaseline writes baselines.json.
func WriteBaseline(b *Baseline) error

// EnsureBaseline auto-promotes snapshotID as baseline if no baselines.json exists yet.
// Returns true if it promoted, false if baseline already existed.
func EnsureBaseline(snapshotID string) (bool, error)
```

**`BaselinePath` note**: The baseline file lives at `<stateDir>/baselines.json`. Per PRD rules it is committed to the repo, so `stateDir` must be the project-local `.kaleidoscope/` directory — not the global `~/.kaleidoscope/`. `ks snapshot` requires project-local state; if running globally (no `.kaleidoscope/` in CWD), it must error.

### `cmd/capture_helpers.go`

Extracted, data-returning versions of the core capture logic from existing commands. The existing `Run*` commands call these helpers and then pass the result to `output.Success`. `RunSnapshot` calls the same helpers to capture without printing.

```go
package cmd

import (
    "github.com/go-rod/rod"
    "github.com/callmeradical/kaleidoscope/snapshot"
)

// captureAuditData runs the full audit on the current page and returns
// the summary and raw audit result (identical to what RunAudit produces).
func captureAuditData(page *rod.Page) (summary snapshot.AuditSummary, raw map[string]any, err error)

// captureAxTreeData captures the accessibility tree and returns
// the node list and node count.
func captureAxTreeData(page *rod.Page) (nodes []map[string]any, nodeCount int, err error)

// captureBreakpointsData takes screenshots at all 4 breakpoints,
// saves PNGs to destDir, and returns the list of filenames written.
// Restores the original viewport after completion.
func captureBreakpointsData(page *rod.Page, destDir string) (filenames []string, err error)
```

**Refactoring existing commands**: `RunAudit`, `RunAxTree`, and `RunBreakpoints` are updated to delegate their core logic to these helpers. Their external behavior (JSON output, error messages) remains identical. This refactor has zero functional impact on existing commands.

### `cmd/snapshot.go` — `RunSnapshot`

```go
func RunSnapshot(args []string) {
    // 1. Locate and load project config
    cfg, err := project.FindConfig()
    // error if cfg is nil (no .ks-project.json found)

    // 2. Require project-local state dir (snapshot must run in-project)
    // error if .kaleidoscope/ does not exist in CWD

    // 3. Generate snapshot ID and create directory
    id := snapshot.GenerateID()
    snapshotDir, err := snapshot.CreateDir(id)

    // 4. Capture each URL
    var urlEntries []snapshot.URLEntry
    err = browser.WithPage(func(page *rod.Page) error {
        for _, rawURL := range cfg.URLs {
            entry := snapshot.URLEntry{URL: rawURL, Dir: snapshot.URLDir(rawURL)}
            urlDir := filepath.Join(snapshotDir, entry.Dir)
            os.MkdirAll(urlDir, 0755)

            // Navigate — fail gracefully per acceptance criteria
            if navErr := page.Navigate(rawURL); navErr != nil {
                entry.Error = navErr.Error()
                urlEntries = append(urlEntries, entry)
                continue
            }
            page.WaitLoad()

            // Screenshots
            entry.Screenshots, _ = captureBreakpointsData(page, urlDir)

            // Audit
            entry.AuditSummary, _, _ = captureAuditData(page)

            // AX tree — save raw JSON to ax-tree.json
            nodes, count, _ := captureAxTreeData(page)
            entry.AxNodeCount = count
            writeJSON(filepath.Join(urlDir, "ax-tree.json"), map[string]any{
                "nodeCount": count, "nodes": nodes,
            })

            urlEntries = append(urlEntries, entry)
        }
        return nil
    })

    // 5. Write manifest
    manifest := &snapshot.Manifest{
        ID: id, Timestamp: time.Now(),
        CommitHash: extractCommitHash(id),
        ProjectConfig: snapshot.ProjectConfig{Name: cfg.Name, URLs: cfg.URLs},
        URLs: urlEntries,
    }
    snapshot.WriteManifest(snapshotDir, manifest)

    // 6. Auto-promote baseline if first snapshot
    promoted, _ := snapshot.EnsureBaseline(id)

    // 7. Output
    output.Success("snapshot", map[string]any{
        "id":               id,
        "snapshotDir":      snapshotDir,
        "urlCount":         len(urlEntries),
        "baselinePromoted": promoted,
        "urls":             urlEntries,
    })
}
```

**Error handling**: If any URL navigation fails, `entry.Error` is set but the snapshot continues. The overall command succeeds (no `os.Exit(2)`) but the failed URL entry records the error. The caller (agent) can inspect `urls[n].error` to detect per-URL failures.

**Browser requirement**: If the browser is not running, `browser.WithPage` returns an error. `RunSnapshot` calls `output.Fail("snapshot", err, "Is the browser running? Run: ks start")` and `os.Exit(2)`.

### `cmd/history.go` — `RunHistory`

```go
func RunHistory(args []string) {
    manifests, err := snapshot.List()
    if err != nil {
        output.Fail("history", err, "No snapshots found. Run: ks snapshot")
        os.Exit(2)
    }

    // Convert manifests to summary list
    summaries := make([]map[string]any, len(manifests))
    for i, m := range manifests {
        summaries[i] = map[string]any{
            "id":         m.ID,
            "timestamp":  m.Timestamp,
            "commitHash": m.CommitHash,
            "urlCount":   len(m.URLs),
            "urls":       urlSummaries(m.URLs), // id, dir, auditSummary per URL
        }
    }

    output.Success("history", map[string]any{
        "count":     len(summaries),
        "snapshots": summaries,
    })
}
```

### `browser/state.go` — Addition

```go
// SnapshotsDir returns (and creates) the snapshots directory under StateDir.
func SnapshotsDir() (string, error) {
    dir, err := StateDir()
    if err != nil {
        return "", err
    }
    ssDir := filepath.Join(dir, "snapshots")
    if err := os.MkdirAll(ssDir, 0755); err != nil {
        return "", err
    }
    return ssDir, nil
}
```

### `main.go` Changes

Add two new cases to the switch:

```go
case "snapshot":
    cmd.RunSnapshot(cmdArgs)
case "history":
    cmd.RunHistory(cmdArgs)
```

Add to the usage string under a new "Snapshot & History" section:

```
Snapshot & History:
  snapshot                Capture full interface state for all project URLs
  history                 List snapshots in reverse chronological order
```

---

## API Definitions (CLI)

### `ks snapshot`

```
ks snapshot [--full-page]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--full-page` | false | Capture full-page screenshots (passed through to breakpoints capture) |

**Success output** (`ok: true`, `command: "snapshot"`):
```json
{
  "ok": true,
  "command": "snapshot",
  "result": {
    "id": "1743720000000-abc1234",
    "snapshotDir": ".kaleidoscope/snapshots/1743720000000-abc1234",
    "urlCount": 2,
    "baselinePromoted": true,
    "urls": [
      {
        "url": "http://localhost:3000",
        "dir": "root",
        "auditSummary": {
          "totalIssues": 3,
          "contrastViolations": 1,
          "touchViolations": 0,
          "typographyWarnings": 2
        },
        "axNodeCount": 42,
        "screenshots": ["mobile.png", "tablet.png", "desktop.png", "wide.png"],
        "error": ""
      }
    ]
  }
}
```

**Failure cases**:
- No `.ks-project.json` found → `output.Fail("snapshot", err, "Create a project config first. Run: ks project init")`
- Browser not running → `output.Fail("snapshot", err, "Is the browser running? Run: ks start")`
- Not in project-local context (no `.kaleidoscope/` in CWD) → `output.Fail("snapshot", err, "Run 'ks start --local' first to create a project-local state directory")`

### `ks history`

```
ks history [--limit N]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | 0 (all) | Show only the N most recent snapshots |

**Success output** (`ok: true`, `command: "history"`):
```json
{
  "ok": true,
  "command": "history",
  "result": {
    "count": 2,
    "snapshots": [
      {
        "id": "1743720001000-def5678",
        "timestamp": "2026-04-03T13:00:00Z",
        "commitHash": "def5678",
        "urlCount": 2,
        "urls": [
          {
            "url": "http://localhost:3000",
            "dir": "root",
            "auditSummary": { "totalIssues": 2, "contrastViolations": 0, "touchViolations": 0, "typographyWarnings": 2 }
          }
        ]
      },
      {
        "id": "1743720000000-abc1234",
        "timestamp": "2026-04-03T12:00:00Z",
        "commitHash": "abc1234",
        "urlCount": 2,
        "urls": [...]
      }
    ]
  }
}
```

---

## Data Model Changes

### New Files (committed to repo)

#### `.ks-project.json` (written by US-001, read by US-002)

```json
{
  "name": "My Project",
  "urls": ["http://localhost:3000", "http://localhost:3000/about"],
  "breakpoints": ["mobile", "tablet", "desktop", "wide"]
}
```

`breakpoints` is optional; defaults to all four if absent.

#### `.kaleidoscope/baselines.json`

```json
{
  "snapshotId": "1743720000000-abc1234",
  "promotedAt": "2026-04-03T12:00:00Z",
  "promotedBy": "auto"
}
```

This file **is committed** to the repo (per PRD rules) so baselines are shared across machines.

### New Files (gitignored)

```
.kaleidoscope/snapshots/          ← entire directory gitignored
```

Per the PRD, add to project's `.gitignore`:
```
.kaleidoscope/snapshots/
.kaleidoscope/state.json
.kaleidoscope/screenshots/
```

### Existing `browser.State` — Unchanged

No changes to the `State` struct. A new `SnapshotsDir()` helper is added to `browser/state.go` for convenience.

---

## Snapshot ID Format

```
<unix-milliseconds>-<7-char-git-hash>   # inside a git repo
<unix-milliseconds>                     # outside git
```

Examples:
- `1743720000000-abc1234`
- `1743720000000`

The millisecond timestamp guarantees chronological sort order by string comparison (lexicographic = chronological). `snapshot.List()` uses `sort.Slice` on `Manifest.Timestamp` for correctness regardless of naming.

---

## Internal Capture Refactor

The three existing commands are refactored to extract testable, reusable helpers. The interface contract of each `Run*` command is **unchanged**.

### Before (in `RunAudit`):

```go
// All logic inline, ends with output.Success(...)
```

### After:

```go
// cmd/capture_helpers.go
func captureAuditData(page *rod.Page) (snapshot.AuditSummary, map[string]any, error) {
    // ... all the JS evaluation and analysis logic ...
    return summary, raw, nil
}

// cmd/audit.go
func RunAudit(args []string) {
    err := browser.WithPage(func(page *rod.Page) error {
        summary, raw, err := captureAuditData(page)
        if err != nil { return err }
        output.Success("audit", raw) // raw is identical to current output
        return nil
    })
    ...
}
```

The same pattern applies to `RunAxTree` → `captureAxTreeData` and `RunBreakpoints` → `captureBreakpointsData`.

---

## Security Considerations

1. **URL validation**: Before navigating to each URL in the project config, validate it is a well-formed URL with `url.Parse`. Reject `file://`, `javascript:`, and other non-http(s) schemes to prevent local file exfiltration or JS injection via a maliciously crafted `.ks-project.json`. Since `.ks-project.json` is a committed file, this is a defense-in-depth measure.

2. **Path traversal in URL → directory mapping**: The `URLDir()` function must produce only safe directory names. After extracting the URL path component, apply a strict allowlist regex (`[^a-zA-Z0-9_-]` → `_`). Never use the raw URL or path as a filesystem path directly.

3. **Git command injection**: `GenerateID()` uses `exec.Command("git", "rev-parse", "--short", "HEAD")` with no user input — no injection risk. The command is hardcoded and receives no arguments from external input.

4. **Screenshot data**: Screenshots are PNG binary data written to disk under `.kaleidoscope/snapshots/` which is gitignored. No screenshot data is included in the JSON output (only file paths).

5. **Snapshot directory permissions**: Created with `0755` (directories) and `0644` (files), matching existing `.kaleidoscope/` conventions.

---

## Quality Gates

- `go test ./...` must pass (per PRD `qualityGates`)
- New packages (`project/`, `snapshot/`) must include unit tests covering:
  - `URLDir` edge cases (root, nested paths, special characters)
  - `GenerateID` format (timestamp prefix, optional hash suffix)
  - `List` sort order (descending by timestamp)
  - `EnsureBaseline` idempotency (second call does not overwrite)
  - `WriteManifest` / `ReadManifest` round-trip
- `captureAuditData`, `captureAxTreeData`, `captureBreakpointsData` are unit-testable with a mock `*rod.Page` or via integration tests with a live browser

---

## Open Questions (from PRD)

> Should snapshot storage deduplicate identical screenshots across runs to save space?

Not in scope for US-002. Snapshot directories are self-contained. Deduplication (e.g., content-addressed storage) can be addressed in a follow-on story.

> Should `ks diff` support comparing two arbitrary snapshots (not just latest vs baseline)?

This is the subject of a future story (US-003 or similar). US-002 only needs to write the data; `ks diff` will consume it. The `Manifest` structure is designed to support arbitrary pair comparison.

---

## Dependency on US-001

US-002 requires:
- `.ks-project.json` to exist with at least one URL in `urls[]`
- The file to be readable by `project.FindConfig()`
- A project-local `.kaleidoscope/` directory (created by `ks start --local`)

If US-001 is not yet implemented when US-002 is built, a minimal stub `project/config.go` implementing only `FindConfig()` and `LoadConfig()` is sufficient. US-001 will write the file; US-002 only reads it.
