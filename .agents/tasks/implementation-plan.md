# Implementation Plan: Project Config Commands (US-001)

## Overview

Introduces `.ks-project.json` and four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) that define a kaleidoscope project as a named set of URL paths and breakpoints.

---

## Phase 1: `project` Package — Data Model & File I/O

### Task 1.1: Create `project/project.go`

**File**: `project/project.go`

- **Sub-task 1.1.1**: Define `Breakpoint` struct with JSON tags (`name`, `width`, `height`) — mirrors `cmd/breakpoints.go` local type.
- **Sub-task 1.1.2**: Define `Config` struct with JSON tags (`name`, `baseURL`, `paths`, `breakpoints`).
- **Sub-task 1.1.3**: Declare `const Filename = ".ks-project.json"`.
- **Sub-task 1.1.4**: Declare `var DefaultBreakpoints` with the four standard presets: mobile (375×812), tablet (768×1024), desktop (1280×720), wide (1920×1080).
- **Sub-task 1.1.5**: Implement `Exists() bool` — calls `os.Stat(Filename)` and returns `true` only if no error.
- **Sub-task 1.1.6**: Implement `Read() (*Config, error)` — `os.ReadFile(Filename)` → `json.Unmarshal` into `*Config`; wrap errors to include filename.
- **Sub-task 1.1.7**: Implement `Write(cfg *Config) error` — `json.MarshalIndent(cfg, "", "  ")` → `os.WriteFile(Filename, data, 0644)`.

### Task 1.2: Create `project/project_test.go`

**File**: `project/project_test.go`

- **Sub-task 1.2.1**: Write table-driven tests for `Exists()`:
  - Returns `false` when file does not exist.
  - Returns `true` after `Write()` creates the file.
- **Sub-task 1.2.2**: Write table-driven tests for `Write()` + `Read()` round-trip:
  - Write a `Config` and re-read it; assert all fields match.
  - Verify JSON indentation (2 spaces) and field names.
- **Sub-task 1.2.3**: Write test for `Read()` on missing file — expects a wrapped error.
- **Sub-task 1.2.4**: Write test for `Read()` on malformed JSON — expects an unmarshal error.
- **Sub-task 1.2.5**: Use `os.MkdirTemp` + `os.Chdir` (restore on `t.Cleanup`) to isolate filesystem state per test.

---

## Phase 2: `cmd/init.go` — `ks init` Command Handler

### Task 2.1: Create `cmd/init.go`

**File**: `cmd/init.go`

- **Sub-task 2.1.1**: Implement `RunInit(args []string)`:
  1. Call `getFlagValue(args, "--name")` — if empty, call `output.Fail("init", errors.New("--name is required"), "")` + `os.Exit(2)`.
  2. Call `getFlagValue(args, "--base-url")` — if empty, call `output.Fail("init", errors.New("--base-url is required"), "")` + `os.Exit(2)`.
  3. Call `getFlagValue(args, "--paths")` — if empty, call `output.Fail("init", errors.New("--paths is required"), "")` + `os.Exit(2)`.
  4. Split `--paths` value on `,`; trim whitespace from each element using `strings.TrimSpace`.
  5. Check `project.Exists()` — if true, call `output.Fail("init", errors.New(".ks-project.json already exists"), "")` + `os.Exit(2)`.
  6. Build `project.Config{Name: name, BaseURL: baseURL, Paths: paths, Breakpoints: project.DefaultBreakpoints}`.
  7. Call `project.Write(cfg)` — on error, call `output.Fail("init", err, "")` + `os.Exit(2)`.
  8. Call `output.Success("init", map[string]any{"path": project.Filename, "name": cfg.Name, "baseURL": cfg.BaseURL, "paths": cfg.Paths, "breakpoints": cfg.Breakpoints})`.

### Task 2.2: Create `cmd/init_test.go`

**File**: `cmd/init_test.go`

- **Sub-task 2.2.1**: Test happy path — `RunInit` with `--name`, `--base-url`, `--paths` creates `.ks-project.json` and outputs valid JSON with `"ok": true`.
- **Sub-task 2.2.2**: Test missing `--name` flag — expects `"ok": false` with appropriate error message.
- **Sub-task 2.2.3**: Test missing `--base-url` flag — expects `"ok": false`.
- **Sub-task 2.2.4**: Test missing `--paths` flag — expects `"ok": false`.
- **Sub-task 2.2.5**: Test already-exists guard — create `.ks-project.json` first, then call `RunInit` again; expect `"ok": false` with "already exists" error.
- **Sub-task 2.2.6**: Test that `breakpoints` in output defaults to the four standard presets when no `--breakpoints` flag is given.
- **Sub-task 2.2.7**: Test comma-separated `--paths` splits and trims correctly (e.g. `/, /dashboard` → `["/", "/dashboard"]`).
- **Sub-task 2.2.8**: Use `os.MkdirTemp` + `os.Chdir` with `t.Cleanup` to isolate each test's working directory.

---

## Phase 3: `cmd/project.go` — `ks project-add`, `ks project-remove`, `ks project-show` Handlers

### Task 3.1: Create `cmd/project.go`

**File**: `cmd/project.go`

- **Sub-task 3.1.1**: Implement `RunProjectAdd(args []string)`:
  1. Extract first non-flag arg via `getArg(args)` — if empty, `output.Fail("project-add", errors.New("path argument is required"), "Usage: ks project-add <path>")` + `os.Exit(2)`.
  2. `project.Read()` — on error, `output.Fail("project-add", err, "Run: ks init --name <name> --base-url <url> --paths <paths>")` + `os.Exit(2)`.
  3. Iterate `cfg.Paths`; if path already present, `output.Fail("project-add", errors.New("path already exists: "+path), "")` + `os.Exit(2)`.
  4. Append path to `cfg.Paths`.
  5. `project.Write(cfg)` — on error, `output.Fail` + `os.Exit(2)`.
  6. `output.Success("project-add", map[string]any{"path": path, "paths": cfg.Paths})`.

- **Sub-task 3.1.2**: Implement `RunProjectRemove(args []string)`:
  1. Extract first non-flag arg via `getArg(args)` — if empty, `output.Fail("project-remove", errors.New("path argument is required"), "Usage: ks project-remove <path>")` + `os.Exit(2)`.
  2. `project.Read()` — on error, `output.Fail` + `os.Exit(2)`.
  3. Search `cfg.Paths` for the path; if not found, `output.Fail("project-remove", errors.New("path not found: "+path), "")` + `os.Exit(2)`.
  4. Filter `cfg.Paths` to remove the matched path (build new slice without it).
  5. `project.Write(cfg)` — on error, `output.Fail` + `os.Exit(2)`.
  6. `output.Success("project-remove", map[string]any{"removed": path, "paths": cfg.Paths})`.

- **Sub-task 3.1.3**: Implement `RunProjectShow(args []string)`:
  1. `project.Read()` — on error, `output.Fail("project-show", err, "Run: ks init --name <name> --base-url <url> --paths <paths>")` + `os.Exit(2)`.
  2. `output.Success("project-show", cfg)` — the full `*Config` struct serialises as the result payload.

### Task 3.2: Create `cmd/project_test.go`

**File**: `cmd/project_test.go`

- **Sub-task 3.2.1**: `RunProjectAdd` happy path — seed `.ks-project.json`, call with a new path, assert JSON output has `"ok": true` and updated `paths` array.
- **Sub-task 3.2.2**: `RunProjectAdd` duplicate path — add same path twice; second call must produce `"ok": false` with "already exists" error.
- **Sub-task 3.2.3**: `RunProjectAdd` missing argument — call with no args; expect `"ok": false`.
- **Sub-task 3.2.4**: `RunProjectAdd` missing project file — call with no `.ks-project.json`; expect `"ok": false` with hint to run `ks init`.
- **Sub-task 3.2.5**: `RunProjectRemove` happy path — seed config with `/settings`, remove it, assert it is absent from `paths`.
- **Sub-task 3.2.6**: `RunProjectRemove` non-existent path — expect `"ok": false` with "path not found" error.
- **Sub-task 3.2.7**: `RunProjectRemove` missing argument — call with no args; expect `"ok": false`.
- **Sub-task 3.2.8**: `RunProjectRemove` missing project file — expect `"ok": false`.
- **Sub-task 3.2.9**: `RunProjectShow` happy path — seed config, call show, assert full config is in `result`.
- **Sub-task 3.2.10**: `RunProjectShow` missing project file — expect `"ok": false` with hint.
- **Sub-task 3.2.11**: Use `os.MkdirTemp` + `os.Chdir` with `t.Cleanup` for filesystem isolation in all tests.

---

## Phase 4: Edits to Existing Files

### Task 4.1: Update `cmd/util.go` — `getNonFlagArgs` Skip List

**File**: `cmd/util.go`

- **Sub-task 4.1.1**: Locate the flag-skip condition inside `getNonFlagArgs` (the `if a == "--selector" || ...` chain).
- **Sub-task 4.1.2**: Append three new flag names to the condition: `|| a == "--name" || a == "--base-url" || a == "--paths"`.

### Task 4.2: Update `cmd/usage.go` — `CommandUsage` Map

**File**: `cmd/usage.go`

- **Sub-task 4.2.1**: Add `"init"` entry with full usage doc (syntax, required options, output format, examples, notes about already-exists guard and default breakpoints).
- **Sub-task 4.2.2**: Add `"project-add"` entry (syntax, argument, output format, notes about duplicate guard and required project file).
- **Sub-task 4.2.3**: Add `"project-remove"` entry (syntax, argument, output format, notes about not-found guard and required project file).
- **Sub-task 4.2.4**: Add `"project-show"` entry (syntax, output format, note about required project file).

### Task 4.3: Update `main.go` — Switch Statement & Usage String

**File**: `main.go`

- **Sub-task 4.3.1**: Add four cases to the `switch command` block in the correct position (after existing design-system commands or in a new logical grouping):
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
- **Sub-task 4.3.2**: Add a `"Project Config:"` section to the top-level usage string:
  ```
  Project Config:
    init --name <n> --base-url <u> --paths <p>  Create .ks-project.json
    project-add <path>                           Add a path to the project
    project-remove <path>                        Remove a path from the project
    project-show                                 Show the project config as JSON
  ```

---

## Phase 5: Verification

### Task 5.1: Run Quality Gate

- **Sub-task 5.1.1**: Run `go build ./...` — confirm no compilation errors across all new and modified files.
- **Sub-task 5.1.2**: Run `go test ./...` — confirm all tests pass, including:
  - `project/project_test.go`
  - `cmd/init_test.go`
  - `cmd/project_test.go`
  - All pre-existing tests.
- **Sub-task 5.1.3**: If any test fails, diagnose root cause and fix — do not skip or suppress.

### Task 5.2: Acceptance Criteria Spot-Check (manual or scripted)

- **Sub-task 5.2.1**: Confirm `ks init --name my-app --base-url http://localhost:3000 --paths /,/dashboard` creates `.ks-project.json` with correct schema.
- **Sub-task 5.2.2**: Confirm a second `ks init` call returns `"ok": false` ("already exists").
- **Sub-task 5.2.3**: Confirm `ks project-add /settings` appends path and emits updated config.
- **Sub-task 5.2.4**: Confirm `ks project-add /settings` a second time returns `"ok": false` ("already exists").
- **Sub-task 5.2.5**: Confirm `ks project-remove /settings` removes path and emits updated config.
- **Sub-task 5.2.6**: Confirm `ks project-remove /nonexistent` returns `"ok": false` ("path not found").
- **Sub-task 5.2.7**: Confirm `ks project-show` returns full config as structured JSON via `output.Result`.
- **Sub-task 5.2.8**: Confirm breakpoints default to all four standard presets when no `--breakpoints` flag is supplied.

---

## File Change Summary

| File | Action |
|------|--------|
| `project/project.go` | **Create** — `Breakpoint`, `Config`, `Filename`, `DefaultBreakpoints`, `Exists`, `Read`, `Write` |
| `project/project_test.go` | **Create** — table-driven tests for all three functions |
| `cmd/init.go` | **Create** — `RunInit` handler |
| `cmd/init_test.go` | **Create** — happy path + all error conditions |
| `cmd/project.go` | **Create** — `RunProjectAdd`, `RunProjectRemove`, `RunProjectShow` |
| `cmd/project_test.go` | **Create** — happy path + all error conditions for all three handlers |
| `cmd/util.go` | **Edit** — add `--name`, `--base-url`, `--paths` to `getNonFlagArgs` skip list |
| `cmd/usage.go` | **Edit** — add four entries to `CommandUsage` map |
| `main.go` | **Edit** — add four switch cases + `"Project Config:"` section to usage string |

---

## Dependency Order

```
Phase 1 (project package)
    └── Phase 2 (cmd/init.go)         [depends on project.Exists, project.Write]
    └── Phase 3 (cmd/project.go)      [depends on project.Read, project.Write]
Phase 4 (existing file edits)         [independent of Phases 1-3 but needed for CLI wiring]
Phase 5 (verification)                [depends on all prior phases complete]
```

Phases 2, 3, and 4 can proceed in parallel once Phase 1 is complete.
