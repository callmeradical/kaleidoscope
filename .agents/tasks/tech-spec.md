# Tech Spec: US-005 — Baseline Manager (`ks accept`)

**PRD:** Snapshot History and Regression Detection
**Story:** US-005 — Baseline Manager
**Depends on:** US-003 (Snapshot System)
**Date:** 2026-04-03

---

## 1. Architecture Overview

US-005 introduces the `ks accept` command, which promotes a snapshot to the baseline used by the diff engine (`ks diff`, US-004). It operates entirely on local disk — reading from `.kaleidoscope/snapshots/` (produced by US-003) and writing to `.kaleidoscope/baselines.json` (shared in version control).

```
┌───────────────────────────────────────────────────────────────┐
│  CLI (main.go)                                                │
│    case "accept": cmd.RunAccept(args)                         │
└────────────────────────────┬──────────────────────────────────┘
                             │
┌────────────────────────────▼──────────────────────────────────┐
│  cmd/accept.go                                                │
│    RunAccept(args []string)                                   │
│    • parse flags: [snapshot-id], --url <path>                 │
│    • load snapshot index from snapshots/index.json            │
│    • resolve target snapshot (specific id or latest)          │
│    • load/create baselines.json                               │
│    • apply updates (all URLs or single URL)                   │
│    • write baselines.json                                     │
│    • output.Success / output.Fail                             │
└────────────────────────────┬──────────────────────────────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
┌─────────▼──────┐  ┌────────▼───────┐  ┌──────▼────────────┐
│ snapshot pkg   │  │ baseline pkg   │  │ output pkg        │
│ LoadIndex()    │  │ Load()         │  │ Success() / Fail() │
│ GetSnapshot()  │  │ Save()         │  └───────────────────┘
│ LatestFor()    │  │ Accept()       │
└────────────────┘  └───────────────┘
```

**Key files to create/modify:**

| File | Action | Purpose |
|------|---------|---------|
| `main.go` | modify | Add `"accept"` case to command switch |
| `cmd/accept.go` | create | `RunAccept` command handler |
| `snapshot/snapshot.go` | create (US-003) | Snapshot index loading & querying |
| `baseline/baseline.go` | create | Baseline CRUD operations |

> **Note:** The `snapshot` package is owned by US-003. This spec describes only the contract this feature relies on, not its implementation. The `baseline` package is owned by US-005.

---

## 2. Detailed Component Design

### 2.1 `snapshot` Package Contract (US-003 interface)

US-005 consumes the following from the snapshot package (to be delivered by US-003):

```go
package snapshot

// SnapshotEntry represents a single captured snapshot.
type SnapshotEntry struct {
    ID        string    `json:"id"`         // e.g. "20260403-153012-a1b2c3"
    CreatedAt time.Time `json:"created_at"`
    URLs      []URLEntry `json:"urls"`      // one per captured URL
}

type URLEntry struct {
    URL            string `json:"url"`           // full URL
    Path           string `json:"path"`          // URL path component, e.g. "/dashboard"
    ScreenshotPath string `json:"screenshot"`    // relative to .kaleidoscope/
    AuditPath      string `json:"audit"`         // relative to .kaleidoscope/
    AXTreePath     string `json:"axtree"`        // relative to .kaleidoscope/
}

// Index is the on-disk snapshot registry.
type Index struct {
    Snapshots []SnapshotEntry `json:"snapshots"`
}

// LoadIndex reads .kaleidoscope/snapshots/index.json from dir.
func LoadIndex(dir string) (*Index, error)

// Latest returns the most-recently-created snapshot, or nil if none.
func (idx *Index) Latest() *SnapshotEntry

// ByID returns the snapshot with the given ID, or nil if not found.
func (idx *Index) ByID(id string) *SnapshotEntry
```

### 2.2 `baseline` Package

**File:** `baseline/baseline.go`

```go
package baseline

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

// BaselineEntry records which snapshot is the accepted baseline for one URL.
type BaselineEntry struct {
    URL        string    `json:"url"`          // full URL
    Path       string    `json:"path"`         // URL path component
    SnapshotID string    `json:"snapshot_id"`
    AcceptedAt time.Time `json:"accepted_at"`
}

// Baselines is the in-memory representation of baselines.json.
type Baselines struct {
    Entries []BaselineEntry `json:"baselines"`
}

// Load reads baselines.json from dir; returns empty Baselines if file absent.
func Load(dir string) (*Baselines, error)

// Save writes b to baselines.json in dir (creates dir if needed).
func (b *Baselines) Save(dir string) error

// Accept updates or inserts the baseline entry for the given URLEntry.
// Returns true if a change was made, false if already up to date (no-op).
func (b *Baselines) Accept(urlEntry snapshot.URLEntry, snapshotID string) (changed bool)

// ForPath returns the baseline for the given URL path, or nil if none.
func (b *Baselines) ForPath(path string) *BaselineEntry
```

**Accept logic (idempotency):**

```go
func (b *Baselines) Accept(u snapshot.URLEntry, snapshotID string) bool {
    for i, e := range b.Entries {
        if e.Path == u.Path {
            if e.SnapshotID == snapshotID {
                return false // already baseline, no-op
            }
            b.Entries[i].SnapshotID = snapshotID
            b.Entries[i].AcceptedAt = time.Now().UTC()
            return true
        }
    }
    b.Entries = append(b.Entries, BaselineEntry{
        URL:        u.URL,
        Path:       u.Path,
        SnapshotID: snapshotID,
        AcceptedAt: time.Now().UTC(),
    })
    return true
}
```

### 2.3 `cmd/accept.go` — Command Handler

```go
package cmd

import (
    "fmt"
    "github.com/callmeradical/kaleidoscope/baseline"
    "github.com/callmeradical/kaleidoscope/output"
    "github.com/callmeradical/kaleidoscope/snapshot"
)

func RunAccept(args []string) {
    dir := kaleidoscopeDir()               // resolves .kaleidoscope/ path
    snapshotID := getArg(args)             // first positional arg (may be "")
    urlFilter := getFlagValue(args, "--url") // e.g. "/dashboard"

    // 1. Load snapshot index
    idx, err := snapshot.LoadIndex(dir)
    if err != nil || len(idx.Snapshots) == 0 {
        output.Fail("accept", fmt.Errorf("no snapshots found"), "run `ks snapshot` first")
        return
    }

    // 2. Resolve target snapshot
    var snap *snapshot.SnapshotEntry
    if snapshotID != "" {
        snap = idx.ByID(snapshotID)
        if snap == nil {
            output.Fail("accept", fmt.Errorf("snapshot %q not found", snapshotID), "")
            return
        }
    } else {
        snap = idx.Latest()
    }

    // 3. Load (or create) baselines
    b, err := baseline.Load(dir)
    if err != nil {
        output.Fail("accept", err, "")
        return
    }

    // 4. Apply updates
    var updated, skipped []string
    for _, u := range snap.URLs {
        if urlFilter != "" && u.Path != urlFilter {
            continue
        }
        if b.Accept(u, snap.ID) {
            updated = append(updated, u.Path)
        } else {
            skipped = append(skipped, u.Path)
        }
    }

    // 5. If --url filter matched nothing
    if urlFilter != "" && len(updated)+len(skipped) == 0 {
        output.Fail("accept", fmt.Errorf("no URL with path %q in snapshot %s", urlFilter, snap.ID), "")
        return
    }

    // 6. Persist
    if err := b.Save(dir); err != nil {
        output.Fail("accept", err, "")
        return
    }

    // 7. Return result
    output.Success("accept", map[string]any{
        "snapshot_id": snap.ID,
        "updated":     updated,
        "skipped":     skipped,
    })
}
```

### 2.4 `main.go` Change

Add to the command switch:

```go
case "accept":
    cmd.RunAccept(args)
```

---

## 3. API Definitions

### CLI Interface

```
ks accept [snapshot-id] [--url <path>]
```

| Argument | Type | Description |
|----------|------|-------------|
| `snapshot-id` | positional, optional | ID of snapshot to accept; defaults to latest |
| `--url <path>` | flag, optional | URL path (e.g. `/dashboard`) to restrict acceptance |

### JSON Output — Success

```json
{
  "ok": true,
  "command": "accept",
  "result": {
    "snapshot_id": "20260403-153012-a1b2c3",
    "updated": ["/", "/dashboard"],
    "skipped": []
  }
}
```

`updated`: paths whose baseline was changed.
`skipped`: paths already at this snapshot (idempotent no-op).

### JSON Output — Errors

```json
{ "ok": false, "command": "accept", "error": "no snapshots found", "hint": "run `ks snapshot` first" }
{ "ok": false, "command": "accept", "error": "snapshot \"abc\" not found" }
{ "ok": false, "command": "accept", "error": "no URL with path \"/foo\" in snapshot 20260403-153012-a1b2c3" }
```

---

## 4. Data Model

### `.kaleidoscope/baselines.json`

Location: `<project-root>/.kaleidoscope/baselines.json`
Committed to version control (shared baseline).

```json
{
  "baselines": [
    {
      "url": "http://localhost:3000/",
      "path": "/",
      "snapshot_id": "20260403-153012-a1b2c3",
      "accepted_at": "2026-04-03T15:30:12Z"
    },
    {
      "url": "http://localhost:3000/dashboard",
      "path": "/dashboard",
      "snapshot_id": "20260403-153012-a1b2c3",
      "accepted_at": "2026-04-03T15:30:12Z"
    }
  ]
}
```

**Field semantics:**

| Field | Type | Notes |
|-------|------|-------|
| `url` | string | Full URL (for display/disambiguation) |
| `path` | string | URL path; used as the lookup key |
| `snapshot_id` | string | References `SnapshotEntry.ID` in index |
| `accepted_at` | RFC 3339 timestamp | When the accept was performed |

**Invariants:**
- `path` values are unique within the array (no duplicate paths).
- `snapshot_id` always references an existing snapshot in `snapshots/index.json`.
- File is valid JSON; missing file is treated as `{ "baselines": [] }`.

### `.kaleidoscope/snapshots/index.json` (US-003 owned)

US-005 reads but never writes this file. Schema defined in §2.1.

### Directory Layout

```
.kaleidoscope/
├── state.json              # browser state (existing)
├── baselines.json          # NEW — committed to git
├── snapshots/              # NEW (US-003) — gitignored
│   ├── index.json
│   └── 20260403-153012-a1b2c3/
│       ├── screenshot-root.png
│       ├── audit-root.json
│       └── axtree-root.json
└── screenshots/            # existing ephemeral screenshots
```

---

## 5. Security Considerations

### Path Traversal
- `snapshotID` (from CLI arg) must be validated to contain only alphanumeric characters, hyphens, and underscores before being used to construct any file path.
- Pattern: `regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)`
- Reject and return an error if it doesn't match.

### File Write Safety
- `baseline.Save()` writes atomically: write to a temp file in the same directory, then `os.Rename()` to `baselines.json`. This prevents corrupt state if the process is interrupted.
- Directory is created with `os.MkdirAll(dir, 0755)` — no world-write permissions.

### Input Validation
- `--url` filter is a path string used only for string comparison, not as a file path — no path traversal risk.
- Snapshot index is loaded from a controlled local directory, not from user-supplied paths.

### No Network Calls
- `ks accept` is pure disk I/O. No browser, no network.

---

## 6. Test Plan (Quality Gate: `go test ./...`)

Tests live in `baseline/baseline_test.go` and `cmd/accept_test.go`.

### Unit Tests — `baseline` Package

| Test | Scenario |
|------|----------|
| `TestLoad_Missing` | Load from dir with no baselines.json → empty Baselines, no error |
| `TestLoad_Valid` | Load well-formed baselines.json → correct entries |
| `TestAccept_Insert` | Accept URL not previously in baselines → added, returns true |
| `TestAccept_Update` | Accept URL with different snapshot ID → updated, returns true |
| `TestAccept_NoOp` | Accept URL already at same snapshot ID → unchanged, returns false |
| `TestSave_Atomic` | Save writes temp file then renames; final file is valid JSON |

### Integration Tests — `RunAccept`

| Test | Scenario |
|------|----------|
| `TestRunAccept_NoSnapshots` | Empty index → Fail with "no snapshots found" |
| `TestRunAccept_Latest` | No args → promotes latest snapshot for all URLs |
| `TestRunAccept_ByID` | Valid snapshot-id arg → promotes that specific snapshot |
| `TestRunAccept_ByID_NotFound` | Unknown snapshot-id → Fail with "not found" |
| `TestRunAccept_URLFilter` | `--url /dashboard` → only that path updated |
| `TestRunAccept_URLFilter_NoMatch` | `--url /nonexistent` → Fail with "no URL with path" |
| `TestRunAccept_Idempotent` | Accept same snapshot twice → second call has empty updated, all in skipped |
| `TestRunAccept_InvalidID` | Snapshot ID with `../` traversal → Fail with validation error |

---

## 7. Open Questions & Decisions

| Question | Decision |
|----------|----------|
| Should `baselines.json` store the full URL or just the path as the key? | Store both; use **path** as the lookup key to handle host/port differences across environments |
| What if `--url` is passed with a full URL instead of a path? | Strip scheme+host before comparing (or error with a clear hint) — defer to US-003 convention |
| Should `ks accept` print a warning if it's a no-op (all skipped)? | Yes: include `skipped` in the JSON result; callers can detect all-skipped and warn |
| Should `snapshot_id` validation be in `cmd/accept.go` or `snapshot` package? | In `cmd/accept.go` — keep the snapshot package free of CLI concerns |
