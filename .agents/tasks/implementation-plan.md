# Implementation Plan: Baseline Manager (US-005)

## Overview

Implements `ks accept [snapshot-id]` — a Chrome-free command that promotes a
snapshot to baseline by updating `.kaleidoscope/baselines.json`. Depends on
US-003 snapshot types; since the `snapshot/` package does not yet exist,
this plan creates the full package from scratch with compatible stubs.

---

## Phase 1 — `snapshot` Package Foundation

> Creates the shared types and store interface consumed by both US-003 and this
> story. No I/O logic yet — only types and function signatures backed by
> filesystem reads.

### Task 1.1 — Create `snapshot/types.go`

**Sub-tasks:**
1. Create `/workspace/snapshot/` directory (new package).
2. Declare `package snapshot`.
3. Define `Snapshot` struct with JSON tags:
   - `ID string` — e.g. `"20260404-153012-a1b2c3"`
   - `URL string` — full URL captured
   - `URLPath string` — path component only, e.g. `/dashboard`
   - `CreatedAt time.Time`
   - `ScreenshotPath string` (omitempty)
   - `AuditResult any` (omitempty)
   - `AXTree any` (omitempty)
4. Define `Baselines` type as `map[string]string` (URLPath → Snapshot.ID).
5. Add imports: `"time"`.

### Task 1.2 — Create `snapshot/store.go`

**Sub-tasks:**
1. Declare `package snapshot`.
2. Import: `"encoding/json"`, `"fmt"`, `"os"`, `"path/filepath"`,
   `"sort"`, `"strings"`, `"github.com/callmeradical/kaleidoscope/browser"`.
3. Implement helper `snapshotsDir() (string, error)`:
   - Calls `browser.StateDir()`.
   - Returns `filepath.Join(dir, "snapshots")`.
4. Implement `ListSnapshots() ([]Snapshot, error)`:
   - Read all subdirectory entries from `snapshotsDir()`.
   - For each entry, read `meta.json` and unmarshal into `Snapshot`.
   - Sort by `CreatedAt` ascending (oldest first).
   - Return empty slice (not error) when directory does not exist.
5. Implement `LatestSnapshot() (*Snapshot, error)`:
   - Call `ListSnapshots()`.
   - Return error `"no snapshots found"` when slice is empty.
   - Return last element (most recent).
6. Implement `LatestSnapshotForURL(urlPath string) (*Snapshot, error)`:
   - Call `ListSnapshots()`.
   - Filter to entries where `Snapshot.URLPath == urlPath`.
   - Return error `"no snapshots found for URL: <urlPath>"` when none match.
   - Return last filtered element.
7. Implement `GetSnapshot(id string) (*Snapshot, error)`:
   - **Security:** Validate `id` against pattern `^[a-zA-Z0-9_-]+$`; return
     error `"invalid snapshot ID"` if it contains `/`, `..`, or disallowed chars.
   - Build path: `filepath.Join(snapshotsDir(), id, "meta.json")`.
   - Read and unmarshal; return descriptive error if not found.

---

## Phase 2 — `snapshot/baseline.go`

> Pure-function baseline persistence module. No Chrome dependency.

### Task 2.1 — Implement `baselinesPath()` (unexported helper)

**Sub-tasks:**
1. Declare `package snapshot`.
2. Import: `"encoding/json"`, `"os"`, `"path/filepath"`,
   `"github.com/callmeradical/kaleidoscope/browser"`.
3. Call `browser.StateDir()` and join with `"baselines.json"`.

### Task 2.2 — Implement `ReadBaselines() (Baselines, error)`

**Sub-tasks:**
1. Call `baselinesPath()`.
2. Attempt `os.ReadFile(path)`.
3. If `os.IsNotExist(err)`: return empty `Baselines{}` map and `nil` error.
4. On other error: return the error.
5. Unmarshal JSON bytes into `Baselines` and return.

### Task 2.3 — Implement `WriteBaselines(b Baselines) error`

**Sub-tasks:**
1. Call `baselinesPath()`.
2. Ensure parent directory exists with `os.MkdirAll(dir, 0755)`.
3. Marshal `b` to indented JSON (`json.MarshalIndent` with 2-space indent).
4. Write to a temp file in the same directory (`os.CreateTemp(dir, "baselines-*.json.tmp")`).
5. Write marshalled bytes to temp file; close it.
6. Set file permissions to `0644` via `os.Chmod`.
7. Atomically rename temp file to final `baselines.json` path using `os.Rename`.
8. Return any error encountered.

### Task 2.4 — Implement `AcceptSnapshot(current Baselines, snap *Snapshot, urlPath string) (Baselines, bool, error)`

**Sub-tasks:**
1. Make a copy of `current` into `updated` (iterate and copy each key/value).
2. Branch on `urlPath`:
   - `urlPath == "*"`:
     - For each existing key in `updated`, set value to `snap.ID`.
     - Also set `updated[snap.URLPath] = snap.ID` (ensure snap's own path covered).
   - Otherwise:
     - Compute `targetPath`: if `urlPath != ""` use `urlPath`, else use `snap.URLPath`.
     - Normalize `targetPath`: ensure it starts with `/`, trim trailing `/` (except root).
     - Set `updated[targetPath] = snap.ID`.
3. Determine `wasNoOp`: compare `updated` map to `current` map key-by-key
   (same length + same values for all keys).
4. Return `(updated, wasNoOp, nil)`.

---

## Phase 3 — `cmd/accept.go`

> CLI entry point. Parses args, delegates to `snapshot` package, emits JSON.

### Task 3.1 — Create `cmd/accept.go`

**Sub-tasks:**
1. Declare `package cmd`.
2. Import: `"os"`, `"github.com/callmeradical/kaleidoscope/output"`,
   `"github.com/callmeradical/kaleidoscope/snapshot"`.
3. Define `func RunAccept(args []string)`.

### Task 3.2 — Implement snapshot resolution logic

**Sub-tasks:**
1. Extract `snapshotID := getArg(args)` (optional positional arg).
2. Extract `urlPath := getFlagValue(args, "--url")`.
3. Resolve target snapshot:
   - If `snapshotID != ""`: call `snapshot.GetSnapshot(snapshotID)`.
   - Else: call `snapshot.LatestSnapshot()`.
4. On error: call `output.Fail("accept", err, "Run 'ks snapshot' to create a snapshot first.")` and `os.Exit(2)`.

### Task 3.3 — Implement baseline update logic

**Sub-tasks:**
1. Call `snapshot.ReadBaselines()`. On error: `output.Fail` + `os.Exit(2)`.
2. Determine scope:
   - If `urlPath == ""`: scope = `"*"` (all URLs, as per spec).
   - Else: scope = `urlPath` (single URL).
3. Call `snapshot.AcceptSnapshot(current, snap, scope)`. On error: `output.Fail` + `os.Exit(2)`.
4. If `!wasNoOp`: call `snapshot.WriteBaselines(updated)`. On error: `output.Fail` + `os.Exit(2)`.

### Task 3.4 — Emit success result

**Sub-tasks:**
1. Call `output.Success("accept", map[string]any{...})` with fields:
   - `"snapshotId"`: `snap.ID`
   - `"noOp"`: `wasNoOp`
   - `"updated"`: `updated` (full baselines map after operation)
   - `"url"`: `urlPath` (empty string means all URLs)

---

## Phase 4 — Wire into CLI

### Task 4.1 — Update `main.go`

**Sub-tasks:**
1. Add `case "accept": cmd.RunAccept(cmdArgs)` to the command switch (after `"catalog-repo"` or similar).
2. Add usage line under the "Snapshot History" section of the help text:
   ```
     accept [snapshot-id]    Promote snapshot to baseline (--url <path> for single URL)
   ```
   (locate the usage/help string — likely in `cmd/usage.go` — and add the entry there).

### Task 4.2 — Update `cmd/util.go`

**Sub-tasks:**
1. Read `getNonFlagArgs` function to find the existing flag-with-value list.
2. Add `"--url"` to the condition that sets `skip = true` so it does not get
   treated as a positional argument. The condition currently covers:
   `--selector`, `--output`, `--depth`, `--width`, `--height`, `--format`,
   `--quality`, `--wait-until`, `--min-size`, `--kind`, `--ref`.
3. Confirm `getFlagValue` already handles `--url` generically (it does; no change needed there).

### Task 4.3 — Update `cmd/usage.go`

**Sub-tasks:**
1. Locate the usage/help string in `cmd/usage.go` (or wherever the help text lives).
2. Add the `accept` entry in the appropriate section.

---

## Phase 5 — Tests

> All tests must pass via `go test ./...`. Unit tests are pure-function and
> require no Chrome. Integration test stubs the snapshot store.

### Task 5.1 — Create `snapshot/baseline_test.go`

**Sub-tasks:**

1. **`TestReadBaselines_FileNotExist`**
   - Point `baselinesPath()` at a non-existent temp dir path.
   - Assert `ReadBaselines()` returns empty map and `nil` error.
   *(Use env-var override or test helper to redirect state dir in tests.)*

2. **`TestWriteAndReadBaselines_RoundTrip`**
   - Create a temp directory.
   - Write a sample `Baselines{"/": "snap-001", "/dashboard": "snap-002"}`.
   - Read back and assert equality.
   - Verify `baselines.json` exists with mode `0644`.

3. **`TestAcceptSnapshot_AllURLs`**
   - `current = Baselines{"/": "old-id", "/dashboard": "old-id"}`.
   - `snap = &Snapshot{ID: "new-id", URLPath: "/"}`.
   - Call `AcceptSnapshot(current, snap, "*")`.
   - Assert all keys in `updated` have value `"new-id"`.
   - Assert `wasNoOp == false`.

4. **`TestAcceptSnapshot_SpecificURL`**
   - `current = Baselines{"/": "old-id", "/dashboard": "old-id"}`.
   - `snap = &Snapshot{ID: "new-id", URLPath: "/dashboard"}`.
   - Call `AcceptSnapshot(current, snap, "/dashboard")`.
   - Assert `updated["/dashboard"] == "new-id"`.
   - Assert `updated["/"] == "old-id"` (unchanged).
   - Assert `wasNoOp == false`.

5. **`TestAcceptSnapshot_Idempotent`**
   - `current = Baselines{"/": "same-id"}`.
   - `snap = &Snapshot{ID: "same-id", URLPath: "/"}`.
   - Call `AcceptSnapshot(current, snap, "*")`.
   - Assert `wasNoOp == true`.

6. **`TestAcceptSnapshot_EmptyCurrentBaselines`**
   - `current = Baselines{}` (no existing entries).
   - `snap = &Snapshot{ID: "snap-001", URLPath: "/new"}`.
   - Call `AcceptSnapshot(current, snap, "*")`.
   - Assert `updated["/new"] == "snap-001"`.
   - Assert `wasNoOp == false`.

7. **`TestAcceptSnapshot_NoURLFlag_UsesSnapURLPath`**
   - `current = Baselines{}`.
   - `snap = &Snapshot{ID: "snap-001", URLPath: "/settings"}`.
   - Call `AcceptSnapshot(current, snap, "")` (empty urlPath = snap's own path).
   - Assert `updated["/settings"] == "snap-001"`.

### Task 5.2 — Create `snapshot/store_test.go`

**Sub-tasks:**

1. **`TestGetSnapshot_InvalidID_PathTraversal`**
   - Call `GetSnapshot("../evil")`.
   - Assert error contains `"invalid snapshot ID"`.

2. **`TestGetSnapshot_InvalidID_WithSlash`**
   - Call `GetSnapshot("foo/bar")`.
   - Assert error contains `"invalid snapshot ID"`.

3. **`TestListSnapshots_EmptyDir`**
   - Point state dir at a temp dir with no `snapshots/` subdirectory.
   - Assert `ListSnapshots()` returns empty slice and `nil` error.

4. **`TestLatestSnapshot_NoSnapshots`**
   - Empty store (temp dir).
   - Assert `LatestSnapshot()` returns `nil` snap and non-nil error with message `"no snapshots found"`.

5. **`TestListSnapshots_SortedByCreatedAt`**
   - Write 3 `meta.json` files with different `createdAt` timestamps to a temp dir.
   - Assert `ListSnapshots()` returns them in ascending order.

### Task 5.3 — Integration test for `RunAccept`

**Sub-tasks:**
1. Create `cmd/accept_test.go`.
2. **`TestRunAccept_NoSnapshots_Exits2`**:
   - Point state dir at an empty temp directory.
   - Capture stdout/stderr output.
   - Call `RunAccept([]string{})` using `os/exec` or output-capture helper.
   - Assert exit code 2.
   - Assert JSON output has `"ok": false` and `"hint"` contains `"ks snapshot"`.
3. **`TestRunAccept_LatestSnapshot_AllURLs`**:
   - Pre-populate temp snapshot dir with one `meta.json`.
   - Pre-populate `baselines.json` with one entry.
   - Call `RunAccept([]string{})`.
   - Assert `"ok": true`, `"noOp": false`, `updated` map has expected values.
4. **`TestRunAccept_AlreadyBaseline_NoOp`**:
   - Same snapshot ID already in `baselines.json`.
   - Call `RunAccept([]string{})`.
   - Assert `"noOp": true` and `baselines.json` is unchanged (same mtime or content).

---

## File Summary

| File | Action | Phase |
|---|---|---|
| `snapshot/types.go` | Create | 1.1 |
| `snapshot/store.go` | Create | 1.2 |
| `snapshot/baseline.go` | Create | 2.1–2.4 |
| `cmd/accept.go` | Create | 3.1–3.4 |
| `main.go` | Modify — add `"accept"` case | 4.1 |
| `cmd/util.go` | Modify — add `--url` to value-consuming flags | 4.2 |
| `cmd/usage.go` | Modify — add `accept` help entry | 4.3 |
| `snapshot/baseline_test.go` | Create | 5.1 |
| `snapshot/store_test.go` | Create | 5.2 |
| `cmd/accept_test.go` | Create | 5.3 |

---

## Dependency Order

```
Phase 1 (types + store) → Phase 2 (baseline.go) → Phase 3 (accept.go) → Phase 4 (wire CLI)
                                                                        → Phase 5 (tests, can run in parallel with Phase 4)
```

All phases within a story are sequential. Tests in Phase 5 require Phases 1–3
to be complete.
