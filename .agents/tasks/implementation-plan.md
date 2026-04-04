# Implementation Plan: Project Config Commands (US-001)

## Overview

Introduce `.ks-project.json` and four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) that define a Kaleidoscope project as a named set of URL paths and breakpoints.

---

## Phase 1: `project` Package — Core Data Types and I/O

### Task 1.1: Create `project/config.go`

**File:** `project/config.go`

- Sub-task 1.1.1: Declare `package project` and define `const ConfigFile = ".ks-project.json"`.
- Sub-task 1.1.2: Define `Breakpoint` struct with JSON tags (`name`, `width`, `height`).
- Sub-task 1.1.3: Define `ProjectConfig` struct with JSON tags (`name`, `baseURL`, `paths`, `breakpoints`).
- Sub-task 1.1.4: Implement `DefaultBreakpoints() []Breakpoint` returning the four standard presets:
  - `mobile` 375x812
  - `tablet` 768x1024
  - `desktop` 1280x720
  - `wide` 1920x1080
- Sub-task 1.1.5: Implement `Load() (*ProjectConfig, error)`:
  - Read `filepath.Join(".", ConfigFile)`.
  - Return a descriptive error if file does not exist.
  - Unmarshal JSON into `ProjectConfig` and return.
- Sub-task 1.1.6: Implement `Save(cfg *ProjectConfig) error`:
  - Marshal `cfg` to JSON with 2-space indentation (`json.MarshalIndent`).
  - Write to `filepath.Join(".", ConfigFile)` with permissions `0644`.

### Task 1.2: Create `project/config_test.go`

**File:** `project/config_test.go`

- Sub-task 1.2.1: Write test `TestLoad_FileAbsent` — verify `Load()` returns a non-nil error when `.ks-project.json` does not exist. Use `t.TempDir()` + `os.Chdir`, restore CWD in `t.Cleanup`.
- Sub-task 1.2.2: Write test `TestSaveLoad_Roundtrip` — call `Save` with a populated `ProjectConfig`, then `Load`, assert all fields are preserved exactly.
- Sub-task 1.2.3: Write test `TestDefaultBreakpoints` — assert exactly 4 presets are returned and each has the correct `Name`, `Width`, `Height` values.

---

## Phase 2: `cmd/util.go` — Flag Parser Update

### Task 2.1: Update `getNonFlagArgs` in `cmd/util.go`

**File:** `cmd/util.go`

- Sub-task 2.1.1: Add `--name`, `--base-url`, and `--paths` to the list of value-taking flags inside the `getNonFlagArgs` skip condition, so their values are not misidentified as positional arguments.

---

## Phase 3: Command Handlers

### Task 3.1: Create `cmd/init.go` — `RunInit`

**File:** `cmd/init.go`

- Sub-task 3.1.1: Declare `func RunInit(args []string)`.
- Sub-task 3.1.2: Check whether `.ks-project.json` already exists using `os.Stat`; if yes, call `output.Fail("init", err, "project already initialised; delete .ks-project.json to reinitialise")` and `os.Exit(2)`.
- Sub-task 3.1.3: Parse `--name` via `getFlagValue`; if empty, call `output.Fail` with hint `"--name is required"` and exit.
- Sub-task 3.1.4: Parse `--base-url` via `getFlagValue`; if empty, call `output.Fail` with hint `"--base-url is required"` and exit.
- Sub-task 3.1.5: Parse `--paths` via `getFlagValue`; split on `,` and trim whitespace from each element; default to `["/"]` if not provided.
- Sub-task 3.1.6: Construct `project.ProjectConfig{Name, BaseURL, Paths, Breakpoints: project.DefaultBreakpoints()}`.
- Sub-task 3.1.7: Call `project.Save(cfg)`; on error call `output.Fail` and exit.
- Sub-task 3.1.8: Call `output.Success("init", cfg)`.

### Task 3.2: Create `cmd/project_add.go` — `RunProjectAdd`

**File:** `cmd/project_add.go`

- Sub-task 3.2.1: Declare `func RunProjectAdd(args []string)`.
- Sub-task 3.2.2: Extract positional argument via `getArg(args)`; if empty, call `output.Fail` with hint `"usage: ks project-add <path>"` and exit.
- Sub-task 3.2.3: Call `project.Load()`; on error call `output.Fail("project-add", err, "run 'ks init' first")` and exit.
- Sub-task 3.2.4: Iterate `cfg.Paths`; if the path already exists, call `output.Fail("project-add", errors.New("path already exists"), "")` and exit.
- Sub-task 3.2.5: Append the new path to `cfg.Paths`.
- Sub-task 3.2.6: Call `project.Save(cfg)`; on error call `output.Fail` and exit.
- Sub-task 3.2.7: Call `output.Success("project-add", cfg)`.

### Task 3.3: Create `cmd/project_remove.go` — `RunProjectRemove`

**File:** `cmd/project_remove.go`

- Sub-task 3.3.1: Declare `func RunProjectRemove(args []string)`.
- Sub-task 3.3.2: Extract positional argument via `getArg(args)`; if empty, call `output.Fail` with hint `"usage: ks project-remove <path>"` and exit.
- Sub-task 3.3.3: Call `project.Load()`; on error call `output.Fail("project-remove", err, "run 'ks init' first")` and exit.
- Sub-task 3.3.4: Search `cfg.Paths` for the target path; if not found, call `output.Fail("project-remove", errors.New("path not found"), "")` and exit.
- Sub-task 3.3.5: Remove the matched element by rebuilding the slice (filter out the target path).
- Sub-task 3.3.6: Call `project.Save(cfg)`; on error call `output.Fail` and exit.
- Sub-task 3.3.7: Call `output.Success("project-remove", cfg)`.

### Task 3.4: Create `cmd/project_show.go` — `RunProjectShow`

**File:** `cmd/project_show.go`

- Sub-task 3.4.1: Declare `func RunProjectShow(args []string)`.
- Sub-task 3.4.2: Call `project.Load()`; on error call `output.Fail("project-show", err, "run 'ks init' first")` and exit.
- Sub-task 3.4.3: Call `output.Success("project-show", cfg)`.

---

## Phase 4: `main.go` — Wire Up Commands

### Task 4.1: Add command cases to `main.go`

**File:** `main.go`

- Sub-task 4.1.1: Add four `case` branches in the `switch command` block:
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
- Sub-task 4.1.2: Add a `Project Config:` section to the `usage` string:
  ```
  Project Config:
    init --name <n> --base-url <u> [--paths /,/dash]   Initialise project config
    project-add <path>             Add a path to the project
    project-remove <path>          Remove a path from the project
    project-show                   Show current project config
  ```

---

## Phase 5: Tests for `cmd/` Handlers

### Task 5.1: Create `cmd/init_test.go`

**File:** `cmd/init_test.go`

- Sub-task 5.1.1: Test `RunInit` creates a valid `.ks-project.json` with correct `name`, `baseURL`, `paths`, and 4 default breakpoints. Use `t.TempDir()` + `os.Chdir` + `t.Cleanup`.
- Sub-task 5.1.2: Test `RunInit` exits with code 2 (via `output.Fail`) when `.ks-project.json` already exists. Verify the file is not overwritten.
- Sub-task 5.1.3: Test `RunInit` with `--paths /,/dashboard` produces `paths: ["/", "/dashboard"]` in the config.

### Task 5.2: Create `cmd/project_add_test.go`

**File:** `cmd/project_add_test.go`

- Sub-task 5.2.1: Test `RunProjectAdd` appends the given path and writes updated config to disk.
- Sub-task 5.2.2: Test `RunProjectAdd` fails (exit 2) when the path is already present in `cfg.Paths`.
- Sub-task 5.2.3: Test `RunProjectAdd` fails (exit 2) when `.ks-project.json` does not exist.

### Task 5.3: Create `cmd/project_remove_test.go`

**File:** `cmd/project_remove_test.go`

- Sub-task 5.3.1: Test `RunProjectRemove` removes the given path and writes updated config to disk.
- Sub-task 5.3.2: Test `RunProjectRemove` fails (exit 2) when the path does not exist in `cfg.Paths`.
- Sub-task 5.3.3: Test `RunProjectRemove` fails (exit 2) when `.ks-project.json` does not exist.

### Task 5.4: Create `cmd/project_show_test.go`

**File:** `cmd/project_show_test.go`

- Sub-task 5.4.1: Test `RunProjectShow` outputs the full config JSON via `output.Success`.
- Sub-task 5.4.2: Test `RunProjectShow` fails (exit 2) when `.ks-project.json` does not exist.

---

## Phase 6: Quality Gate

### Task 6.1: Verify all tests pass

- Sub-task 6.1.1: Run `go test ./...` from the workspace root and confirm all tests pass with no compilation errors.
- Sub-task 6.1.2: Verify `go build ./...` succeeds (no import cycles, no unused imports).

---

## File Change Summary

| File | Action |
|------|--------|
| `project/config.go` | Create |
| `project/config_test.go` | Create |
| `cmd/util.go` | Modify — add `--name`, `--base-url`, `--paths` to `getNonFlagArgs` skip list |
| `cmd/init.go` | Create |
| `cmd/project_add.go` | Create |
| `cmd/project_remove.go` | Create |
| `cmd/project_show.go` | Create |
| `cmd/init_test.go` | Create |
| `cmd/project_add_test.go` | Create |
| `cmd/project_remove_test.go` | Create |
| `cmd/project_show_test.go` | Create |
| `main.go` | Modify — add 4 `case` branches + `Project Config:` usage section |

## Dependencies Between Phases

```
Phase 1 (project package)
    → Phase 2 (util.go flag parsing)
    → Phase 3 (cmd handlers — all depend on Phase 1)
        → Phase 4 (main.go wiring — depends on Phase 3)
        → Phase 5 (cmd tests — depends on Phases 1 and 3)
            → Phase 6 (quality gate)
```
