# Implementation Plan: Project Config Commands (US-001)

## Overview

Implement four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) that manage a `.ks-project.json` file in the current working directory.

Files to create: `cmd/project_config.go`, `cmd/project.go`
Files to modify: `cmd/util.go`, `cmd/usage.go`, `main.go`

---

## Phase 1: Data Model and Config Helpers

**Goal:** Introduce the `ProjectConfig` struct and file I/O helpers that all four commands depend on.

### Task 1.1 — Create `cmd/project_config.go`

- **Sub-task 1.1.1:** Declare package `cmd` and import `encoding/json`, `os`.
- **Sub-task 1.1.2:** Define `ProjectBreakpoint` struct with fields `Name string`, `Width int`, `Height int` and JSON tags `name`, `width`, `height`.
- **Sub-task 1.1.3:** Define `ProjectConfig` struct with fields `Name string` (`json:"name"`), `BaseURL string` (`json:"baseUrl"`), `Paths []string` (`json:"paths"`), `Breakpoints []ProjectBreakpoint` (`json:"breakpoints"`).
- **Sub-task 1.1.4:** Declare constant `projectConfigFile = ".ks-project.json"`.
- **Sub-task 1.1.5:** Declare package-level `var defaultProjectBreakpoints` slice with the four standard presets:
  - `{"mobile", 375, 812}`
  - `{"tablet", 768, 1024}`
  - `{"desktop", 1280, 720}`
  - `{"wide", 1920, 1080}`
- **Sub-task 1.1.6:** Implement `loadProjectConfig() (*ProjectConfig, error)`:
  - Call `os.ReadFile(projectConfigFile)`.
  - Return the raw error if the read fails (callers can use `os.IsNotExist` to detect "not initialized").
  - Unmarshal bytes into `ProjectConfig` with `json.Unmarshal`; return error on failure.
  - Return pointer to populated struct.
- **Sub-task 1.1.7:** Implement `saveProjectConfig(cfg *ProjectConfig) error`:
  - Marshal with `json.MarshalIndent(cfg, "", "  ")`.
  - Write to `projectConfigFile` with `os.WriteFile` using mode `0644`.
  - Return any error.

---

## Phase 2: Command Handlers

**Goal:** Implement the four command functions in a new `cmd/project.go` file.

### Task 2.1 — Create `cmd/project.go` (file scaffold)

- **Sub-task 2.1.1:** Declare package `cmd`; import `os`, `strings`, `github.com/callmeradical/kaleidoscope/output`.

### Task 2.2 — Implement `RunInit(args []string)`

- **Sub-task 2.2.1:** Extract `--name` via `getFlagValue(args, "--name")`.
- **Sub-task 2.2.2:** Extract `--base-url` via `getFlagValue(args, "--base-url")`.
- **Sub-task 2.2.3:** Extract `--paths` via `getFlagValue(args, "--paths")`.
- **Sub-task 2.2.4:** Validate `--name` is non-empty; if missing call `output.Fail("init", "missing --name", "")` and `os.Exit(2)`.
- **Sub-task 2.2.5:** Validate `--base-url` is non-empty; if missing call `output.Fail("init", "missing --base-url", "")` and `os.Exit(2)`.
- **Sub-task 2.2.6:** Check if `.ks-project.json` exists via `os.Stat(projectConfigFile)`; if `err == nil` (file exists) call `output.Fail("init", "project already initialized", "")` and `os.Exit(2)`.
- **Sub-task 2.2.7:** Parse paths: if `--paths` is non-empty split on `,`; otherwise default to `[]string{"/"}`.
- **Sub-task 2.2.8:** Construct `ProjectConfig` with parsed `Name`, `BaseURL`, `Paths`, and `Breakpoints: defaultProjectBreakpoints`.
- **Sub-task 2.2.9:** Call `saveProjectConfig(&cfg)`; on error call `output.Fail` and `os.Exit(2)`.
- **Sub-task 2.2.10:** Call `output.Success("init", cfg)` (pass entire struct as result map).

### Task 2.3 — Implement `RunProjectAdd(args []string)`

- **Sub-task 2.3.1:** Get first positional arg via `getArg(args)`; if empty call `output.Fail("project-add", "missing path argument", "")` and `os.Exit(2)`.
- **Sub-task 2.3.2:** Call `loadProjectConfig()`; if error call `output.Fail("project-add", "project not initialized", "run ks init first")` and `os.Exit(2)`.
- **Sub-task 2.3.3:** Iterate `cfg.Paths` to check for duplicate; if found call `output.Fail("project-add", "path already exists", "")` and `os.Exit(2)`.
- **Sub-task 2.3.4:** Append `path` to `cfg.Paths`.
- **Sub-task 2.3.5:** Call `saveProjectConfig(cfg)`; on error call `output.Fail` and `os.Exit(2)`.
- **Sub-task 2.3.6:** Call `output.Success("project-add", map[string]any{"path": path, "paths": cfg.Paths})`.

### Task 2.4 — Implement `RunProjectRemove(args []string)`

- **Sub-task 2.4.1:** Get first positional arg via `getArg(args)`; if empty call `output.Fail("project-remove", "missing path argument", "")` and `os.Exit(2)`.
- **Sub-task 2.4.2:** Call `loadProjectConfig()`; if error call `output.Fail("project-remove", "project not initialized", "run ks init first")` and `os.Exit(2)`.
- **Sub-task 2.4.3:** Search `cfg.Paths` for the target path; if not found call `output.Fail("project-remove", "path not found", "")` and `os.Exit(2)`.
- **Sub-task 2.4.4:** Rebuild `cfg.Paths` slice without the removed element (preserve order using append over two sub-slices).
- **Sub-task 2.4.5:** Call `saveProjectConfig(cfg)`; on error call `output.Fail` and `os.Exit(2)`.
- **Sub-task 2.4.6:** Call `output.Success("project-remove", map[string]any{"path": path, "paths": cfg.Paths})`.

### Task 2.5 — Implement `RunProjectShow(args []string)`

- **Sub-task 2.5.1:** Call `loadProjectConfig()`; if error call `output.Fail("project-show", "project not initialized", "run ks init first")` and `os.Exit(2)`.
- **Sub-task 2.5.2:** Call `output.Success("project-show", cfg)` passing the full struct.

---

## Phase 3: Extend Existing Files

**Goal:** Wire the new commands into the CLI routing, flag parsing, and usage system.

### Task 3.1 — Modify `cmd/util.go`: extend `getNonFlagArgs` skip list

- **Sub-task 3.1.1:** In the `getNonFlagArgs` function, add `--name`, `--base-url`, and `--paths` to the flag-value-skip condition alongside the existing flags so their values are not mistakenly returned as positional args:
  ```go
  a == "--name" || a == "--base-url" || a == "--paths" ||
  ```

### Task 3.2 — Modify `cmd/usage.go`: add four new `CommandUsage` entries

- **Sub-task 3.2.1:** Add entry for `"init"` documenting synopsis `ks init --name <n> --base-url <u> --paths <p>`, required flags, default breakpoints, output shape, and error cases.
- **Sub-task 3.2.2:** Add entry for `"project-add"` documenting synopsis `ks project-add <path>`, argument, output shape, and error cases.
- **Sub-task 3.2.3:** Add entry for `"project-remove"` documenting synopsis `ks project-remove <path>`, argument, output shape, and error cases.
- **Sub-task 3.2.4:** Add entry for `"project-show"` documenting synopsis `ks project-show`, output shape, and error cases.

### Task 3.3 — Modify `main.go`: add four switch cases and update usage string

- **Sub-task 3.3.1:** Add four `case` branches to the `switch command` statement:
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
- **Sub-task 3.3.2:** Add a `Project:` section to the `usage` var string documenting all four commands, placed after the existing `Design System Catalog:` section and before `Skills:`:
  ```
  Project:
    init --name <n> --base-url <u> --paths <p>  Initialize project config
    project-add <path>                           Add a path to the project
    project-remove <path>                        Remove a path from the project
    project-show                                 Show project config as JSON
  ```

---

## Phase 4: Verification

**Goal:** Confirm the implementation compiles and all tests pass.

### Task 4.1 — Build verification

- **Sub-task 4.1.1:** Run `go build ./...` to confirm no compilation errors.

### Task 4.2 — Test suite

- **Sub-task 4.2.1:** Run `go test ./...` to confirm all existing tests continue to pass (quality gate from PRD).

---

## File Change Summary

| File | Action | Description |
|---|---|---|
| `cmd/project_config.go` | Create | `ProjectConfig`, `ProjectBreakpoint` structs; `loadProjectConfig`, `saveProjectConfig` helpers |
| `cmd/project.go` | Create | `RunInit`, `RunProjectAdd`, `RunProjectRemove`, `RunProjectShow` |
| `cmd/util.go` | Modify | Add `--name`, `--base-url`, `--paths` to `getNonFlagArgs` skip list |
| `cmd/usage.go` | Modify | Add `CommandUsage` entries for all four new commands |
| `main.go` | Modify | Add four `case` branches; add `Project:` section to usage string |

---

## Error Reference

| Command | Condition | Error message |
|---|---|---|
| `init` | `.ks-project.json` exists | `"project already initialized"` |
| `init` | `--name` missing | `"missing --name"` |
| `init` | `--base-url` missing | `"missing --base-url"` |
| `project-add` | no path argument | `"missing path argument"` |
| `project-add` | path already in list | `"path already exists"` |
| `project-add` | config not found | `"project not initialized"` |
| `project-remove` | no path argument | `"missing path argument"` |
| `project-remove` | path not in list | `"path not found"` |
| `project-remove` | config not found | `"project not initialized"` |
| `project-show` | config not found | `"project not initialized"` |
