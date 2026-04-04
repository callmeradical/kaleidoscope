# Progress Log

## US-001: Project Config Commands

### Run kal-3254b-autofix-kal-960c4-github-callmer-us-001 | Iteration 1

**Status:** in_progress (tests-only iteration)

**Commands run:**
- `go build ./...` → PASS
- `go test ./...` → PASS (ok github.com/callmeradical/kaleidoscope/cmd 0.052s)

**Files changed:**
- `cmd/project.go` — Created: ProjectConfig/Breakpoint structs, defaultProjectBreakpoints, readProjectConfig/writeProjectConfig helpers, RunInit, RunProjectAdd, RunProjectRemove, RunProjectShow
- `cmd/project_test.go` — Created: 14 tests covering all 8 acceptance criteria (success paths + error/subprocess tests for os.Exit paths)
- `main.go` — Added: 4 switch cases (init, project-add, project-remove, project-show) + Project Config usage section
- `cmd/usage.go` — Added: 4 CommandUsage entries (init, project-add, project-remove, project-show)
- `cmd/util.go` — Added: --name, --base-url, --paths to flag-value skip set in getNonFlagArgs

**Test results (14 tests, all PASS):**
- TestRunInit_Success, TestRunInit_DefaultBreakpoints, TestRunInit_PathsTrimmed, TestRunInit_OutputJSON, TestRunInit_ErrorIfExists (subprocess)
- TestRunProjectAdd_Success, TestRunProjectAdd_OutputJSON, TestRunProjectAdd_DuplicateError (subprocess)
- TestRunProjectRemove_Success, TestRunProjectRemove_OutputJSON, TestRunProjectRemove_PreservesOrder, TestRunProjectRemove_NotFoundError (subprocess)
- TestRunProjectShow_Success, TestRunProjectShow_MissingFileError (subprocess)

**Key learnings:**
- Error-path tests (os.Exit) require subprocess pattern with TEST_SUBPROCESS env var
- captureStdout() via os.Pipe is needed for success-path JSON output verification
- Go binary: ~/.local/share/mise/installs/go/1.22.12/bin/go (actual version: go1.26.1)

### Run kal-3254b-autofix-kal-960c4-github-callmer-us-001 | Iteration 2

**Status:** done

**Commands run:**
- `go test ./... -count=1` → PASS (ok github.com/callmeradical/kaleidoscope/cmd 0.030s)

**Files changed:**
- None (all work completed in iteration 1, committed as 165e050)

**Verification:**
- All 14 tests pass fresh (no cache)
- US-001 status updated to `done` in PRD JSON

**Key learnings:**
- Iteration 2 was a verification-only pass; no code changes needed
