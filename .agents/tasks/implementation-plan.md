# Implementation Plan: Baseline Manager (US-005)

## Overview

Implements `ks accept [snapshot-id] [--url <path>]`, which promotes a snapshot to baseline by updating `.kaleidoscope/baselines.json`. Depends on US-003 (snapshot package).

**Dependency note:** The `snapshot` package must exist before this feature can compile. If US-003 is not yet merged, a minimal stub satisfying the interface below must be provided before Phase 2 can begin.

**Required snapshot interface (from US-003):**
```go
package snapshot

type Snapshot struct {
    ID        string        `json:"id"`
    CreatedAt time.Time     `json:"createdAt"`
    URLs      []SnapshotURL `json:"urls"`
}

type SnapshotURL struct {
    URL            string `json:"url"`
    Path           string `json:"path"`
    ScreenshotPath string `json:"screenshotPath"`
    AuditResult    any    `json:"auditResult"`
    AXTree         any    `json:"axTree"`
}

type Store struct { /* ... */ }

func OpenStore(dir string) (*Store, error)
func (s *Store) Latest() (*Snapshot, error)
func (s *Store) ByID(id string) (*Snapshot, error)
```

---

## Phase 1: `baseline` Package

Create the new `baseline/` package — pure file I/O, no browser/Chrome dependency.

### Task 1.1: Create `baseline/manager.go`

**File:** `baseline/manager.go`
**Package:** `package baseline`

#### Sub-tasks

1. **Define `Entry` struct**
   - Fields: `SnapshotID string` (`json:"snapshotId"`), `AcceptedAt time.Time` (`json:"acceptedAt"`)

2. **Define `BaselinesFile` struct**
   - Fields: `Version int` (`json:"version"`), `Updated time.Time` (`json:"updated"`), `Baselines map[string]Entry` (`json:"baselines"`)
   - Key of `Baselines` map is URL path (e.g. `"/dashboard"`), not full URL

3. **Define `Manager` struct**
   - Single unexported field: `path string` (absolute path to `baselines.json`)

4. **Implement `NewManager(kaleidoscopeDir string) *Manager`**
   - Sets `path` to `filepath.Join(kaleidoscopeDir, "baselines.json")`

5. **Implement `Load() (*BaselinesFile, error)`**
   - If file does not exist (`os.IsNotExist`), return `&BaselinesFile{Version: 1, Baselines: map[string]Entry{}}` with nil error (not an error condition)
   - Otherwise read, `json.Unmarshal`, and return

6. **Implement `Save(f *BaselinesFile) error`**
   - Use atomic write: write to `baselines.json.tmp` in same directory, then `os.Rename` to final path
   - Write with file mode `0644`
   - Use `json.MarshalIndent` for human-readable output

7. **Implement `Accept(snapshotID string, paths []string) (*BaselinesFile, error)`**
   - Call `Load()` to get current state (or empty baseline if file missing)
   - For each path in `paths`:
     - If `baselines[path].SnapshotID == snapshotID` → skip (idempotent no-op)
     - Otherwise set `baselines[path] = Entry{SnapshotID: snapshotID, AcceptedAt: time.Now().UTC()}`
   - Set `f.Updated = time.Now().UTC()`
   - Call `Save(f)`
   - Return updated `*BaselinesFile`

---

### Task 1.2: Create `baseline/manager_test.go`

**File:** `baseline/manager_test.go`
**Package:** `package baseline`

Uses `t.TempDir()` for isolated file I/O in each test.

#### Sub-tasks

1. **Test: Accept on empty store creates file with correct entry**
   - Call `Accept("snap-001", []string{"/"})`
   - Assert `baselines["/"].SnapshotID == "snap-001"`
   - Assert file now exists on disk

2. **Test: Accept same snapshot ID twice is idempotent**
   - Call `Accept("snap-001", []string{"/"})` twice
   - Assert `AcceptedAt` timestamp is the same after both calls (file unchanged on second call)
   - Assert `baselines` map has exactly one entry

3. **Test: Accept with single path only updates that path, preserves others**
   - Pre-populate `baselines.json` with entries for `"/"` and `"/dashboard"`
   - Call `Accept("snap-002", []string{"/dashboard"})`
   - Assert `baselines["/"].SnapshotID` is still the old snapshot ID (unchanged)
   - Assert `baselines["/dashboard"].SnapshotID == "snap-002"`

4. **Test: Accept writes correct `updated` timestamp**
   - Record time before call, call `Accept`, record time after
   - Assert `f.Updated` is between before and after timestamps

5. **Test: Load on missing file returns empty BaselinesFile (not error)**
   - Point `Manager` at a nonexistent path
   - Call `Load()`
   - Assert error is nil
   - Assert `len(f.Baselines) == 0` and `f.Version == 1`

6. **Test: Atomic write — temp file cleaned up on success**
   - Call `Save` with valid data
   - Assert `.tmp` file does not exist after completion
   - Assert final file exists and is valid JSON

---

## Phase 2: Command Layer

### Task 2.1: Update `cmd/util.go`

**File:** `cmd/util.go` (existing)

#### Sub-tasks

1. **Add `"--url"` to the flag-value skip list in `getNonFlagArgs`**
   - In the condition that checks for flags taking a value argument, add:
     ```go
     a == "--url" || ...
     ```
   - This ensures `--url /dashboard` does not treat `/dashboard` as a positional argument

---

### Task 2.2: Create `cmd/accept.go`

**File:** `cmd/accept.go`
**Package:** `package cmd`

#### Sub-tasks

1. **Define `RunAccept(args []string)` function**
   - Export signature matching all other `Run*` commands in the `cmd` package

2. **Parse arguments**
   - `snapshotID := getArg(args)` — first non-flag argument (optional: specific snapshot ID)
   - `urlFilter := getFlagValue(args, "--url")` — optional URL path filter

3. **Resolve `.kaleidoscope` directory**
   - Call `browser.StateDir()` to get the kaleidoscope state directory
   - On error: `output.Fail("accept", err, "")` + `os.Exit(2)`

4. **Open snapshot store**
   - Call `snapshot.OpenStore(filepath.Join(ksDir, "snapshots"))`
   - On error: `output.Fail("accept", err, "")` + `os.Exit(2)`

5. **Resolve target snapshot**
   - If `snapshotID != ""`: call `store.ByID(snapshotID)`, error message: `"snapshot not found: <id>"`
   - If `snapshotID == ""`: call `store.Latest()`, error message: `"no snapshots found"`, hint: `"Run: ks snapshot"`
   - On either error: `output.Fail("accept", err, hint)` + `os.Exit(2)`

6. **Determine URL paths to update**
   - If `urlFilter != ""`:
     - Validate that `snap.URLs` contains a `SnapshotURL` with `.Path == urlFilter`
     - If not found: `output.Fail("accept", errors.New("snapshot does not contain URL path: "+urlFilter), "")` + `os.Exit(2)`
     - `paths = []string{urlFilter}`
   - If `urlFilter == ""`:
     - Collect all `.Path` values from `snap.URLs` into `paths`

7. **Validate `--url` starts with `/`**
   - If `urlFilter != ""` and does not start with `"/"`: `output.Fail` with descriptive error

8. **Call baseline manager**
   - `mgr := baseline.NewManager(ksDir)`
   - `updated, err := mgr.Accept(snap.ID, paths)`
   - On error: `output.Fail("accept", err, "")` + `os.Exit(2)`

9. **Emit success result**
   ```go
   output.Success("accept", map[string]any{
       "snapshotId": snap.ID,
       "paths":      paths,
       "baselines":  updated.Baselines,
   })
   ```

10. **Handle all error paths consistently**
    - Every error terminates via `output.Fail("accept", err, hint)` followed by `os.Exit(2)`
    - Matches the pattern used by every other command in the `cmd` package

---

### Task 2.3: Create `cmd/accept_test.go`

**File:** `cmd/accept_test.go`
**Package:** `package cmd`

Integration tests that exercise `RunAccept` end-to-end with a real (temp-dir-backed) snapshot store and baselines file.

#### Sub-tasks

1. **Test: No snapshots exist → error output**
   - Set up empty snapshot store directory
   - Assert `output.Fail` result with `ok: false` and message containing `"no snapshots found"`
   - Assert process would exit with code 2

2. **Test: Latest snapshot accepted → all URL paths in baselines**
   - Pre-populate snapshot store with one snapshot containing multiple URLs
   - Call `RunAccept([]string{})` (no args)
   - Assert result `paths` length equals number of URLs in snapshot
   - Assert `baselines.json` is written correctly

3. **Test: Specific snapshot ID accepted**
   - Pre-populate snapshot store with two snapshots
   - Call `RunAccept([]string{"<older-snapshot-id>"})`
   - Assert `result.snapshotId == <older-snapshot-id>` (not latest)

4. **Test: Unknown snapshot ID → error**
   - Call `RunAccept([]string{"nonexistent-id"})`
   - Assert `ok: false` in output

5. **Test: `--url` path not in snapshot → error**
   - Pre-populate snapshot store with snapshot for `/` only
   - Call `RunAccept([]string{"--url", "/missing"})`
   - Assert `ok: false` with message containing `"snapshot does not contain URL path"`

6. **Test: `--url` filters to single path, others unchanged**
   - Pre-populate baselines with existing entry for `"/"`
   - Snapshot has paths `"/"` and `"/dashboard"`
   - Call `RunAccept([]string{"--url", "/dashboard"})`
   - Assert only `"/dashboard"` is updated; `"/"` baseline entry is preserved

---

## Phase 3: Wire Up Existing Files

### Task 3.1: Update `main.go`

**File:** `main.go` (existing)

#### Sub-tasks

1. **Add `"accept"` case to the command switch**
   ```go
   case "accept":
       cmd.RunAccept(cmdArgs)
   ```
   - Insert after existing cases, before `default`

2. **Add `"accept"` to the usage string**
   - Add under a new `"Snapshot Management:"` section or alongside future snapshot commands:
     ```
   accept [snapshot-id] [--url <path>]   Accept snapshot as new baseline
     ```

---

### Task 3.2: Update `cmd/usage.go`

**File:** `cmd/usage.go` (existing)

#### Sub-tasks

1. **Add `"accept"` entry to the `CommandUsage` map**
   ```
   ks accept [snapshot-id] [--url <path>]

   Promote a snapshot to baseline for future diff comparisons.

   Arguments:
     snapshot-id    ID of the snapshot to accept (default: latest)

   Options:
     --url <path>   Accept baseline for a single URL path only (e.g. /dashboard)

   Forms:
     ks accept                              Latest snapshot, all URLs
     ks accept <snapshot-id>                Specific snapshot, all URLs
     ks accept --url /dashboard             Latest snapshot, /dashboard only
     ks accept <snapshot-id> --url /path    Specific snapshot, one path

   Output:
     { "ok": true, "command": "accept", "result": {
       "snapshotId": "20260404T123000Z",
       "paths": ["/", "/dashboard"],
       "baselines": {
         "/": { "snapshotId": "...", "acceptedAt": "..." },
         "/dashboard": { "snapshotId": "...", "acceptedAt": "..." }
       }
     }}

   Error cases:
     No snapshots exist           → "no snapshots found" (hint: Run: ks snapshot)
     Snapshot ID not found        → "snapshot not found: <id>"
     --url path not in snapshot   → "snapshot does not contain URL path: <path>"

   Notes:
     Calling accept twice with the same snapshot is a no-op (idempotent).
     baselines.json is committed to the repo; snapshots/ is gitignored.
     --url updates only the specified path; all other baselines are preserved.
   ```

---

## Phase 4: Quality Gate

### Task 4.1: Run test suite

#### Sub-tasks

1. **Run `go build ./...`**
   - Verify the project compiles with no errors after all files are added

2. **Run `go test ./baseline/...`**
   - All unit tests in the `baseline` package pass

3. **Run `go test ./cmd/...`**
   - All integration tests for `cmd/accept` pass

4. **Run `go test ./...`**
   - Full test suite passes (quality gate per project rules)

---

## File Summary

| File | Action | Purpose |
|------|--------|---------|
| `baseline/manager.go` | Create | `Manager`, `BaselinesFile`, `Entry`, `Accept`, `Load`, `Save` |
| `baseline/manager_test.go` | Create | Unit tests for baseline package |
| `cmd/accept.go` | Create | `RunAccept` command handler |
| `cmd/accept_test.go` | Create | Integration tests for accept command |
| `cmd/util.go` | Edit | Add `"--url"` to flag-value skip list in `getNonFlagArgs` |
| `cmd/usage.go` | Edit | Add `"accept"` entry to `CommandUsage` map |
| `main.go` | Edit | Add `case "accept"` to switch; add to usage string |

## Data Flow

```
ks accept [snapshot-id] [--url /path]
    │
    ├── browser.StateDir()
    │       └── resolves .kaleidoscope/ dir
    │
    ├── snapshot.OpenStore(.kaleidoscope/snapshots/)
    │       └── store.Latest() or store.ByID(id)
    │               └── returns *Snapshot with .ID and .URLs[].Path
    │
    ├── filter paths (all URLs or --url match)
    │       └── validate --url path exists in snapshot
    │
    ├── baseline.NewManager(.kaleidoscope/)
    │       └── mgr.Accept(snap.ID, paths)
    │               ├── Load .kaleidoscope/baselines.json (or empty)
    │               ├── For each path: set Entry{SnapshotID, AcceptedAt} if changed
    │               └── Save atomically (tmp → rename)
    │
    └── output.Success("accept", {snapshotId, paths, baselines})
```

## Error Handling Reference

| Condition | Error message | Hint |
|-----------|---------------|------|
| No snapshots in store | `"no snapshots found"` | `"Run: ks snapshot"` |
| Snapshot ID not found | `"snapshot not found: <id>"` | `""` |
| `--url` path not in snapshot | `"snapshot does not contain URL path: <path>"` | `""` |
| `--url` does not start with `/` | `"url path must start with /: <value>"` | `""` |
| Cannot write `baselines.json` | underlying OS error message | `""` |
| `StateDir` fails | underlying OS error message | `""` |
