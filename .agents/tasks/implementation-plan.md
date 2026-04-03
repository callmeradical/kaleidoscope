# Implementation Plan: US-005 — Baseline Manager (`ks accept`)

**Story:** US-005 — Baseline Manager
**Depends on:** US-003 (Snapshot System — `snapshot` package contract)
**Quality Gate:** `go test ./...`

---

## Phase 1: Snapshot Package Stub (US-003 Contract)

> The `snapshot` package is owned by US-003. US-005 requires it to compile. If US-003 is not yet merged, a minimal stub must be created that satisfies the interface contract. If US-003 is already present, skip this phase.

### Task 1.1 — Check for existing `snapshot` package
- Sub-task: Run `ls /workspace/snapshot/` to determine if package exists
- Sub-task: If present, verify `LoadIndex`, `Index.Latest`, `Index.ByID`, `SnapshotEntry`, `URLEntry` are exported — skip Phase 1 if all present

### Task 1.2 — Create `snapshot/snapshot.go` stub
- Sub-task: Create directory `/workspace/snapshot/`
- Sub-task: Define `package snapshot` with `SnapshotEntry`, `URLEntry`, `Index` structs matching the spec (§2.1 of tech spec)
- Sub-task: Implement `LoadIndex(dir string) (*Index, error)` — reads `.kaleidoscope/snapshots/index.json` via `os.ReadFile` + `json.Unmarshal`; returns `&Index{}` (empty, no error) if file is absent (`os.IsNotExist`)
- Sub-task: Implement `(idx *Index) Latest() *SnapshotEntry` — returns last element of `idx.Snapshots`, or nil if empty
- Sub-task: Implement `(idx *Index) ByID(id string) *SnapshotEntry` — linear scan by `ID` field, returns nil if not found

---

## Phase 2: `baseline` Package

### Task 2.1 — Create `baseline/baseline.go`
- Sub-task: Create directory `/workspace/baseline/`
- Sub-task: Define `package baseline` with `BaselineEntry` struct: `URL`, `Path`, `SnapshotID`, `AcceptedAt` fields with JSON tags
- Sub-task: Define `Baselines` struct: `Entries []BaselineEntry` with JSON tag `"baselines"`
- Sub-task: Implement `Load(dir string) (*Baselines, error)`:
  - Construct path: `filepath.Join(dir, "baselines.json")`
  - Read file; if `os.IsNotExist`, return `&Baselines{}` with nil error
  - `json.Unmarshal` into `Baselines`; return on parse error
- Sub-task: Implement `(b *Baselines) Save(dir string) error`:
  - `os.MkdirAll(dir, 0755)` to ensure directory exists
  - `json.Marshal` the `Baselines` struct (with indent via `json.MarshalIndent` for readability)
  - Write to a temp file in same directory: `os.CreateTemp(dir, "baselines-*.json.tmp")`
  - Write marshalled bytes, `Close()` the temp file
  - `os.Rename(tmpPath, filepath.Join(dir, "baselines.json"))` (atomic replace)
- Sub-task: Implement `(b *Baselines) Accept(u snapshot.URLEntry, snapshotID string) bool`:
  - Iterate `b.Entries`; if `e.Path == u.Path` found:
    - If `e.SnapshotID == snapshotID` → return `false` (no-op / idempotent)
    - Else update `SnapshotID` and `AcceptedAt = time.Now().UTC()` → return `true`
  - If not found: append new `BaselineEntry{URL, Path, SnapshotID, AcceptedAt}` → return `true`
- Sub-task: Implement `(b *Baselines) ForPath(path string) *BaselineEntry`:
  - Linear scan; return pointer to matching entry or nil

### Task 2.2 — Create `baseline/baseline_test.go`
- Sub-task: `TestLoad_Missing` — call `Load` on empty temp dir; assert empty `Baselines`, nil error
- Sub-task: `TestLoad_Valid` — write fixture `baselines.json` to temp dir; assert entries parsed correctly
- Sub-task: `TestAccept_Insert` — call `Accept` on empty `Baselines` with a new URL; assert returns `true`, entry appended
- Sub-task: `TestAccept_Update` — call `Accept` on `Baselines` that has entry for same path but different `snapshot_id`; assert returns `true`, `SnapshotID` updated, `AcceptedAt` changed
- Sub-task: `TestAccept_NoOp` — call `Accept` with same `snapshot_id` as existing entry; assert returns `false`, no mutation
- Sub-task: `TestSave_Atomic` — call `Save` on a temp dir; assert `baselines.json` exists and is valid JSON; assert no temp files remain

---

## Phase 3: `cmd/accept.go` — Command Handler

### Task 3.1 — Create `cmd/accept.go`
- Sub-task: Define `package cmd` and `RunAccept(args []string)` function
- Sub-task: Resolve `.kaleidoscope/` directory:
  - Call `browser.StateDir()` (already used elsewhere in cmd/) — this returns the local `.kaleidoscope/` path if it exists, else the global one. Alternatively, check if a project-local helper exists in the codebase; use the same approach other commands use.
- Sub-task: Extract `snapshotID` — first positional non-flag arg via `getArg(args)` (from `cmd/util.go`)
- Sub-task: Extract `urlFilter` — flag value via `getFlagValue(args, "--url")`
- Sub-task: Validate `snapshotID` (if non-empty):
  - Compile pattern `^[a-zA-Z0-9_-]+$`
  - Return `output.Fail("accept", err, "snapshot ID contains invalid characters")` if mismatch
- Sub-task: Load snapshot index via `snapshot.LoadIndex(dir)`:
  - On error or empty `idx.Snapshots` → `output.Fail("accept", ..., "run \`ks snapshot\` first")`
- Sub-task: Resolve target snapshot:
  - If `snapshotID != ""`: call `idx.ByID(snapshotID)`; if nil → `output.Fail`
  - Else: call `idx.Latest()`
- Sub-task: Load baselines via `baseline.Load(dir)`; on error → `output.Fail`
- Sub-task: Iterate `snap.URLs`, apply `urlFilter` path match, call `b.Accept(u, snap.ID)`:
  - Track `updated []string` and `skipped []string` from return values
- Sub-task: After loop, if `urlFilter != ""` and `len(updated)+len(skipped) == 0` → `output.Fail` with "no URL with path %q in snapshot %s"
- Sub-task: Persist via `b.Save(dir)`; on error → `output.Fail`
- Sub-task: Call `output.Success("accept", map[string]any{"snapshot_id": snap.ID, "updated": updated, "skipped": skipped})`
  - Ensure `updated` and `skipped` are initialized as empty slices (not nil) so JSON renders `[]` not `null`

### Task 3.2 — Update `cmd/util.go` to register `--url` flag
- Sub-task: In `getNonFlagArgs()`, add `"--url"` to the set of flags that consume the next argument (alongside `--selector`, `--output`, etc.)

### Task 3.3 — Create `cmd/accept_test.go` — Integration Tests
- Sub-task: Define test helper `makeSnapshotIndex(dir string, entries []snapshot.SnapshotEntry)` — writes `snapshots/index.json` to temp dir
- Sub-task: `TestRunAccept_NoSnapshots` — empty index → capture output, assert `ok: false`, error contains "no snapshots found"
- Sub-task: `TestRunAccept_Latest` — index with two snapshots, two URLs → `RunAccept([]string{})` → assert `ok: true`, `snapshot_id` = latest ID, `updated` has both paths
- Sub-task: `TestRunAccept_ByID` — index with two snapshots → `RunAccept([]string{"<first-id>"})` → assert `snapshot_id` = first ID
- Sub-task: `TestRunAccept_ByID_NotFound` — `RunAccept([]string{"unknown-id"})` → assert `ok: false`, error contains "not found"
- Sub-task: `TestRunAccept_URLFilter` — snapshot with `/` and `/dashboard` → `RunAccept([]string{"--url", "/dashboard"})` → assert `updated = ["/dashboard"]`, `/` not in updated or skipped
- Sub-task: `TestRunAccept_URLFilter_NoMatch` — `--url /nonexistent` → assert `ok: false`, error contains "no URL with path"
- Sub-task: `TestRunAccept_Idempotent` — call `RunAccept` twice with same args → second call: `updated = []`, `skipped = [all paths]`
- Sub-task: `TestRunAccept_InvalidID` — `RunAccept([]string{"../evil"})` → assert `ok: false`, error contains "invalid"

---

## Phase 4: Wire `main.go`

### Task 4.1 — Add `"accept"` case to command switch in `main.go`
- Sub-task: Locate the `switch` block (after `os.Args[1]` parsing)
- Sub-task: Add `case "accept": cmd.RunAccept(args)` in alphabetical position among existing cases
- Sub-task: Add `"--url"` to any flag-parsing lists in `main.go` if applicable (e.g., `hasFlag`-based arg slicing)

### Task 4.2 — Update usage documentation
- Sub-task: In `cmd/usage.go`, add usage entry for `accept` command: synopsis, description, flags, and example JSON output matching the spec

---

## Phase 5: Quality Gate

### Task 5.1 — Run `go build ./...`
- Sub-task: Resolve any compilation errors (missing imports, type mismatches, unused variables)
- Sub-task: Ensure `snapshot` import path is `github.com/callmeradical/kaleidoscope/snapshot`
- Sub-task: Ensure `baseline` import path is `github.com/callmeradical/kaleidoscope/baseline`

### Task 5.2 — Run `go test ./...`
- Sub-task: Verify all `baseline/baseline_test.go` tests pass
- Sub-task: Verify all `cmd/accept_test.go` tests pass
- Sub-task: Verify no regressions in existing tests

---

## Dependency & Ordering Notes

| Phase | Prerequisite |
|-------|-------------|
| Phase 1 (snapshot stub) | None — only if US-003 not present |
| Phase 2 (baseline pkg) | Phase 1 complete (snapshot types needed for `Accept` signature) |
| Phase 3 (cmd/accept) | Phase 1 + Phase 2 complete |
| Phase 4 (main.go wire) | Phase 3 complete |
| Phase 5 (quality gate) | All prior phases complete |

## Key Invariants to Enforce

1. `baselines.json` `path` values must be unique — `Accept()` enforces this via scan-then-update.
2. `Save()` must be atomic — temp file + rename pattern.
3. `snapshotID` from CLI args validated against `^[a-zA-Z0-9_-]+$` before any file path construction.
4. `updated` and `skipped` slices in JSON output must be `[]` (empty array), never `null`.
5. `Load()` of absent `baselines.json` returns empty struct with nil error (not an error condition).
