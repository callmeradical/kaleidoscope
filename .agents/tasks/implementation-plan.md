# Implementation Plan: US-003 — Audit and Element Diff Engine

**Story**: `ks diff [snapshot-id]` — compare a snapshot against a baseline and output structured JSON describing audit deltas and element changes.

**Module**: `github.com/callmeradical/kaleidoscope`
**Quality Gate**: `go test ./...`

---

## Phase 1: `snapshot` Package — Shared Types and Storage Helpers

> Establishes the data model and disk I/O utilities consumed by both US-002 (snapshot creation) and US-003 (diff engine). No Chrome dependency.

### Task 1.1 — Create `snapshot/snapshot.go` (core types)

**Sub-tasks**:
- [ ] Create file `snapshot/snapshot.go` with `package snapshot`
- [ ] Define `Viewport` struct (`Width int`, `Height int`, JSON tags)
- [ ] Define `BoundingBox` struct (`X`, `Y`, `Width`, `Height float64`, JSON tags)
- [ ] Define `AuditIssue` struct (`Selector string`, `Message string`, JSON tags)
- [ ] Define `AuditData` struct with four category slices (`Contrast`, `Touch`, `Typography`, `Spacing []AuditIssue`, JSON tags)
- [ ] Define `Element` struct (`Role string`, `Name string`, `Box BoundingBox`, JSON tags)
- [ ] Define `Snapshot` struct (`ID string`, `CreatedAt time.Time`, `URL string`, `Viewport Viewport`, `Audit AuditData`, `Elements []Element`, JSON tags)

### Task 1.2 — Create `snapshot/index.go` (index types + read/write helpers)

**Sub-tasks**:
- [ ] Create file `snapshot/index.go` with `package snapshot`
- [ ] Define `SnapshotMeta` struct (`ID string`, `CreatedAt time.Time`, `URL string`, JSON tags)
- [ ] Define `SnapshotIndex` struct (`Entries []SnapshotMeta`, JSON tag)
- [ ] Implement `readIndex(dir string) (*SnapshotIndex, error)` — reads `<dir>/snapshots/index.json`; returns empty index if file absent
- [ ] Implement `writeIndex(dir string, idx *SnapshotIndex) error` — marshals and writes index atomically
- [ ] Implement `LatestID(dir string) (string, error)` — reads index, returns last entry ID; error if empty

### Task 1.3 — Create `snapshot/store.go` (snapshot load/save + baselines)

**Sub-tasks**:
- [ ] Create file `snapshot/store.go` with `package snapshot`
- [ ] Define `BaselinesFile` struct (`Default string`, `Named map[string]string`, JSON tags)
- [ ] Implement `KaleidoscopeDir(local bool) string` — returns `~/.kaleidoscope` (global) or `.kaleidoscope` (local); mirrors existing `browser.StateDir()` logic
- [ ] Implement `validateSnapshotID(id string) error` — rejects any ID not matching `^[a-zA-Z0-9\-_]+$` (path traversal guard per security spec §7.1)
- [ ] Implement `Load(dir, id string) (*Snapshot, error)`:
  - Validate ID via `validateSnapshotID`
  - Construct path `<dir>/snapshots/<id>.json`
  - Open file with `io.LimitReader` capped at 50 MB (§7.2)
  - Unmarshal JSON into `*Snapshot`; return descriptive error on failure
- [ ] Implement `Save(dir string, s *Snapshot) error`:
  - Ensure `<dir>/snapshots/` directory exists (`os.MkdirAll`)
  - Marshal snapshot to JSON and write to `<dir>/snapshots/<id>.json`
  - Read current index, append new `SnapshotMeta`, write updated index
- [ ] Implement `LoadBaselines(dir string) (*BaselinesFile, error)`:
  - Path: `<dir>/baselines.json`
  - Return clear error message if file does not exist (consumed by `RunDiff` to distinguish "not set" from parse error)

---

## Phase 2: `diff` Package — Pure-Function Diff Engine

> Implements the audit and element comparison algorithms. Zero Chrome dependency, zero disk I/O — takes two `*snapshot.Snapshot` values and returns `*Result`.

### Task 2.1 — Create `diff/engine.go` (result types)

**Sub-tasks**:
- [ ] Create file `diff/engine.go` with `package diff`
- [ ] Import only `"math"` and `"github.com/callmeradical/kaleidoscope/snapshot"`
- [ ] Define `Thresholds` struct (`PositionDelta float64`, `SizeDelta float64`)
- [ ] Implement `DefaultThresholds() Thresholds` returning `{PositionDelta: 4.0, SizeDelta: 4.0}`
- [ ] Define `CategoryDelta` struct (`Baseline int`, `Target int`, `Delta int`, JSON tags)
- [ ] Define `IssueChange` struct (`Category string`, `Selector string`, `Message string`, JSON tags)
- [ ] Define `AuditDiff` struct (`Categories map[string]CategoryDelta`, `NewIssues []IssueChange`, `ResolvedIssues []IssueChange`, JSON tags)
- [ ] Define `Delta2D` struct (`DX float64`, `DY float64`, JSON tags)
- [ ] Define `ElementChange` struct (`Role string`, `Name string`, `BaselineBox *snapshot.BoundingBox`, `TargetBox *snapshot.BoundingBox`, `PositionDelta *Delta2D`, `SizeDelta *Delta2D`, JSON tags with `omitempty`)
- [ ] Define `ElementDiff` struct (`Appeared []ElementChange`, `Disappeared []ElementChange`, `Moved []ElementChange`, `Resized []ElementChange`, JSON tags)
- [ ] Define `Result` struct (`SnapshotID string`, `BaselineID string`, `HasRegressions bool`, `Audit AuditDiff`, `Elements ElementDiff`, JSON tags)

### Task 2.2 — Implement `DiffAudit()` in `diff/engine.go`

**Sub-tasks**:
- [ ] Implement helper `issuesForCategory(data snapshot.AuditData, cat string) []snapshot.AuditIssue` — switch on category name to return the right slice
- [ ] Implement `DiffAudit(baseline, target snapshot.AuditData) AuditDiff`:
  - Iterate over `["contrast", "touch", "typography", "spacing"]`
  - For each category, build `map[selector]AuditIssue` for baseline and target issues
  - Compute `CategoryDelta{Baseline: len(baseMap), Target: len(targetMap), Delta: target-baseline}`
  - Detect new issues: selectors in `targetMap` not in `baseMap` → append `IssueChange` to `NewIssues`
  - Detect resolved issues: selectors in `baseMap` not in `targetMap` → append `IssueChange` to `ResolvedIssues`
  - Match key is `category + ":" + selector`; message is NOT used for matching

### Task 2.3 — Implement `DiffElements()` in `diff/engine.go`

**Sub-tasks**:
- [ ] Implement helper `semanticKey(e snapshot.Element) string` — returns `strings.ToLower(strings.TrimSpace(e.Role)) + ":" + strings.ToLower(strings.TrimSpace(e.Name))`
- [ ] Implement `DiffElements(baseline, target []snapshot.Element, t Thresholds) ElementDiff`:
  - Build `baselineMap map[string]snapshot.Element` keyed by semantic key; skip elements where `strings.TrimSpace(e.Name) == ""`
  - Build `targetMap map[string]snapshot.Element` with same exclusion rule
  - Detect appeared: keys in `targetMap` not in `baselineMap` → `ElementChange{TargetBox: &box}`
  - Detect disappeared: keys in `baselineMap` not in `targetMap` → `ElementChange{BaselineBox: &box}`
  - Detect moved: for keys in both, compute `dx = tBox.X - bBox.X`, `dy = tBox.Y - bBox.Y`; if `math.Abs(dx) > t.PositionDelta || math.Abs(dy) > t.PositionDelta` → `ElementChange{Moved, BaselineBox, TargetBox, PositionDelta{dx,dy}}`
  - Detect resized: for same matched elements, compute `dw = tBox.Width - bBox.Width`, `dh = tBox.Height - bBox.Height`; if `math.Abs(dw) > t.SizeDelta || math.Abs(dh) > t.SizeDelta` → `ElementChange{Resized, BaselineBox, TargetBox, SizeDelta{dw,dh}}`
  - Note: an element can appear in both `Moved` and `Resized` slices if both conditions hold

### Task 2.4 — Implement `Run()` entry point in `diff/engine.go`

**Sub-tasks**:
- [ ] Implement `Run(baseline, target *snapshot.Snapshot, t Thresholds) *Result`:
  - Call `DiffAudit(baseline.Audit, target.Audit)`
  - Call `DiffElements(baseline.Elements, target.Elements, t)`
  - Determine `HasRegressions`:
    - `true` if `len(auditDiff.NewIssues) > 0`
    - `true` if `len(elemDiff.Disappeared) > 0 || len(elemDiff.Moved) > 0 || len(elemDiff.Resized) > 0`
    - `false` if only `elemDiff.Appeared` is non-empty (appeared is informational)
  - Return `&Result{SnapshotID: target.ID, BaselineID: baseline.ID, HasRegressions: ..., Audit: auditDiff, Elements: elemDiff}`

---

## Phase 3: `cmd/diff.go` — CLI Command

> Wires snapshot storage helpers and the diff engine into the existing command-dispatch pattern.

### Task 3.1 — Create `cmd/diff.go`

**Sub-tasks**:
- [ ] Create `cmd/diff.go` with `package cmd`
- [ ] Implement `RunDiff(args []string)`:
  1. **Parse args**: extract optional snapshot ID using existing `getArg(args)` utility from `cmd/util.go`
  2. **Resolve kaleidoscope dir**: call `snapshot.KaleidoscopeDir(localFlag)` — use the same `--local` flag handling pattern as other commands (read from `os.Args` or pass via existing mechanism)
  3. **Load baselines.json**: call `snapshot.LoadBaselines(dir)`; on file-not-found error → `output.Fail("diff", "baselines.json not found", "No baseline set. Run: ks baseline set")` + `os.Exit(2)`
  4. **Check default baseline set**: if `baselines.Default == ""` → same `output.Fail` + `os.Exit(2)`
  5. **Load baseline snapshot**: call `snapshot.Load(dir, baselines.Default)`; on error → `output.Fail` + `os.Exit(2)`
  6. **Resolve target snapshot**:
     - If `snapshotID == ""`: call `snapshot.LatestID(dir)`; on error → `output.Fail("diff", "No snapshots found. Run: ks snapshot")` + `os.Exit(2)`
     - If `snapshotID != ""`: use as-is (validation happens inside `snapshot.Load`)
  7. **Load target snapshot**: call `snapshot.Load(dir, snapshotID)`; on error → `output.Fail` + `os.Exit(2)`
  8. **Run diff**: `result := diff.Run(baseline, target, diff.DefaultThresholds())`
  9. **Output**: `output.Success("diff", result)`
  10. **Exit**: `os.Exit(1)` if `result.HasRegressions`, else `os.Exit(0)`

### Task 3.2 — Wire `RunDiff` into `main.go`

**Sub-tasks**:
- [ ] Open `main.go` and locate the command dispatch switch statement
- [ ] Add `case "diff": cmd.RunDiff(cmdArgs)` in the switch block (after existing UX evaluation commands, before default)
- [ ] Add usage line to the usage string under the "UX Evaluation" section: `  diff [snapshot-id]      Compare snapshot against baseline (exit 1 on regression)`
- [ ] Add required imports for `diff` and `snapshot` packages in `cmd/diff.go` (not `main.go` — imports live in the cmd file)

---

## Phase 4: Tests

> Covers the diff engine (pure-function unit tests) and snapshot store helpers (using `t.TempDir()`). All tests must pass `go test ./...`.

### Task 4.1 — `diff/engine_test.go` — Audit diff tests

**Sub-tasks**:
- [ ] Create `diff/engine_test.go` with `package diff`
- [ ] `TestDiffAudit_NoChange`: identical `AuditData` on both sides → all `CategoryDelta.Delta == 0`, empty `NewIssues`, empty `ResolvedIssues`
- [ ] `TestDiffAudit_NewIssue`: one extra issue in target contrast → appears in `NewIssues` with correct `Category`, `Selector`, `Message`
- [ ] `TestDiffAudit_ResolvedIssue`: one issue in baseline touch not in target → appears in `ResolvedIssues`
- [ ] `TestDiffAudit_AllCategories`: each of the four categories independently has a different delta — assert all four `CategoryDelta` entries are correct
- [ ] `TestDiffAudit_SelectorMatching`: two issues with the same message but different selectors → counted as two separate issues (both in `NewIssues`)
- [ ] `TestDiffAudit_MessageNotUsedForMatching`: same selector, different message → treated as same issue (no new issue)

### Task 4.2 — `diff/engine_test.go` — Element diff tests

**Sub-tasks**:
- [ ] `TestDiffElements_Appeared`: element in target only → in `Appeared`, `TargetBox` set, `BaselineBox` nil
- [ ] `TestDiffElements_Disappeared`: element in baseline only → in `Disappeared`, `BaselineBox` set, `TargetBox` nil
- [ ] `TestDiffElements_Moved`: element position shifts by more than 4px → in `Moved` with correct `PositionDelta`
- [ ] `TestDiffElements_NotMoved`: element position shifts by ≤4px → not in `Moved`
- [ ] `TestDiffElements_Resized`: element size changes by more than 4px → in `Resized` with correct `SizeDelta`
- [ ] `TestDiffElements_EmptyNameSkipped`: element with empty/whitespace `Name` → excluded from matching, not in any output slice
- [ ] `TestDiffElements_SemanticKey`: element with `role="button"`, `name="Submit"` in both snapshots but at different positions → matched by semantic key, appears in `Moved` not `Appeared`/`Disappeared`

### Task 4.3 — `diff/engine_test.go` — `HasRegressions` tests

**Sub-tasks**:
- [ ] `TestHasRegressions_NewAuditIssue`: new issue in audit → `HasRegressions == true`
- [ ] `TestHasRegressions_Disappeared`: element disappeared → `HasRegressions == true`
- [ ] `TestHasRegressions_Moved`: element moved beyond threshold → `HasRegressions == true`
- [ ] `TestHasRegressions_Resized`: element resized beyond threshold → `HasRegressions == true`
- [ ] `TestHasRegressions_AppearedOnly`: only new elements appeared, no audit regressions → `HasRegressions == false`
- [ ] `TestHasRegressions_ResolvedOnly`: issues resolved, nothing new → `HasRegressions == false`

### Task 4.4 — `snapshot/store_test.go` — Storage helper tests

**Sub-tasks**:
- [ ] Create `snapshot/store_test.go` with `package snapshot`
- [ ] `TestLoadBaselines_Missing`: call `LoadBaselines` on a dir with no `baselines.json` → returns non-nil error
- [ ] `TestLoadBaselines_NoDefault`: write `baselines.json` with empty `default` field → returns `BaselinesFile` with `Default == ""`
- [ ] `TestLatestID_Empty`: write empty index (`{"entries": []}`) → `LatestID` returns error
- [ ] `TestLatestID_ReturnsLast`: write index with two entries → `LatestID` returns the second entry's ID
- [ ] `TestLoad_PathTraversal`: call `Load(dir, "../etc/passwd")` → returns error containing "invalid snapshot ID"
- [ ] `TestLoad_ValidID`: write a minimal snapshot JSON file → `Load` returns correct `*Snapshot`
- [ ] `TestSave_WritesFileAndUpdatesIndex`: call `Save` with a minimal `Snapshot` → file appears at correct path, `index.json` contains the new entry

---

## Phase 5: Verification

> Final checks to ensure the implementation compiles, tests pass, and output shape matches the spec.

### Task 5.1 — Compile check

**Sub-tasks**:
- [ ] Run `go build ./...` and fix any compilation errors
- [ ] Verify no circular imports (snapshot ← diff ← cmd ← main; not the other way)

### Task 5.2 — Test run

**Sub-tasks**:
- [ ] Run `go test ./...` and ensure all tests pass with zero failures
- [ ] Verify test coverage includes all 15 diff engine tests and 7 store tests listed in the tech spec

### Task 5.3 — Manual output shape validation

**Sub-tasks**:
- [ ] Review `RunDiff` output against the JSON shapes defined in tech spec §5:
  - Success (no regressions): `ok: true`, `hasRegressions: false`, all four category keys present, empty arrays for issues and elements
  - Regression detected: `ok: true`, `hasRegressions: true`, correct `newIssues`/`disappeared` populated
  - Error (no baseline): `ok: false`, `error` and `hint` fields present
- [ ] Confirm exit codes: 0 (no regression), 1 (regression), 2 (error/misconfiguration)

### Task 5.4 — Security validation

**Sub-tasks**:
- [ ] Confirm `validateSnapshotID` rejects `../foo`, `../../etc/passwd`, `foo/bar`
- [ ] Confirm `io.LimitReader` with 50 MB cap is applied in `Load`
- [ ] Confirm no `os/exec`, shell invocations, or dynamic code loading in diff or snapshot packages

---

## File Manifest

| File | Action | Description |
|------|--------|-------------|
| `snapshot/snapshot.go` | **Create** | Core data model: Snapshot, AuditData, AuditIssue, Element, BoundingBox, Viewport |
| `snapshot/index.go` | **Create** | SnapshotIndex, SnapshotMeta types + read/write helpers + LatestID |
| `snapshot/store.go` | **Create** | BaselinesFile, KaleidoscopeDir, Load, Save, LoadBaselines, validateSnapshotID |
| `snapshot/store_test.go` | **Create** | Unit tests for store helpers (7 tests) |
| `diff/engine.go` | **Create** | Thresholds, all Result types, Run(), DiffAudit(), DiffElements() |
| `diff/engine_test.go` | **Create** | Unit tests for diff engine (15 tests) |
| `cmd/diff.go` | **Create** | RunDiff() command implementation |
| `main.go` | **Modify** | Add `case "diff"` to switch + usage line |

---

## Dependency Order

```
Phase 1 (snapshot package)
  └── Phase 2 (diff engine — imports snapshot)
        └── Phase 3 (cmd/diff.go — imports diff + snapshot)
              └── Phase 3.2 (main.go wiring)
Phase 4 (tests — can be written alongside phases 1–3)
Phase 5 (verification — must follow all prior phases)
```

Phases 1, 2, and 4 can be worked in parallel within each phase; Phase 3 requires Phase 1 and Phase 2 to be complete.
