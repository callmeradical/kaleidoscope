# Implementation Plan: US-001 — Project Config Commands

## Overview

Introduce `.ks-project.json` as a project-level configuration file and four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) to the Kaleidoscope tool.

**Quality Gate:** `go test ./...` must pass.

---

## Phase 1: Data Structures and File I/O Helpers

### Task 1.1: Create `cmd/project.go` with structs and constants

**File:** `cmd/project.go`

- **Sub-task 1.1.1:** Declare package `cmd` and import required packages: `encoding/json`, `fmt`, `os`, `strings`, `github.com/callmeradical/kaleidoscope/output`.
- **Sub-task 1.1.2:** Define the `projectConfigFile` constant:
  ```go
  const projectConfigFile = ".ks-project.json"
  ```
- **Sub-task 1.1.3:** Define the `Breakpoint` struct (exported, separate from the unexported `breakpoint` in `cmd/breakpoints.go`):
  ```go
  type Breakpoint struct {
      Name   string `json:"name"`
      Width  int    `json:"width"`
      Height int    `json:"height"`
  }
  ```
- **Sub-task 1.1.4:** Define the `ProjectConfig` struct:
  ```go
  type ProjectConfig struct {
      Name        string       `json:"name"`
      BaseURL     string       `json:"baseURL"`
      Paths       []string     `json:"paths"`
      Breakpoints []Breakpoint `json:"breakpoints"`
  }
  ```
- **Sub-task 1.1.5:** Define the `defaultProjectBreakpoints` variable with four standard presets:
  - `{"mobile", 375, 812}`
  - `{"tablet", 768, 1024}`
  - `{"desktop", 1280, 720}`
  - `{"wide", 1920, 1080}`

### Task 1.2: Implement `readProjectConfig()` helper

**File:** `cmd/project.go`

- **Sub-task 1.2.1:** Implement `readProjectConfig() (*ProjectConfig, error)`:
  - Read `.ks-project.json` from CWD using `os.ReadFile(projectConfigFile)`.
  - If file does not exist (`os.IsNotExist`), return a descriptive error: `"no .ks-project.json found in current directory"`.
  - JSON-decode the bytes into a `ProjectConfig` and return it.

### Task 1.3: Implement `writeProjectConfig()` helper

**File:** `cmd/project.go`

- **Sub-task 1.3.1:** Implement `writeProjectConfig(cfg *ProjectConfig) error`:
  - Marshal `cfg` with `json.MarshalIndent` using 2-space indent.
  - Write bytes to `projectConfigFile` using `os.WriteFile` with permissions `0644`.
  - Return any error encountered.

---

## Phase 2: Command Handler Functions

### Task 2.1: Implement `RunInit`

**File:** `cmd/project.go`

- **Sub-task 2.1.1:** Parse `--name`, `--base-url`, and `--paths` from `args` using the existing `getFlagValue()` helper from `cmd/util.go`.
- **Sub-task 2.1.2:** Validate all three flags are non-empty; call `output.Fail("init", fmt.Errorf("..."), "")` and `os.Exit(2)` for any missing required flag.
- **Sub-task 2.1.3:** Check for existing `.ks-project.json` via `os.Stat(projectConfigFile)`. If file exists, call `output.Fail("init", fmt.Errorf(".ks-project.json already exists"), "")` and `os.Exit(2)`.
- **Sub-task 2.1.4:** Split the `--paths` value on `,` using `strings.Split`; trim whitespace from each element with `strings.TrimSpace`.
- **Sub-task 2.1.5:** Build a `ProjectConfig` with parsed values and `defaultProjectBreakpoints`.
- **Sub-task 2.1.6:** Call `writeProjectConfig(&cfg)`. On error, call `output.Fail` and `os.Exit(2)`.
- **Sub-task 2.1.7:** Call `output.Success("init", map[string]any{"path": projectConfigFile, "name": cfg.Name, "baseURL": cfg.BaseURL, "paths": cfg.Paths, "breakpoints": cfg.Breakpoints})`.

### Task 2.2: Implement `RunProjectAdd`

**File:** `cmd/project.go`

- **Sub-task 2.2.1:** Get path argument using `getArg(args)`. If empty, call `output.Fail("project-add", fmt.Errorf("path argument required"), "")` and `os.Exit(2)`.
- **Sub-task 2.2.2:** Call `readProjectConfig()`. On error, call `output.Fail("project-add", err, "Run: ks init --name <name> --base-url <url> --paths /")` and `os.Exit(2)`.
- **Sub-task 2.2.3:** Linear scan of `cfg.Paths` for duplicate. If found, call `output.Fail("project-add", fmt.Errorf("path already exists: %s", path), "")` and `os.Exit(2)`.
- **Sub-task 2.2.4:** Append path to `cfg.Paths` and call `writeProjectConfig`. On error, call `output.Fail` and `os.Exit(2)`.
- **Sub-task 2.2.5:** Call `output.Success("project-add", map[string]any{"path": path, "paths": cfg.Paths})`.

### Task 2.3: Implement `RunProjectRemove`

**File:** `cmd/project.go`

- **Sub-task 2.3.1:** Get path argument using `getArg(args)`. If empty, call `output.Fail("project-remove", fmt.Errorf("path argument required"), "")` and `os.Exit(2)`.
- **Sub-task 2.3.2:** Call `readProjectConfig()`. On error, call `output.Fail("project-remove", err, "Run: ks init --name <name> --base-url <url> --paths /")` and `os.Exit(2)`.
- **Sub-task 2.3.3:** Linear scan of `cfg.Paths` for the path. If not found, call `output.Fail("project-remove", fmt.Errorf("path not found: %s", path), "")` and `os.Exit(2)`.
- **Sub-task 2.3.4:** Remove the element from the slice (preserve order using append of slices before and after index). Call `writeProjectConfig`. On error, call `output.Fail` and `os.Exit(2)`.
- **Sub-task 2.3.5:** Call `output.Success("project-remove", map[string]any{"path": path, "paths": cfg.Paths})`.

### Task 2.4: Implement `RunProjectShow`

**File:** `cmd/project.go`

- **Sub-task 2.4.1:** Call `readProjectConfig()`. On error, call `output.Fail("project-show", err, "Run: ks init --name <name> --base-url <url> --paths /")` and `os.Exit(2)`.
- **Sub-task 2.4.2:** Call `output.Success("project-show", cfg)` — marshals the full struct as the result payload.

---

## Phase 3: CLI Wiring

### Task 3.1: Update `main.go` — add switch cases

**File:** `main.go`

- **Sub-task 3.1.1:** Locate the existing `switch command` block.
- **Sub-task 3.1.2:** Add four new cases at a logical position (e.g., after existing non-browser commands or grouped with future project commands):
  ```go
  case "init":
      cmd.RunInit(cmdArgs)
  case "project-add":
      cmd.RunProjectAdd(cmdArgs)
  case "project-remove":
      cmd.RunProjectRemove(cmdArgs)
  case "project-show":
      cmd.RunProjectShow(cmdArgs)
  ```

### Task 3.2: Update `main.go` — add usage section

**File:** `main.go`

- **Sub-task 3.2.1:** Locate the top-level usage string printed when no command or `--help` is given.
- **Sub-task 3.2.2:** Add a new `Project Config:` section with four entries:
  ```
  Project Config:
    init --name <n> --base-url <u> --paths <p>   Create .ks-project.json
    project-add <path>                            Add a path to the project
    project-remove <path>                         Remove a path from the project
    project-show                                  Show project config as JSON
  ```

### Task 3.3: Update `cmd/usage.go` — add four CommandUsage entries

**File:** `cmd/usage.go`

- **Sub-task 3.3.1:** Add entry for `"init"` with syntax, options (`--name`, `--base-url`, `--paths`), output shape, and notes about default breakpoints and error on existing file.
- **Sub-task 3.3.2:** Add entry for `"project-add"` with argument, output shape, and duplicate-error note.
- **Sub-task 3.3.3:** Add entry for `"project-remove"` with argument, output shape, and not-found-error note.
- **Sub-task 3.3.4:** Add entry for `"project-show"` with output shape and note to run `ks init` first.

---

## Phase 4: Tests

### Task 4.1: Create `cmd/project_test.go`

**File:** `cmd/project_test.go`

- **Sub-task 4.1.1:** Declare package `cmd` and import `encoding/json`, `os`, `path/filepath`, `strings`, `testing`.
- **Sub-task 4.1.2:** Create a helper `setupTempDir(t *testing.T) (string, func())` that:
  - Creates a temp directory via `t.TempDir()`.
  - Saves the original CWD.
  - `os.Chdir`s into the temp dir.
  - Returns original CWD and a cleanup function that `os.Chdir`s back.
- **Sub-task 4.1.3:** Create a helper `captureOutput(fn func()) string` (or use a pipe-based approach) to capture stdout JSON from command handlers. Since `output.Success`/`output.Fail` print to stdout, use `os.Pipe()` + redirect `os.Stdout` temporarily, or factor tests to read the written file directly and check exit-code-free paths.

  **Note:** Because `output.Fail` calls `os.Exit(2)`, error-path tests must use `exec.Command` subprocess pattern (run test binary as subprocess) or use `t.SkipNow()` guards. Alternatively, tests may focus on verifying file content for success paths and use integration-style subprocess tests for error paths.

### Task 4.2: Test scenario — `RunInit` success

- **Sub-task 4.2.1:** Call `RunInit([]string{"--name", "my-app", "--base-url", "https://example.com", "--paths", "/,/dashboard"})` in temp dir.
- **Sub-task 4.2.2:** Read and JSON-decode `.ks-project.json`.
- **Sub-task 4.2.3:** Assert `cfg.Name == "my-app"`, `cfg.BaseURL == "https://example.com"`, `cfg.Paths == ["/", "/dashboard"]`.
- **Sub-task 4.2.4:** Assert `cfg.Breakpoints` equals the four default presets (mobile, tablet, desktop, wide) with correct widths/heights.

### Task 4.3: Test scenario — `RunInit` errors on existing file

- **Sub-task 4.3.1:** Pre-create `.ks-project.json` in temp dir.
- **Sub-task 4.3.2:** Run `RunInit` in a subprocess (using `os.Args[0]` exec pattern with a test helper flag) and assert exit code is 2.
- **Sub-task 4.3.3:** Assert stdout JSON contains `"ok": false` and error message referencing "already exists".

### Task 4.4: Test scenario — `RunProjectAdd` success

- **Sub-task 4.4.1:** Write a valid `.ks-project.json` with `paths: ["/"]` to temp dir.
- **Sub-task 4.4.2:** Call `RunProjectAdd([]string{"/settings"})`.
- **Sub-task 4.4.3:** Read and decode `.ks-project.json`; assert `cfg.Paths == ["/", "/settings"]`.

### Task 4.5: Test scenario — `RunProjectAdd` duplicate path error

- **Sub-task 4.5.1:** Write a valid `.ks-project.json` with `paths: ["/", "/settings"]` to temp dir.
- **Sub-task 4.5.2:** Run `RunProjectAdd([]string{"/settings"})` via subprocess.
- **Sub-task 4.5.3:** Assert exit code 2 and `"ok": false` JSON with "already exists" message.

### Task 4.6: Test scenario — `RunProjectRemove` success

- **Sub-task 4.6.1:** Write a valid `.ks-project.json` with `paths: ["/", "/settings"]` to temp dir.
- **Sub-task 4.6.2:** Call `RunProjectRemove([]string{"/settings"})`.
- **Sub-task 4.6.3:** Read and decode `.ks-project.json`; assert `cfg.Paths == ["/"]`.

### Task 4.7: Test scenario — `RunProjectRemove` non-existent path error

- **Sub-task 4.7.1:** Write a valid `.ks-project.json` with `paths: ["/"]` to temp dir.
- **Sub-task 4.7.2:** Run `RunProjectRemove([]string{"/settings"})` via subprocess.
- **Sub-task 4.7.3:** Assert exit code 2 and `"ok": false` JSON with "not found" message.

### Task 4.8: Test scenario — `RunProjectShow` success

- **Sub-task 4.8.1:** Write a valid `.ks-project.json` to temp dir.
- **Sub-task 4.8.2:** Capture stdout and call `RunProjectShow(nil)`.
- **Sub-task 4.8.3:** Decode captured JSON; assert `result.ok == true` and result payload matches the written config.

### Task 4.9: Test scenario — `RunProjectShow` missing file error

- **Sub-task 4.9.1:** Run `RunProjectShow(nil)` in a temp dir with no `.ks-project.json` via subprocess.
- **Sub-task 4.9.2:** Assert exit code 2 and `"ok": false` JSON with hint containing "ks init".

---

## Phase 5: Flag Parsing Update (if needed)

### Task 5.1: Verify `getFlagValue` handles new flags

**File:** `cmd/util.go`

- **Sub-task 5.1.1:** Read `cmd/util.go` to check which flags are listed in the hardcoded "flags that take values" set (used by `getArg` to skip flag values).
- **Sub-task 5.1.2:** If `--name`, `--base-url`, and `--paths` are not in that set, add them so that `getArg()` correctly skips their values when searching for positional arguments. (This only matters if `RunInit` uses `getArg` — since it doesn't, this may be a no-op, but verify.)

---

## Phase 6: Verification

### Task 6.1: Run tests

- **Sub-task 6.1.1:** Run `go test ./...` from `/workspace` and confirm all tests pass.
- **Sub-task 6.1.2:** If any test fails due to stdout capture issues with `output.Fail`/`os.Exit`, refactor error-path tests to use subprocess execution pattern.

### Task 6.2: Manual smoke test (optional, if browser available)

- **Sub-task 6.2.1:** Build binary with `go build -o ks .`.
- **Sub-task 6.2.2:** Run `ks init --name test --base-url https://example.com --paths /,/dashboard` and verify output JSON.
- **Sub-task 6.2.3:** Run `ks project-add /settings` and verify paths updated.
- **Sub-task 6.2.4:** Run `ks project-remove /settings` and verify paths updated.
- **Sub-task 6.2.5:** Run `ks project-show` and verify full config output.
- **Sub-task 6.2.6:** Verify duplicate add and missing-file remove both exit 2 with `"ok": false`.

---

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `cmd/project.go` | Create | Structs, helpers, four `Run*` functions |
| `cmd/project_test.go` | Create | Unit tests for all 8 scenarios |
| `main.go` | Edit | Add 4 switch cases + Project Config usage section |
| `cmd/usage.go` | Edit | Add 4 `CommandUsage` entries |
| `cmd/util.go` | Edit (maybe) | Add `--name`, `--base-url`, `--paths` to flag-value set if needed |

## Dependency Map

```
Phase 1 (data layer)
  └─► Phase 2 (command handlers, depend on Phase 1 helpers)
        └─► Phase 3 (CLI wiring, depends on Phase 2 exports)
        └─► Phase 4 (tests, depend on Phase 1 & 2)
Phase 5 (util check, independent, can run in parallel with Phase 1)
Phase 6 (verification, depends on all prior phases)
```
