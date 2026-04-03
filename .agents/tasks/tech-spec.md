# Tech Spec: US-003 — Audit and Element Diff Engine

**Story**: As an AI agent, I want to compare a snapshot against a known-good baseline and receive structured JSON describing what changed, so that I can distinguish intentional changes from regressions.

**Command**: `ks diff [snapshot-id]`

**Dependencies**: US-001 (project config), US-002 (snapshot creation + baseline management)

---

## 1. Architecture Overview

US-003 introduces a **pure-function diff engine** that operates entirely on previously persisted snapshot data. No Chrome dependency. The engine takes two snapshots (baseline + target), computes audit deltas and element changes, and returns a structured result that the CLI command formats as JSON.

### Component Diagram

```
ks diff [snapshot-id]
        │
        ▼
cmd/diff.go (RunDiff)
  │  Reads snapshots from disk
  │  Loads baselines.json
        │
        ▼
snapshot/snapshot.go        ← shared types (snapshot data model)
        │
        ▼
diff/engine.go              ← pure-function diff engine
  ├── DiffAudit()           ← audit category + per-issue delta
  └── DiffElements()        ← AX-tree element comparison
        │
        ▼
output.Success / output.Fail + os.Exit(0/1)
```

### Constraints (from PRD rules)
- All output via `output.Success` / `output.Fail` (existing JSON convention).
- Diff engine is a **pure-function module**: two data structures in, diff out; no Chrome or I/O dependency.
- Screenshot diffing is out of scope for this story.
- `.kaleidoscope/baselines.json` is committed to repo (shared baseline reference).
- `.kaleidoscope/snapshots/` is gitignored (local data).

---

## 2. Data Model

### 2.1 Snapshot (produced by US-002, consumed here)

**File location**: `.kaleidoscope/snapshots/<snapshot-id>.json`

```go
// Package: snapshot
// File: snapshot/snapshot.go

package snapshot

import "time"

// Snapshot is the full persisted state of a single capture run.
type Snapshot struct {
    ID        string    `json:"id"`         // e.g. "20260403-153045-abc123"
    CreatedAt time.Time `json:"createdAt"`
    URL       string    `json:"url"`
    Viewport  Viewport  `json:"viewport"`
    Audit     AuditData `json:"audit"`
    Elements  []Element `json:"elements"`
}

type Viewport struct {
    Width  int `json:"width"`
    Height int `json:"height"`
}

// AuditData holds per-category issue lists captured at snapshot time.
type AuditData struct {
    Contrast    []AuditIssue `json:"contrast"`
    Touch       []AuditIssue `json:"touch"`
    Typography  []AuditIssue `json:"typography"`
    Spacing     []AuditIssue `json:"spacing"`
}

// AuditIssue is a single violation or warning from an audit category.
// Selector is the primary match key for diff purposes.
type AuditIssue struct {
    Selector string `json:"selector"`  // CSS selector path or semantic ID
    Message  string `json:"message"`   // Human-readable description
}

// Element represents a single accessibility tree node with layout data,
// captured at snapshot time via CDP. No Chrome dependency at diff time.
type Element struct {
    Role string      `json:"role"`
    Name string      `json:"name"`
    Box  BoundingBox `json:"box"`
}

type BoundingBox struct {
    X      float64 `json:"x"`
    Y      float64 `json:"y"`
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}
```

**Snapshot index** (also written by US-002):

```
.kaleidoscope/snapshots/index.json
```

```go
// SnapshotIndex lists all known snapshots for fast "latest" lookup.
type SnapshotIndex struct {
    Entries []SnapshotMeta `json:"entries"`
}

type SnapshotMeta struct {
    ID        string    `json:"id"`
    CreatedAt time.Time `json:"createdAt"`
    URL       string    `json:"url"`
}
```

Entries are kept in ascending creation order. The last entry is always "latest."

### 2.2 Baselines (produced by US-002, consumed here)

**File location**: `.kaleidoscope/baselines.json`

```go
// BaselinesFile maps named baselines to snapshot IDs.
// "default" is the unnamed baseline set by `ks baseline set`.
type BaselinesFile struct {
    Default string            `json:"default"` // snapshot ID
    Named   map[string]string `json:"named"`   // name -> snapshot ID
}
```

---

## 3. Diff Package (`diff/engine.go`)

A pure-function package with no imports outside stdlib and the `snapshot` package.

### 3.1 Public API

```go
package diff

import "github.com/callmeradical/kaleidoscope/snapshot"

// Thresholds control what counts as a "moved" or "resized" regression.
type Thresholds struct {
    PositionDelta float64 // px; default 4.0
    SizeDelta     float64 // px; default 4.0
}

// DefaultThresholds returns the built-in threshold values.
func DefaultThresholds() Thresholds

// Run computes the full diff between baseline and target snapshots.
// This is the single entry point for the diff engine.
func Run(baseline, target *snapshot.Snapshot, t Thresholds) *Result
```

### 3.2 Result Types

```go
// Result is the top-level diff output.
type Result struct {
    SnapshotID     string       `json:"snapshotId"`
    BaselineID     string       `json:"baselineId"`
    HasRegressions bool         `json:"hasRegressions"`
    Audit          AuditDiff    `json:"audit"`
    Elements       ElementDiff  `json:"elements"`
}

// AuditDiff reports per-category count deltas and per-issue new/resolved status.
type AuditDiff struct {
    Categories map[string]CategoryDelta `json:"categories"` // "contrast","touch","typography","spacing"
    NewIssues      []IssueChange        `json:"newIssues"`      // in target, not in baseline
    ResolvedIssues []IssueChange        `json:"resolvedIssues"` // in baseline, not in target
}

// CategoryDelta reports the numeric change for one audit category.
type CategoryDelta struct {
    Baseline int `json:"baseline"`
    Target   int `json:"target"`
    Delta    int `json:"delta"` // target - baseline; positive = regression
}

// IssueChange identifies a single audit issue that appeared or was resolved.
type IssueChange struct {
    Category string `json:"category"`
    Selector string `json:"selector"`
    Message  string `json:"message"`
}

// ElementDiff reports element-level changes detected via AX-tree comparison.
type ElementDiff struct {
    Appeared    []ElementChange `json:"appeared"`
    Disappeared []ElementChange `json:"disappeared"`
    Moved       []ElementChange `json:"moved"`
    Resized     []ElementChange `json:"resized"`
}

// ElementChange describes one element's state change between snapshots.
type ElementChange struct {
    Role         string                  `json:"role"`
    Name         string                  `json:"name"`
    BaselineBox  *snapshot.BoundingBox   `json:"baselineBox,omitempty"`
    TargetBox    *snapshot.BoundingBox   `json:"targetBox,omitempty"`
    PositionDelta *Delta2D               `json:"positionDelta,omitempty"`
    SizeDelta     *Delta2D               `json:"sizeDelta,omitempty"`
}

// Delta2D represents a 2-dimensional change.
type Delta2D struct {
    DX float64 `json:"dx"`
    DY float64 `json:"dy"`
}
```

### 3.3 Audit Diff Algorithm

```
DiffAudit(baseline AuditData, target AuditData) AuditDiff:

  For each category in ["contrast", "touch", "typography", "spacing"]:
    baselineIssues = map[selector]AuditIssue from baseline
    targetIssues   = map[selector]AuditIssue from target

    CategoryDelta.Baseline = len(baselineIssues)
    CategoryDelta.Target   = len(targetIssues)
    CategoryDelta.Delta    = Target - Baseline

    For each issue in targetIssues not in baselineIssues:
      → append to NewIssues

    For each issue in baselineIssues not in targetIssues:
      → append to ResolvedIssues

  HasRegressions = len(NewIssues) > 0
```

**Issue matching key**: `category + ":" + selector`
Two issues are the same if they share the same category and selector. The message is not used for matching (it may change due to value drift, e.g., contrast ratio 3.8 vs 3.9).

### 3.4 Element Diff Algorithm

```
DiffElements(baseline []Element, target []Element, t Thresholds) ElementDiff:

  baselineMap = map["role:name"] → Element
  targetMap   = map["role:name"] → Element

  Semantic identity key = role + ":" + name  (both lowercased, trimmed)
  Empty-name elements are excluded from matching (they have no stable identity)

  For each key in targetMap not in baselineMap:
    → ElementChange{Appeared, TargetBox: targetMap[key].Box}

  For each key in baselineMap not in targetMap:
    → ElementChange{Disappeared, BaselineBox: baselineMap[key].Box}

  For each key in both:
    bBox = baselineMap[key].Box
    tBox = targetMap[key].Box

    dx = tBox.X - bBox.X
    dy = tBox.Y - bBox.Y
    if abs(dx) > t.PositionDelta OR abs(dy) > t.PositionDelta:
      → ElementChange{Moved, BaselineBox, TargetBox, PositionDelta{dx,dy}}

    dw = tBox.Width  - bBox.Width
    dh = tBox.Height - bBox.Height
    if abs(dw) > t.SizeDelta OR abs(dh) > t.SizeDelta:
      → ElementChange{Resized, BaselineBox, TargetBox, SizeDelta{dw,dh}}

  HasRegressions |= len(Disappeared) > 0 OR len(Moved) > 0 OR len(Resized) > 0
```

**Regression definition for elements**:
- `Appeared` is informational (not a regression by default — new UI can be intentional).
- `Disappeared`, `Moved`, `Resized` are regressions.

---

## 4. Command: `ks diff [snapshot-id]`

### 4.1 File: `cmd/diff.go`

```go
package cmd

func RunDiff(args []string) {
    // 1. Parse args
    snapshotID := getArg(args)  // optional; empty = use latest

    // 2. Load baselines.json
    // Error if file does not exist: "No baseline set. Run: ks baseline set"
    baselines, err := snapshot.LoadBaselines(kaleidoscopeDir())
    if err != nil { output.Fail(...); os.Exit(2) }
    if baselines.Default == "" { output.Fail(..., "No baseline set. Run: ks baseline set"); os.Exit(2) }

    // 3. Load baseline snapshot
    baseline, err := snapshot.Load(kaleidoscopeDir(), baselines.Default)
    if err != nil { output.Fail(...); os.Exit(2) }

    // 4. Resolve target snapshot (latest or by ID)
    if snapshotID == "" {
        snapshotID, err = snapshot.LatestID(kaleidoscopeDir())
        if err != nil { output.Fail(..., "No snapshots found. Run: ks snapshot"); os.Exit(2) }
    }
    target, err := snapshot.Load(kaleidoscopeDir(), snapshotID)
    if err != nil { output.Fail(...); os.Exit(2) }

    // 5. Run diff engine (pure function, no Chrome)
    result := diff.Run(baseline, target, diff.DefaultThresholds())

    // 6. Output JSON result
    output.Success("diff", result)

    // 7. Exit code
    if result.HasRegressions {
        os.Exit(1)
    }
    os.Exit(0)
}
```

### 4.2 `main.go` Change

Add to the switch statement in `main.go`:

```go
case "diff":
    cmd.RunDiff(cmdArgs)
```

Add to usage string under "UX Evaluation":
```
  diff [snapshot-id]      Compare snapshot against baseline (exit 1 on regression)
```

### 4.3 Snapshot Storage Helpers (`snapshot/store.go`)

The following helpers are needed (defined in the `snapshot` package, used by both US-002 and US-003):

```go
// kaleidoscopeDir returns the .kaleidoscope/ directory path.
// Uses project-local if --local flag was passed (same logic as browser.ReadState).
func KaleidoscopeDir(local bool) string

// Load reads and deserializes a snapshot by ID.
func Load(dir, id string) (*Snapshot, error)
// Path: <dir>/snapshots/<id>.json

// LatestID returns the ID of the most recently created snapshot.
// Reads <dir>/snapshots/index.json and returns the last entry's ID.
func LatestID(dir string) (string, error)

// LoadBaselines reads baselines.json from the kaleidoscope dir.
func LoadBaselines(dir string) (*BaselinesFile, error)
// Path: <dir>/baselines.json

// Save writes a snapshot to disk and updates the index.
// (Called by US-002's RunSnapshot, included here for completeness.)
func Save(dir string, s *Snapshot) error
```

---

## 5. Full JSON Output Shape

### Success (no regressions)
```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "snapshotId": "20260403-153045-abc123",
    "baselineId": "20260402-090000-def456",
    "hasRegressions": false,
    "audit": {
      "categories": {
        "contrast":   { "baseline": 2, "target": 2, "delta": 0 },
        "touch":      { "baseline": 0, "target": 0, "delta": 0 },
        "typography": { "baseline": 1, "target": 1, "delta": 0 },
        "spacing":    { "baseline": 3, "target": 3, "delta": 0 }
      },
      "newIssues": [],
      "resolvedIssues": []
    },
    "elements": {
      "appeared":    [],
      "disappeared": [],
      "moved":       [],
      "resized":     []
    }
  }
}
```

### Failure (regressions detected, exit code 1)
```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "snapshotId": "20260403-160000-xyz789",
    "baselineId": "20260402-090000-def456",
    "hasRegressions": true,
    "audit": {
      "categories": {
        "contrast":   { "baseline": 2, "target": 4, "delta": 2 },
        "touch":      { "baseline": 0, "target": 0, "delta": 0 },
        "typography": { "baseline": 1, "target": 1, "delta": 0 },
        "spacing":    { "baseline": 3, "target": 3, "delta": 0 }
      },
      "newIssues": [
        { "category": "contrast", "selector": "h2", "message": "Contrast ratio 2.5:1 fails AA" },
        { "category": "contrast", "selector": ".hero-cta", "message": "Contrast ratio 3.1:1 fails AA" }
      ],
      "resolvedIssues": []
    },
    "elements": {
      "appeared":    [],
      "disappeared": [
        { "role": "button", "name": "Submit", "baselineBox": { "x": 120, "y": 440, "width": 80, "height": 48 } }
      ],
      "moved":  [],
      "resized": []
    }
  }
}
```

### Error (no baseline set)
```json
{
  "ok": false,
  "command": "diff",
  "error": "baselines.json not found",
  "hint": "No baseline set. Run: ks baseline set"
}
```

---

## 6. File Layout

New files to create:

```
snapshot/
  snapshot.go       ← Snapshot, AuditData, AuditIssue, Element, BoundingBox, Viewport types
  store.go          ← Load, Save, LatestID, LoadBaselines, KaleidoscopeDir helpers
  index.go          ← SnapshotIndex, SnapshotMeta types + index read/write helpers

diff/
  engine.go         ← Run(), DiffAudit(), DiffElements(), all Result types, Thresholds

cmd/
  diff.go           ← RunDiff()

main.go             ← add "diff" case + usage line
```

Modified files:

```
main.go             ← add case "diff": cmd.RunDiff(cmdArgs) in switch
```

---

## 7. Security Considerations

1. **Path traversal**: Snapshot IDs are user-supplied via CLI. Before constructing the file path `<dir>/snapshots/<id>.json`, validate that `id` matches the pattern `[a-zA-Z0-9\-_]+` to prevent directory traversal (e.g., `../../etc/passwd`).

2. **File size limits**: Snapshots could grow large for complex pages. Add a read limit (e.g., 50 MB) when loading snapshot JSON to prevent excessive memory allocation.

3. **JSON unmarshal safety**: Use strict struct unmarshalling (`json.Decoder` with `DisallowUnknownFields` optional). Malformed snapshot files should produce a clear error, not a panic.

4. **No code execution**: The diff engine is pure data comparison. No `eval`, shell exec, or dynamic code loading. Safe by construction.

---

## 8. Testing Strategy (Quality Gate: `go test ./...`)

### `diff` package unit tests (`diff/engine_test.go`)

All tests are pure-function, no disk I/O:

| Test | Scenario |
|------|----------|
| `TestDiffAudit_NoChange` | Identical audits → delta=0, no new/resolved |
| `TestDiffAudit_NewIssue` | Extra issue in target → appears in NewIssues |
| `TestDiffAudit_ResolvedIssue` | Issue removed in target → appears in ResolvedIssues |
| `TestDiffAudit_AllCategories` | All four categories have independent deltas |
| `TestDiffAudit_SelectorMatching` | Same message different selector = two separate issues |
| `TestDiffElements_Appeared` | Element in target, not baseline |
| `TestDiffElements_Disappeared` | Element in baseline, not target |
| `TestDiffElements_Moved` | Position delta > threshold |
| `TestDiffElements_NotMoved` | Position delta ≤ threshold (no change) |
| `TestDiffElements_Resized` | Size delta > threshold |
| `TestDiffElements_EmptyNameSkipped` | Empty-name elements are not matched |
| `TestDiffElements_SemanticKey` | "button:Submit" matches across snapshots |
| `TestHasRegressions_NewAuditIssue` | HasRegressions=true when audit has new issues |
| `TestHasRegressions_Disappeared` | HasRegressions=true when element disappeared |
| `TestHasRegressions_AppearedOnly` | HasRegressions=false when only elements appeared |

### `snapshot` package unit tests (`snapshot/store_test.go`)

| Test | Scenario |
|------|----------|
| `TestLoadBaselines_Missing` | Returns error when file absent |
| `TestLoadBaselines_NoDefault` | Returns struct with empty Default |
| `TestLatestID_Empty` | Returns error when index is empty |
| `TestLatestID_ReturnsLast` | Returns ID of last index entry |
| `TestLoad_PathTraversal` | Rejects ID containing `../` |

---

## 9. Acceptance Criteria Mapping

| Acceptance Criterion | Implementation |
|---------------------|----------------|
| `ks diff` compares latest snapshot against baseline | `RunDiff` with no arg → `snapshot.LatestID()` |
| `ks diff <snapshot-id>` compares specific snapshot | `RunDiff` with arg → `snapshot.Load(dir, id)` |
| Audit deltas per-category (contrast, spacing, typography, touch) | `AuditDiff.Categories` map with `CategoryDelta` |
| Per-issue new/resolved by selector | `AuditDiff.NewIssues` / `ResolvedIssues` matched by `category:selector` |
| Element changes: appeared, disappeared, moved, resized | `ElementDiff` with four slices |
| Element matching by semantic identity (role + name) | `role + ":" + name` key in `DiffElements` |
| Exit code 0 when no regressions | `os.Exit(0)` after `!result.HasRegressions` |
| Exit code 1 when regressions exist | `os.Exit(1)` after `result.HasRegressions` |
| Error if no baseline exists | Check `baselines.Default == ""` → `output.Fail` + `os.Exit(2)` |
