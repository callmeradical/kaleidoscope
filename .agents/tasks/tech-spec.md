# Tech Spec: Snapshot Capture and History (US-002)

## Overview

Introduce `ks snapshot` and `ks history` commands that capture full interface state (screenshots at 4 breakpoints, audit results, and accessibility tree) for every URL listed in a project config, persist them under `.kaleidoscope/snapshots/`, and auto-promote the first snapshot as a baseline.

Depends on US-001 (`.ks-project.json` project config).

---

## Architecture Overview

```
main.go                       ← register "snapshot" and "history" cases
cmd/snapshot.go               ← ks snapshot command handler
cmd/history.go                ← ks history command handler
snapshot/                     ← new package: pure data model + storage
  model.go                    ← Manifest, URLEntry, AuditSummary types
  store.go                    ← SnapshotDir(), Save(), Load(), List()
  project.go                  ← ProjectConfig type + LoadProjectConfig()
  baseline.go                 ← Baseline type + LoadBaseline(), SaveBaseline()
```

All browser interaction remains in `cmd/` using the existing `browser.WithPage` pattern.
The `snapshot/` package is a pure-function module with no Chrome dependency.

---

## Data Model

### `.ks-project.json` (from US-001, read-only here)

Located at `<cwd>/.ks-project.json`. Committed to the repo.

```json
{
  "name": "my-project",
  "urls": [
    "http://localhost:3000",
    "http://localhost:3000/about"
  ]
}
```

### Snapshot Directory Layout

`.kaleidoscope/` is resolved via the existing `browser.StateDir()` logic (project-local `.kaleidoscope/` preferred over `~/.kaleidoscope/`). The `snapshots/` subdirectory is gitignored.

```
.kaleidoscope/
  snapshots/
    20260404T120000Z-abc1234/     ← snapshot ID (timestamp + short commit hash)
      snapshot.json               ← manifest for this snapshot
      localhost-3000/             ← sanitized URL directory (one per URL)
        mobile-375x812.png
        tablet-768x1024.png
        desktop-1280x720.png
        wide-1920x1080.png
        audit.json
        ax-tree.json
      localhost-3000-about/
        mobile-375x812.png
        ...
        audit.json
        ax-tree.json
  baselines.json                  ← committed to repo (shared baseline pointer)
```

### Snapshot ID Format

- In a git repo: `<RFC3339-compact-UTC>-<7-char-commit-hash>` — e.g. `20260404T120000Z-abc1234`
- Outside git: `<RFC3339-compact-UTC>` — e.g. `20260404T120000Z`

Time format string: `"20060102T150405Z"` (Go reference time in UTC).

### `snapshot.json` — Manifest

```json
{
  "id": "20260404T120000Z-abc1234",
  "timestamp": "2026-04-04T12:00:00Z",
  "commitHash": "abc1234",
  "projectConfig": {
    "name": "my-project",
    "urls": ["http://localhost:3000", "http://localhost:3000/about"]
  },
  "urls": [
    {
      "url": "http://localhost:3000",
      "dir": "localhost-3000",
      "breakpoints": [
        "mobile-375x812.png",
        "tablet-768x1024.png",
        "desktop-1280x720.png",
        "wide-1920x1080.png"
      ],
      "auditSummary": {
        "totalIssues": 3,
        "contrastViolations": 1,
        "touchViolations": 0,
        "typographyWarnings": 2
      },
      "axNodeCount": 47,
      "error": ""
    }
  ]
}
```

Fields:
- `id` — snapshot directory name and stable identifier
- `timestamp` — ISO 8601 UTC capture time
- `commitHash` — 7-char git short hash, empty string when not in git
- `projectConfig` — copy of `.ks-project.json` at capture time
- `urls[].dir` — sanitized subdirectory name for this URL
- `urls[].auditSummary` — top-level counts from audit (mirrors `audit` command `summary` field)
- `urls[].axNodeCount` — total non-ignored accessibility nodes
- `urls[].error` — non-empty if URL was unreachable; other fields may be empty

### `baselines.json`

Located at `.kaleidoscope/baselines.json`. Committed to the repo.

```json
{
  "snapshotId": "20260404T120000Z-abc1234",
  "setAt": "2026-04-04T12:00:00Z"
}
```

---

## Package Design: `snapshot/`

### `snapshot/model.go`

```go
package snapshot

import "time"

type ProjectConfig struct {
    Name string   `json:"name"`
    URLs []string `json:"urls"`
}

type AuditSummary struct {
    TotalIssues        int `json:"totalIssues"`
    ContrastViolations int `json:"contrastViolations"`
    TouchViolations    int `json:"touchViolations"`
    TypographyWarnings int `json:"typographyWarnings"`
}

type URLEntry struct {
    URL          string       `json:"url"`
    Dir          string       `json:"dir"`
    Breakpoints  []string     `json:"breakpoints"`
    AuditSummary AuditSummary `json:"auditSummary"`
    AxNodeCount  int          `json:"axNodeCount"`
    Error        string       `json:"error,omitempty"`
}

type Manifest struct {
    ID            string        `json:"id"`
    Timestamp     time.Time     `json:"timestamp"`
    CommitHash    string        `json:"commitHash,omitempty"`
    ProjectConfig ProjectConfig `json:"projectConfig"`
    URLs          []URLEntry    `json:"urls"`
}

type Baseline struct {
    SnapshotID string    `json:"snapshotId"`
    SetAt      time.Time `json:"setAt"`
}
```

### `snapshot/store.go`

```go
// SnapshotsDir returns <stateDir>/snapshots/, creating it if needed.
func SnapshotsDir() (string, error)

// SnapshotPath returns the directory for a given snapshot ID.
func SnapshotPath(id string) (string, error)

// Save writes a Manifest as snapshot.json inside its snapshot directory.
// Creates the snapshot directory if needed.
func Save(m *Manifest) error

// Load reads snapshot.json from the given snapshot ID directory.
func Load(id string) (*Manifest, error)

// List returns all Manifests sorted reverse-chronologically by Timestamp.
// Skips directories that cannot be parsed (logs a warning, does not fail).
func List() ([]*Manifest, error)
```

### `snapshot/project.go`

```go
// LoadProjectConfig reads .ks-project.json from the current working directory.
// Returns a descriptive error if not found (instructs user to create it).
func LoadProjectConfig() (*ProjectConfig, error)
```

### `snapshot/baseline.go`

```go
// BaselinePath returns <stateDir>/baselines.json.
func BaselinePath() (string, error)

// LoadBaseline reads baselines.json. Returns nil, nil if file does not exist.
func LoadBaseline() (*Baseline, error)

// SaveBaseline writes baselines.json.
func SaveBaseline(b *Baseline) error
```

### `snapshot/urldir.go`

```go
// URLToDir converts a URL to a safe filesystem directory name.
// Removes the scheme, then replaces non-alphanumeric characters with "-",
// collapses consecutive dashes, and trims leading/trailing dashes.
// Examples:
//   "http://localhost:3000"        → "localhost-3000"
//   "http://localhost:3000/about"  → "localhost-3000-about"
//   "https://example.com/foo/bar"  → "example-com-foo-bar"
func URLToDir(rawURL string) string
```

This function must prevent path traversal: the output must never contain `/`, `\`, or `..`.

---

## Command: `ks snapshot`

**File:** `cmd/snapshot.go`
**Entry point:** `func RunSnapshot(args []string)`

### Algorithm

1. **Load project config** — call `snapshot.LoadProjectConfig()`. On error: `output.Fail("snapshot", err, "Create a .ks-project.json file with a 'urls' array")` and exit 2.

2. **Resolve git commit hash** — run `git rev-parse --short HEAD` via `exec.Command`. Capture stdout. If the command fails or is not in a git repo, use empty string. Trim whitespace from output.

3. **Build snapshot ID** — format current UTC time as `"20060102T150405Z"`. If commit hash is non-empty, append `-<hash>`.

4. **Create snapshot directory** — call `snapshot.SnapshotPath(id)`, then `os.MkdirAll`.

5. **Capture each URL** — for each URL in `projectConfig.URLs`:

   a. Sanitize to dir name via `snapshot.URLToDir(url)`.
   b. Create `<snapshotDir>/<urlDir>/`.
   c. Open `browser.WithPage(func(page *rod.Page) error { ... })`:
      - Navigate to URL: `page.Navigate(url)` + `page.WaitLoad()`.
      - On navigation error: record `URLEntry{URL: url, Dir: urlDir, Error: err.Error()}`, skip remaining steps for this URL.
      - **Screenshots:** iterate `defaultBreakpoints` (reuse the `breakpoint` struct and `defaultBreakpoints` slice from `cmd/breakpoints.go` — move them to a shared location or duplicate the definition). For each breakpoint:
        - `page.SetViewport(...)` + `page.MustWaitStable()`
        - `page.Screenshot(true, nil)` (full page)
        - Write PNG to `<urlDir>/<name>-<W>x<H>.png`
        - Append filename to `breakpointFiles`
      - Restore original viewport.
      - **Audit:** inline the audit logic from `cmd/audit.go` (same JS evaluation + `analysis.*` calls). Produce an `AuditSummary`.
      - **Ax-tree:** call `proto.AccessibilityGetFullAXTree{}.Call(page)`. Count non-ignored nodes. Marshal full node list to JSON. Write to `<urlDir>/ax-tree.json`.
      - Write `audit.json` — marshal the full audit result map (same structure as `output.Success("audit", ...)` result field).
      - Append `URLEntry` to entries list.

6. **Write manifest** — construct `Manifest`, call `snapshot.Save(&m)`.

7. **Auto-promote baseline** — call `snapshot.LoadBaseline()`. If result is nil (no baselines.json), call `snapshot.SaveBaseline(&Baseline{SnapshotID: id, SetAt: time.Now().UTC()})`. Set `autoBaseline = true` in output.

8. **Output success:**

```json
{
  "ok": true,
  "command": "snapshot",
  "result": {
    "id": "20260404T120000Z-abc1234",
    "snapshotDir": ".kaleidoscope/snapshots/20260404T120000Z-abc1234",
    "urlCount": 2,
    "autoBaseline": true,
    "urls": [
      {
        "url": "http://localhost:3000",
        "dir": "localhost-3000",
        "breakpoints": ["mobile-375x812.png", "tablet-768x1024.png", "desktop-1280x720.png", "wide-1920x1080.png"],
        "auditSummary": { "totalIssues": 3, "contrastViolations": 1, "touchViolations": 0, "typographyWarnings": 2 },
        "axNodeCount": 47
      }
    ]
  }
}
```

### Flags

| Flag | Description |
|------|-------------|
| _(none for now)_ | No flags required for US-002 |

### Error Handling

| Condition | Behavior |
|-----------|----------|
| `.ks-project.json` missing | `output.Fail` + exit 2 |
| Browser not running | `output.Fail` + hint "Run: ks start" + exit 2 |
| URL unreachable / navigation error | Record error in URLEntry, continue other URLs |
| Not in a git repo | Use timestamp-only ID, no error |
| Screenshot write failure | Return error from `WithPage`, `output.Fail` + exit 2 |

---

## Command: `ks history`

**File:** `cmd/history.go`
**Entry point:** `func RunHistory(args []string)`

### Algorithm

1. Call `snapshot.List()` — returns `[]*Manifest` sorted newest-first.
2. Load `snapshot.LoadBaseline()` to annotate which snapshot is the current baseline.
3. Build output list: for each manifest, emit a summary object.
4. `output.Success("history", { "count": N, "snapshots": [...] })`

### Output Shape

```json
{
  "ok": true,
  "command": "history",
  "result": {
    "count": 2,
    "baseline": "20260403T090000Z-def5678",
    "snapshots": [
      {
        "id": "20260404T120000Z-abc1234",
        "timestamp": "2026-04-04T12:00:00Z",
        "commitHash": "abc1234",
        "urlCount": 2,
        "totalIssues": 5,
        "isBaseline": false
      },
      {
        "id": "20260403T090000Z-def5678",
        "timestamp": "2026-04-03T09:00:00Z",
        "commitHash": "def5678",
        "urlCount": 2,
        "totalIssues": 3,
        "isBaseline": true
      }
    ]
  }
}
```

`totalIssues` is the sum of `auditSummary.totalIssues` across all URL entries in the snapshot.

### Error Handling

| Condition | Behavior |
|-----------|----------|
| No snapshots exist yet | Return `{ "count": 0, "snapshots": [] }` — not an error |
| State dir missing | `output.Fail` + hint "Run: ks snapshot" |
| Corrupt snapshot.json | Skip that entry silently (log to stderr if `--human`) |

---

## `main.go` Changes

Add two new cases to the `switch` statement:

```go
case "snapshot":
    cmd.RunSnapshot(cmdArgs)
case "history":
    cmd.RunHistory(cmdArgs)
```

Add to the usage string under a new `Project` section:

```
Project:
  snapshot                Capture full interface state for all project URLs
  history                 List snapshots in reverse chronological order
```

---

## Breakpoint Sharing

The `breakpoint` struct and `defaultBreakpoints` slice are currently defined locally in `cmd/breakpoints.go`. To reuse them in `cmd/snapshot.go` without duplication, move the definition to a new unexported helper in the `cmd` package:

**`cmd/breakpoints_common.go`** (new file):
```go
package cmd

type breakpoint struct {
    Name   string
    Width  int
    Height int
}

var defaultBreakpoints = []breakpoint{
    {"mobile", 375, 812},
    {"tablet", 768, 1024},
    {"desktop", 1280, 720},
    {"wide", 1920, 1080},
}
```

Remove the duplicate definition from `cmd/breakpoints.go`.

---

## Audit/AX-Tree Logic Sharing

The JS evaluation and `analysis.*` calls in `cmd/audit.go` should be extracted into an internal function callable from both `cmd/audit.go` and `cmd/snapshot.go`.

**`cmd/audit_internal.go`** (new file):
```go
package cmd

import (
    "github.com/go-rod/rod"
    "github.com/callmeradical/kaleidoscope/snapshot"
)

// runAudit executes the full audit against the current page and returns
// structured results. selector may be empty for full-page audit.
func runAudit(page *rod.Page, selector string) (map[string]any, snapshot.AuditSummary, error)

// runAxTree executes the accessibility tree dump and returns the node list
// and non-ignored node count.
func runAxTree(page *rod.Page) ([]map[string]any, int, error)
```

`cmd/audit.go`'s `RunAudit` becomes a thin wrapper calling `runAudit` + `output.Success`.

---

## File and Directory Conventions

| Path | Committed? | Description |
|------|-----------|-------------|
| `.ks-project.json` | Yes | Project config (URLs, name) |
| `.kaleidoscope/baselines.json` | Yes | Current baseline pointer |
| `.kaleidoscope/snapshots/` | No (gitignored) | All snapshot data |
| `.kaleidoscope/state.json` | No | Browser state (existing) |
| `.kaleidoscope/screenshots/` | No | Ad-hoc screenshots (existing) |

The `.gitignore` entry `snapshots/` must be added under `.kaleidoscope/` — this is a note for project setup docs, not code to generate automatically.

---

## Security Considerations

1. **Path traversal prevention** — `snapshot.URLToDir()` must strip or replace all `/`, `\`, `.`, `:`, and non-alphanumeric characters. The output is used directly as a directory name under the snapshot root. Unit tests must cover `../`, absolute paths embedded in URLs, and null bytes.

2. **No shell execution of URLs** — URLs are passed directly to `page.Navigate()` via the Chrome DevTools Protocol; they are never interpolated into shell commands.

3. **Git hash via exec, not shell** — `exec.Command("git", "rev-parse", "--short", "HEAD")` is called directly with separate args, not via a shell. No user input is passed to this command.

4. **Write scope** — all writes are confined to the `.kaleidoscope/` state directory resolved by `browser.StateDir()`. No writes to paths derived from URL content.

5. **File permissions** — snapshot directories and files use `0755` / `0644` (same as existing state files).

---

## Quality Gates

The implementation must pass `go test ./...`. The following test cases are required:

| Test | Location | Assertion |
|------|----------|-----------|
| `URLToDir` path traversal | `snapshot/urldir_test.go` | `../etc` → `--etc` or similar safe form |
| `URLToDir` typical URLs | `snapshot/urldir_test.go` | See examples in model section |
| `Save` + `Load` roundtrip | `snapshot/store_test.go` | Manifest marshals/unmarshals correctly |
| `List` sort order | `snapshot/store_test.go` | Newest snapshot appears first |
| `LoadBaseline` returns nil when file absent | `snapshot/baseline_test.go` | No error, nil result |
| `LoadProjectConfig` error on missing file | `snapshot/project_test.go` | Error contains actionable message |
