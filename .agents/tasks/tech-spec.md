# Tech Spec: Baseline Manager (US-005)

## Overview

This spec covers `ks accept`, the Baseline Manager command. It promotes a snapshot to baseline status by updating `.kaleidoscope/baselines.json`, enabling future `ks diff` runs to compare against a known-good state.

**Depends on:** US-003 (Snapshot system — provides the snapshot store and data model)

---

## Architecture Overview

The Baseline Manager is a thin command layer (`cmd/accept.go`) that:

1. Reads the snapshot store to locate the target snapshot (latest or by ID)
2. Reads the current `.kaleidoscope/baselines.json`
3. Updates baseline entries for the relevant URL(s)
4. Writes the updated baselines file back to disk
5. Emits a JSON result via `output.Success` / `output.Fail`

No browser or Chrome dependency is required. This is purely a local file operation.

```
main.go
  └── case "accept" → cmd.RunAccept(args)
        └── snapshot.Store.Latest(url) / snapshot.Store.ByID(id)
        └── baseline.Manager.Accept(snapshotID, url?)
              └── reads/writes .kaleidoscope/baselines.json
        └── output.Success("accept", result)
```

---

## Component Design

### 1. `snapshot` Package (provided by US-003)

The accept command depends on the snapshot store. This spec assumes US-003 defines:

```go
// snapshot/store.go

type Snapshot struct {
    ID        string            `json:"id"`         // e.g. "20260404T123456Z"
    CreatedAt time.Time         `json:"createdAt"`
    URLs      []SnapshotURL     `json:"urls"`
}

type SnapshotURL struct {
    URL          string `json:"url"`          // full URL, e.g. "http://localhost:3000/dashboard"
    Path         string `json:"path"`         // URL path only, e.g. "/dashboard"
    ScreenshotPath string `json:"screenshotPath"` // relative path to PNG
    AuditResult  any    `json:"auditResult"`
    AXTree       any    `json:"axTree"`
}

type Store struct {
    dir string // .kaleidoscope/snapshots/
}

func OpenStore(dir string) (*Store, error)
func (s *Store) Latest() (*Snapshot, error)           // most recent snapshot (all URLs)
func (s *Store) ByID(id string) (*Snapshot, error)    // find snapshot by ID
func (s *Store) ListURLPaths() ([]string, error)      // all URL paths seen across all snapshots
```

If US-003 defines different types, the accept command must adapt to match.

---

### 2. `baseline` Package (new — `baseline/manager.go`)

Encapsulates all read/write logic for `baselines.json`. Pure file I/O, no Chrome dependency.

```go
package baseline

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

// BaselinesFile is the top-level structure of .kaleidoscope/baselines.json.
type BaselinesFile struct {
    Version  int                 `json:"version"`
    Updated  time.Time           `json:"updated"`
    Baselines map[string]Entry   `json:"baselines"` // key: URL path (e.g. "/dashboard")
}

// Entry records which snapshot is the accepted baseline for a URL path.
type Entry struct {
    SnapshotID  string    `json:"snapshotId"`
    AcceptedAt  time.Time `json:"acceptedAt"`
}

type Manager struct {
    path string // absolute path to baselines.json
}

func NewManager(kaleidoscopeDir string) *Manager
func (m *Manager) Load() (*BaselinesFile, error)
func (m *Manager) Save(f *BaselinesFile) error
func (m *Manager) Accept(snapshotID string, paths []string) (*BaselinesFile, error)
```

#### `Accept` Logic

```
func (m *Manager) Accept(snapshotID string, paths []string) (*BaselinesFile, error):
    1. Load existing baselines (or start with empty BaselinesFile{Version:1})
    2. For each path in paths:
       a. If baselines[path].SnapshotID == snapshotID → skip (idempotent no-op)
       b. Otherwise set baselines[path] = Entry{SnapshotID: snapshotID, AcceptedAt: now()}
    3. Set Updated = now()
    4. Save to disk
    5. Return updated BaselinesFile
```

---

### 3. `cmd/accept.go` (new command)

```go
package cmd

func RunAccept(args []string) {
    snapshotID := getArg(args)       // optional positional: specific snapshot ID
    urlFilter  := getFlagValue(args, "--url")  // optional: single URL path

    // 1. Resolve .kaleidoscope dir (respects --local flag pattern from other commands)
    ksDir, err := browser.KaleidoscopeDir()

    // 2. Open snapshot store
    store, err := snapshot.OpenStore(filepath.Join(ksDir, "snapshots"))

    // 3. Resolve target snapshot
    var snap *snapshot.Snapshot
    if snapshotID != "" {
        snap, err = store.ByID(snapshotID)
    } else {
        snap, err = store.Latest()
    }
    // error if no snapshots exist

    // 4. Determine which URL paths to update
    var paths []string
    if urlFilter != "" {
        // validate that the snapshot contains this path
        paths = []string{urlFilter}
    } else {
        // accept for all URL paths in the snapshot
        for _, u := range snap.URLs {
            paths = append(paths, u.Path)
        }
    }

    // 5. Accept
    mgr := baseline.NewManager(ksDir)
    updated, err := mgr.Accept(snap.ID, paths)

    // 6. Emit result
    output.Success("accept", map[string]any{
        "snapshotId": snap.ID,
        "paths":      paths,
        "baselines":  updated.Baselines,
    })
}
```

---

### 4. `main.go` — add `accept` case

```go
case "accept":
    cmd.RunAccept(cmdArgs)
```

Add `--url` to the `getFlagValue` skip list in `cmd/util.go`:

```go
a == "--url" || ...
```

---

## Data Model

### `.kaleidoscope/baselines.json`

```json
{
  "version": 1,
  "updated": "2026-04-04T12:34:56Z",
  "baselines": {
    "/": {
      "snapshotId": "20260404T123000Z",
      "acceptedAt": "2026-04-04T12:34:56Z"
    },
    "/dashboard": {
      "snapshotId": "20260404T123000Z",
      "acceptedAt": "2026-04-04T12:34:56Z"
    }
  }
}
```

**Key:** URL path (e.g. `/dashboard`), not full URL. This makes the file portable across different host/port combos used during development.

**File location:** `.kaleidoscope/baselines.json`
**Git tracking:** Committed to the repo (shared baseline, per project rules).
**Snapshot storage:** `.kaleidoscope/snapshots/` is gitignored (per project rules).

---

## API / CLI Reference

### `ks accept [snapshot-id] [--url <path>]`

| Form | Behavior |
|------|----------|
| `ks accept` | Promote latest snapshot as baseline for all its URLs |
| `ks accept <snapshot-id>` | Promote specific snapshot as baseline for all its URLs |
| `ks accept --url /dashboard` | Promote latest snapshot as baseline for `/dashboard` only |
| `ks accept <snapshot-id> --url /dashboard` | Promote specific snapshot for `/dashboard` only |

**Success output:**
```json
{
  "ok": true,
  "command": "accept",
  "result": {
    "snapshotId": "20260404T123000Z",
    "paths": ["/", "/dashboard"],
    "baselines": {
      "/": { "snapshotId": "20260404T123000Z", "acceptedAt": "2026-04-04T12:34:56Z" },
      "/dashboard": { "snapshotId": "20260404T123000Z", "acceptedAt": "2026-04-04T12:34:56Z" }
    }
  }
}
```

**Error cases:**

| Condition | Error message |
|-----------|---------------|
| No snapshots exist | `"no snapshots found"`, hint: `"Run: ks snapshot"` |
| Snapshot ID not found | `"snapshot not found: <id>"` |
| `--url` path not in snapshot | `"snapshot does not contain URL path: <path>"` |
| Cannot write baselines.json | underlying OS error |

---

## File Layout

```
cmd/
  accept.go          # RunAccept() — new file
  util.go            # add "--url" to flag-value skip list
baseline/
  manager.go         # Manager, BaselinesFile, Entry — new package
main.go              # add "accept" case
```

No changes to `analysis/`, `browser/`, `output/`, or `report/` packages.

---

## Error Handling

- All errors surface through `output.Fail("accept", err, hint)` followed by `os.Exit(2)`, matching the pattern used by every other command.
- Idempotency: calling `ks accept` twice with the same snapshot produces the same `baselines.json`. The second call returns success with the same data (not an error).
- Partial `--url` updates: only the specified path is written; other paths in `baselines.json` are preserved as-is.

---

## Security Considerations

- **Path traversal:** The `--url` value is used as a map key in JSON, never as a filesystem path. No sanitization beyond validation that it starts with `/` is required.
- **File permissions:** `baselines.json` is written with mode `0644` (readable by all, writable by owner), consistent with how other kaleidoscope files are stored.
- **Atomic writes:** To avoid corrupting `baselines.json` on crash mid-write, use `os.WriteFile` with a temp file + rename pattern (write to `.kaleidoscope/baselines.json.tmp`, then `os.Rename`).
- **No external input execution:** The accept command performs no shell execution, network calls, or JavaScript evaluation. It is purely local file I/O.

---

## Test Cases (for `go test ./...`)

### `baseline` package unit tests

| Test | Assertion |
|------|-----------|
| Accept on empty store creates file with correct entry | `baselines["/"].snapshotId == snap.ID` |
| Accept same snapshot ID twice is idempotent | file unchanged on second call |
| Accept with `--url` only updates that path, preserves others | other paths unmodified |
| Accept writes correct `updated` timestamp | `updated` reflects call time |
| Load on missing file returns empty BaselinesFile (not error) | `len(baselines) == 0` |

### `cmd/accept` integration tests

| Test | Assertion |
|------|-----------|
| No snapshots → `output.Fail` with "no snapshots found" | exit code 2, `ok: false` |
| Latest snapshot accepted → all URL paths in baselines | `len(result.paths) == len(snap.URLs)` |
| Specific snapshot ID accepted | `result.snapshotId == requested ID` |
| Unknown snapshot ID → error | `ok: false` |
| `--url /foo` not in snapshot → error | `ok: false` |
