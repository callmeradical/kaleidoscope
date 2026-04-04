# Progress Log

## US-001: Project Config Commands

### Run kal-960c4-github-callmeradical-kaleidoscop-us-001 — Iteration 1 (tests-only)

**Status:** in_progress

**Files created:**
- `project/project.go` — full implementation: `Load`, `Save`, `Validate`, `DefaultBreakpoints`, `Config`, `Breakpoint` types
- `project/project_test.go` — 7 tests: Load_NotFound, SaveLoad_RoundTrip, Validate_Empty{Name,BaseURL,Paths}, Validate_Valid, DefaultBreakpoints
- `cmd/init.go` — stub (empty body)
- `cmd/project_add.go` — stub (empty body)
- `cmd/project_remove.go` — stub (empty body)
- `cmd/project_show.go` — stub (empty body)
- `cmd/testmain_test.go` — TestMain builds binary; `runKS`, `resultMap`, `initProject` helpers
- `cmd/init_test.go` — 6 tests covering all acceptance criteria for `ks init`
- `cmd/project_add_test.go` — 4 tests covering all acceptance criteria for `ks project-add`
- `cmd/project_remove_test.go` — 4 tests covering all acceptance criteria for `ks project-remove`
- `cmd/project_show_test.go` — 2 tests covering all acceptance criteria for `ks project-show`

**Files edited:**
- `cmd/util.go` — added `--name`, `--base-url`, `--paths` to `getNonFlagArgs` skip list
- `main.go` — added `init`, `project-add`, `project-remove`, `project-show` case statements

**Test results:**
- `go test ./project/...` → **7/7 PASS** ✅
- `go test ./cmd/... -run TestRunInit|TestRunProject` → **16/16 FAIL** (RED — stubs produce no output) ✅ expected for tests-only iteration

**Key learnings:**
- cmd tests use binary exec via TestMain; tests are RED because stubs exit 0 with no JSON output
- `cmd.Output()` in Go captures stdout even when process exits non-zero (via *exec.ExitError)
- project package is pure I/O, safe to fully implement before cmd integration

**Next iteration:** Implement `RunInit`, `RunProjectAdd`, `RunProjectRemove`, `RunProjectShow` so cmd tests go GREEN.

---

### Run kal-960c4-github-callmeradical-kaleidoscop-us-001 — Iteration 2 (implementation)

**Status:** done

**Files edited:**
- `cmd/init.go` — implemented `RunInit`: parses --name/--base-url/--paths, checks for existing config, validates paths start with /, saves Config with DefaultBreakpoints
- `cmd/project_add.go` — implemented `RunProjectAdd`: loads config, checks for duplicate path, appends and saves
- `cmd/project_remove.go` — implemented `RunProjectRemove`: loads config, checks path exists, filters out and saves
- `cmd/project_show.go` — implemented `RunProjectShow`: loads config and outputs full struct via output.Success

**Test results:**
- `go test ./...` → **ALL PASS** ✅ (cmd: ok 1.117s, project: ok cached)

**Key learnings:**
- output.Fail takes an error (not string), use errors.New or fmt.Errorf
- project_remove: when all paths removed, cfg.Paths becomes nil; JSON marshals nil slice as null vs [] — acceptable for acceptance criteria
- Go binary build is cached by TestMain; changes take effect on next test run which rebuilds

