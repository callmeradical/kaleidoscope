# Tech Spec: Audit and Element Diff Engine (US-003)

## Overview

Implements `ks diff [snapshot-id]` which compares a snapshot against a stored baseline and emits structured JSON describing regressions. Depends on US-001 (project config + snapshot storage) and US-002 (snapshot capture command). This story adds only the diff engine and CLI surface; it does not capture new snapshots.

---

## Architecture Overview

```
main.go
  └── case "diff": cmd.RunDiff(args)

cmd/diff.go                         ← CLI entry point
  ├── loads snapshot (latest or by ID) from snapshot store
  ├── loads baseline snapshot from baselines.json
  └── calls diff.Compare(baseline, target) → DiffResult

diff/diff.go                        ← pure-function diff engine (no Chrome)
  ├── CompareAudit(a, b AuditSummary) → AuditDiff
  └── CompareElements(a, b []AXNode) → ElementDiff

snapshot/store.go                   ← snapshot read/write (from US-001/002)
snapshot/types.go                   ← Snapshot, AuditSummary, AXNode types

output/format.go                    ← existing Result/Success/Fail (unchanged)
```

The diff engine (`diff/`) is a standalone package with no browser or filesystem dependency — it accepts two data structures and returns a diff. This makes it trivially testable via `go test ./diff/...`.

---

## Assumed Snapshot Data Model (from US-001/US-002)

US-003 depends on the following types being established by prior stories. They are reproduced here as the contract the diff engine consumes.

### `snapshot/types.go`

```go
// Snapshot is the root record written by `ks snapshot`.
type Snapshot struct {
    ID         string      `json:"id"`          // e.g. "20260404-153201"
    CreatedAt  time.Time   `json:"createdAt"`
    CommitSHA  string      `json:"commitSha,omitempty"`
    URL        string      `json:"url"`
    Audit      AuditSummary `json:"audit"`
    AXNodes    []AXNode    `json:"axNodes"`
    Screenshot string      `json:"screenshot"`  // relative path to PNG
}

// AuditSummary mirrors the output of `ks audit`, extended with per-issue detail.
type AuditSummary struct {
    TotalIssues        int           `json:"totalIssues"`
    ContrastViolations []AuditIssue  `json:"contrastViolations"`
    TouchViolations    []AuditIssue  `json:"touchViolations"`
    TypographyWarnings []AuditIssue  `json:"typographyWarnings"`
}

// AuditIssue is a single finding with enough identity to match across snapshots.
type AuditIssue struct {
    Selector string `json:"selector"` // CSS selector or tag identifying the element
    Detail   string `json:"detail"`   // human-readable description (e.g. "ratio 2.1:1 < 4.5:1")
}

// AXNode is a simplified accessibility tree node.
type AXNode struct {
    Role     string  `json:"role"`
    Name     string  `json:"name"`
    X        float64 `json:"x"`
    Y        float64 `json:"y"`
    Width    float64 `json:"width"`
    Height   float64 `json:"height"`
}
```

### `snapshot/store.go` (relevant read surface)

```go
// SnapshotsDir returns .kaleidoscope/snapshots/ (created if absent).
func SnapshotsDir() (string, error)

// ListSnapshots returns all snapshot IDs sorted newest-first.
func ListSnapshots() ([]string, error)

// LoadSnapshot reads a snapshot by ID.
func LoadSnapshot(id string) (*Snapshot, error)

// LoadLatestSnapshot reads the most recent snapshot.
func LoadLatestSnapshot() (*Snapshot, error)

// LoadBaseline reads .kaleidoscope/baselines.json and returns the baseline snapshot.
// Returns ErrNoBaseline if no baseline is set.
func LoadBaseline() (*Snapshot, error)

var ErrNoBaseline = errors.New("no baseline set; run `ks snapshot --set-baseline` first")
```

Storage layout (established by US-001):
```
.kaleidoscope/
  snapshots/
    20260404-153201/
      snapshot.json
      screenshot.png
  baselines.json          ← {"baseline": "20260404-153201"}
```

---

## Component Design

### `diff/diff.go` — Pure Diff Engine

```go
package diff

// DiffResult is the top-level output of a diff operation.
type DiffResult struct {
    SnapshotID  string      `json:"snapshotId"`
    BaselineID  string      `json:"baselineId"`
    Regressions bool        `json:"regressions"`  // true if any new issues
    Audit       AuditDiff   `json:"audit"`
    Elements    ElementDiff `json:"elements"`
}

// AuditDiff describes changes in audit findings between baseline and target.
type AuditDiff struct {
    // Per-category count deltas (positive = more issues = regression).
    ContrastDelta    int `json:"contrastDelta"`
    TouchDelta       int `json:"touchDelta"`
    TypographyDelta  int `json:"typographyDelta"`
    TotalDelta       int `json:"totalDelta"`

    // Per-issue classification.
    NewIssues      []IssueDelta `json:"newIssues"`      // appeared in target, not in baseline
    ResolvedIssues []IssueDelta `json:"resolvedIssues"` // in baseline, gone in target
}

// IssueDelta is a single audit issue annotated with its category and change status.
type IssueDelta struct {
    Category string `json:"category"` // "contrast" | "touch" | "typography"
    Selector string `json:"selector"`
    Detail   string `json:"detail"`
}

// ElementDiff describes DOM element changes via accessibility tree comparison.
type ElementDiff struct {
    Appeared    []ElementChange `json:"appeared"`
    Disappeared []ElementChange `json:"disappeared"`
    Moved       []ElementChange `json:"moved"`
    Resized     []ElementChange `json:"resized"`
}

// ElementChange describes one changed element, identified by semantic identity.
type ElementChange struct {
    Role     string   `json:"role"`
    Name     string   `json:"name"`
    Baseline *Rect    `json:"baseline,omitempty"`
    Target   *Rect    `json:"target,omitempty"`
    Delta    *Delta   `json:"delta,omitempty"` // set for moved/resized
}

type Rect struct {
    X      float64 `json:"x"`
    Y      float64 `json:"y"`
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}

type Delta struct {
    DX      float64 `json:"dx,omitempty"`
    DY      float64 `json:"dy,omitempty"`
    DWidth  float64 `json:"dWidth,omitempty"`
    DHeight float64 `json:"dHeight,omitempty"`
}
```

#### `Compare` — top-level entry point

```go
// Compare produces a DiffResult from two snapshots. Pure function; no I/O.
func Compare(baseline, target *snapshot.Snapshot) DiffResult
```

#### `CompareAudit` — audit diff logic

```go
func CompareAudit(baseline, target snapshot.AuditSummary) AuditDiff
```

**Algorithm:**
1. Build a string key for each issue: `category:selector` (e.g. `"contrast:button"`).
2. Build a set from baseline issues and a set from target issues.
3. `newIssues` = target_set − baseline_set (regression: exists in target, not in baseline).
4. `resolvedIssues` = baseline_set − target_set (improvement: gone from target).
5. Compute per-category deltas as `len(target_category) - len(baseline_category)`.
6. `TotalDelta` = sum of category deltas.

**Matching key:** `category + ":" + selector`. Two issues with the same selector in the same category are considered the same issue. Issues that share a selector but differ in `Detail` are treated as the same issue (selector identity wins), so wording changes do not create false regressions.

#### `CompareElements` — ax-tree diff logic

```go
// PositionThreshold is the minimum pixel delta to classify a move as significant.
const PositionThreshold = 4.0

// SizeThreshold is the minimum pixel delta to classify a resize as significant.
const SizeThreshold = 4.0

func CompareElements(baseline, target []snapshot.AXNode) ElementDiff
```

**Semantic identity key:** `role + "|" + name` (both lowercased and trimmed). Elements with empty name are excluded from comparison to avoid false positives from anonymous containers.

**Algorithm:**
1. Index baseline nodes: `map[key]AXNode`.
2. Index target nodes: `map[key]AXNode`.
3. For each target node not in baseline → `Appeared`.
4. For each baseline node not in target → `Disappeared`.
5. For each key present in both:
   - If `|ΔX| > PositionThreshold || |ΔY| > PositionThreshold` → `Moved`.
   - If `|ΔW| > SizeThreshold || |ΔH| > SizeThreshold` → `Resized`.
   - An element can appear in both `Moved` and `Resized` independently.

**Regression determination:** `Regressions = len(newIssues) > 0 || len(Appeared)+len(Disappeared)+len(Moved)+len(Resized) > 0`. Element changes are treated as regressions because appeared/disappeared/moved elements all represent unexpected DOM changes against a known-good baseline.

---

### `cmd/diff.go` — CLI Command

```go
func RunDiff(args []string)
```

**Flags:**
- Positional arg 0 (optional): snapshot ID. If absent, loads latest.

**Flow:**
1. Load baseline via `snapshot.LoadBaseline()`. If `ErrNoBaseline`, call `output.Fail` and `os.Exit(2)`.
2. Load target snapshot (by ID or latest). If not found, call `output.Fail` and `os.Exit(2)`.
3. Call `diff.Compare(baseline, target)`.
4. Call `output.Success("diff", result)`.
5. If `result.Regressions`, call `os.Exit(1)`. Otherwise exit 0.

**Output schema** (JSON, follows `output.Result` envelope):

```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "snapshotId": "20260404-160000",
    "baselineId": "20260404-153201",
    "regressions": true,
    "audit": {
      "contrastDelta": 2,
      "touchDelta": 0,
      "typographyDelta": -1,
      "totalDelta": 1,
      "newIssues": [
        { "category": "contrast", "selector": "button", "detail": "ratio 2.1:1 < 4.5:1" },
        { "category": "contrast", "selector": "a",      "detail": "ratio 3.0:1 < 4.5:1" }
      ],
      "resolvedIssues": [
        { "category": "typography", "selector": "p", "detail": "line-height too tight" }
      ]
    },
    "elements": {
      "appeared":    [],
      "disappeared": [{ "role": "button", "name": "Submit", "baseline": {"x":100,"y":200,"width":80,"height":40}, "target": null }],
      "moved":       [],
      "resized":     []
    }
  }
}
```

**Exit codes:**
| Code | Meaning |
|------|---------|
| 0 | No regressions detected |
| 1 | Regressions detected (`regressions: true`) |
| 2 | Fatal error (no baseline, snapshot not found, I/O error) |

---

### `main.go` — Registration

Add `"diff"` case to the command switch:

```go
case "diff":
    cmd.RunDiff(cmdArgs)
```

Add to the usage string under a new "Regression Detection" section:

```
Regression Detection:
  snapshot [options]      Capture a snapshot (audit + ax-tree + screenshot)
  snapshot --set-baseline Mark a snapshot as the known-good baseline
  diff [snapshot-id]      Compare snapshot against baseline; exit 1 on regression
```

---

## Data Model Changes

No new persistent data is written by `ks diff`. It is a read-only operation over:

| File | Owner | Committed? |
|------|-------|------------|
| `.kaleidoscope/snapshots/<id>/snapshot.json` | US-002 | No (gitignored) |
| `.kaleidoscope/baselines.json` | US-001/002 | Yes |

---

## API Definitions

`ks diff` has no HTTP API. All I/O is stdin/stdout JSON following the existing `output.Result` convention.

---

## Security Considerations

- **Path traversal:** Snapshot IDs are used to construct file paths. The store loader must validate that the resolved path stays within `.kaleidoscope/snapshots/` using `filepath.Clean` and a prefix check before opening any file.
- **Untrusted snapshot content:** Snapshot JSON is loaded from the local filesystem written by previous `ks snapshot` runs. No remote content is fetched. No `exec` calls are made. The diff engine performs pure arithmetic — no eval, no template rendering, no shell expansion.
- **Exit code 1 is not an error:** Callers (pre-commit hooks, CI scripts) must not conflate exit 1 ("regressions found") with exit 2 ("fatal error"). The JSON `ok` field is always `true` when the diff itself succeeded, regardless of whether regressions were found.

---

## Testing Strategy

All tests live in `diff/diff_test.go`. The pure-function design means no mocks or browser setup are needed.

**Required test cases:**

| Test | Description |
|------|-------------|
| `TestCompareAudit_NoChange` | Identical audits → zero deltas, empty new/resolved |
| `TestCompareAudit_NewIssues` | Target has extra contrast violations → `contrastDelta > 0`, issues in `newIssues` |
| `TestCompareAudit_ResolvedIssues` | Target has fewer issues → issues in `resolvedIssues` |
| `TestCompareAudit_SelectorMatching` | Same selector = same issue; different selectors = different issues |
| `TestCompareElements_Appeared` | Node in target not in baseline → `Appeared` |
| `TestCompareElements_Disappeared` | Node in baseline not in target → `Disappeared` |
| `TestCompareElements_Moved` | Position delta > threshold → `Moved` |
| `TestCompareElements_MovedBelowThreshold` | Position delta <= threshold → no change |
| `TestCompareElements_Resized` | Size delta > threshold → `Resized` |
| `TestCompareElements_EmptyNameExcluded` | Nodes with empty name are skipped |
| `TestRegressionFlag` | `Regressions` is true iff new issues or element changes exist |
| `TestRunDiff_NoBaseline` | Exit 2, `ok: false` when no baseline |

Quality gate: `go test ./...` must pass (existing gate, unchanged).

---

## Implementation Checklist

1. `snapshot/types.go` — define `Snapshot`, `AuditSummary`, `AuditIssue`, `AXNode` (US-001/002 prerequisite; verify or stub if not present)
2. `snapshot/store.go` — implement `LoadBaseline`, `LoadLatestSnapshot`, `LoadSnapshot`, `ErrNoBaseline`
3. `diff/diff.go` — implement `DiffResult`, `AuditDiff`, `ElementDiff`, `Compare`, `CompareAudit`, `CompareElements`
4. `diff/diff_test.go` — full unit test suite
5. `cmd/diff.go` — implement `RunDiff`
6. `main.go` — register `"diff"` case and update usage string
