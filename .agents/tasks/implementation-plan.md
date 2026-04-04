# Implementation Plan: Audit and Element Diff Engine (US-003)

## Overview

Implements `ks diff [snapshot-id]` — a read-only command that compares a snapshot against a stored baseline and emits structured JSON describing regressions. Depends on US-001/US-002 types and store APIs being present; if absent, they must be stubbed/created as part of this work.

---

## Phase 1: Snapshot Package Foundation

Establish the `snapshot` package that the diff engine depends on. This is a prerequisite for all subsequent phases.

### Task 1.1 — Define snapshot data types (`snapshot/types.go`)

- **Sub-task 1.1.1** — Create `snapshot/types.go` with the `Snapshot` struct (fields: `ID`, `CreatedAt`, `CommitSHA`, `URL`, `Audit`, `AXNodes`, `Screenshot`)
- **Sub-task 1.1.2** — Define `AuditSummary` struct (fields: `TotalIssues`, `ContrastViolations []AuditIssue`, `TouchViolations []AuditIssue`, `TypographyWarnings []AuditIssue`)
- **Sub-task 1.1.3** — Define `AuditIssue` struct (fields: `Selector`, `Detail`)
- **Sub-task 1.1.4** — Define `AXNode` struct (fields: `Role`, `Name`, `X`, `Y`, `Width`, `Height`)
- **Sub-task 1.1.5** — Add JSON struct tags to all fields matching the spec
- **Sub-task 1.1.6** — Verify package compiles with `go build ./snapshot/...`

### Task 1.2 — Implement snapshot store read surface (`snapshot/store.go`)

- **Sub-task 1.2.1** — Define `ErrNoBaseline` sentinel error: `errors.New("no baseline set; run \`ks snapshot --set-baseline\` first")`
- **Sub-task 1.2.2** — Implement `SnapshotsDir() (string, error)` — returns `.kaleidoscope/snapshots/`, creates it if absent
- **Sub-task 1.2.3** — Implement `ListSnapshots() ([]string, error)` — reads snapshot subdirectory names, sorts newest-first (lexicographic descending, since IDs are timestamp-formatted)
- **Sub-task 1.2.4** — Implement `LoadSnapshot(id string) (*Snapshot, error)` — reads `.kaleidoscope/snapshots/<id>/snapshot.json`; validate path with `filepath.Clean` and prefix check against snapshots dir to prevent path traversal
- **Sub-task 1.2.5** — Implement `LoadLatestSnapshot() (*Snapshot, error)` — calls `ListSnapshots`, picks first entry, delegates to `LoadSnapshot`
- **Sub-task 1.2.6** — Implement `LoadBaseline() (*Snapshot, error)` — reads `.kaleidoscope/baselines.json`, parses `{"baseline": "<id>"}`, then calls `LoadSnapshot`; return `ErrNoBaseline` if file absent or field empty
- **Sub-task 1.2.7** — Verify package compiles with `go build ./snapshot/...`

---

## Phase 2: Diff Engine (Pure Functions)

Implement the `diff` package as a pure-function module with no browser or filesystem dependency. This is the core logic for the feature.

### Task 2.1 — Define diff output types (`diff/diff.go`)

- **Sub-task 2.1.1** — Create `diff/diff.go` in package `diff`
- **Sub-task 2.1.2** — Define `DiffResult` struct (fields: `SnapshotID`, `BaselineID`, `Regressions bool`, `Audit AuditDiff`, `Elements ElementDiff`) with JSON tags
- **Sub-task 2.1.3** — Define `AuditDiff` struct (fields: `ContrastDelta`, `TouchDelta`, `TypographyDelta`, `TotalDelta int`, `NewIssues []IssueDelta`, `ResolvedIssues []IssueDelta`) with JSON tags
- **Sub-task 2.1.4** — Define `IssueDelta` struct (fields: `Category`, `Selector`, `Detail string`) with JSON tags
- **Sub-task 2.1.5** — Define `ElementDiff` struct (fields: `Appeared`, `Disappeared`, `Moved`, `Resized []ElementChange`) with JSON tags
- **Sub-task 2.1.6** — Define `ElementChange` struct (fields: `Role`, `Name string`, `Baseline *Rect`, `Target *Rect`, `Delta *Delta`) with JSON `omitempty` tags
- **Sub-task 2.1.7** — Define `Rect` struct (fields: `X`, `Y`, `Width`, `Height float64`) with JSON tags
- **Sub-task 2.1.8** — Define `Delta` struct (fields: `DX`, `DY`, `DWidth`, `DHeight float64`) with JSON `omitempty` tags
- **Sub-task 2.1.9** — Declare `PositionThreshold = 4.0` and `SizeThreshold = 4.0` constants

### Task 2.2 — Implement `CompareAudit`

- **Sub-task 2.2.1** — Implement helper to build an issue key set: iterate each category's `[]AuditIssue`, produce `map[string]IssueDelta` keyed by `"<category>:<selector>"`
- **Sub-task 2.2.2** — Compute `newIssues` as target set minus baseline set (keys present in target, absent in baseline)
- **Sub-task 2.2.3** — Compute `resolvedIssues` as baseline set minus target set (keys present in baseline, absent in target)
- **Sub-task 2.2.4** — Compute per-category deltas: `ContrastDelta = len(target.ContrastViolations) - len(baseline.ContrastViolations)`, same pattern for Touch and Typography
- **Sub-task 2.2.5** — Compute `TotalDelta = ContrastDelta + TouchDelta + TypographyDelta`
- **Sub-task 2.2.6** — Return populated `AuditDiff`; ensure `NewIssues` and `ResolvedIssues` default to empty slice (not nil) so JSON serializes as `[]`

### Task 2.3 — Implement `CompareElements`

- **Sub-task 2.3.1** — Implement semantic identity key function: `strings.ToLower(strings.TrimSpace(role)) + "|" + strings.ToLower(strings.TrimSpace(name))`
- **Sub-task 2.3.2** — Skip nodes with empty name (after trim) during indexing to avoid false positives from anonymous containers
- **Sub-task 2.3.3** — Build `baselineIndex map[string]AXNode` and `targetIndex map[string]AXNode`
- **Sub-task 2.3.4** — Populate `Appeared`: iterate target index, if key absent from baseline index, add `ElementChange{Role, Name, Target: &Rect{...}}`
- **Sub-task 2.3.5** — Populate `Disappeared`: iterate baseline index, if key absent from target index, add `ElementChange{Role, Name, Baseline: &Rect{...}}`
- **Sub-task 2.3.6** — For each key present in both: compute `|ΔX|` and `|ΔY|`; if either exceeds `PositionThreshold`, append to `Moved` with `Baseline`, `Target`, and `Delta{DX, DY}` populated
- **Sub-task 2.3.7** — For each key present in both: compute `|ΔW|` and `|ΔH|`; if either exceeds `SizeThreshold`, append to `Resized` with `Baseline`, `Target`, and `Delta{DWidth, DHeight}` populated
- **Sub-task 2.3.8** — Note: an element may appear in both `Moved` and `Resized` independently (separate append operations)
- **Sub-task 2.3.9** — Ensure all slices default to empty slice (not nil) so JSON serializes as `[]`

### Task 2.4 — Implement top-level `Compare`

- **Sub-task 2.4.1** — Implement `Compare(baseline, target *snapshot.Snapshot) DiffResult`
- **Sub-task 2.4.2** — Call `CompareAudit(baseline.Audit, target.Audit)` to get `AuditDiff`
- **Sub-task 2.4.3** — Call `CompareElements(baseline.AXNodes, target.AXNodes)` to get `ElementDiff`
- **Sub-task 2.4.4** — Set `Regressions = len(auditDiff.NewIssues) > 0 || len(elemDiff.Appeared)+len(elemDiff.Disappeared)+len(elemDiff.Moved)+len(elemDiff.Resized) > 0`
- **Sub-task 2.4.5** — Return `DiffResult{SnapshotID: target.ID, BaselineID: baseline.ID, Regressions: ..., Audit: ..., Elements: ...}`

---

## Phase 3: Unit Tests

Write comprehensive unit tests for the diff engine. All tests live in `diff/diff_test.go` and require no mocks, browser, or filesystem.

### Task 3.1 — Audit diff tests

- **Sub-task 3.1.1** — `TestCompareAudit_NoChange`: pass identical `AuditSummary` values; assert all deltas are 0 and both issue slices are empty
- **Sub-task 3.1.2** — `TestCompareAudit_NewIssues`: target has extra contrast violations not in baseline; assert `ContrastDelta > 0`, issues appear in `NewIssues` with correct `Category = "contrast"`
- **Sub-task 3.1.3** — `TestCompareAudit_ResolvedIssues`: baseline has issues absent from target; assert they appear in `ResolvedIssues`
- **Sub-task 3.1.4** — `TestCompareAudit_SelectorMatching`: same selector in same category = same issue (no new/resolved); different selectors = distinct issues
- **Sub-task 3.1.5** — `TestCompareAudit_DetailIgnored`: same selector but different `Detail` text = treated as the same issue (selector identity wins), no false regression

### Task 3.2 — Element diff tests

- **Sub-task 3.2.1** — `TestCompareElements_Appeared`: node in target not in baseline appears in `Appeared` with `Baseline = nil` and `Target` populated
- **Sub-task 3.2.2** — `TestCompareElements_Disappeared`: node in baseline not in target appears in `Disappeared` with `Target = nil` and `Baseline` populated
- **Sub-task 3.2.3** — `TestCompareElements_Moved`: position delta > `PositionThreshold` (e.g. 5px) → node in `Moved` with correct `Delta.DX`/`DY`
- **Sub-task 3.2.4** — `TestCompareElements_MovedBelowThreshold`: position delta <= `PositionThreshold` (e.g. 2px) → node not in `Moved`
- **Sub-task 3.2.5** — `TestCompareElements_Resized`: size delta > `SizeThreshold` → node in `Resized` with correct `Delta.DWidth`/`DHeight`
- **Sub-task 3.2.6** — `TestCompareElements_EmptyNameExcluded`: nodes with empty or whitespace-only `Name` are not indexed and produce no changes
- **Sub-task 3.2.7** — `TestCompareElements_MovedAndResized`: one node both moves and resizes → appears in both `Moved` and `Resized`

### Task 3.3 — Regression flag and integration tests

- **Sub-task 3.3.1** — `TestRegressionFlag_FalseWhenClean`: `Compare` on two identical snapshots → `Regressions = false`
- **Sub-task 3.3.2** — `TestRegressionFlag_TrueOnNewAuditIssue`: new audit issue → `Regressions = true`
- **Sub-task 3.3.3** — `TestRegressionFlag_TrueOnElementChange`: any element appeared/disappeared/moved/resized → `Regressions = true`

### Task 3.4 — Run quality gate

- **Sub-task 3.4.1** — Run `go test ./diff/...` and confirm all tests pass
- **Sub-task 3.4.2** — Run `go test ./...` (full suite) and confirm no regressions in existing tests

---

## Phase 4: CLI Command

Wire the diff engine into the CLI surface.

### Task 4.1 — Implement `cmd/diff.go`

- **Sub-task 4.1.1** — Create `cmd/diff.go` with `func RunDiff(args []string)`
- **Sub-task 4.1.2** — Parse optional positional arg 0 as snapshot ID; if absent, set `id = ""`
- **Sub-task 4.1.3** — Load baseline: call `snapshot.LoadBaseline()`; if error is `snapshot.ErrNoBaseline`, call `output.Fail("diff", err, "Run: ks snapshot --set-baseline")` and `os.Exit(2)`; for other I/O errors, same exit-2 pattern
- **Sub-task 4.1.4** — Load target: if `id != ""`, call `snapshot.LoadSnapshot(id)`; otherwise call `snapshot.LoadLatestSnapshot()`; on error, call `output.Fail` and `os.Exit(2)`
- **Sub-task 4.1.5** — Call `diff.Compare(baseline, target)` to produce `DiffResult`
- **Sub-task 4.1.6** — Call `output.Success("diff", result)` to emit JSON
- **Sub-task 4.1.7** — If `result.Regressions == true`, call `os.Exit(1)`; otherwise return (exit 0)

### Task 4.2 — Register command in `main.go`

- **Sub-task 4.2.1** — Add `case "diff": cmd.RunDiff(cmdArgs)` to the switch statement in `main.go`
- **Sub-task 4.2.2** — Add a "Regression Detection" section to the `usage` string:
  ```
  Regression Detection:
    snapshot [options]      Capture a snapshot (audit + ax-tree + screenshot)
    snapshot --set-baseline Mark a snapshot as the known-good baseline
    diff [snapshot-id]      Compare snapshot against baseline; exit 1 on regression
  ```
- **Sub-task 4.2.3** — Verify `go build ./...` succeeds with the new case

---

## Phase 5: Integration Verification

Final checks to confirm the whole feature compiles, tests pass, and the CLI behaves correctly.

### Task 5.1 — Build verification

- **Sub-task 5.1.1** — Run `go build ./...` from workspace root; confirm zero errors
- **Sub-task 5.1.2** — Run `go vet ./...`; confirm zero warnings

### Task 5.2 — Full test suite

- **Sub-task 5.2.1** — Run `go test ./...`; confirm all tests pass (quality gate)

### Task 5.3 — Exit code verification (manual smoke test, if snapshots available)

- **Sub-task 5.3.1** — Confirm `ks diff` with no baseline exits 2 and prints `ok: false`
- **Sub-task 5.3.2** — Confirm `ks diff` with clean snapshot exits 0 and `regressions: false`
- **Sub-task 5.3.3** — Confirm `ks diff` with regression exits 1 and `regressions: true`

---

## File Inventory

| File | Action | Phase |
|------|--------|-------|
| `snapshot/types.go` | Create | 1.1 |
| `snapshot/store.go` | Create | 1.2 |
| `diff/diff.go` | Create | 2 |
| `diff/diff_test.go` | Create | 3 |
| `cmd/diff.go` | Create | 4.1 |
| `main.go` | Edit (add case + usage) | 4.2 |

## Dependency Order

```
Phase 1 (snapshot package)
  └── Phase 2 (diff engine — imports snapshot types)
        └── Phase 3 (unit tests — imports diff)
              └── Phase 4 (CLI — imports diff + snapshot + output)
                    └── Phase 5 (verification)
```

Phases 1→2→3→4→5 must be executed sequentially due to import dependencies. Within each phase, sub-tasks within a single task are sequential; tasks within the same phase are independent where noted.
