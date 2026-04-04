# Progress Log

## US-003: Audit and Element Diff Engine

### Run kal-41edb-autofix-kal-0d599-github-callmer-us-003 — Iteration 1

**Status:** in_progress (tests-only iteration)

**Commands run:**
- `go test ./...` → PASS (diff: 0.030s)

**Files created:**
- `snapshot/types.go` — Snapshot, AuditSnapshot, AuditSummary, AuditIssueRecord, ElementRecord, ElementRect
- `snapshot/storage.go` — SnapshotsDir, BaselinesFile, sanitizeID, snapshotPath (with path-traversal protection)
- `snapshot/load.go` — LoadByID, LoadLatest, LoadBaseline
- `diff/types.go` — DiffResult, AuditDiff, ElementDiff, CategoryDeltas, IssueDelta, ElementChange, RectDelta, ElementRect
- `diff/audit.go` — ComputeAuditDiff, extractCounts
- `diff/elements.go` — ComputeElementDiff, SemanticKey, toElementRect, abs
- `diff/audit_test.go` — 5 test cases covering identical/regression/resolved/per-issue/typography
- `diff/elements_test.go` — 9 test cases + SemanticKey table tests

**Key learnings:**
- Go not on PATH by default; use `mise exec go@1.26.1 --` or `mise use go@1.26.1` first
- diff package is pure-function (no I/O, no browser); snapshot package handles all I/O
- IssueDelta.New/Resolved initialized as `[]AuditIssue{}` (not nil) to avoid JSON null

---

### Run kal-41edb-autofix-kal-0d599-github-callmer-us-003 — Iteration 2

**Status:** done

**Commands run:**
- `go build ./...` → PASS
- `go test ./...` → PASS (diff: cached)

**Files created/modified:**
- `cmd/diff.go` — RunDiff, parseFlagFloat (Phase 3 CLI integration)
- `cmd/util.go` — added `--pos-threshold`, `--size-threshold` to getNonFlagArgs
- `cmd/usage.go` — added "diff" entry to CommandUsage map
- `main.go` — added `case "diff": cmd.RunDiff(cmdArgs)` and "Snapshot & Regression" usage section

**Key learnings:**
- util.go uses tabs for indentation; Edit tool must match exact whitespace
- All acceptance criteria satisfied: ks diff [snapshot-id], baseline error, exit 0/1/2, structured JSON output

