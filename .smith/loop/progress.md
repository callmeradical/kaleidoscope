# Progress Log

## US-002: Snapshot Capture and History


### Run: kal-c2358-autofix-kal-7e88f-github-callmer-us-002 | Iteration 1 | 2026-04-03

**Status**: Tests-only iteration (TDD red phase)

**Files Created**:
- `project/config.go` — Full implementation of `ProjectConfig`, `LoadConfig`, `FindConfig`
- `project/config_test.go` — 4 tests; all pass (project package fully implemented)
- `snapshot/model.go` — Data structures: `AuditSummary`, `URLEntry`, `ProjectConfig`, `Manifest`
- `snapshot/store.go` — Stubs: `URLDir` returns `""`, `GenerateID` returns `""`. Full impl: `WriteManifest`, `ReadManifest`, `List`, `SnapshotsDir`, `CreateDir`
- `snapshot/store_test.go` — 9 tests: 6 FAIL (URLDir×5, GenerateID×1), 3 PASS
- `snapshot/baseline.go` — Full implementation: `Baseline`, `BaselinePath`, `ReadBaseline`, `WriteBaseline`, `EnsureBaseline`
- `snapshot/baseline_test.go` — 4 tests; all pass (baseline fully implemented)

**go test ./... Result**: FAIL (expected — red phase)
- `project`: ok (all 4 pass)
- `snapshot`: FAIL — 6 failing tests (URLDir×5, GenerateID×1)

**Failing Tests (TDD red)**:
- `TestURLDir_Root` — stub returns `""`, want `"root"`
- `TestURLDir_SingleSegment` — stub returns `""`, want `"about"`
- `TestURLDir_MultiSegment` — stub returns `""`, want `"products_items"`
- `TestURLDir_StripQueryString` — stub returns `""`, want `"page"`
- `TestURLDir_SpecialChars` — stub returns `""`, want non-empty
- `TestGenerateID_Format` — stub returns `""`

**Key Learnings**:
- `t.Chdir()` available in Go 1.26 — used for CWD-dependent tests (SnapshotsDir, BaselinePath)
- `browser.StateDir()` checks `./kaleidoscope` first — tests create this dir in TempDir and chdir there
- URLDir and GenerateID left as stubs to define the red test boundary for iteration 2

**Next Iteration**: Implement `URLDir` and `GenerateID` logic to make the 6 failing tests pass; then implement Phases 5-12 (cmd/capture_helpers, cmd/snapshot.go, cmd/history.go, main.go wiring).

### Run: kal-c2358-autofix-kal-7e88f-github-callmer-us-002 | Iteration 2 | 2026-04-03

**Status**: COMPLETE — all acceptance criteria implemented, `go test ./...` PASS

**Files Changed**:
- `snapshot/store.go` — Implemented `URLDir` (url.Parse path extraction, sanitize to `[a-zA-Z0-9_-]`, collapse `__`, trim `_`, return `"root"` for empty) and `GenerateID` (unix-ms + git rev-parse --short HEAD with 2s timeout)
- `cmd/capture_helpers.go` — New: `validateURL` (http/https only), `captureAuditData`, `captureAxTreeData`, `captureBreakpointsData`, `writeJSON`
- `cmd/snapshot.go` — New: `RunSnapshot` (full capture loop: project config, generate ID, per-URL dir, breakpoints+audit+axtree, manifest write, baseline promotion)
- `cmd/history.go` — New: `RunHistory` (list snapshots with optional --limit, summary output)
- `main.go` — Added `case "snapshot"` and `case "history"` to command switch; added "Snapshot & History" section to usage string
- `.gitignore` — Replaced blanket `.kaleidoscope/` with specific entries: `.kaleidoscope/snapshots/`, `.kaleidoscope/screenshots/`, `.kaleidoscope/state.json` (baselines.json is committed, not ignored)

**go test ./... Result**: PASS (project: ok, snapshot: ok)

**Key Learnings**:
- `url.Parse` handles query stripping cleanly via `.Path` field — no manual splitting needed
- Collapsing consecutive underscores requires a loop (`strings.Contains` + `strings.ReplaceAll`) since Go lacks a "replace all repeated" function
- `captureBreakpointsData` writes `<breakpoint-name>.png` (not timestamped) for stable snapshot filenames
- `.gitignore` needed to be narrowed: blanket `.kaleidoscope/` exclusion would prevent committing `baselines.json`
