# Implementation Plan: Project Config Commands (US-001)

## Overview

Introduce `.ks-project.json` and four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) to define a kaleidoscope project as a named set of URL paths and breakpoints to track.

---

## Phase 1: `project` Package (Data Layer)

**Goal:** Create a new `project` package with the `Config` struct, constants, and pure I/O helpers. No CLI or browser dependencies.

### Task 1.1 — Create `project/project.go`

- **Sub-task 1.1.1:** Create the file `project/project.go` with `package project`.
- **Sub-task 1.1.2:** Define the `Breakpoint` struct with JSON tags (`name`, `width`, `height`).
- **Sub-task 1.1.3:** Define the `Config` struct with JSON tags (`name`, `baseURL`, `paths`, `breakpoints`).
- **Sub-task 1.1.4:** Define the `ConfigFile` constant (`".ks-project.json"`).
- **Sub-task 1.1.5:** Define the `DefaultBreakpoints` var with all four standard presets: mobile (375×812), tablet (768×1024), desktop (1280×720), wide (1920×1080).

### Task 1.2 — Implement `Load(dir string) (*Config, error)`

- **Sub-task 1.2.1:** Construct the full config path via `filepath.Join(dir, ConfigFile)`.
- **Sub-task 1.2.2:** Read the file with `os.ReadFile`; if the error wraps `fs.ErrNotExist`, return `fs.ErrNotExist` directly so callers can use `errors.Is`.
- **Sub-task 1.2.3:** Unmarshal JSON bytes into a `Config` struct and return it.

### Task 1.3 — Implement `Save(dir string, cfg *Config) error`

- **Sub-task 1.3.1:** Marshal `cfg` to indented JSON (`json.MarshalIndent`).
- **Sub-task 1.3.2:** Write marshaled bytes to a temp file `<dir>/.ks-project.json.tmp` with mode `0644`.
- **Sub-task 1.3.3:** Use `os.Rename` to atomically move the temp file to `<dir>/.ks-project.json`.

### Task 1.4 — Implement `Validate(cfg *Config) error`

- **Sub-task 1.4.1:** Return an error if `cfg.Name` is empty (`"name is required"`).
- **Sub-task 1.4.2:** Return an error if `cfg.BaseURL` is empty (`"baseURL is required"`).
- **Sub-task 1.4.3:** Return an error if `cfg.Paths` is empty (`"paths must not be empty"`).

---

## Phase 2: Command Implementations

**Goal:** Add the four new command files in `cmd/`, each following the existing `output.Success`/`output.Fail` JSON convention.

### Task 2.1 — Create `cmd/init.go` (`RunInit`)

- **Sub-task 2.1.1:** Add `func RunInit(args []string)` in `cmd/init.go`.
- **Sub-task 2.1.2:** Parse `--name`, `--base-url`, and `--paths` using the existing `getFlagValue` helper.
- **Sub-task 2.1.3:** Validate all three flags are non-empty; call `output.Fail` + `os.Exit(2)` with an appropriate error message for each missing flag.
- **Sub-task 2.1.4:** Obtain `dir` via `os.Getwd()`; call `project.Load(dir)` — if successful (no error), call `output.Fail` with `".ks-project.json already exists"` and hint `"Delete .ks-project.json first, or use ks project-add to modify it."` then exit 2.
- **Sub-task 2.1.5:** Split `--paths` value on `,`, trim whitespace from each element.
- **Sub-task 2.1.6:** Validate each trimmed path starts with `/`; call `output.Fail` with `"path must start with /: <path>"` if not.
- **Sub-task 2.1.7:** Build a `project.Config` with parsed name, baseURL, paths, and `project.DefaultBreakpoints`.
- **Sub-task 2.1.8:** Call `project.Save(dir, &cfg)`; handle I/O errors with `output.Fail` + exit 2.
- **Sub-task 2.1.9:** Call `output.Success("init", ...)` with a result map containing `name`, `baseURL`, `paths`, `breakpoints`, and `configPath` (the absolute path to `.ks-project.json`).

### Task 2.2 — Create `cmd/project_add.go` (`RunProjectAdd`)

- **Sub-task 2.2.1:** Add `func RunProjectAdd(args []string)` in `cmd/project_add.go`.
- **Sub-task 2.2.2:** Get the path argument via `getArg(args)`; fail with `"path argument is required"` if empty.
- **Sub-task 2.2.3:** Validate the path starts with `/`; fail with `"path must start with /: <path>"` if not.
- **Sub-task 2.2.4:** Obtain `dir` via `os.Getwd()`; call `project.Load(dir)` — if `errors.Is(err, fs.ErrNotExist)`, fail with `".ks-project.json not found"` and hint `"Run: ks init"`.
- **Sub-task 2.2.5:** Scan `cfg.Paths` for the path; if it already exists, fail with `"path already exists: <path>"` + exit 2.
- **Sub-task 2.2.6:** Append the path to `cfg.Paths`.
- **Sub-task 2.2.7:** Call `project.Save(dir, cfg)`; handle I/O errors.
- **Sub-task 2.2.8:** Call `output.Success("project-add", ...)` with result map containing `added`, `paths`, and `configPath`.

### Task 2.3 — Create `cmd/project_remove.go` (`RunProjectRemove`)

- **Sub-task 2.3.1:** Add `func RunProjectRemove(args []string)` in `cmd/project_remove.go`.
- **Sub-task 2.3.2:** Get the path argument via `getArg(args)`; fail with `"path argument is required"` if empty.
- **Sub-task 2.3.3:** Obtain `dir` via `os.Getwd()`; call `project.Load(dir)` — if `errors.Is(err, fs.ErrNotExist)`, fail with `".ks-project.json not found"` and hint `"Run: ks init"`.
- **Sub-task 2.3.4:** Scan `cfg.Paths` for the path; if absent, fail with `"path not found: <path>"` + exit 2.
- **Sub-task 2.3.5:** Filter the path out of `cfg.Paths` (build a new slice excluding the target).
- **Sub-task 2.3.6:** Call `project.Save(dir, cfg)`; handle I/O errors.
- **Sub-task 2.3.7:** Call `output.Success("project-remove", ...)` with result map containing `removed`, `paths`, and `configPath`.

### Task 2.4 — Create `cmd/project_show.go` (`RunProjectShow`)

- **Sub-task 2.4.1:** Add `func RunProjectShow(args []string)` in `cmd/project_show.go`.
- **Sub-task 2.4.2:** Obtain `dir` via `os.Getwd()`; call `project.Load(dir)` — if `errors.Is(err, fs.ErrNotExist)`, fail with `".ks-project.json not found"` and hint `"Run: ks init"`.
- **Sub-task 2.4.3:** Call `output.Success("project-show", cfg)` — the full `Config` struct is the result.

---

## Phase 3: Modifications to Existing Files

**Goal:** Wire up the new commands into the CLI router, flag parser, and usage system.

### Task 3.1 — Edit `cmd/util.go`

- **Sub-task 3.1.1:** In `getNonFlagArgs`, add `--name`, `--base-url`, and `--paths` to the `skip = true` branch so their values are not treated as positional arguments.

### Task 3.2 — Edit `cmd/usage.go`

- **Sub-task 3.2.1:** Add a `"init"` entry to `CommandUsage` describing flags (`--name`, `--base-url`, `--paths`), output shape, notes (creates file, fails if exists, default breakpoints).
- **Sub-task 3.2.2:** Add a `"project-add"` entry describing the positional path argument, output shape, and error cases.
- **Sub-task 3.2.3:** Add a `"project-remove"` entry describing the positional path argument, output shape, and error cases.
- **Sub-task 3.2.4:** Add a `"project-show"` entry describing the output shape (full config JSON).

### Task 3.3 — Edit `main.go`

- **Sub-task 3.3.1:** Add four `case` entries to the command `switch`:
  - `"init"` → `cmd.RunInit(cmdArgs)`
  - `"project-add"` → `cmd.RunProjectAdd(cmdArgs)`
  - `"project-remove"` → `cmd.RunProjectRemove(cmdArgs)`
  - `"project-show"` → `cmd.RunProjectShow(cmdArgs)`
- **Sub-task 3.3.2:** Update the `usage` string in `main.go` to add a "Project" section listing all four commands with brief one-line descriptions.

---

## Phase 4: Tests

**Goal:** Ensure all acceptance criteria are covered by automated tests that pass `go test ./...`.

### Task 4.1 — Create `project/project_test.go`

- **Sub-task 4.1.1:** Test `Load` returns `fs.ErrNotExist` (via `errors.Is`) when the config file does not exist.
- **Sub-task 4.1.2:** Test `Save` + `Load` round-trip: save a `Config`, load it back, assert equality.
- **Sub-task 4.1.3:** Test `Validate` rejects an empty `Name`.
- **Sub-task 4.1.4:** Test `Validate` rejects an empty `BaseURL`.
- **Sub-task 4.1.5:** Test `Validate` rejects an empty `Paths` slice.
- **Sub-task 4.1.6:** Test `Validate` accepts a fully populated valid `Config`.

### Task 4.2 — Create `cmd/init_test.go`

- **Sub-task 4.2.1:** Test `RunInit` succeeds when config does not yet exist; assert the output JSON has `"ok": true` and the correct fields in `result`.
- **Sub-task 4.2.2:** Test `RunInit` fails (exit 2, `"ok": false`) when `.ks-project.json` already exists.
- **Sub-task 4.2.3:** Test `RunInit` fails when `--name` is missing.
- **Sub-task 4.2.4:** Test `RunInit` fails when `--base-url` is missing.
- **Sub-task 4.2.5:** Test `RunInit` fails when `--paths` is missing.
- **Sub-task 4.2.6:** Test `RunInit` fails when a path does not start with `/`.

### Task 4.3 — Create `cmd/project_add_test.go`

- **Sub-task 4.3.1:** Test `RunProjectAdd` appends a new path and outputs `"ok": true` with updated `paths`.
- **Sub-task 4.3.2:** Test `RunProjectAdd` fails with `"path already exists"` when adding a duplicate path.
- **Sub-task 4.3.3:** Test `RunProjectAdd` fails with `".ks-project.json not found"` when no config file exists.
- **Sub-task 4.3.4:** Test `RunProjectAdd` fails when no path argument is provided.

### Task 4.4 — Create `cmd/project_remove_test.go`

- **Sub-task 4.4.1:** Test `RunProjectRemove` removes an existing path and outputs `"ok": true` with updated `paths`.
- **Sub-task 4.4.2:** Test `RunProjectRemove` fails with `"path not found"` when the path is not in the config.
- **Sub-task 4.4.3:** Test `RunProjectRemove` fails with `".ks-project.json not found"` when no config file exists.
- **Sub-task 4.4.4:** Test `RunProjectRemove` fails when no path argument is provided.

### Task 4.5 — Create `cmd/project_show_test.go`

- **Sub-task 4.5.1:** Test `RunProjectShow` outputs `"ok": true` with the full config as `result`.
- **Sub-task 4.5.2:** Test `RunProjectShow` fails with `".ks-project.json not found"` when no config file exists.

---

## File Summary

| File | Action | Phase |
|------|--------|-------|
| `project/project.go` | **Create** | 1 |
| `project/project_test.go` | **Create** | 4 |
| `cmd/init.go` | **Create** | 2 |
| `cmd/project_add.go` | **Create** | 2 |
| `cmd/project_remove.go` | **Create** | 2 |
| `cmd/project_show.go` | **Create** | 2 |
| `cmd/init_test.go` | **Create** | 4 |
| `cmd/project_add_test.go` | **Create** | 4 |
| `cmd/project_remove_test.go` | **Create** | 4 |
| `cmd/project_show_test.go` | **Create** | 4 |
| `cmd/util.go` | **Edit** | 3 |
| `cmd/usage.go` | **Edit** | 3 |
| `main.go` | **Edit** | 3 |

---

## Dependency Order

```
Phase 1 (project package)
    ↓
Phase 2 (cmd files — import project package)
    ↓
Phase 3 (wire into main.go, util.go, usage.go)
    ↓
Phase 4 (tests — can be written alongside phases 1–3 but verified last)
```

Phases 2 and 3 can be developed in parallel once Phase 1 is complete. Individual command files in Phase 2 are independent of each other.
