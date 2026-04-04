# Progress Log

## US-005: Baseline Manager

### Run kal-276cc / Iteration 1 (tests-only gate)

**Status:** in_progress

**Commands run:**
- `go build ./...` — PASS
- `go test ./...` — PASS (baseline: 6 tests, cmd: 6 tests)

**Files created:**
- `snapshot/store.go` — functional stub: `OpenStore`, `Save`, `Latest`, `ByID`
- `baseline/manager.go` — full implementation: `Manager`, `Entry`, `BaselinesFile`, `Load`, `Save`, `Accept`
- `baseline/manager_test.go` — 6 unit tests covering: empty-store create, idempotency, single-path filter, updated timestamp, missing-file load, atomic write
- `cmd/accept.go` — `RunAccept` + internal `acceptCmd(args, ksDir)` for testability
- `cmd/accept_test.go` — 6 integration tests: no snapshots, latest accepted, specific ID, unknown ID, --url not in snapshot, --url preserves others

**Files modified:**
- `cmd/util.go` — added `--url` to flag-value skip list in `getNonFlagArgs`
- `cmd/usage.go` — added `"accept"` entry
- `main.go` — added `case "accept": cmd.RunAccept(cmdArgs)`
- `.smith/loop/input/prd.json` — US-005 status → `in_progress`

**Key patterns:**
- `acceptCmd(args, ksDir)` pattern decouples `browser.StateDir()` from test logic
- `captureOutput()` helper in tests redirects `os.Stdout` to capture JSON output
- `baseline.Accept` is idempotent: skips entries where snapshotID already matches; only calls `Save` when something changed

### Run kal-276cc / Iteration 2 (verification)

**Status:** done

**Commands run:**
- `go test ./...` — PASS (baseline: 6, cmd: 6, all others: no test files)
- `go test ./baseline/... ./cmd/... -v` — all 12 tests PASS

**Outcome:** All production code implemented in iteration 1 passes quality gate. No changes needed this iteration. US-005 marked done.

