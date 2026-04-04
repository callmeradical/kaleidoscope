# Tech Spec: Baseline Manager (US-005)

## Overview

Implements `ks accept [snapshot-id]` — a command that promotes a snapshot to
baseline by updating `.kaleidoscope/baselines.json`. Future `ks diff` runs
compare against the accepted baseline rather than triggering false regressions
on intentional changes.

**Depends on:** US-003 (Snapshot system), which provides the snapshot store and
`Snapshot` data structures used here.

---

## Architecture Overview

```
cmd/accept.go          CLI entry point — parses flags, delegates to snapshot pkg
snapshot/baseline.go   Pure-function baseline read/write logic (no Chrome dep)
snapshot/store.go      (from US-003) Snapshot enumeration and lookup
.kaleidoscope/
  snapshots/           (gitignored) Snapshot data written by US-003
  baselines.json       (committed) Maps URL path → snapshot ID
```

The `accept` command is intentionally Chrome-free: it only reads the snapshot
store on disk and rewrites `baselines.json`. No browser lifecycle involved.

---

## Component Design

### 1. `snapshot` package (shared with US-003)

US-003 must expose the following types and functions. If US-003 is not yet
merged, the `accept` command must define compatible stubs or import from a
shared internal package.

#### Types (defined by US-003, consumed here)

```go
// snapshot/types.go

// Snapshot represents one recorded point-in-time capture for a URL.
type Snapshot struct {
    ID          string    `json:"id"`           // e.g. "20260404-153012-a1b2c3"
    URL         string    `json:"url"`          // full URL captured
    URLPath     string    `json:"urlPath"`      // path component only, e.g. "/dashboard"
    CreatedAt   time.Time `json:"createdAt"`
    ScreenshotPath string `json:"screenshotPath,omitempty"`
    AuditResult    any    `json:"auditResult,omitempty"`
    AXTree         any    `json:"axTree,omitempty"`
}

// Baselines maps URL path → snapshot ID.
type Baselines map[string]string  // key: URLPath, value: Snapshot.ID
```

#### Functions (defined by US-003 or stubs)

```go
// snapshot/store.go

// ListSnapshots returns all snapshots in creation order (oldest first).
func ListSnapshots() ([]Snapshot, error)

// LatestSnapshot returns the most recently created snapshot, or an error if none exist.
func LatestSnapshot() (*Snapshot, error)

// LatestSnapshotForURL returns the most recent snapshot for a given URL path.
func LatestSnapshotForURL(urlPath string) (*Snapshot, error)

// GetSnapshot returns the snapshot with the given ID, or error if not found.
func GetSnapshot(id string) (*Snapshot, error)
```

### 2. `snapshot/baseline.go` (new file, this story)

Pure-function module: two data structures in, updated data structure out.
No side effects beyond disk I/O on `baselines.json`.

```go
package snapshot

import (
    "encoding/json"
    "os"
    "path/filepath"

    "github.com/callmeradical/kaleidoscope/browser"
)

// baselinesPath returns the absolute path to baselines.json.
func baselinesPath() (string, error) {
    dir, err := browser.StateDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(dir, "baselines.json"), nil
}

// ReadBaselines loads baselines.json from disk.
// Returns an empty Baselines map (not an error) when the file does not exist.
func ReadBaselines() (Baselines, error)

// WriteBaselines atomically writes baselines to baselines.json.
// Uses a temp-file + rename pattern to avoid partial writes.
func WriteBaselines(b Baselines) error

// AcceptSnapshot promotes snap to baseline.
//   - urlPath == "": promotes snap for snap.URLPath only (single-URL update).
//   - urlPath == "*": promotes snap for ALL URLs currently in baselines, plus snap.URLPath.
// Returns (updated Baselines, wasNoOp, error).
// wasNoOp is true when the baseline already pointed to snap.ID for every
// affected URL (idempotent path).
func AcceptSnapshot(current Baselines, snap *Snapshot, urlPath string) (Baselines, bool, error)
```

**`AcceptSnapshot` logic (pure, no I/O):**

```
updated = copy of current
if urlPath == "*":
    for each key in updated:
        updated[key] = snap.ID
    updated[snap.URLPath] = snap.ID   // also cover snap's own path
else:
    targetPath = urlPath if urlPath != "" else snap.URLPath
    updated[targetPath] = snap.ID

wasNoOp = (updated == current)
return updated, wasNoOp, nil
```

### 3. `cmd/accept.go` (new file, this story)

CLI wiring. Follows the same pattern as all other `cmd/*.go` files.

```go
package cmd

func RunAccept(args []string) {
    snapshotID := getArg(args)          // optional positional arg
    urlPath    := getFlagValue(args, "--url")

    // 1. Resolve target snapshot
    var snap *snapshot.Snapshot
    var err error
    if snapshotID != "" {
        snap, err = snapshot.GetSnapshot(snapshotID)
    } else {
        snap, err = snapshot.LatestSnapshot()
    }
    if err != nil {
        output.Fail("accept", err, "Run 'ks snapshot' to create a snapshot first.")
        os.Exit(2)
    }

    // 2. Load current baselines
    current, err := snapshot.ReadBaselines()
    if err != nil {
        output.Fail("accept", err, "")
        os.Exit(2)
    }

    // 3. Apply accept logic (pure function)
    scope := urlPath   // "" means snap's own URL; handled inside AcceptSnapshot
    if urlPath == "" {
        scope = "*"    // default: all URLs
    }
    updated, wasNoOp, err := snapshot.AcceptSnapshot(current, snap, scope)
    if err != nil {
        output.Fail("accept", err, "")
        os.Exit(2)
    }

    // 4. Persist (skip write on no-op)
    if !wasNoOp {
        if err := snapshot.WriteBaselines(updated); err != nil {
            output.Fail("accept", err, "")
            os.Exit(2)
        }
    }

    // 5. Emit result
    output.Success("accept", map[string]any{
        "snapshotId": snap.ID,
        "noOp":       wasNoOp,
        "updated":    updated,
        "url":        urlPath,   // empty string means "all URLs"
    })
}
```

**Flag semantics:**

| Invocation | Behavior |
|---|---|
| `ks accept` | Promote latest snapshot as baseline for ALL project URLs |
| `ks accept <id>` | Promote specific snapshot for ALL project URLs |
| `ks accept --url /dashboard` | Promote latest snapshot for `/dashboard` only |
| `ks accept <id> --url /dashboard` | Promote specific snapshot for `/dashboard` only |

### 4. `main.go` changes

Add `"accept"` to the switch and usage string:

```go
case "accept":
    cmd.RunAccept(cmdArgs)
```

Usage string addition (under "Snapshot History"):

```
  accept [snapshot-id]    Promote snapshot to baseline (--url <path> for single URL)
```

`util.go` must be updated to recognise `--url` as a flag that consumes the next
argument (prevent it from being treated as a positional arg):

```go
// In getNonFlagArgs, add "--url" to the flag-with-value list:
if a == "--selector" || a == "--output" || ... || a == "--url" {
    skip = true
}
```

---

## Data Model

### `.kaleidoscope/baselines.json`

Committed to version control. Simple flat map from URL path to snapshot ID.

```json
{
  "/":           "20260404-153012-a1b2c3",
  "/dashboard":  "20260404-153012-a1b2c3",
  "/settings":   "20260401-090000-d4e5f6"
}
```

- Key: `URLPath` (path component only, e.g. `/dashboard`, never the full URL).
- Value: `Snapshot.ID` as written by US-003.
- Missing key = no baseline set for that URL (first `ks diff` will prompt user
  to run `ks accept`).

### `.kaleidoscope/snapshots/` (owned by US-003)

The accept command reads from this directory but never writes to it.
Layout (defined by US-003):

```
.kaleidoscope/snapshots/
  <snapshot-id>/
    meta.json          Snapshot struct serialised as JSON
    screenshot.png     (optional)
    ax-tree.json       (optional)
    audit.json         (optional)
```

---

## API / CLI Contract

### Output (success)

```json
{
  "ok": true,
  "command": "accept",
  "result": {
    "snapshotId": "20260404-153012-a1b2c3",
    "noOp": false,
    "url": "",
    "updated": {
      "/":          "20260404-153012-a1b2c3",
      "/dashboard": "20260404-153012-a1b2c3"
    }
  }
}
```

- `url` is empty string when all URLs were updated (default behaviour).
- `noOp: true` when the baseline was already pointing to the same snapshot ID.
- `updated` reflects the full baselines map after the operation.

### Output (error — no snapshots)

```json
{
  "ok": false,
  "command": "accept",
  "error": "no snapshots found",
  "hint": "Run 'ks snapshot' to create a snapshot first."
}
```

### Output (error — snapshot ID not found)

```json
{
  "ok": false,
  "command": "accept",
  "error": "snapshot not found: 20260101-000000-xxxxxx",
  "hint": ""
}
```

---

## File Layout Summary

| File | Status | Notes |
|---|---|---|
| `cmd/accept.go` | New | CLI entry point |
| `snapshot/baseline.go` | New | `ReadBaselines`, `WriteBaselines`, `AcceptSnapshot` |
| `snapshot/types.go` | New (or from US-003) | `Snapshot`, `Baselines` types |
| `snapshot/store.go` | New (or from US-003) | `ListSnapshots`, `LatestSnapshot`, `GetSnapshot` |
| `main.go` | Modified | Add `"accept"` case and usage entry |
| `cmd/util.go` | Modified | Add `--url` to value-consuming flags list |
| `.kaleidoscope/baselines.json` | Runtime artifact | Written by `ks accept`, committed to repo |

---

## Security Considerations

1. **Path traversal in snapshot IDs** — `GetSnapshot(id)` must sanitize the ID
   before using it as a directory name. Reject any ID containing `/`, `..`, or
   non-alphanumeric characters outside `[a-zA-Z0-9-_]`.

2. **Atomic writes** — `WriteBaselines` must write to a temp file in the same
   directory then `os.Rename` to avoid a torn read by a concurrent `ks diff`.

3. **File permissions** — `baselines.json` should be created with mode `0644`
   (readable by all, writable only by owner). The `.kaleidoscope/` directory
   itself should be `0755`.

4. **No user-supplied code execution** — The accept command performs only
   structured JSON serialisation. No shell exec, no template rendering.

5. **URL path validation** — The `--url` value is stored as a map key, not used
   in any filesystem path. No sanitisation beyond trimming trailing slashes is
   required, but the implementation should normalise paths consistently
   (e.g., always leading `/`).

---

## Test Plan (`go test ./...`)

| Test | Type | Notes |
|---|---|---|
| `AcceptSnapshot` promotes latest to all URLs | Unit | Pure function, no I/O |
| `AcceptSnapshot` with specific ID | Unit | |
| `AcceptSnapshot` with `--url` scopes correctly | Unit | Other URLs unchanged |
| `AcceptSnapshot` is idempotent when already baseline | Unit | `wasNoOp == true` |
| `AcceptSnapshot` returns error with empty snapshot list | Unit | `LatestSnapshot` stub returns error |
| `ReadBaselines` returns empty map when file missing | Unit | |
| `WriteBaselines` round-trips through `ReadBaselines` | Unit | Temp dir |
| `RunAccept` integration: no snapshots → exit 2 + fail JSON | Integration | Requires stub store |
| Snapshot ID with path traversal rejected by `GetSnapshot` | Unit | |

---

## Open Questions / Assumptions

1. **US-003 snapshot store interface** — This spec assumes US-003 exposes
   `ListSnapshots`, `LatestSnapshot`, and `GetSnapshot` from a `snapshot`
   package. If the actual interface differs, `cmd/accept.go` must adapt.

2. **"All URLs" scope** — When no `--url` flag is given, the spec promotes the
   snapshot for *all URLs already in `baselines.json`*, plus the snapshot's own
   URL path. If the project has URLs not yet in baselines, they remain absent
   (not silently added). This matches the PRD: "promotes … for all project
   URLs" means all URLs *with an existing baseline entry*.

3. **Project config (`ks-project.json`)** — The PRD mentions a
   `.ks-project.json` file. If US-003 uses it to enumerate project URLs, the
   accept command could use it to initialise baselines for all declared URLs.
   This spec does not depend on that file; it is left for US-003 to define.
