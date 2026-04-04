# Progress Log

## US-005: Baseline Manager

### Run kal-021d8-autofix-kal-276cc-github-callmer-us-005 — Iteration 2

**Status**: DONE — all quality gates pass

**Commands run:**
- `go clean -testcache && go test ./...` → PASS (cmd: 0.014s, snapshot: 0.007s)
- `go build ./...` → PASS

**Summary**: Iteration 2 verified all code from iteration 1 is correct. No new changes needed. All acceptance criteria satisfied by existing implementation.

---

### Run kal-021d8-autofix-kal-276cc-github-callmer-us-005 — Iteration 1

**Status**: complete (implementation iteration)

**Commands run:**
- `go build ./...` → PASS
- `go test ./...` → PASS (cmd: 0.007s, snapshot: 0.008s)

**Files created:**
- `snapshot/types.go` — Snapshot struct and Baselines type
- `snapshot/store.go` — ListSnapshots, LatestSnapshot, GetSnapshot with stateDirOverride for tests
- `snapshot/baseline.go` — ReadBaselines, WriteBaselines (atomic), AcceptSnapshot (pure function)
- `cmd/accept.go` — RunAccept CLI handler with osExit var for test injection
- `snapshot/baseline_test.go` — 6 tests covering all AcceptSnapshot scenarios + round-trip I/O
- `snapshot/store_test.go` — 5 tests covering invalid IDs, empty dir, and sort order
- `cmd/accept_test.go` — 3 integration tests via stdout capture

**Files modified:**
- `main.go` — added `case "accept"` + Snapshot History usage section
- `cmd/util.go` — added `--url` to value-consuming flags list

**Key patterns:**
- `stateDirOverride` package-level var (exported via `SetStateDirOverride`) for test isolation
- `osExit` var in cmd/accept.go for test-safe exit injection
- Atomic baselines write via temp file + os.Rename

