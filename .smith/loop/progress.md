# Progress Log

## US-005: Baseline Manager

### Run: kal-d05a5-autofix-kal-276cc-github-callmer-us-005 | Iteration 1

**Status:** in_progress (tests-only gate)

**Files Created:**
- `snapshot/index.go` — SnapshotMeta, Index types; LoadIndex() stub (US-003 contract)
- `snapshot/baseline.go` — BaselineEntry, Baselines types; LoadBaselines, SaveBaselines, Accept (stub returning nil,nil)
- `snapshot/baseline_test.go` — 7 unit tests covering all acceptance criteria

**Test Results:** `go test ./...`
- All other packages: no test files (pass)
- `snapshot` package: 6 FAIL, 1 PASS (TestAccept_DoesNotMutateCurrent passes on stub since nil doesn't mutate)
- Tests compile and fail as expected for tests-first iteration

**Key Learnings:**
- `browser.StateDir()` returns `(string, error)` — baseline functions must propagate the error
- Accept is a pure function; stubs returning `nil, nil` cause all assertion tests to fail on nil check
- Go toolchain accessed via `$HOME/.local/share/mise/shims:$PATH`

**Next Iteration:** N/A — story complete.

---

### Run: kal-d05a5-autofix-kal-276cc-github-callmer-us-005 | Iteration 2

**Status:** done

**Files Changed:**
- `snapshot/baseline.go` — Implemented Accept() pure function (deep-copy, idempotent, changed slice never nil)
- `cmd/accept.go` — Created RunAccept CLI handler (index load, snapshot resolution, --url filter, baseline save, output.Success)
- `cmd/util.go` — Added `--url` to value-taking flags in getNonFlagArgs
- `cmd/usage.go` — Added `"accept"` entry to CommandUsage map
- `main.go` — Added `case "accept": cmd.RunAccept(cmdArgs)`

**Test Results:** `go test ./...`
- `snapshot`: ok (7/7 tests pass)
- All other packages: no test files (pass)
- `go build ./...`: clean

**Key Learnings:**
- File uses tabs (not spaces) — Edit tool string matching failed on mixed-indentation strings; used sed fallback
- Accept() deep-copy via `copy(entries, current.Entries)` is sufficient since BaselineEntry contains only value types (strings)

