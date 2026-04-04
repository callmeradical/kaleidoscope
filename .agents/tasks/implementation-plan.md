# Implementation Plan: US-003 — Audit and Element Diff Engine

## Overview

Implements `ks diff [snapshot-id]` — a pure-function diff engine that compares a snapshot against a baseline and outputs structured JSON describing audit regressions and element-level DOM changes.

**Depends on:** US-002 snapshot infrastructure (`snapshot` package, `.kaleidoscope/` storage layout, `baselines.json`). Since that package does not yet exist, Phase 1 creates it as a prerequisite.

**Quality gate:** `go test ./...` must pass with no failures.

---

## Phase 1: Snapshot Package (US-002 Foundation)

The `snapshot` package provides the shared data types and I/O functions required by both the snapshot capture commands (US-002) and the diff engine (US-003). It has no browser dependency.

### Task 1.1 — Define snapshot types (`snapshot/types.go`)

- Sub-task 1.1.1: Create `snapshot/types.go` with package declaration `package snapshot`.
- Sub-task 1.1.2: Define `AuditSummary` struct with fields: `ContrastViolations int`, `TouchViolations int`, `TypographyWarnings int`, `TotalIssues int` — all JSON-tagged.
- Sub-task 1.1.3: Define `AuditIssueRecord` struct with fields: `Category string` (`"contrast" | "touch" | "typography"`), `Selector string` — JSON-tagged.
- Sub-task 1.1.4: Define `AuditSnapshot` struct with fields: `Summary AuditSummary`, `Issues []AuditIssueRecord` — JSON-tagged.
- Sub-task 1.1.5: Define `ElementRect` struct with fields: `X float64`, `Y float64`, `Width float64`, `Height float64` — JSON-tagged.
- Sub-task 1.1.6: Define `ElementRecord` struct with fields: `Role string`, `Name string`, `Rect *ElementRect` — JSON-tagged (Rect is optional, may be nil if position not captured).
- Sub-task 1.1.7: Define top-level `Snapshot` struct with fields: `ID string`, `CreatedAt time.Time`, `URL string`, `AuditData AuditSnapshot` (json tag: `"audit"`), `Elements []ElementRecord` — all JSON-tagged.

### Task 1.2 — Implement storage path helpers (`snapshot/storage.go`)

- Sub-task 1.2.1: Create `snapshot/storage.go` with package declaration `package snapshot`.
- Sub-task 1.2.2: Implement `SnapshotsDir() string` — returns the resolved path to `.kaleidoscope/snapshots/` relative to the current working directory.
- Sub-task 1.2.3: Implement `BaselinesFile() string` — returns path to `.kaleidoscope/baselines.json`.
- Sub-task 1.2.4: Implement `sanitizeID(id string) error` — rejects any snapshot ID containing `/`, `\`, `..`, or null bytes; returns a descriptive error if invalid. Used to prevent path traversal.
- Sub-task 1.2.5: Implement `snapshotPath(id string) (string, error)` — calls `sanitizeID`, joins the sanitized ID with `SnapshotsDir()`, calls `filepath.Clean`, and verifies the resolved path is still under `SnapshotsDir()` (double-checks for traversal). Returns an error if the path escapes the root.

### Task 1.3 — Implement snapshot load functions (`snapshot/load.go`)

- Sub-task 1.3.1: Create `snapshot/load.go` with package declaration `package snapshot`.
- Sub-task 1.3.2: Implement `LoadByID(id string) (*Snapshot, error)` — calls `snapshotPath(id)`, reads `<path>/snapshot.json`, unmarshals JSON into `*Snapshot`, returns error with context if file not found or invalid.
- Sub-task 1.3.3: Implement `LoadLatest() (*Snapshot, error)` — reads all entries in `SnapshotsDir()`, sorts by directory name lexicographically descending (since IDs are timestamps), loads and returns the first valid snapshot. Returns a descriptive error if directory is empty or no valid snapshot exists.
- Sub-task 1.3.4: Implement `LoadBaseline() (*Snapshot, error)` — reads `.kaleidoscope/baselines.json`, unmarshals `{ "baseline": "<id>" }`, calls `LoadByID` with that ID. Returns a specific sentinel error message `"no baseline set"` if the file does not exist, so the caller can produce the correct hint (`"Run: ks baseline set <snapshot-id>"`).

---

## Phase 2: Diff Package (Core Pure-Function Logic)

All types and logic live in a new top-level `diff/` package. No I/O, no browser dependency, no subprocess calls — pure functions only.

### Task 2.1 — Define diff types (`diff/types.go`)

- Sub-task 2.1.1: Create `diff/types.go` with package declaration `package diff`.
- Sub-task 2.1.2: Define `CategoryDelta` struct: `Baseline int`, `Snapshot int`, `Delta int` (snapshot − baseline; positive = regression) — JSON-tagged.
- Sub-task 2.1.3: Define `CategoryDeltas` struct: `Contrast CategoryDelta`, `TouchTargets CategoryDelta`, `Typography CategoryDelta` — JSON-tagged (`"contrast"`, `"touchTargets"`, `"typography"`).
- Sub-task 2.1.4: Define `AuditIssue` struct: `Category string`, `Selector string` — JSON-tagged. (Normalized issue, keyed by selector for diff purposes.)
- Sub-task 2.1.5: Define `IssueDelta` struct: `New []AuditIssue`, `Resolved []AuditIssue` — JSON-tagged (use `json:"new"` and `json:"resolved"`; initialize as empty slices, never nil, so JSON output is `[]` not `null`).
- Sub-task 2.1.6: Define `AuditDiff` struct: `Categories CategoryDeltas`, `Issues IssueDelta`, `HasRegression bool` — JSON-tagged.
- Sub-task 2.1.7: Define `ElementRect` struct (mirroring snapshot.ElementRect, keeping diff package self-contained): `X float64`, `Y float64`, `Width float64`, `Height float64` — JSON-tagged.
- Sub-task 2.1.8: Define `RectDelta` struct: `DX float64`, `DY float64`, `DW float64`, `DH float64` — JSON-tagged.
- Sub-task 2.1.9: Define `ElementChange` struct: `Role string`, `Name string`, `Baseline *ElementRect` (omitempty), `Snapshot *ElementRect` (omitempty), `Delta *RectDelta` (omitempty) — JSON-tagged.
- Sub-task 2.1.10: Define `ElementDiff` struct: `Appeared []ElementChange`, `Disappeared []ElementChange`, `Moved []ElementChange`, `Resized []ElementChange`, `HasRegression bool` — JSON-tagged. Initialize all slices as empty (not nil).
- Sub-task 2.1.11: Define `DiffResult` struct: `SnapshotID string`, `BaselineID string`, `Audit AuditDiff`, `Elements ElementDiff`, `HasRegression bool` — JSON-tagged.

### Task 2.2 — Implement audit diff logic (`diff/audit.go`)

- Sub-task 2.2.1: Create `diff/audit.go` with package declaration `package diff`.
- Sub-task 2.2.2: Implement helper `extractCounts(audit snapshot.AuditSnapshot) (contrast, touch, typo int)` — reads `.Summary.ContrastViolations`, `.Summary.TouchViolations`, `.Summary.TypographyWarnings`.
- Sub-task 2.2.3: Implement `ComputeAuditDiff(baselineAudit, snapshotAudit snapshot.AuditSnapshot) AuditDiff`:
  - Step 1: Call `extractCounts` on both inputs.
  - Step 2: Build `CategoryDeltas` — for each category, set `Baseline`, `Snapshot`, `Delta = snapshot − baseline`.
  - Step 3: Build a `map[string]string` (selector → category) from `baselineAudit.Issues` for O(1) lookup.
  - Step 4: Build a `map[string]string` from `snapshotAudit.Issues`.
  - Step 5: Compute `IssueDelta.New` — issues present in snapshot map but not baseline map (by selector key).
  - Step 6: Compute `IssueDelta.Resolved` — issues present in baseline map but not snapshot map.
  - Step 7: Set `HasRegression = contrast.Delta > 0 || touch.Delta > 0 || typo.Delta > 0`.
  - Step 8: Return populated `AuditDiff`.

### Task 2.3 — Implement element diff logic (`diff/elements.go`)

- Sub-task 2.3.1: Create `diff/elements.go` with package declaration `package diff`.
- Sub-task 2.3.2: Implement `SemanticKey(role, name string) string` — returns `strings.ToLower(strings.TrimSpace(role)) + ":" + strings.ToLower(strings.TrimSpace(name))`. Handles empty name correctly (key becomes `"role:"`).
- Sub-task 2.3.3: Implement `toElementRect(r *snapshot.ElementRect) *ElementRect` — converts from the snapshot package type to the diff package type (field-by-field copy); returns nil if input is nil.
- Sub-task 2.3.4: Implement `ComputeElementDiff(baseline, snapshot []snapshot.ElementRecord, posThreshold, sizeThreshold float64) ElementDiff`:
  - Step 1: Build `baselineMap map[string]snapshot.ElementRecord` keyed by `SemanticKey(r.Role, r.Name)`.
  - Step 2: Build `snapshotMap map[string]snapshot.ElementRecord` keyed by `SemanticKey(r.Role, r.Name)`.
  - Step 3: Appeared — iterate `snapshotMap`; for each key not in `baselineMap`, append `ElementChange{Role, Name, Snapshot: toElementRect(r.Rect)}` to `Appeared`.
  - Step 4: Disappeared — iterate `baselineMap`; for each key not in `snapshotMap`, append `ElementChange{Role, Name, Baseline: toElementRect(r.Rect)}` to `Disappeared`.
  - Step 5: For keys in both maps: if both have non-nil `Rect`, compute `dx = snap.X − base.X`, `dy = snap.Y − base.Y`, `dw = snap.Width − base.Width`, `dh = snap.Height − base.Height`. If `abs(dx) > posThreshold || abs(dy) > posThreshold` → append to `Moved` with `Baseline`, `Snapshot`, and `Delta`. If `abs(dw) > sizeThreshold || abs(dh) > sizeThreshold` → append to `Resized` with same fields. (An element can appear in both `Moved` and `Resized`.)
  - Step 6: Set `HasRegression = len(Appeared) > 0 || len(Disappeared) > 0 || len(Moved) > 0 || len(Resized) > 0`.
  - Step 7: Sort all result slices by SemanticKey for deterministic output.
  - Step 8: Return populated `ElementDiff`.
- Sub-task 2.3.5: Implement `abs(v float64) float64` unexported helper — returns `math.Abs(v)` (import `"math"`).

### Task 2.4 — Write unit tests for audit diff (`diff/audit_test.go`)

- Sub-task 2.4.1: Create `diff/audit_test.go` with package declaration `package diff`.
- Sub-task 2.4.2: Test case — identical audits: `HasRegression: false`, all `CategoryDelta.Delta == 0`, `IssueDelta.New` and `Resolved` both empty.
- Sub-task 2.4.3: Test case — snapshot has more contrast violations than baseline: `HasRegression: true`, `Categories.Contrast.Delta > 0`.
- Sub-task 2.4.4: Test case — snapshot resolves all touch violations (baseline had some, snapshot has zero): `HasRegression: false`, `Categories.TouchTargets.Delta < 0`.
- Sub-task 2.4.5: Test case — per-issue tracking: baseline has issue with selector `"p"` (contrast), snapshot adds issue with selector `"h1"` (contrast) and removes `"p"`. Verify `IssueDelta.New` contains `{Category:"contrast", Selector:"h1"}` and `IssueDelta.Resolved` contains `{Category:"contrast", Selector:"p"}`.
- Sub-task 2.4.6: Test case — snapshot has more typography warnings: `HasRegression: true`, `Categories.Typography.Delta > 0`.

### Task 2.5 — Write unit tests for element diff (`diff/elements_test.go`)

- Sub-task 2.5.1: Create `diff/elements_test.go` with package declaration `package diff`.
- Sub-task 2.5.2: Test case — identical trees (same elements, same rects): `ElementDiff` has empty slices everywhere, `HasRegression: false`.
- Sub-task 2.5.3: Test case — element present in snapshot but not baseline: appears in `Appeared` with correct `Role`, `Name`, `Snapshot` rect; `HasRegression: true`.
- Sub-task 2.5.4: Test case — element present in baseline but not snapshot: appears in `Disappeared` with correct `Baseline` rect; `HasRegression: true`.
- Sub-task 2.5.5: Test case — same role+name, position shifted by more than `posThreshold`: appears in `Moved` with correct `Delta`; `HasRegression: true`.
- Sub-task 2.5.6: Test case — same role+name, size changed by more than `sizeThreshold`: appears in `Resized` with correct `Delta`; `HasRegression: true`.
- Sub-task 2.5.7: Test case — same role+name, position shifted by less than `posThreshold`: not in `Moved`, `HasRegression: false`.
- Sub-task 2.5.8: Test case — element with empty name: `SemanticKey` produces `"role:"` and matching still works correctly (no panic, correct diff behavior).
- Sub-task 2.5.9: Test case — element both moved and resized: appears in both `Moved` and `Resized`.
- Sub-task 2.5.10: Test `SemanticKey` directly: verify case-folding and trimming behavior (e.g., `" Button "` → `"button:"`).

---

## Phase 3: CLI Integration

### Task 3.1 — Implement `cmd/diff.go`

- Sub-task 3.1.1: Create `cmd/diff.go` with package declaration `package cmd`.
- Sub-task 3.1.2: Add imports: `"os"`, `"strconv"`, `"github.com/callmeradical/kaleidoscope/diff"`, `"github.com/callmeradical/kaleidoscope/output"`, `"github.com/callmeradical/kaleidoscope/snapshot"`.
- Sub-task 3.1.3: Implement `parseFlagFloat(args []string, flag string, defaultVal float64) float64` — reads the string value via `getFlagValue(args, flag)`, parses with `strconv.ParseFloat`; returns `defaultVal` if flag is absent or unparseable.
- Sub-task 3.1.4: Implement `RunDiff(args []string)`:
  - Step 1: Parse `snapshotID := getArg(args)` (empty = use latest).
  - Step 2: Parse `posThreshold := parseFlagFloat(args, "--pos-threshold", 4.0)`.
  - Step 3: Parse `sizeThreshold := parseFlagFloat(args, "--size-threshold", 4.0)`.
  - Step 4: Load baseline via `snapshot.LoadBaseline()`; if error, call `output.Fail("diff", err, "Run: ks baseline set <snapshot-id>")` and `os.Exit(2)`.
  - Step 5: Load target snapshot — if `snapshotID == ""`, call `snapshot.LoadLatest()`; otherwise call `snapshot.LoadByID(snapshotID)`. On error, call `output.Fail("diff", err, "")` and `os.Exit(2)`.
  - Step 6: Call `diff.ComputeAuditDiff(baseline.AuditData, target.AuditData)`.
  - Step 7: Call `diff.ComputeElementDiff(baseline.Elements, target.Elements, posThreshold, sizeThreshold)`.
  - Step 8: Assemble `diff.DiffResult{SnapshotID: target.ID, BaselineID: baseline.ID, Audit: auditDiff, Elements: elemDiff, HasRegression: auditDiff.HasRegression || elemDiff.HasRegression}`.
  - Step 9: Call `output.Success("diff", result)`.
  - Step 10: If `result.HasRegression`, call `os.Exit(1)`.

### Task 3.2 — Update `cmd/util.go` — register new flags

- Sub-task 3.2.1: Read `cmd/util.go` (already done; current flags-with-values list is in `getNonFlagArgs`).
- Sub-task 3.2.2: Add `"--pos-threshold"` and `"--size-threshold"` to the `if a == ...` condition in `getNonFlagArgs` so their values are correctly skipped when collecting positional args.

### Task 3.3 — Update `cmd/usage.go` — add "diff" usage entry

- Sub-task 3.3.1: Add a new key `"diff"` to the `CommandUsage` map in `cmd/usage.go` with the following content:
  ```
  ks diff [snapshot-id] [--pos-threshold N] [--size-threshold N]

  Compare a snapshot against the baseline and report regressions as structured JSON.

  Arguments:
    snapshot-id           ID of snapshot to compare (default: latest)

  Options:
    --pos-threshold N     Pixel threshold to classify position change as "moved" (default: 4.0)
    --size-threshold N    Pixel threshold to classify size change as "resized" (default: 4.0)

  Exit codes:
    0   No regressions detected
    1   One or more regressions detected
    2   Error (no baseline, snapshot not found, I/O error)

  Output:
    { "ok": true, "command": "diff", "result": {
      "snapshotId": "...", "baselineId": "...", "hasRegression": true,
      "audit": { "hasRegression": true, "categories": {...}, "issues": {...} },
      "elements": { "hasRegression": false, "appeared": [], "disappeared": [], "moved": [], "resized": [] }
    }}

  Examples:
    ks diff                      # Compare latest snapshot to baseline
    ks diff 1712000000000        # Compare specific snapshot to baseline
    ks diff --pos-threshold 8    # Custom position threshold

  Notes:
    Returns error (exit 2) if no baseline has been set.
    Run 'ks baseline set <snapshot-id>' to set a baseline.
  ```

### Task 3.4 — Update `main.go` — wire "diff" command

- Sub-task 3.4.1: Add `case "diff": cmd.RunDiff(cmdArgs)` to the `switch command` block in `main.go`, after the existing cases (e.g., after `"install-skills"`).
- Sub-task 3.4.2: Add a "Snapshot & Regression" section to the `usage` string in `main.go`:
  ```
  Snapshot & Regression:
    diff [snapshot-id]      Compare snapshot to baseline; exit 1 on regression
  ```

---

## Phase 4: Quality Gate

### Task 4.1 — Verify the build compiles

- Sub-task 4.1.1: Run `go build ./...` from `/workspace`. Fix any compilation errors (import cycles, missing symbols, type mismatches).

### Task 4.2 — Run the full test suite

- Sub-task 4.2.1: Run `go test ./...` from `/workspace`.
- Sub-task 4.2.2: Confirm all tests pass — specifically `diff/audit_test.go` and `diff/elements_test.go`.
- Sub-task 4.2.3: Fix any test failures before declaring the story done.

---

## File Inventory

| File | Action | Notes |
|---|---|---|
| `snapshot/types.go` | Create | Snapshot, AuditSnapshot, AuditSummary, AuditIssueRecord, ElementRecord, ElementRect types |
| `snapshot/storage.go` | Create | SnapshotsDir, BaselinesFile, sanitizeID, snapshotPath |
| `snapshot/load.go` | Create | LoadByID, LoadLatest, LoadBaseline |
| `diff/types.go` | Create | DiffResult, AuditDiff, ElementDiff, CategoryDeltas, IssueDelta, ElementChange, ElementRect, RectDelta |
| `diff/audit.go` | Create | ComputeAuditDiff, extractCounts |
| `diff/elements.go` | Create | ComputeElementDiff, SemanticKey, toElementRect, abs |
| `diff/audit_test.go` | Create | 6 unit test cases for audit diff |
| `diff/elements_test.go` | Create | 10 unit test cases for element diff |
| `cmd/diff.go` | Create | RunDiff, parseFlagFloat |
| `cmd/util.go` | Edit | Add --pos-threshold, --size-threshold to getNonFlagArgs |
| `cmd/usage.go` | Edit | Add "diff" entry to CommandUsage map |
| `main.go` | Edit | Add "diff" case to switch; add usage string section |

---

## Security Checklist

- [ ] `snapshot/storage.go`: `sanitizeID` rejects `/`, `\`, `..`, and null bytes.
- [ ] `snapshot/storage.go`: `snapshotPath` verifies resolved path is under `SnapshotsDir()` after `filepath.Clean`.
- [ ] `cmd/diff.go`: No shell execution anywhere in the diff path.
- [ ] `cmd/diff.go`: Exit code 1 = regression (not error); exit code 2 = error. These are kept distinct.
- [ ] `diff/` package: No I/O, no subprocess calls — pure functions only.
