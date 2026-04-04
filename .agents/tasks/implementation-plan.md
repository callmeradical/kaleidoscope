# Implementation Plan: Baseline Manager (US-005)

## Overview

Implements `ks accept [snapshot-id] [--url <path>]` — a CLI command that promotes a snapshot to baseline for all project URLs (or a single URL), persisting results to `.kaleidoscope/baselines.json`.

**Dependency:** US-003 (Snapshot System) must provide `snapshot.LoadIndex()`, `snapshot.SnapshotMeta`, and `snapshot.Index` types. This plan assumes US-003 stubs or full implementations are present before US-005 is wired end-to-end.

**Quality Gate:** `go test ./...` must pass.

---

## Phase 1: Snapshot Package Foundation (US-003 Contracts)

> Establish the types and stub functions that US-005 depends on from US-003. If US-003 is already implemented, skip tasks that duplicate existing work.

### Task 1.1 — Verify or create `snapshot/` package

**Sub-tasks:**
- 1.1.1 Check whether `snapshot/` directory exists in the repo
- 1.1.2 If missing, create the directory and an empty `snapshot/doc.go` with `package snapshot` to initialize the package
- 1.1.3 Confirm `go.mod` module name (`github.com/callmeradical/kaleidoscope`) so import paths are correct

### Task 1.2 — Define US-003 contract types in `snapshot/index.go`

**Sub-tasks:**
- 1.2.1 Create (or verify) `snapshot/index.go` with:
  - `SnapshotMeta` struct (`ID string`, `CreatedAt time.Time`, `CommitSHA string`, `URLs []string`) with JSON tags matching the tech spec
  - `Index` struct (`Snapshots []SnapshotMeta`) with JSON tag `"snapshots"`
- 1.2.2 Implement (or verify) `LoadIndex() (*Index, error)` that reads `.kaleidoscope/snapshots/index.json` using `browser.StateDir()` for the base path
- 1.2.3 Ensure `LoadIndex` returns a clear error if `index.json` does not exist (not a silent empty index — callers check `len(index.Snapshots)`)

---

## Phase 2: Baseline Logic — `snapshot/baseline.go`

> Pure-function module with no Chrome/browser dependency. This is the core business logic of US-005.

### Task 2.1 — Define `Baselines` and `BaselineEntry` types

**Sub-tasks:**
- 2.1.1 Create `snapshot/baseline.go` with `package snapshot`
- 2.1.2 Define `BaselineEntry` struct:
  - `URL string` with JSON tag `"url"`
  - `SnapshotID string` with JSON tag `"snapshotId"`
- 2.1.3 Define `Baselines` struct:
  - `Entries []BaselineEntry` with JSON tag `"baselines"`

### Task 2.2 — Implement `LoadBaselines() (*Baselines, error)`

**Sub-tasks:**
- 2.2.1 Resolve file path: `filepath.Join(browser.StateDir(), "baselines.json")`
- 2.2.2 If file does not exist (`os.IsNotExist`), return `&Baselines{Entries: []BaselineEntry{}}` and `nil` error (empty baselines, not an error)
- 2.2.3 Read and unmarshal JSON from file; return error on parse failure
- 2.2.4 Guard against nil `Entries` slice after unmarshal (normalize to empty slice)

### Task 2.3 — Implement `SaveBaselines(b *Baselines) error`

**Sub-tasks:**
- 2.3.1 Marshal `b` to JSON with 2-space indentation (`json.MarshalIndent`)
- 2.3.2 Write to `filepath.Join(browser.StateDir(), "baselines.json")` with file mode `0644`
- 2.3.3 Return any I/O error to caller

### Task 2.4 — Implement `Accept(current *Baselines, meta *SnapshotMeta, urls []string) (updated *Baselines, changed []string)` (pure function)

**Sub-tasks:**
- 2.4.1 If `urls` is nil or empty, set `urls = meta.URLs` (promote all URLs in snapshot)
- 2.4.2 Deep-copy `current.Entries` into a new `Baselines` so the function is non-mutating
- 2.4.3 For each URL in `urls`:
  - Search `updated.Entries` for an entry where `entry.URL == url`
  - If found and `entry.SnapshotID == meta.ID`: skip (idempotent — do NOT add to `changed`)
  - If found and `entry.SnapshotID != meta.ID`: update `entry.SnapshotID = meta.ID`, append `url` to `changed`
  - If not found: append new `BaselineEntry{URL: url, SnapshotID: meta.ID}`, append `url` to `changed`
- 2.4.4 Return `updated` and `changed` (may be empty slice, never nil)

---

## Phase 3: Unit Tests — `snapshot/baseline_test.go`

> All tests are pure-function tests (no I/O, no filesystem). Cover all acceptance criteria.

### Task 3.1 — Create `snapshot/baseline_test.go`

**Sub-tasks:**
- 3.1.1 Create file with `package snapshot` (same package, for access to internals if needed)
- 3.1.2 Define shared helper: `makeBaselines(entries ...BaselineEntry) *Baselines` for concise test setup
- 3.1.3 Define shared helper: `makeMeta(id string, urls ...string) *SnapshotMeta`

### Task 3.2 — Write `TestAccept_EmptyBaselines`

**Sub-tasks:**
- 3.2.1 Call `Accept` with empty `&Baselines{}` and a meta with two URLs, `urls=nil`
- 3.2.2 Assert `len(updated.Entries) == 2`
- 3.2.3 Assert both URLs appear in `changed`
- 3.2.4 Assert each entry has the correct `SnapshotID`

### Task 3.3 — Write `TestAccept_AllURLs`

**Sub-tasks:**
- 3.3.1 Call `Accept` with a pre-existing baselines (different snapshot IDs) and `urls=nil`
- 3.3.2 Assert all entries are updated to the new snapshot ID
- 3.3.3 Assert all URLs appear in `changed`

### Task 3.4 — Write `TestAccept_SingleURL`

**Sub-tasks:**
- 3.4.1 Create baselines with entries for `/dashboard` and `/login`
- 3.4.2 Call `Accept` with `urls=[]string{"/dashboard"}` and a new snapshot ID
- 3.4.3 Assert only `/dashboard` is in `changed`
- 3.4.4 Assert `/login` entry is unchanged in `updated`
- 3.4.5 Assert `/dashboard` entry has new snapshot ID

### Task 3.5 — Write `TestAccept_AlreadyBaseline` (idempotency)

**Sub-tasks:**
- 3.5.1 Create baselines where `/dashboard` already points to snapshot `"snap-A"`
- 3.5.2 Call `Accept` with meta ID `"snap-A"` and `urls=[]string{"/dashboard"}`
- 3.5.3 Assert `changed` is empty (no-op)
- 3.5.4 Assert `updated.Entries` is unchanged

### Task 3.6 — Write `TestAccept_UpdatesExisting`

**Sub-tasks:**
- 3.6.1 Create baselines where `/login` points to old snapshot `"snap-old"`
- 3.6.2 Call `Accept` with meta ID `"snap-new"` and `urls=[]string{"/login"}`
- 3.6.3 Assert `"/login"` is in `changed`
- 3.6.4 Assert `updated.Entries` shows `"snap-new"` for `/login`
- 3.6.5 Assert `len(updated.Entries) == 1` (no duplicate entries created)

### Task 3.7 — Run `go test ./snapshot/...` and confirm all tests pass

---

## Phase 4: CLI Command — `cmd/accept.go`

> Implements `RunAccept(args []string)` following the standard command pattern.

### Task 4.1 — Create `cmd/accept.go`

**Sub-tasks:**
- 4.1.1 Create file with `package cmd`, imports: `os`, `fmt`, `snapshot`, `output`
- 4.1.2 Declare `func RunAccept(args []string)`

### Task 4.2 — Implement snapshot index loading and validation

**Sub-tasks:**
- 4.2.1 Call `snapshot.LoadIndex()`; on error call `output.Fail("accept", err, "")` and `os.Exit(2)`
- 4.2.2 Check `len(index.Snapshots) == 0`; if true call `output.Fail("accept", fmt.Errorf("no snapshots exist"), "Run: ks snapshot")` and `os.Exit(2)`

### Task 4.3 — Implement snapshot ID resolution (positional arg)

**Sub-tasks:**
- 4.3.1 Call `getArg(args)` to get optional positional snapshot ID
- 4.3.2 If empty: set `meta = &index.Snapshots[len(index.Snapshots)-1]` (latest)
- 4.3.3 If non-empty: iterate `index.Snapshots` to find matching `meta.ID`; if not found call `output.Fail("accept", fmt.Errorf("snapshot not found: %s", snapshotID), "")` and `os.Exit(2)`

### Task 4.4 — Implement `--url` flag handling

**Sub-tasks:**
- 4.4.1 Call `getFlagValue(args, "--url")` to get optional URL filter
- 4.4.2 If URL filter is non-empty: check that it exists in `meta.URLs`; if not call `output.Fail("accept", fmt.Errorf("url %q not in snapshot %s", urlFilter, meta.ID), "")` and `os.Exit(2)`
- 4.4.3 Build `urls []string`: if filter is non-empty set `urls = []string{urlFilter}`, otherwise leave empty (Accept will use all)

### Task 4.5 — Implement baseline load, update, and save

**Sub-tasks:**
- 4.5.1 Call `snapshot.LoadBaselines()`; on error call `output.Fail` and `os.Exit(2)`
- 4.5.2 Call `snapshot.Accept(baselines, meta, urls)` to get `updated` and `changed`
- 4.5.3 Call `snapshot.SaveBaselines(updated)`; on error call `output.Fail` and `os.Exit(2)`

### Task 4.6 — Emit success output

**Sub-tasks:**
- 4.6.1 Call `output.Success("accept", map[string]any{...})` with fields:
  - `"snapshotId"`: `meta.ID`
  - `"changed"`: `changed` (empty slice if no-op, never nil)
  - `"noOp"`: `len(changed) == 0`
  - `"baselines"`: `updated.Entries`

---

## Phase 5: Wire Up in Existing Files

> Minimal modifications to existing files to register the new command.

### Task 5.1 — Update `cmd/util.go`: add `--url` to value-taking flags

**Sub-tasks:**
- 5.1.1 Read `cmd/util.go` and locate `getNonFlagArgs()` function
- 5.1.2 Find the slice/map of flags that take a value argument (e.g., `--selector`, `--output`, `--depth`, etc.)
- 5.1.3 Add `"--url"` to that collection so `getNonFlagArgs` correctly skips the value after `--url`
- 5.1.4 Verify `getFlagValue` already handles `--url` generically (it should, since it takes any flag name)

### Task 5.2 — Update `cmd/usage.go`: add `"accept"` entry

**Sub-tasks:**
- 5.2.1 Read `cmd/usage.go` and locate the `CommandUsage` map
- 5.2.2 Add entry for `"accept"` with:
  - Usage: `ks accept [snapshot-id] [--url <path>]`
  - Description: promotes a snapshot to baseline, persisted in `.kaleidoscope/baselines.json`
  - Options: `--url <path>` — limit accept to a single URL path
  - Output: JSON with `snapshotId`, `changed`, `noOp`, `baselines`
  - Examples: `ks accept`, `ks accept 20260404-153012`, `ks accept --url /dashboard`

### Task 5.3 — Update `main.go`: add `case "accept"`

**Sub-tasks:**
- 5.3.1 Read `main.go` and locate the command switch statement
- 5.3.2 Add `case "accept": cmd.RunAccept(cmdArgs)` in the appropriate position (alphabetical or grouped with data commands)
- 5.3.3 Verify the import of the `cmd` package is already present (it will be)

---

## Phase 6: Integration Verification

### Task 6.1 — Build the binary

**Sub-tasks:**
- 6.1.1 Run `go build ./...` and resolve any compile errors
- 6.1.2 Common errors to watch for: missing US-003 types (`SnapshotMeta`, `Index`, `LoadIndex`), wrong package imports, unused imports

### Task 6.2 — Run full test suite

**Sub-tasks:**
- 6.2.1 Run `go test ./...`
- 6.2.2 Confirm `snapshot/baseline_test.go` tests all pass
- 6.2.3 Confirm no regressions in other packages

### Task 6.3 — Manual smoke tests (if binary builds successfully)

**Sub-tasks:**
- 6.3.1 `ks accept` with no snapshots → verify JSON output: `ok: false`, `error: "no snapshots exist"`, `hint: "Run: ks snapshot"`
- 6.3.2 `ks accept bad-id` → verify JSON output: `ok: false`, `error: "snapshot not found: bad-id"`
- 6.3.3 With a valid snapshot in `index.json`: `ks accept` → verify `baselines.json` is created/updated
- 6.3.4 `ks accept --url /dashboard` → verify only `/dashboard` updated in `baselines.json`
- 6.3.5 Run `ks accept` twice with same snapshot → verify `noOp: true` on second run

---

## File Summary

### New Files

| File | Owner | Description |
|------|-------|-------------|
| `snapshot/baseline.go` | US-005 | `BaselineEntry`, `Baselines` types; `LoadBaselines`, `SaveBaselines`, `Accept` |
| `snapshot/baseline_test.go` | US-005 | Unit tests for `Accept` pure function |
| `cmd/accept.go` | US-005 | `RunAccept` CLI handler |

### Modified Files

| File | Change |
|------|--------|
| `main.go` | Add `case "accept": cmd.RunAccept(cmdArgs)` |
| `cmd/usage.go` | Add `"accept"` entry to `CommandUsage` map |
| `cmd/util.go` | Add `"--url"` to value-taking flags in `getNonFlagArgs` |

### Prerequisite Files (US-003 — create stubs if not present)

| File | Owner | Description |
|------|-------|-------------|
| `snapshot/index.go` | US-003 | `SnapshotMeta`, `Index` types; `LoadIndex` function |

---

## Acceptance Criteria Traceability

| Acceptance Criterion | Covered By |
|---------------------|------------|
| `ks accept` promotes latest snapshot to baseline for all project URLs | Task 4.3 (default to latest), Task 2.4 (urls=nil → all URLs) |
| `ks accept <snapshot-id>` promotes specific snapshot | Task 4.3 (positional arg resolution) |
| `ks accept --url /dashboard` updates baseline for only that URL | Task 4.4, Task 2.4 (single-URL path) |
| `baselines.json` is correctly updated on disk | Task 2.3 (SaveBaselines), Task 4.5 |
| `ks accept` returns error if no snapshots exist | Task 4.2 |
| Accepting an already-baseline snapshot is a no-op (idempotent) | Task 2.4.3, Task 3.5 |
