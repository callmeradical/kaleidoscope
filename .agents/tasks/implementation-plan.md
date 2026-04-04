# Implementation Plan: Audit and Element Diff Engine (US-003)

**Story:** `task-0d599df2a460a91e`
**Depends on:** US-001 (Snapshot Capture), US-002 (Project Config / Baseline Manager)
**Date:** 2026-04-04

---

## Overview

Implement `ks diff [snapshot-id]` — a pure-function diff engine that compares a snapshot against a baseline and outputs structured JSON describing audit regressions and element-level changes. No Chrome dependency. Exit code 0 = no regressions, 1 = regressions, 2 = errors.

---

## Phase 1: Shared Type Definitions (`diff/types.go`)

**Goal:** Define all data structures for diff output before implementing logic.

### Task 1.1 — Create `diff/` package directory and `diff/types.go`

- **Sub-task 1.1.1:** Create `/workspace/diff/types.go` with `package diff` declaration.
- **Sub-task 1.1.2:** Define `DiffResult` struct:
  - Fields: `SnapshotID string`, `BaselineID string`, `Regressions bool`, `Audit AuditDelta`, `Elements ElementDelta`
  - JSON tags: `snapshotId`, `baselineId`, `regressions`, `audit`, `elements`
- **Sub-task 1.1.3:** Define `AuditDelta` struct:
  - Fields: `Categories map[string]CategoryDelta`, `NewIssues []IssueDiff`, `Resolved []IssueDiff`, `HasRegression bool`
  - JSON tags: `categories`, `newIssues`, `resolved`, `hasRegression`
- **Sub-task 1.1.4:** Define `CategoryDelta` struct:
  - Fields: `Category string`, `Baseline int`, `Current int`, `Delta int`
  - JSON tags: `category`, `baseline`, `current`, `delta`
  - Note: `Delta` = positive means regression (more issues)
- **Sub-task 1.1.5:** Define `IssueDiff` struct:
  - Fields: `Selector string`, `Category string`, `Message string`, `Status string`
  - JSON tags: `selector`, `category`, `message`, `status`
  - Note: `Status` values are `"new"` or `"resolved"`
- **Sub-task 1.1.6:** Define `ElementDelta` struct:
  - Fields: `Appeared []ElementChange`, `Disappeared []ElementChange`, `Moved []ElementChange`, `Resized []ElementChange`, `HasRegression bool`
  - JSON tags: `appeared`, `disappeared`, `moved`, `resized`, `hasRegression`
- **Sub-task 1.1.7:** Define `ElementChange` struct:
  - Fields: `Role string`, `Name string`, `Identity string`, `Before *ElementState`, `After *ElementState`, `Delta *MoveDelta`
  - JSON tags: `role`, `name`, `identity`, `before` (omitempty), `after` (omitempty), `delta` (omitempty)
- **Sub-task 1.1.8:** Define `ElementState` struct:
  - Fields: `X float64`, `Y float64`, `Width float64`, `Height float64`
  - JSON tags: `x`, `y`, `width`, `height`
- **Sub-task 1.1.9:** Define `MoveDelta` struct:
  - Fields: `DX float64`, `DY float64`, `DW float64`, `DH float64`
  - JSON tags: `dx`, `dy`, `dw`, `dh`

---

## Phase 2: Audit Delta Engine (`diff/audit.go`)

**Goal:** Implement `ComputeAuditDelta` as a pure function with no Chrome or filesystem dependency.

### Task 2.1 — Create `diff/audit.go`

- **Sub-task 2.1.1:** Add `package diff` declaration and required imports (`strings`, `github.com/callmeradical/kaleidoscope/snapshot`).
- **Sub-task 2.1.2:** Implement helper `groupByCategory(issues []snapshot.AuditIssue) map[string][]snapshot.AuditIssue`:
  - Iterates over issues and groups them by `issue.Category`
  - Returns a map from category string to slice of issues
- **Sub-task 2.1.3:** Implement helper `issueKey(issue snapshot.AuditIssue) string`:
  - Returns `strings.ToLower(strings.TrimSpace(issue.Category)) + ":" + strings.ToLower(strings.TrimSpace(issue.Selector))`
  - Used for matching issues between baseline and current
- **Sub-task 2.1.4:** Implement `ComputeAuditDelta(baseline, current snapshot.AuditData) AuditDelta`:
  - Step 1: Group baseline issues by category using `groupByCategory`
  - Step 2: Group current issues by category using `groupByCategory`
  - Step 3: Iterate over the fixed set of categories `["contrast", "touch", "typography", "spacing"]`
    - For each category, compute `CategoryDelta` with `Baseline = len(baselineIssues)`, `Current = len(currentIssues)`, `Delta = Current - Baseline`
  - Step 4: Build a set of issue keys from baseline across all categories
  - Step 5: Build a set of issue keys from current across all categories
  - Step 6: Issues with keys in current but not baseline → `IssueDiff{Status: "new"}`; append to `NewIssues`
  - Step 7: Issues with keys in baseline but not current → `IssueDiff{Status: "resolved"}`; append to `Resolved`
  - Step 8: Set `HasRegression = true` if any `CategoryDelta.Delta > 0`
  - Return completed `AuditDelta`

---

## Phase 3: Element Delta Engine (`diff/element.go`)

**Goal:** Implement `ComputeElementDelta` as a pure function using semantic identity matching.

### Task 3.1 — Create `diff/element.go`

- **Sub-task 3.1.1:** Add `package diff` declaration, imports (`math`, `strings`, `github.com/callmeradical/kaleidoscope/snapshot`), and threshold constants:
  ```go
  const (
      PositionThreshold = 4.0
      SizeThreshold     = 4.0
  )
  ```
- **Sub-task 3.1.2:** Implement helper `semanticID(node snapshot.AXNode) string`:
  - Returns `strings.ToLower(strings.TrimSpace(node.Role)) + ":" + strings.ToLower(strings.TrimSpace(node.Name))`
  - Used as the map key for matching elements across snapshots
- **Sub-task 3.1.3:** Implement helper `buildNodeMap(nodes []snapshot.AXNode) map[string]snapshot.AXNode`:
  - Iterates over nodes; skips nodes where both `Role == ""` and `Name == ""`
  - Returns map from `semanticID` → `AXNode`
- **Sub-task 3.1.4:** Implement helper `toElementState(bb *snapshot.BoundingBox) *ElementState`:
  - Returns `nil` if `bb == nil`
  - Otherwise returns `&ElementState{X: bb.X, Y: bb.Y, Width: bb.Width, Height: bb.Height}`
- **Sub-task 3.1.5:** Implement `ComputeElementDelta(baseline, current []snapshot.AXNode) ElementDelta`:
  - Step 1: Build `baselineMap` using `buildNodeMap(baseline)`
  - Step 2: Build `currentMap` using `buildNodeMap(current)`
  - Step 3: Iterate `currentMap`; keys not in `baselineMap` → `ElementChange` added to `Appeared`
    - Set `Role`, `Name`, `Identity`, `After = toElementState(node.BoundingBox)`, `Before = nil`
  - Step 4: Iterate `baselineMap`; keys not in `currentMap` → `ElementChange` added to `Disappeared`
    - Set `Role`, `Name`, `Identity`, `Before = toElementState(node.BoundingBox)`, `After = nil`
  - Step 5: Iterate keys in both maps:
    - Skip positional checks if either node's `BoundingBox` is `nil`
    - Compute `dx = current.BoundingBox.X - baseline.BoundingBox.X`
    - Compute `dy = current.BoundingBox.Y - baseline.BoundingBox.Y`
    - Compute `dw = current.BoundingBox.Width - baseline.BoundingBox.Width`
    - Compute `dh = current.BoundingBox.Height - baseline.BoundingBox.Height`
    - If `math.Abs(dx) > PositionThreshold || math.Abs(dy) > PositionThreshold` → append to `Moved`
    - If `math.Abs(dw) > SizeThreshold || math.Abs(dh) > SizeThreshold` → append to `Resized`
    - An element can appear in both `Moved` and `Resized` (independent checks)
    - Build `ElementChange` with `Before`, `After`, and `Delta = &MoveDelta{DX: dx, DY: dy, DW: dw, DH: dh}` for positional entries
  - Step 6: Set `HasRegression = len(Disappeared) > 0 || len(Moved) > 0 || len(Resized) > 0`
    - Note: `Appeared` is informational only, does NOT trigger regression
  - Return completed `ElementDelta`

---

## Phase 4: CLI Command Handler (`cmd/diff.go`)

**Goal:** Wire the diff engine to the `ks diff` CLI command following the existing `Run*` pattern.

### Task 4.1 — Create `cmd/diff.go`

- **Sub-task 4.1.1:** Add `package cmd` declaration and imports:
  - `"fmt"`, `"os"`, `"github.com/callmeradical/kaleidoscope/diff"`, `"github.com/callmeradical/kaleidoscope/output"`, `"github.com/callmeradical/kaleidoscope/snapshot"`
- **Sub-task 4.1.2:** Implement `RunDiff(args []string)`:
  - Call `getArg(args)` to extract optional `<snapshot-id>` (returns `""` if not provided)
  - Call `snapshot.LoadBaseline()`:
    - On error: call `output.Fail("diff", fmt.Errorf("no baseline set"), "Run: ks snapshot set-baseline <snapshot-id>")` then `os.Exit(2)`
  - Load target snapshot:
    - If `snapshotID == ""`: call `snapshot.LoadLatest()`
    - Else: call `snapshot.Load(snapshotID)`
    - On error: call `output.Fail("diff", err, "Run: ks snapshot list")` then `os.Exit(2)`
  - Compute `auditDelta := diff.ComputeAuditDelta(baseline.AuditData, current.AuditData)`
  - Compute `elementDelta := diff.ComputeElementDelta(baseline.AXNodes, current.AXNodes)`
  - Build `result := diff.DiffResult{SnapshotID: current.ID, BaselineID: baseline.ID, Regressions: auditDelta.HasRegression || elementDelta.HasRegression, Audit: auditDelta, Elements: elementDelta}`
  - Call `output.Success("diff", result)`
  - If `result.Regressions`: call `os.Exit(1)`

---

## Phase 5: `main.go` Integration

**Goal:** Register the `diff` command in the CLI dispatcher and usage string.

### Task 5.1 — Add `diff` case to `main.go` command switch

- **Sub-task 5.1.1:** Locate the `switch command` block in `main.go`.
- **Sub-task 5.1.2:** Add `case "diff": cmd.RunDiff(cmdArgs)` in the appropriate location (after existing evaluation commands, e.g., after `report`).

### Task 5.2 — Update usage string in `main.go`

- **Sub-task 5.2.1:** Locate the "UX Evaluation" section of the usage/help string.
- **Sub-task 5.2.2:** Add the diff command entry:
  ```
    diff [snapshot-id]      Compare snapshot against baseline; exit 1 on regressions
  ```

---

## Phase 6: Unit Tests (`diff/diff_test.go`)

**Goal:** Achieve full coverage of pure-function diff logic with no browser or filesystem dependency.

### Task 6.1 — Create `diff/diff_test.go`

- **Sub-task 6.1.1:** Add `package diff` declaration and imports (`testing`, `github.com/callmeradical/kaleidoscope/snapshot`).

### Task 6.2 — Audit Delta Tests (`ComputeAuditDelta`)

- **Sub-task 6.2.1:** `TestComputeAuditDelta_EmptyBoth`
  - Input: empty `AuditData` for both baseline and current
  - Assert: all category deltas are 0, `NewIssues` and `Resolved` are empty, `HasRegression = false`
- **Sub-task 6.2.2:** `TestComputeAuditDelta_NoChange`
  - Input: identical issues in baseline and current across multiple categories
  - Assert: all deltas are 0, no new/resolved issues, `HasRegression = false`
- **Sub-task 6.2.3:** `TestComputeAuditDelta_NewIssue`
  - Input: one contrast issue in current not present in baseline
  - Assert: `contrast` delta = +1, one entry in `NewIssues` with `Status: "new"`, `HasRegression = true`
- **Sub-task 6.2.4:** `TestComputeAuditDelta_ResolvedIssue`
  - Input: one issue present in baseline but absent from current
  - Assert: category delta = -1, one entry in `Resolved` with `Status: "resolved"`, `HasRegression = false`
- **Sub-task 6.2.5:** `TestComputeAuditDelta_MultiCategory`
  - Input: mixed changes across all 4 categories (some new, some resolved, some unchanged)
  - Assert: correct per-category deltas, correct `NewIssues`/`Resolved` lists, `HasRegression` reflects any positive delta

### Task 6.3 — Element Delta Tests (`ComputeElementDelta`)

- **Sub-task 6.3.1:** `TestComputeElementDelta_Appeared`
  - Input: element in current only (not in baseline)
  - Assert: element in `Appeared`, `HasRegression = false`
- **Sub-task 6.3.2:** `TestComputeElementDelta_Disappeared`
  - Input: element in baseline only (not in current)
  - Assert: element in `Disappeared`, `HasRegression = true`
- **Sub-task 6.3.3:** `TestComputeElementDelta_Moved`
  - Input: same element in both; position delta > `PositionThreshold` (e.g., X offset by 10px)
  - Assert: element in `Moved`, correct `Delta.DX`, `HasRegression = true`
- **Sub-task 6.3.4:** `TestComputeElementDelta_MovedBelowThreshold`
  - Input: same element in both; position delta ≤ `PositionThreshold` (e.g., X offset by 2px)
  - Assert: `Moved` is empty, `HasRegression = false`
- **Sub-task 6.3.5:** `TestComputeElementDelta_Resized`
  - Input: same element in both; size delta > `SizeThreshold` (e.g., Width change of 10px)
  - Assert: element in `Resized`, correct `Delta.DW`, `HasRegression = true`
- **Sub-task 6.3.6:** `TestComputeElementDelta_MovedAndResized`
  - Input: element with both position and size changes beyond thresholds
  - Assert: element appears in both `Moved` and `Resized`, `HasRegression = true`
- **Sub-task 6.3.7:** `TestComputeElementDelta_NoBoundingBox`
  - Input: matching elements with `BoundingBox = nil` on one or both sides
  - Assert: no `Moved` or `Resized` entries reported; element not reported if both nil
- **Sub-task 6.3.8:** `TestComputeElementDelta_SemanticIdentity`
  - Input: elements in different order in slices but matching `role+name`
  - Assert: correctly matched by semantic identity, no false positives/negatives
- **Sub-task 6.3.9:** `TestSemanticIDNormalization`
  - Input: elements with mixed-case or whitespace-padded `Role` and `Name` values
  - Assert: they are treated as matching (lowercased, trimmed before keying)

---

## Phase 7: Dependency Verification

**Goal:** Confirm that upstream US-001/US-002 contracts are met before integration.

### Task 7.1 — Verify `snapshot` package exports

- **Sub-task 7.1.1:** Confirm `snapshot.Snapshot` struct has `ID string`, `AuditData snapshot.AuditData`, `AXNodes []snapshot.AXNode` fields.
- **Sub-task 7.1.2:** Confirm `snapshot.AuditData` has `ContrastIssues`, `TouchIssues`, `TypographyIssues`, `SpacingIssues` fields of type `[]AuditIssue`.
- **Sub-task 7.1.3:** Confirm `snapshot.AuditIssue` has `Selector string`, `Message string`, `Category string` fields.
- **Sub-task 7.1.4:** Confirm `snapshot.AXNode` has `Role string`, `Name string`, `BoundingBox *BoundingBox` fields.
- **Sub-task 7.1.5:** Confirm `snapshot.BoundingBox` has `X`, `Y`, `Width`, `Height float64` fields.
- **Sub-task 7.1.6:** Confirm `snapshot.Load(id string) (*Snapshot, error)` exists.
- **Sub-task 7.1.7:** Confirm `snapshot.LoadLatest() (*Snapshot, error)` exists.
- **Sub-task 7.1.8:** Confirm `snapshot.LoadBaseline() (*Snapshot, error)` exists.
- **Sub-task 7.1.9:** If any contract is missing, adapt `diff/` package imports accordingly without changing the pure-function logic.

---

## Phase 8: Build and Quality Gate

**Goal:** Ensure `go test ./...` passes and the binary compiles cleanly.

### Task 8.1 — Build verification

- **Sub-task 8.1.1:** Run `go build ./...` from `/workspace` to verify no compilation errors across all packages.
- **Sub-task 8.1.2:** Address any import or type mismatch errors revealed by the build.

### Task 8.2 — Test execution

- **Sub-task 8.2.1:** Run `go test ./diff/...` to execute the unit tests in isolation.
- **Sub-task 8.2.2:** Run `go test ./...` to execute all tests across the full codebase.
- **Sub-task 8.2.3:** Fix any failing tests.

### Task 8.3 — Manual smoke test (optional, browser required)

- **Sub-task 8.3.1:** If browser is available and a snapshot/baseline exist, run `ks diff` and verify JSON output structure matches the spec.
- **Sub-task 8.3.2:** Run `ks diff <nonexistent-id>` and verify exit code 2 with appropriate error JSON.
- **Sub-task 8.3.3:** If regressions are present, verify exit code is 1; if none, verify exit code is 0.

---

## Implementation Order (Recommended Sequence)

```
Phase 7 (Verify upstream contracts) → Phase 1 (types.go) → Phase 2 (audit.go) →
Phase 3 (element.go) → Phase 6 (diff_test.go) → Phase 4 (cmd/diff.go) →
Phase 5 (main.go) → Phase 8 (build + test)
```

**Rationale:**
- Phase 7 must come first to understand what snapshot types are actually available
- Types (Phase 1) must exist before logic (Phases 2–3) or tests (Phase 6) can compile
- Tests (Phase 6) should be written alongside logic to catch issues early
- CLI wiring (Phases 4–5) is last because it depends on the diff package being stable
- Build/test (Phase 8) validates the complete implementation

---

## File Manifest

| File | Action | Description |
|---|---|---|
| `/workspace/diff/types.go` | Create | All diff data structures |
| `/workspace/diff/audit.go` | Create | `ComputeAuditDelta` pure function |
| `/workspace/diff/element.go` | Create | `ComputeElementDelta` pure function + constants |
| `/workspace/diff/diff_test.go` | Create | Unit tests for both pure functions |
| `/workspace/cmd/diff.go` | Create | `RunDiff` CLI command handler |
| `/workspace/main.go` | Modify | Add `case "diff"` and update usage string |

---

## Security Notes (from spec)

- `snapshot.Load(id)` must reject IDs containing `/`, `..`, or path separators — verify upstream implementation does this; if not, add sanitization in `cmd/diff.go` before passing to `snapshot.Load`.
- No `exec.Command` calls in `diff/` package.
- No network access; all reads are from local JSON files.
- Use typed structs with `json.Unmarshal`, not `map[string]any`.
- Exit code 1 = regressions only; exit code 2 = errors only; never conflate them.
