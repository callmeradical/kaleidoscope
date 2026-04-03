# Progress Log

## US-005: Baseline Manager

### Run: kal-83e54-autofix-kal-276cc-github-callmer-us-005 | Iteration: 1

**Date:** 2026-04-03

**Status:** tests-only iteration complete — failing tests written

**Files Created:**
- `snapshot/snapshot.go` — minimal stub: `SnapshotEntry`, `URLEntry`, `Index`, `LoadIndex`, `Latest`, `ByID`
- `baseline/baseline.go` — stub with TODO implementations: `BaselineEntry`, `Baselines`, `Load`, `Save`, `Accept`, `ForPath`
- `baseline/baseline_test.go` — 7 failing tests covering all acceptance criteria for the `baseline` package
- `cmd/accept.go` — stub `RunAccept` returning `output.Fail("accept", errors.New("not implemented"), "")`
- `cmd/accept_test.go` — 9 failing integration tests covering all `ks accept` acceptance criteria

**Test Results:**
```
baseline: 5 FAIL (TestLoad_Valid, TestAccept_Insert, TestAccept_Update, TestSave_Atomic, TestForPath_Found)
cmd:      9 FAIL (all accept tests return "not implemented")
No regressions in pre-existing packages
```

**Key Learnings:**
- `browser.StateDir()` checks for `.kaleidoscope/` in CWD, so cmd tests use `os.Chdir(t.TempDir())` with cleanup
- Stdout capture uses `os.Pipe()` to intercept `output.Success`/`output.Fail` calls
- The `snapshot.LoadIndex` path is `dir/snapshots/index.json` (not `dir/.kaleidoscope/snapshots/index.json`) — the caller passes the `.kaleidoscope/` dir
- `baselines.json` key is `"baselines"` (array field name matches struct tag)

**Next Iteration:** Implement `baseline` package and `cmd/accept.go` to make all tests pass.

---

### Run: kal-83e54-autofix-kal-276cc-github-callmer-us-005 | Iteration: 2

**Date:** 2026-04-03

**Status:** DONE — all tests pass

**Files Changed:**
- `baseline/baseline.go` — full implementation: `Load`, `Save` (atomic via tmp+rename), `Accept`, `ForPath`
- `cmd/accept.go` — full `RunAccept` implementation with snapshot ID validation, URL filter, idempotent accept
- `cmd/util.go` — added `--url` to the list of flags that consume the next argument in `getNonFlagArgs`
- `main.go` — wired `case "accept": cmd.RunAccept(cmdArgs)`

**Test Results:**
```
baseline: PASS (7/7 tests)
cmd:      PASS (9/9 accept tests + no regressions)
go test ./...: all packages pass
```

**Key Learnings:**
- `Save()` temp file pattern: `os.CreateTemp(dir, "baselines-*.json.tmp")` then `os.Rename` — works atomically
- `updated`/`skipped` initialized as `[]string{}` (not nil) to ensure JSON renders `[]` not `null`
- `--url` filter skips non-matching paths entirely (not added to skipped), matching test assertions
