# Tech Spec: Audit and Element Diff Engine (US-003)

**Story:** `task-0d599df2a460a91e`
**Depends on:** US-001 (Snapshot Capture), US-002 (Project Config / Baseline Manager)
**Date:** 2026-04-04

---

## 1. Architecture Overview

US-003 introduces `ks diff [snapshot-id]` — a pure-function diff engine that takes two snapshots and computes structured deltas without requiring a running Chrome browser.

```
main.go
  └── case "diff": cmd.RunDiff(args)
        └── snapshot.Load(id)            // reads snapshot from .kaleidoscope/snapshots/
        └── snapshot.LoadBaseline()      // reads .kaleidoscope/baselines.json
        └── diff.ComputeAuditDelta(a, b) // pure function: AuditData → AuditDelta
        └── diff.ComputeElementDelta(a, b) // pure function: []AXNode → ElementDelta
        └── output.Success("diff", DiffResult{...})
        └── os.Exit(1) if regressions
```

**Key principle:** The diff engine (`diff/`) is a pure-function package with zero Chrome dependency. All data comes from previously-saved snapshot files written by US-001.

---

## 2. Component Design

### 2.1 New Package: `diff/`

`diff/audit.go` — audit delta computation
`diff/element.go` — element delta computation
`diff/types.go` — shared types for diff output

### 2.2 New Command: `cmd/diff.go`

Implements `ks diff [snapshot-id]` following the existing `Run*` command pattern.

### 2.3 Dependencies on US-001 Data Structures

The diff engine consumes snapshot data produced by US-001. The following types are assumed to exist in a `snapshot` package (introduced by US-001 and US-002):

```go
// snapshot/types.go (US-001)
type Snapshot struct {
    ID          string    `json:"id"`           // e.g. "20260404-153012-abc12"
    CreatedAt   time.Time `json:"createdAt"`
    URL         string    `json:"url"`
    GitCommit   string    `json:"gitCommit,omitempty"`
    AuditData   AuditData `json:"auditData"`
    AXNodes     []AXNode  `json:"axNodes"`
    ScreenshotPath string `json:"screenshotPath,omitempty"`
}

type AuditData struct {
    ContrastIssues  []AuditIssue `json:"contrastIssues"`
    TouchIssues     []AuditIssue `json:"touchIssues"`
    TypographyIssues []AuditIssue `json:"typographyIssues"`
    SpacingIssues   []AuditIssue `json:"spacingIssues"`
}

type AuditIssue struct {
    Selector string `json:"selector"`
    Message  string `json:"message"`
    Category string `json:"category"` // "contrast" | "touch" | "typography" | "spacing"
}

type AXNode struct {
    Role       string            `json:"role"`
    Name       string            `json:"name"`
    BoundingBox *BoundingBox     `json:"boundingBox,omitempty"`
    Properties map[string]any    `json:"properties,omitempty"`
}

type BoundingBox struct {
    X      float64 `json:"x"`
    Y      float64 `json:"y"`
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}
```

```go
// snapshot/store.go (US-001)
func Load(id string) (*Snapshot, error)      // load by ID
func LoadLatest() (*Snapshot, error)         // load most recent
func LoadBaseline() (*Snapshot, error)       // load baseline (from baselines.json, US-002)
```

---

## 3. Data Models

### 3.1 `diff/types.go`

```go
package diff

// DiffResult is the top-level output of ks diff.
type DiffResult struct {
    SnapshotID  string        `json:"snapshotId"`
    BaselineID  string        `json:"baselineId"`
    Regressions bool          `json:"regressions"`
    Audit       AuditDelta    `json:"audit"`
    Elements    ElementDelta  `json:"elements"`
}

// AuditDelta summarizes changes in audit issue counts and per-issue status.
type AuditDelta struct {
    Categories map[string]CategoryDelta `json:"categories"`
    NewIssues  []IssueDiff              `json:"newIssues"`
    Resolved   []IssueDiff              `json:"resolved"`
    HasRegression bool                  `json:"hasRegression"`
}

// CategoryDelta holds count changes per audit category.
type CategoryDelta struct {
    Category string `json:"category"`
    Baseline int    `json:"baseline"`
    Current  int    `json:"current"`
    Delta    int    `json:"delta"` // positive = regression (more issues)
}

// IssueDiff describes a single issue that is new or resolved.
type IssueDiff struct {
    Selector string `json:"selector"`
    Category string `json:"category"`
    Message  string `json:"message"`
    Status   string `json:"status"` // "new" | "resolved"
}

// ElementDelta summarizes DOM element changes via accessibility tree comparison.
type ElementDelta struct {
    Appeared   []ElementChange `json:"appeared"`
    Disappeared []ElementChange `json:"disappeared"`
    Moved      []ElementChange `json:"moved"`
    Resized    []ElementChange `json:"resized"`
    HasRegression bool         `json:"hasRegression"`
}

// ElementChange describes a single element that changed state.
type ElementChange struct {
    Role     string   `json:"role"`
    Name     string   `json:"name"`
    Identity string   `json:"identity"` // "role:name" semantic key
    Before   *ElementState `json:"before,omitempty"`
    After    *ElementState `json:"after,omitempty"`
    Delta    *MoveDelta    `json:"delta,omitempty"`
}

// ElementState captures position and size at a point in time.
type ElementState struct {
    X      float64 `json:"x"`
    Y      float64 `json:"y"`
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}

// MoveDelta holds the numeric difference in position/size.
type MoveDelta struct {
    DX float64 `json:"dx"`
    DY float64 `json:"dy"`
    DW float64 `json:"dw"`
    DH float64 `json:"dh"`
}
```

### 3.2 Thresholds (constants in `diff/element.go`)

```go
const (
    // PositionThreshold is the minimum pixel delta to report as "moved".
    PositionThreshold = 4.0

    // SizeThreshold is the minimum pixel delta in width or height to report as "resized".
    SizeThreshold = 4.0
)
```

---

## 4. API Definitions

### 4.1 CLI Command

```
ks diff [<snapshot-id>] [--local]
```

| Argument | Description |
|---|---|
| `<snapshot-id>` | Optional. Specific snapshot to compare. Defaults to latest. |
| `--local` | Use project-local `.kaleidoscope/` state directory. |

**Exit codes:**
- `0` — No regressions detected (diff may still show resolved issues)
- `1` — Regressions detected (new audit issues or elements disappeared/moved/resized beyond threshold)
- `2` — Error (no baseline, snapshot not found, etc.)

**Output (stdout, always JSON):**
```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "snapshotId": "20260404-153012-abc12",
    "baselineId": "20260401-090000-def34",
    "regressions": false,
    "audit": {
      "categories": {
        "contrast": { "category": "contrast", "baseline": 3, "current": 3, "delta": 0 },
        "touch":    { "category": "touch",    "baseline": 1, "current": 2, "delta": 1 },
        "typography": { "category": "typography", "baseline": 0, "current": 0, "delta": 0 },
        "spacing":  { "category": "spacing",  "baseline": 2, "current": 1, "delta": -1 }
      },
      "newIssues": [
        { "selector": "button.submit", "category": "touch", "message": "...", "status": "new" }
      ],
      "resolved": [
        { "selector": ".card p", "category": "spacing", "message": "...", "status": "resolved" }
      ],
      "hasRegression": true
    },
    "elements": {
      "appeared":    [],
      "disappeared": [],
      "moved":       [],
      "resized":     [],
      "hasRegression": false
    }
  }
}
```

**Error output (no baseline):**
```json
{
  "ok": false,
  "command": "diff",
  "error": "no baseline set",
  "hint": "Run: ks snapshot set-baseline <snapshot-id>"
}
```

### 4.2 Pure-Function API (`diff` package)

```go
// ComputeAuditDelta compares two AuditData structs and returns an AuditDelta.
// No Chrome, no filesystem access. Pure inputs → pure output.
func ComputeAuditDelta(baseline, current snapshot.AuditData) AuditDelta

// ComputeElementDelta compares two AXNode slices and returns an ElementDelta.
// Matching is by semantic identity: canonicalize(role + ":" + name).
// No Chrome, no filesystem access. Pure inputs → pure output.
func ComputeElementDelta(baseline, current []snapshot.AXNode) ElementDelta
```

---

## 5. Algorithm Design

### 5.1 Audit Delta (`diff.ComputeAuditDelta`)

```
1. Group baseline issues by category → map[category][]AuditIssue
2. Group current issues by category → map[category][]AuditIssue
3. For each category in {"contrast", "touch", "typography", "spacing"}:
   a. Compute count delta: len(current) - len(baseline)
   b. Build selector sets for each side
   c. Issues in current but not in baseline → IssueDiff{Status: "new"}
   d. Issues in baseline but not in current → IssueDiff{Status: "resolved"}
4. HasRegression = any category with delta > 0
```

**Issue matching key:** `category + ":" + selector` (normalized to lowercase, trimmed).

### 5.2 Element Delta (`diff.ComputeElementDelta`)

```
1. Build baseline map: semanticID(node) → AXNode
2. Build current map:  semanticID(node) → AXNode
3. semanticID = strings.ToLower(strings.TrimSpace(role)) + ":" +
                strings.ToLower(strings.TrimSpace(name))
   - Skip nodes where role == "" AND name == ""
4. Keys only in current  → Appeared
5. Keys only in baseline → Disappeared
6. Keys in both:
   a. If BoundingBox missing in either → skip positional check
   b. |dx| > PositionThreshold || |dy| > PositionThreshold → Moved
   c. |dw| > SizeThreshold || |dh| > SizeThreshold → Resized
   d. An element can be both Moved and Resized
7. HasRegression = len(Disappeared) > 0 || len(Moved) > 0 || len(Resized) > 0
   Note: Appeared is informational only, not a regression.
```

**Overall `DiffResult.Regressions`** = `Audit.HasRegression || Elements.HasRegression`

---

## 6. File Structure

```
/workspace/
├── diff/
│   ├── types.go        # DiffResult, AuditDelta, ElementDelta, IssueDiff, ElementChange, etc.
│   ├── audit.go        # ComputeAuditDelta(baseline, current AuditData) AuditDelta
│   ├── element.go      # ComputeElementDelta(baseline, current []AXNode) ElementDelta
│   └── diff_test.go    # Unit tests (pure functions, no browser)
├── cmd/
│   └── diff.go         # RunDiff(args []string) — CLI glue
└── main.go             # Add case "diff": cmd.RunDiff(cmdArgs)
```

---

## 7. `cmd/diff.go` Implementation Sketch

```go
package cmd

import (
    "os"
    "github.com/callmeradical/kaleidoscope/diff"
    "github.com/callmeradical/kaleidoscope/output"
    "github.com/callmeradical/kaleidoscope/snapshot"
)

func RunDiff(args []string) {
    snapshotID := getArg(args) // empty string if not provided

    // Load baseline (from baselines.json, managed by US-002)
    baseline, err := snapshot.LoadBaseline()
    if err != nil {
        output.Fail("diff", fmt.Errorf("no baseline set"), "Run: ks snapshot set-baseline <snapshot-id>")
        os.Exit(2)
    }

    // Load target snapshot (latest or specific ID)
    var current *snapshot.Snapshot
    if snapshotID == "" {
        current, err = snapshot.LoadLatest()
    } else {
        current, err = snapshot.Load(snapshotID)
    }
    if err != nil {
        output.Fail("diff", err, "Run: ks snapshot list")
        os.Exit(2)
    }

    // Compute deltas (pure functions, no Chrome)
    auditDelta   := diff.ComputeAuditDelta(baseline.AuditData, current.AuditData)
    elementDelta := diff.ComputeElementDelta(baseline.AXNodes, current.AXNodes)

    result := diff.DiffResult{
        SnapshotID:  current.ID,
        BaselineID:  baseline.ID,
        Regressions: auditDelta.HasRegression || elementDelta.HasRegression,
        Audit:       auditDelta,
        Elements:    elementDelta,
    }

    output.Success("diff", result)

    if result.Regressions {
        os.Exit(1)
    }
}
```

---

## 8. `main.go` Changes

Add one case to the switch statement:

```go
case "diff":
    cmd.RunDiff(cmdArgs)
```

Add to the usage string (under "UX Evaluation"):

```
  diff [snapshot-id]      Compare snapshot against baseline; exit 1 on regressions
```

---

## 9. Test Plan (`diff/diff_test.go`)

Tests cover pure functions only — no browser, no filesystem.

| Test | Description |
|---|---|
| `TestComputeAuditDelta_NoChange` | Same issues in baseline and current → all deltas 0, no regressions |
| `TestComputeAuditDelta_NewIssue` | One new contrast issue in current → delta +1, regression=true |
| `TestComputeAuditDelta_ResolvedIssue` | Issue present in baseline but not current → resolved list, no regression |
| `TestComputeAuditDelta_MultiCategory` | Mixed changes across all 4 categories |
| `TestComputeAuditDelta_EmptyBoth` | Both sides empty → no issues, no regressions |
| `TestComputeElementDelta_Appeared` | Element in current only → appeared list, no regression |
| `TestComputeElementDelta_Disappeared` | Element in baseline only → disappeared list, regression=true |
| `TestComputeElementDelta_Moved` | Position delta > threshold → moved list, regression=true |
| `TestComputeElementDelta_MovedBelowThreshold` | Position delta ≤ threshold → no change reported |
| `TestComputeElementDelta_Resized` | Size delta > threshold → resized list, regression=true |
| `TestComputeElementDelta_MovedAndResized` | Both position and size change → appears in both lists |
| `TestComputeElementDelta_NoBoundingBox` | Nodes without bounding boxes → no position/size diff |
| `TestComputeElementDelta_SemanticIdentity` | Matching uses role+name, not node order |
| `TestSemanticIDNormalization` | Role/name are lowercased and trimmed before matching |

Quality gate: `go test ./...` must pass.

---

## 10. Security Considerations

- **No shell execution:** The diff engine is pure Go with no `exec.Command` calls.
- **No network access:** All data is read from local JSON files written by US-001.
- **Path traversal:** `snapshot.Load(id)` must sanitize the `id` parameter: reject any value containing `/`, `..`, or path separators before constructing the file path.
- **JSON parsing:** Use `json.Unmarshal` with typed structs (not `map[string]any`) to avoid prototype pollution and unintended field access.
- **Exit code integrity:** `os.Exit(1)` is reserved for regressions; `os.Exit(2)` for errors. Callers (pre-commit hooks, agents) must not conflate them.

---

## 11. Assumptions About US-001 / US-002

This spec assumes the following contracts from upstream stories:

| Contract | Source |
|---|---|
| `snapshot.Snapshot` struct with `AuditData` and `AXNodes` fields | US-001 |
| `snapshot.Load(id string) (*Snapshot, error)` | US-001 |
| `snapshot.LoadLatest() (*Snapshot, error)` | US-001 |
| `snapshot.LoadBaseline() (*Snapshot, error)` | US-002 |
| `AuditIssue.Selector` is a stable CSS-like string for matching | US-001 |
| `AXNode.BoundingBox` is populated when available (CDP layout info) | US-001 |
| Snapshots stored in `.kaleidoscope/snapshots/<id>.json` | US-001 |
| Baseline pointer stored in `.kaleidoscope/baselines.json` | US-002 |

If US-001 uses different field names, the `diff` package imports are updated accordingly — the pure-function logic is unchanged.
