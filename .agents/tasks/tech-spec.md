# Tech Spec: US-003 — Audit and Element Diff Engine

**Story:** As an AI agent, I want to compare a snapshot against a known-good baseline and receive structured JSON describing what changed, so that I can distinguish intentional changes from regressions.

**Depends on:** US-002 (Snapshot system, `.ks-project.json`, `.kaleidoscope/snapshots/`, `.kaleidoscope/baselines.json`)

---

## Architecture Overview

US-003 introduces a pure-function diff engine and the `ks diff [snapshot-id]` command. The engine accepts two snapshot data structures (baseline + target) and produces a structured diff with no Chrome dependency.

```
main.go
  └── case "diff" → cmd.RunDiff(args)
        └── snapshot.Load(id)            // from US-002 snapshot package
        └── snapshot.LoadBaseline()       // from US-002 snapshot package
        └── diff.ComputeAuditDiff(...)    // new: diff/audit.go
        └── diff.ComputeElementDiff(...)  // new: diff/elements.go
        └── output.Success("diff", result)
```

All diff logic lives in a new `diff/` package — pure functions, no I/O, no browser dependency. The `cmd/diff.go` file handles I/O, flag parsing, and exit codes.

---

## Detailed Component Design

### 1. `diff/` package

#### `diff/types.go` — Shared types

```go
package diff

// AuditDiff is the top-level audit comparison result.
type AuditDiff struct {
    Categories  CategoryDeltas  `json:"categories"`
    Issues      IssueDelta      `json:"issues"`
    HasRegression bool          `json:"hasRegression"`
}

// CategoryDeltas reports per-category count changes.
type CategoryDeltas struct {
    Contrast   CategoryDelta `json:"contrast"`
    TouchTargets CategoryDelta `json:"touchTargets"`
    Typography CategoryDelta `json:"typography"`
}

// CategoryDelta holds baseline vs snapshot counts and the delta.
type CategoryDelta struct {
    Baseline int `json:"baseline"`
    Snapshot int `json:"snapshot"`
    Delta    int `json:"delta"`   // snapshot - baseline; positive = regression
}

// IssueDelta lists per-issue new/resolved status.
type IssueDelta struct {
    New      []AuditIssue `json:"new"`
    Resolved []AuditIssue `json:"resolved"`
}

// AuditIssue is a normalized audit issue keyed by selector.
type AuditIssue struct {
    Category string `json:"category"`
    Selector string `json:"selector"`
}

// ElementDiff is the top-level accessibility tree comparison result.
type ElementDiff struct {
    Appeared    []ElementChange `json:"appeared"`
    Disappeared []ElementChange `json:"disappeared"`
    Moved       []ElementChange `json:"moved"`
    Resized     []ElementChange `json:"resized"`
    HasRegression bool          `json:"hasRegression"`
}

// ElementChange describes a change to a single element.
type ElementChange struct {
    Role     string   `json:"role"`
    Name     string   `json:"name"`
    Baseline *ElementRect `json:"baseline,omitempty"`
    Snapshot *ElementRect `json:"snapshot,omitempty"`
    Delta    *RectDelta   `json:"delta,omitempty"`
}

// ElementRect stores position and size.
type ElementRect struct {
    X      float64 `json:"x"`
    Y      float64 `json:"y"`
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}

// RectDelta is the difference between two rects.
type RectDelta struct {
    DX float64 `json:"dx"`
    DY float64 `json:"dy"`
    DW float64 `json:"dw"`
    DH float64 `json:"dh"`
}

// DiffResult is the complete output of `ks diff`.
type DiffResult struct {
    SnapshotID string      `json:"snapshotId"`
    BaselineID string      `json:"baselineId"`
    Audit      AuditDiff   `json:"audit"`
    Elements   ElementDiff `json:"elements"`
    HasRegression bool     `json:"hasRegression"`
}
```

#### `diff/audit.go` — Audit diff logic

```go
package diff

// ComputeAuditDiff compares two audit summaries from snapshot data.
// baselineAudit and snapshotAudit are the "summary" sub-objects from
// the audit result stored in each snapshot (as map[string]any from JSON).
func ComputeAuditDiff(baselineAudit, snapshotAudit map[string]any) AuditDiff
```

**Algorithm:**
1. Extract per-category counts from each audit summary (`contrastViolations`, `touchViolations`, `typographyWarnings`).
2. Compute `CategoryDelta` for each: `Delta = snapshot - baseline`.
3. For per-issue tracking: if snapshot data includes per-issue selector lists (see data model note below), compute set difference to produce `IssueDelta.New` (in snapshot, not in baseline) and `IssueDelta.Resolved` (in baseline, not in snapshot).
4. `HasRegression = true` if any `CategoryDelta.Delta > 0`.

**Note on per-issue matching:** The current audit command records only counts, not per-issue selectors. US-002 (or this story) must extend the audit snapshot data to include structured per-issue records. See Data Model section.

#### `diff/elements.go` — Element diff logic

```go
package diff

// ElementRecord is the normalized form of an ax-tree node used for diffing.
// It holds the semantic identity key and optional bounding rect.
type ElementRecord struct {
    Role string
    Name string
    Rect *ElementRect // nil if position data not captured
}

// SemanticKey returns the matching key: "role:name" (lowercased, trimmed).
func SemanticKey(e ElementRecord) string

// ComputeElementDiff compares two slices of ElementRecord.
// Position/size thresholds: moved if |dx|>threshold or |dy|>threshold,
// resized if |dw|>threshold or |dh|>threshold.
func ComputeElementDiff(baseline, snapshot []ElementRecord, posThreshold, sizeThreshold float64) ElementDiff
```

**Algorithm:**
1. Build a `map[string]ElementRecord` for baseline keyed by `SemanticKey`.
2. Build a `map[string]ElementRecord` for snapshot keyed by `SemanticKey`.
3. Appeared: keys in snapshot map not in baseline map.
4. Disappeared: keys in baseline map not in snapshot map.
5. For keys present in both: if both have `Rect`, compute deltas. If position delta exceeds `posThreshold` → Moved; if size delta exceeds `sizeThreshold` → Resized. A single element can be in both Moved and Resized.
6. `HasRegression = len(Appeared) > 0 || len(Disappeared) > 0 || len(Moved) > 0 || len(Resized) > 0`.

**Default thresholds:** `posThreshold = 4.0` px, `sizeThreshold = 4.0` px (configurable via flags).

---

### 2. `cmd/diff.go` — CLI command

```go
package cmd

func RunDiff(args []string) {
    // 1. Parse args: optional positional snapshot-id, flags
    snapshotID := getArg(args)   // empty = use latest
    posThreshold  := parseFlagFloat(args, "--pos-threshold", 4.0)
    sizeThreshold := parseFlagFloat(args, "--size-threshold", 4.0)

    // 2. Load baseline; error if none exists
    baseline, err := snapshot.LoadBaseline()
    if err != nil { output.Fail(...); os.Exit(2) }

    // 3. Load target snapshot (latest or by ID)
    var target *snapshot.Snapshot
    if snapshotID == "" {
        target, err = snapshot.LoadLatest()
    } else {
        target, err = snapshot.LoadByID(snapshotID)
    }
    if err != nil { output.Fail(...); os.Exit(2) }

    // 4. Run diff engine (pure functions, no browser)
    auditDiff  := diff.ComputeAuditDiff(baseline.AuditData, target.AuditData)
    elemDiff   := diff.ComputeElementDiff(baseline.Elements, target.Elements,
                      posThreshold, sizeThreshold)

    result := diff.DiffResult{
        SnapshotID:    target.ID,
        BaselineID:    baseline.ID,
        Audit:         auditDiff,
        Elements:      elemDiff,
        HasRegression: auditDiff.HasRegression || elemDiff.HasRegression,
    }

    output.Success("diff", result)

    // 5. Exit code
    if result.HasRegression {
        os.Exit(1)
    }
}
```

**Flag additions to `cmd/util.go`:** Add `--pos-threshold` and `--size-threshold` to the flag-value list in `getNonFlagArgs`.

---

### 3. `main.go` changes

Add to the switch:
```go
case "diff":
    cmd.RunDiff(cmdArgs)
```

Add to the usage string under a new "Snapshot & Regression" section:
```
  diff [snapshot-id]      Compare snapshot to baseline; exit 1 on regression
```

---

## Data Model Changes

### Snapshot structure (owned by US-002)

US-003 requires that snapshots stored by US-002 contain structured audit data with per-issue selector lists (not just counts), and element records with bounding rects. The snapshot package must expose:

```go
// snapshot/types.go (US-002)
type Snapshot struct {
    ID        string          `json:"id"`        // e.g. timestamp or commit SHA
    CreatedAt time.Time       `json:"createdAt"`
    URL       string          `json:"url"`
    AuditData AuditSnapshot   `json:"audit"`
    Elements  []ElementRecord `json:"elements"`  // normalized ax-tree nodes with rects
}

type AuditSnapshot struct {
    Summary AuditSummary  `json:"summary"`
    Issues  []AuditIssueRecord `json:"issues"`
}

type AuditSummary struct {
    ContrastViolations int `json:"contrastViolations"`
    TouchViolations    int `json:"touchViolations"`
    TypographyWarnings int `json:"typographyWarnings"`
    TotalIssues        int `json:"totalIssues"`
}

type AuditIssueRecord struct {
    Category string `json:"category"` // "contrast" | "touch" | "typography"
    Selector string `json:"selector"`
}
```

**Element records with rects** require US-002's snapshot command to correlate ax-tree nodes with DOM bounding rects (via `getBoundingClientRect` in JS, joined by nodeId or by role+name).

### Baseline pointer (owned by US-002)

`.kaleidoscope/baselines.json`:
```json
{
  "baseline": "<snapshot-id>"
}
```

The `snapshot` package exposes `LoadBaseline()` returning the snapshot pointed to by this file.

### Storage layout (owned by US-002, referenced here)

```
.kaleidoscope/
  snapshots/
    <snapshot-id>/
      snapshot.json      ← Snapshot struct above
      screenshot.png     ← optional, not used by diff engine
  baselines.json         ← committed; points to baseline snapshot ID
```

---

## API Definitions

### CLI interface

```
ks diff [snapshot-id] [--pos-threshold N] [--size-threshold N]
```

| Argument | Default | Description |
|---|---|---|
| `snapshot-id` | (latest) | ID of snapshot to compare against baseline |
| `--pos-threshold` | `4.0` | Pixel threshold to classify position change as "moved" |
| `--size-threshold` | `4.0` | Pixel threshold to classify size change as "resized" |

**Exit codes:**
- `0` — no regressions detected (or snapshot equals baseline)
- `1` — one or more regressions detected
- `2` — error (no baseline, snapshot not found, I/O error)

### JSON output format

```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "snapshotId": "1712000000000",
    "baselineId": "1711900000000",
    "hasRegression": true,
    "audit": {
      "hasRegression": true,
      "categories": {
        "contrast":     { "baseline": 2, "snapshot": 4, "delta": 2 },
        "touchTargets": { "baseline": 1, "snapshot": 1, "delta": 0 },
        "typography":   { "baseline": 0, "snapshot": 0, "delta": 0 }
      },
      "issues": {
        "new":      [{ "category": "contrast", "selector": "p" }],
        "resolved": []
      }
    },
    "elements": {
      "hasRegression": true,
      "appeared":    [],
      "disappeared": [{ "role": "button", "name": "Submit", "baseline": {"x":100,"y":200,"width":80,"height":44} }],
      "moved":       [{ "role": "link", "name": "Home", "baseline": {"x":0,"y":0,"width":60,"height":24}, "snapshot": {"x":0,"y":4,"width":60,"height":24}, "delta": {"dx":0,"dy":4,"dw":0,"dh":0} }],
      "resized":     []
    }
  }
}
```

**Error output (no baseline):**
```json
{ "ok": false, "command": "diff", "error": "no baseline set", "hint": "Run: ks baseline set <snapshot-id>" }
```

---

## File Layout

```
diff/
  types.go        ← all shared types (DiffResult, AuditDiff, ElementDiff, ...)
  audit.go        ← ComputeAuditDiff pure function
  elements.go     ← ComputeElementDiff, SemanticKey, ElementRecord pure functions
  audit_test.go   ← unit tests for audit diff
  elements_test.go ← unit tests for element diff
cmd/
  diff.go         ← RunDiff: I/O, flag parsing, exit codes
main.go           ← add "diff" case + usage string entry
```

---

## Security Considerations

1. **Path traversal:** The snapshot-id argument is used to construct a filesystem path inside `.kaleidoscope/snapshots/<id>/`. The implementation must sanitize the ID — reject any value containing `/`, `..`, or null bytes before joining with the base directory. Use `filepath.Clean` and verify the resolved path is still under the snapshots root.

2. **JSON input trust:** Snapshot files are read from the local filesystem (written by `ks snapshot`). They are not user-supplied at diff time; no external input is deserialized. No additional sanitization is required beyond path safety.

3. **No shell execution:** The diff engine is pure Go with no subprocess invocation, eliminating command injection risk entirely.

4. **Exit code contract:** Exit code `1` means "regression detected" (not "error"). Callers (pre-commit hooks, agents) must distinguish `1` from `2` (error). The `ok` field in JSON output disambiguates programmatically.

---

## Testing Strategy

All diff logic is pure functions with no external dependencies — straightforward to unit test.

### `diff/audit_test.go`
- Baseline == snapshot → `HasRegression: false`, all deltas = 0
- Snapshot has more contrast violations → `HasRegression: true`, `contrast.delta > 0`
- Snapshot resolves all touch violations → `HasRegression: false`, `delta < 0`
- Per-issue: verify `New` and `Resolved` slices by selector

### `diff/elements_test.go`
- Identical trees → empty diffs, `HasRegression: false`
- Element in snapshot not in baseline → Appeared
- Element in baseline not in snapshot → Disappeared
- Same role+name, position shifted beyond threshold → Moved
- Same role+name, size changed beyond threshold → Resized
- Same role+name, position shifted below threshold → no change
- Element with empty name: key is `"role:"` — verify matching still works

### `cmd/diff.go` (integration-level, via `go test ./...`)
- No baseline → exit 2, `ok: false`
- No snapshots → exit 2, `ok: false`
- Regression present → exit 1, `ok: true`, `hasRegression: true`
- No regression → exit 0, `ok: true`, `hasRegression: false`
- Explicit snapshot-id → loads correct snapshot

**Quality gate:** `go test ./...` must pass with no failures.
