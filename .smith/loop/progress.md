# Progress Log

## US-003: Audit and Element Diff Engine

### Run: kal-93484-autofix-kal-0d599-github-callmer-us-003 | Iteration 1

**Status**: in_progress (tests-only iteration)

**Commands run**:
- `go test ./...` → PASS (diff: 0.003s, all others no test files)
- `go build ./...` → OK
- `go vet ./...` → OK

**Files created**:
- `snapshot/types.go` — Snapshot, AuditSummary, AuditIssue, AXNode types
- `snapshot/store.go` — ErrNoBaseline, SnapshotsDir, ListSnapshots, LoadSnapshot, LoadLatestSnapshot, LoadBaseline
- `diff/diff.go` — DiffResult, AuditDiff, IssueDelta, ElementDiff, ElementChange, Rect, Delta types; Compare, CompareAudit, CompareElements functions; PositionThreshold=4.0, SizeThreshold=4.0 constants
- `diff/diff_test.go` — 14 tests covering all acceptance criteria scenarios

**Tests written** (all passing):
- TestCompareAudit_NoChange
- TestCompareAudit_NewIssues
- TestCompareAudit_ResolvedIssues
- TestCompareAudit_SelectorMatching
- TestCompareAudit_DetailIgnored
- TestCompareAudit_MultiCategory
- TestCompareAudit_EmptySlicesNotNil
- TestCompareElements_Appeared
- TestCompareElements_Disappeared
- TestCompareElements_Moved
- TestCompareElements_MovedBelowThreshold
- TestCompareElements_Resized
- TestCompareElements_EmptyNameExcluded
- TestCompareElements_MovedAndResized
- TestCompareElements_EmptySlicesNotNil
- TestRegressionFlag_FalseWhenClean
- TestRegressionFlag_TrueOnNewAuditIssue
- TestRegressionFlag_TrueOnElementChange

**Key patterns**:
- Semantic identity key: `role|name` (both lowercased/trimmed); empty-name nodes skipped
- Audit matching: `category:selector` key; detail text ignored for identity
- Separate Moved and Resized detection allows a node to appear in both
- All slices initialized to empty (not nil) for correct JSON `[]` serialization

**Remaining** (next iteration): cmd/diff.go + main.go wiring (Phase 4), then mark done

