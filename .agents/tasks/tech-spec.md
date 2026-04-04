# Tech Spec: Baseline Manager (US-005)

## Story Summary

`ks accept [snapshot-id]` promotes a snapshot to baseline for all project URLs (or a single URL via `--url`), persisting the result to `.kaleidoscope/baselines.json`.

**Depends on:** US-003 (Snapshot System — defines snapshot storage layout and the `Snapshot` type)

---

## Architecture Overview

The Baseline Manager is a thin command layer (`cmd/accept.go`) backed by a pure-function package (`snapshot/baseline.go`) that reads/writes `.kaleidoscope/baselines.json`. It follows the same pattern as all other `ks` commands:

1. Parse CLI args
2. Load data from disk (snapshot index + baselines file)
3. Apply business logic (promote snapshot → baseline)
4. Write result to disk
5. Emit `output.Success` / `output.Fail` JSON

No browser or Chrome dependency. No goroutines. Pure file I/O.

```
main.go
  └── cmd/accept.go          RunAccept(args)
        ├── snapshot.LoadIndex()       reads snapshots index
        ├── snapshot.LoadBaselines()   reads baselines.json
        ├── snapshot.Accept(...)       pure-function promotion logic
        └── snapshot.SaveBaselines()  writes baselines.json
```

---

## Assumed US-003 Contracts

US-005 depends on the following types and functions being provided by US-003. This spec defines the interface; US-003 owns the implementation.

### Snapshot Index (`snapshot.Index`)

```go
// snapshot/index.go  (owned by US-003)

type SnapshotMeta struct {
    ID        string    `json:"id"`         // e.g. "20260404-153012"
    CreatedAt time.Time `json:"createdAt"`
    CommitSHA string    `json:"commitSha,omitempty"`
    URLs      []string  `json:"urls"`       // project URLs captured in this snapshot
}

type Index struct {
    Snapshots []SnapshotMeta `json:"snapshots"` // ordered oldest-first
}

// LoadIndex reads .kaleidoscope/snapshots/index.json
func LoadIndex() (*Index, error)
```

### Snapshot Audit Data (`snapshot.AuditSummary`)

```go
// snapshot/snapshot.go  (owned by US-003)

type AuditSummary struct {
    TotalIssues        int `json:"totalIssues"`
    ContrastViolations int `json:"contrastViolations"`
    TouchViolations    int `json:"touchViolations"`
    TypographyWarnings int `json:"typographyWarnings"`
}

// Per-URL data stored inside a snapshot
type URLSnapshot struct {
    URL          string       `json:"url"`
    ScreenshotPath string     `json:"screenshotPath"`
    Audit        AuditSummary `json:"audit"`
    // ax-tree and other fields owned by US-003
}
```

---

## Detailed Component Design

### 1. `snapshot/baseline.go` — Pure-function baseline logic

This file is **new** and **owned by US-005**.

```go
package snapshot

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// BaselineEntry records which snapshot is the accepted baseline for a URL.
type BaselineEntry struct {
    URL        string `json:"url"`
    SnapshotID string `json:"snapshotId"`
}

// Baselines is the in-memory representation of baselines.json.
type Baselines struct {
    Entries []BaselineEntry `json:"baselines"`
}

// LoadBaselines reads .kaleidoscope/baselines.json.
// Returns an empty Baselines (not an error) if the file does not exist yet.
func LoadBaselines() (*Baselines, error)

// SaveBaselines writes b to .kaleidoscope/baselines.json with 2-space indentation.
func SaveBaselines(b *Baselines) error

// Accept promotes snapshotID as the baseline for each URL in urls.
// If urls is nil/empty, all URLs in the snapshot are promoted.
// Returns the updated Baselines and a list of URLs that were changed.
// Pure function: does not perform I/O.
func Accept(current *Baselines, meta *SnapshotMeta, urls []string) (updated *Baselines, changed []string)
```

#### `Accept` logic (pure function)

```
if urls is empty:
    urls = meta.URLs          // promote all URLs in snapshot

for each url in urls:
    find existing entry in current.Entries where entry.URL == url
    if entry exists:
        if entry.SnapshotID == snapshotID:
            skip (already baseline — idempotent)
        else:
            update entry.SnapshotID = snapshotID
            add url to changed list
    else:
        append new BaselineEntry{URL: url, SnapshotID: snapshotID}
        add url to changed list

return updated Baselines, changed list
```

#### `baselines.json` file path resolution

Use the same `browser.StateDir()` logic (project-local `.kaleidoscope/` takes precedence over `~/.kaleidoscope/`). The file is:
```
<stateDir>/baselines.json
```

### 2. `cmd/accept.go` — CLI command

```go
package cmd

func RunAccept(args []string)
```

**Flag parsing:**

| Flag | Type | Description |
|------|------|-------------|
| `[snapshot-id]` | positional arg (optional) | Snapshot to promote; defaults to latest |
| `--url <path>` | string flag | Limit accept to a single URL path |

**Algorithm:**

```
1. index, err = snapshot.LoadIndex()
   if err or len(index.Snapshots) == 0:
       output.Fail("accept", "no snapshots exist", "Run: ks snapshot")
       os.Exit(2)

2. snapshotID = getArg(args)
   if snapshotID == "":
       meta = index.Snapshots[last]      // default: latest
   else:
       meta = find in index where meta.ID == snapshotID
       if not found:
           output.Fail("accept", fmt.Errorf("snapshot not found: %s", snapshotID), "")
           os.Exit(2)

3. urlFilter = getFlagValue(args, "--url")   // may be empty

4. urls = []string{}
   if urlFilter != "":
       if urlFilter not in meta.URLs:
           output.Fail("accept", fmt.Errorf("url %q not in snapshot %s", urlFilter, meta.ID), "")
           os.Exit(2)
       urls = []string{urlFilter}
   // else urls stays empty → Accept() will use all meta.URLs

5. baselines, err = snapshot.LoadBaselines()
   if err: output.Fail(...); os.Exit(2)

6. updated, changed = snapshot.Accept(baselines, meta, urls)

7. err = snapshot.SaveBaselines(updated)
   if err: output.Fail(...); os.Exit(2)

8. output.Success("accept", map[string]any{
       "snapshotId": meta.ID,
       "changed":    changed,
       "noOp":       len(changed) == 0,
       "baselines":  updated.Entries,
   })
```

### 3. `main.go` — wire up command

Add `case "accept": cmd.RunAccept(cmdArgs)` to the switch in `main.go`.

Add usage string for `accept` to `CommandUsage` map in `cmd/usage.go`.

Add `--url` to the list of value-taking flags in `getNonFlagArgs()` in `cmd/util.go`.

---

## API Definitions

### CLI Interface

```
ks accept [snapshot-id] [--url <path>]

Arguments:
  snapshot-id    ID of the snapshot to accept as baseline (default: latest)

Options:
  --url <path>   Accept baseline for only this URL path; others are unchanged

Output (success):
  {
    "ok": true,
    "command": "accept",
    "result": {
      "snapshotId": "20260404-153012",
      "changed": ["/dashboard", "/login"],
      "noOp": false,
      "baselines": [
        { "url": "/dashboard", "snapshotId": "20260404-153012" },
        { "url": "/login",     "snapshotId": "20260404-153012" }
      ]
    }
  }

Output (no-op — already baseline):
  {
    "ok": true,
    "command": "accept",
    "result": {
      "snapshotId": "20260404-153012",
      "changed": [],
      "noOp": true,
      "baselines": [...]
    }
  }

Output (error — no snapshots):
  { "ok": false, "command": "accept", "error": "no snapshots exist", "hint": "Run: ks snapshot" }

Output (error — snapshot not found):
  { "ok": false, "command": "accept", "error": "snapshot not found: bad-id", "hint": "" }

Output (error — URL not in snapshot):
  { "ok": false, "command": "accept", "error": "url \"/foo\" not in snapshot 20260404-153012", "hint": "" }
```

---

## Data Model

### `.kaleidoscope/baselines.json`

```json
{
  "baselines": [
    {
      "url": "/",
      "snapshotId": "20260404-153012"
    },
    {
      "url": "/dashboard",
      "snapshotId": "20260404-153012"
    }
  ]
}
```

- **Location:** `<stateDir>/baselines.json` (same directory as `state.json`)
- **Committed:** yes — this file is committed to git (shared baseline across team)
- **Written by:** `ks accept` only
- **Read by:** `ks diff` (US-004) to know the baseline snapshot per URL

### `.kaleidoscope/snapshots/` (owned by US-003)

```
.kaleidoscope/
  snapshots/
    index.json              # list of all snapshot metadata
    20260404-153012/
      snapshot.json         # per-URL audit + ax-tree data
      screenshots/
        dashboard.png
        login.png
  baselines.json            # committed — US-005 owns writes
  state.json                # gitignored — browser state
```

---

## File Layout

New files introduced by US-005:

| File | Purpose |
|------|---------|
| `snapshot/baseline.go` | `Baselines` type, `LoadBaselines`, `SaveBaselines`, `Accept` |
| `snapshot/baseline_test.go` | Unit tests for `Accept` (pure function — no I/O needed) |
| `cmd/accept.go` | `RunAccept` CLI handler |

Modified files:

| File | Change |
|------|--------|
| `main.go` | Add `case "accept"` to command switch |
| `cmd/usage.go` | Add `"accept"` entry to `CommandUsage` map |
| `cmd/util.go` | Add `"--url"` to value-taking flags in `getNonFlagArgs` |

---

## Security Considerations

- **Path traversal:** `--url` is stored verbatim in `baselines.json` as a URL path (e.g. `/dashboard`). It is never used to construct a filesystem path in US-005, so no traversal risk. US-003/US-004 are responsible for safe path construction when loading snapshot data.
- **File permissions:** `baselines.json` is written with `0644` (same as other state files). No secrets are stored.
- **JSON injection:** All output uses `encoding/json` marshaling; no string concatenation into JSON.
- **Snapshot ID validation:** The snapshot ID is looked up against the index; unrecognized IDs are rejected with a clear error before any file is written.

---

## Test Plan

### Unit tests — `snapshot/baseline_test.go`

All tests target the pure `Accept` function (no I/O required):

| Test case | Description |
|-----------|-------------|
| `TestAccept_AllURLs` | Accept latest snapshot for all URLs; all entries in `changed` |
| `TestAccept_SingleURL` | Accept with `urls=[]string{"/dashboard"}`; only that URL in `changed` |
| `TestAccept_AlreadyBaseline` | Accept same snapshot twice; `changed` is empty, `noOp` semantics |
| `TestAccept_UpdatesExisting` | Accept a newer snapshot over an older baseline; entry is updated |
| `TestAccept_URLNotInSnapshot` | Caller validates URL is in snapshot before calling `Accept`; tested in `cmd` layer |
| `TestAccept_EmptyBaselines` | First accept on fresh install; entries are created from scratch |

### Integration / CLI behavior tests

Covered by `go test ./...` via `cmd` package tests (if harness exists) or manual verification:

- `ks accept` with no snapshots → `ok: false`
- `ks accept` with snapshots → promotes latest, writes file
- `ks accept <id>` → promotes specific snapshot
- `ks accept --url /dashboard` → updates only that URL's entry
- `ks accept` run twice with same snapshot → `noOp: true`

Quality gate: `go test ./...` must pass.
