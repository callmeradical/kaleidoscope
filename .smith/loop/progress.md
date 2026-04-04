# Progress Log

## US-001: Project Config Commands

### Run: kal-869bc-autofix-kal-960c4-github-callmer-us-001 | Iteration 1

**Status:** in_progress (tests-only iteration)

**Files Changed:**
- `cmd/project_test.go` — created (new, 270 lines)
- `.smith/loop/input/prd.json` — US-001 status set to `in_progress`

**Commands Run:**
- `/usr/local/go/bin/go test ./...` → FAIL (expected: build failed due to undefined symbols)

**Failing Tests Added (cmd/project_test.go):**
- `TestHelperProcess` — subprocess helper for os.Exit-based command testing
- `TestDefaultBreakpoints` — verifies 4 default breakpoints (mobile/tablet/desktop/wide) with correct dimensions
- `TestProjectConfigJSONRoundtrip` — struct marshal/unmarshal round-trip
- `TestProjectConfigJSONFieldNames` — JSON field names: name, baseUrl, paths, breakpoints
- `TestSaveAndLoadProjectConfig` — saveProjectConfig writes file; loadProjectConfig reads it back
- `TestLoadProjectConfigNotFound` — loadProjectConfig returns os.IsNotExist error when no file
- `TestRunInit_CreatesConfig` — init creates .ks-project.json with correct content, exits 0
- `TestRunInit_DefaultBreakpoints` — init without --paths still gets 4 default breakpoints
- `TestRunInit_AlreadyExists` — init fails (exit non-0, ok=false) when config exists
- `TestRunInit_MissingName` — init fails when --name missing
- `TestRunInit_MissingBaseURL` — init fails when --base-url missing
- `TestRunProjectAdd_AppendPath` — project-add appends path to config
- `TestRunProjectAdd_DuplicatePath` — project-add fails for duplicate path
- `TestRunProjectAdd_NoConfig` — project-add fails when project not initialized
- `TestRunProjectRemove_RemovesPath` — project-remove removes path from config
- `TestRunProjectRemove_NotFound` — project-remove fails for non-existent path
- `TestRunProjectRemove_NoConfig` — project-remove fails when project not initialized
- `TestRunProjectShow_OutputsConfig` — project-show outputs valid JSON result
- `TestRunProjectShow_NoConfig` — project-show fails when project not initialized
- `TestGetNonFlagArgs_SkipsNewProjectFlags` — --name/--base-url/--paths values not treated as positional args
- `TestGetNonFlagArgs_NameValueSkipped` — --name value skipped
- `TestGetNonFlagArgs_BaseURLValueSkipped` — --base-url value skipped
- `TestGetNonFlagArgs_PathsValueSkipped` — --paths value skipped

**Compilation errors (confirming tests are failing):**
```
cmd/project_test.go:31:3: undefined: RunInit
cmd/project_test.go:33:3: undefined: RunProjectAdd
cmd/project_test.go:35:3: undefined: RunProjectRemove
cmd/project_test.go:37:3: undefined: RunProjectShow
cmd/project_test.go:87:9: undefined: defaultProjectBreakpoints
cmd/project_test.go:107:9: undefined: ProjectConfig
cmd/project_test.go:111:18: undefined: ProjectBreakpoint
... (too many errors)
```

**Key Patterns:**
- Commands use `os.Exit(2)` on error, requiring subprocess-based testing via `TestHelperProcess` pattern
- `output.Fail` takes `error` not `string` — implementation must use `errors.New(...)` or `fmt.Errorf(...)`
- Test binary subprocess pattern: `exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", cmdName, args...)`
- `loadProjectConfig`/`saveProjectConfig` are unexported — tests in `package cmd` (internal)
- `projectConfigFile = ".ks-project.json"` is a constant used by both helpers

**Next Iteration (2):**
- Implement `cmd/project_config.go` (structs + helpers)
- Implement `cmd/project.go` (RunInit, RunProjectAdd, RunProjectRemove, RunProjectShow)
- Modify `cmd/util.go` to add --name/--base-url/--paths to getNonFlagArgs skip list
- Modify `cmd/usage.go` and `main.go` to wire in new commands
- Quality gate: `go test ./...` must pass

---

### Run: kal-869bc-autofix-kal-960c4-github-callmer-us-001 | Iteration 2

**Status:** done

**Files Changed:**
- `cmd/project_config.go` — created: ProjectConfig, ProjectBreakpoint structs; defaultProjectBreakpoints; loadProjectConfig, saveProjectConfig helpers
- `cmd/project.go` — created: RunInit, RunProjectAdd, RunProjectRemove, RunProjectShow
- `cmd/util.go` — modified: added --name, --base-url, --paths to getNonFlagArgs skip list
- `cmd/usage.go` — modified: added CommandUsage entries for init, project-add, project-remove, project-show
- `main.go` — modified: added 4 case branches + Project section in usage string
- `.smith/loop/input/prd.json` — US-001 status set to `done`

**Commands Run:**
- `/usr/local/go/bin/go build ./...` → PASS (no output)
- `/usr/local/go/bin/go test ./...` → PASS (`ok github.com/callmeradical/kaleidoscope/cmd 0.209s`)

**Key Patterns:**
- output.Fail takes an `error` type — used `errors.New(...)` throughout
- getArg delegates to getNonFlagArgs, so the skip list fix was essential for project-add/remove positional arg detection
- defaultProjectBreakpoints is a package-level var (not const) — slice of structs

