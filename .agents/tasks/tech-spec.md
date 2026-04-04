# Tech Spec: Project Config Commands (US-001)

## Overview

Introduces `.ks-project.json` and four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) that define a kaleidoscope project as a named set of URL paths and breakpoints to track. This config is the foundation for subsequent snapshot and diff features.

---

## Architecture Overview

```
/workspace/
  project/
    project.go          # Data model + Load/Save/Validate helpers (new package)
  cmd/
    init.go             # RunInit
    project_add.go      # RunProjectAdd
    project_remove.go   # RunProjectRemove
    project_show.go     # RunProjectShow
    util.go             # Add new flag names to getNonFlagArgs skip list
    usage.go            # Add usage strings for 4 new commands
  main.go               # Add 4 new cases to switch; update usage string
```

These commands are **browser-independent** — no `browser.WithPage` calls, no Chrome dependency.

---

## Data Model

### `.ks-project.json`

Committed to the repository root. Created by `ks init`, mutated by `ks project-add` / `ks project-remove`.

```json
{
  "name": "my-app",
  "baseURL": "https://example.com",
  "paths": ["/", "/dashboard", "/settings"],
  "breakpoints": [
    { "name": "mobile",  "width": 375,  "height": 812  },
    { "name": "tablet",  "width": 768,  "height": 1024 },
    { "name": "desktop", "width": 1280, "height": 720  },
    { "name": "wide",    "width": 1920, "height": 1080 }
  ]
}
```

**Field rules:**
- `name` — required, non-empty string.
- `baseURL` — required, non-empty string (URL format not validated at this layer; consumers validate).
- `paths` — required, non-empty array. Each path must start with `/`. Duplicates disallowed.
- `breakpoints` — always written as all four standard presets; not user-configurable at this story's scope.

### Go struct (`project` package)

```go
package project

type Breakpoint struct {
    Name   string `json:"name"`
    Width  int    `json:"width"`
    Height int    `json:"height"`
}

type Config struct {
    Name        string       `json:"name"`
    BaseURL     string       `json:"baseURL"`
    Paths       []string     `json:"paths"`
    Breakpoints []Breakpoint `json:"breakpoints"`
}
```

---

## Package: `project`

**File:** `/workspace/project/project.go`

Provides pure data-manipulation functions with no CLI or browser dependencies.

### Constants & Defaults

```go
const ConfigFile = ".ks-project.json"

var DefaultBreakpoints = []Breakpoint{
    {"mobile",  375,  812},
    {"tablet",  768,  1024},
    {"desktop", 1280, 720},
    {"wide",    1920, 1080},
}
```

### Functions

#### `Load(dir string) (*Config, error)`
- Reads `<dir>/.ks-project.json`.
- Returns `fs.ErrNotExist` (unwrapped) if the file is absent so callers can distinguish "not found" from parse errors.
- Unmarshals JSON into `Config`.

#### `Save(dir string, cfg *Config) error`
- Marshals `cfg` to indented JSON.
- Writes atomically: writes to `<dir>/.ks-project.json.tmp`, then `os.Rename` to `<dir>/.ks-project.json`.
- File mode `0644`.

#### `Validate(cfg *Config) error`
- Returns an error if `Name` is empty.
- Returns an error if `BaseURL` is empty.
- Returns an error if `Paths` is empty.

---

## Command Implementations

All commands use `os.Getwd()` to determine `dir` and delegate file I/O to the `project` package. All output follows the existing `output.Success` / `output.Fail` JSON convention.

---

### `ks init`

**File:** `cmd/init.go` — `func RunInit(args []string)`

**CLI:** `ks init --name <name> --base-url <url> --paths /,/dashboard`

**Flag parsing:**
| Flag | Type | Required |
|------|------|----------|
| `--name` | string | yes |
| `--base-url` | string | yes |
| `--paths` | comma-separated string | yes |

**Logic:**
1. Parse `--name`, `--base-url`, `--paths` via `getFlagValue`.
2. Validate all three are non-empty; call `output.Fail` + `os.Exit(2)` if not.
3. Check if `.ks-project.json` already exists in cwd: call `project.Load(dir)`. If no error (file exists), call `output.Fail` with hint `"Delete .ks-project.json first, or use ks project-add to modify it."` and exit 2.
4. Split `--paths` on `,`, trim whitespace from each element.
5. Validate each path starts with `/`; fail if not.
6. Build `project.Config` with `DefaultBreakpoints`.
7. Call `project.Save(dir, &cfg)`.
8. Call `output.Success("init", ...)` with the full config as result.

**Success output shape:**
```json
{
  "ok": true,
  "command": "init",
  "result": {
    "name": "my-app",
    "baseURL": "https://example.com",
    "paths": ["/", "/dashboard"],
    "breakpoints": [...],
    "configPath": "/abs/path/to/.ks-project.json"
  }
}
```

---

### `ks project-add`

**File:** `cmd/project_add.go` — `func RunProjectAdd(args []string)`

**CLI:** `ks project-add /settings`

**Logic:**
1. `path := getArg(args)`. Fail if empty.
2. Validate `path` starts with `/`; fail if not.
3. `project.Load(dir)` — fail with hint `"Run: ks init"` if not found.
4. Check for duplicate: if `path` already in `cfg.Paths`, call `output.Fail` with error `"path already exists: /settings"` and exit 2.
5. Append `path` to `cfg.Paths`.
6. `project.Save(dir, cfg)`.
7. `output.Success("project-add", map with updated paths list and configPath)`.

**Success output shape:**
```json
{
  "ok": true,
  "command": "project-add",
  "result": {
    "added": "/settings",
    "paths": ["/", "/dashboard", "/settings"],
    "configPath": "/abs/path/to/.ks-project.json"
  }
}
```

---

### `ks project-remove`

**File:** `cmd/project_remove.go` — `func RunProjectRemove(args []string)`

**CLI:** `ks project-remove /settings`

**Logic:**
1. `path := getArg(args)`. Fail if empty.
2. `project.Load(dir)` — fail with hint `"Run: ks init"` if not found.
3. Scan `cfg.Paths` for `path`. If absent, call `output.Fail` with error `"path not found: /settings"` and exit 2.
4. Remove the path (filter slice).
5. `project.Save(dir, cfg)`.
6. `output.Success("project-remove", ...)`.

**Success output shape:**
```json
{
  "ok": true,
  "command": "project-remove",
  "result": {
    "removed": "/settings",
    "paths": ["/", "/dashboard"],
    "configPath": "/abs/path/to/.ks-project.json"
  }
}
```

---

### `ks project-show`

**File:** `cmd/project_show.go` — `func RunProjectShow(args []string)`

**CLI:** `ks project-show`

**Logic:**
1. `project.Load(dir)` — fail with hint `"Run: ks init"` if not found.
2. `output.Success("project-show", cfg)` — the full `Config` struct is the result.

**Success output shape:**
```json
{
  "ok": true,
  "command": "project-show",
  "result": {
    "name": "my-app",
    "baseURL": "https://example.com",
    "paths": ["/", "/dashboard"],
    "breakpoints": [
      { "name": "mobile",  "width": 375,  "height": 812  },
      { "name": "tablet",  "width": 768,  "height": 1024 },
      { "name": "desktop", "width": 1280, "height": 720  },
      { "name": "wide",    "width": 1920, "height": 1080 }
    ]
  }
}
```

---

## Changes to Existing Files

### `cmd/util.go` — `getNonFlagArgs`

Add `--name`, `--base-url`, and `--paths` to the `skip = true` branch so their values are not mistaken for positional arguments:

```go
if a == "--selector" || a == "--output" || ... ||
    a == "--name" || a == "--base-url" || a == "--paths" {
    skip = true
}
```

### `cmd/usage.go` — `CommandUsage`

Add four entries: `"init"`, `"project-add"`, `"project-remove"`, `"project-show"`.

Example for `"init"`:
```
ks init --name <name> --base-url <url> --paths <path,...>

Initialize a kaleidoscope project config in the current directory.

Options:
  --name <name>       Project name (required)
  --base-url <url>    Base URL for all paths (required)
  --paths <paths>     Comma-separated list of paths to track, e.g. /,/dashboard (required)

Output:
  { "ok": true, "result": { "name": "...", "baseURL": "...", "paths": [...], "breakpoints": [...], "configPath": "..." } }

Notes:
  Creates .ks-project.json in the current directory.
  Fails if .ks-project.json already exists.
  Breakpoints default to: mobile (375x812), tablet (768x1024), desktop (1280x720), wide (1920x1080).
```

### `main.go` — command switch

Add four cases:
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

Update the `usage` string to add a "Project" section listing all four commands.

---

## File Layout Summary

| File | Status | Purpose |
|------|--------|---------|
| `project/project.go` | **New** | `Config` struct, `DefaultBreakpoints`, `Load`, `Save`, `Validate` |
| `cmd/init.go` | **New** | `RunInit` |
| `cmd/project_add.go` | **New** | `RunProjectAdd` |
| `cmd/project_remove.go` | **New** | `RunProjectRemove` |
| `cmd/project_show.go` | **New** | `RunProjectShow` |
| `cmd/util.go` | **Edit** | Add `--name`, `--base-url`, `--paths` to skip list |
| `cmd/usage.go` | **Edit** | Add 4 usage entries |
| `main.go` | **Edit** | Add 4 switch cases + update help string |

---

## Security Considerations

- **Path traversal:** `project.Load` and `project.Save` construct the config path as `filepath.Join(dir, ConfigFile)` where `ConfigFile` is the constant `".ks-project.json"`. The `dir` value comes from `os.Getwd()`, not user input, so there is no path traversal risk.
- **No shell execution:** All operations are pure Go file I/O; no `exec.Command` calls.
- **Atomic writes:** `Save` uses a temp file + `os.Rename` to prevent partial writes corrupting the config.
- **No secrets:** `.ks-project.json` contains only URL paths and viewport dimensions; no credentials or tokens.

---

## Error Handling Matrix

| Scenario | Error message | Exit code |
|----------|--------------|-----------|
| `init` missing `--name` | `"--name is required"` | 2 |
| `init` missing `--base-url` | `"--base-url is required"` | 2 |
| `init` missing `--paths` | `"--paths is required"` | 2 |
| `init` with path not starting `/` | `"path must start with /: foo"` | 2 |
| `init` when `.ks-project.json` exists | `".ks-project.json already exists"` | 2 |
| `project-add` with no arg | `"path argument is required"` | 2 |
| `project-add` duplicate path | `"path already exists: /foo"` | 2 |
| `project-add/remove/show` no config file | `".ks-project.json not found"` + hint `"Run: ks init"` | 2 |
| `project-remove` path not found | `"path not found: /foo"` | 2 |
| Any file I/O failure | underlying OS error message | 2 |

---

## Quality Gate

`go test ./...` must pass. Tests should cover:

- `project.Load` returns `fs.ErrNotExist` when file is absent.
- `project.Save` round-trips: load after save returns identical struct.
- `project.Validate` rejects empty `Name`, `BaseURL`, empty `Paths`.
- `RunInit` fails when config already exists.
- `RunProjectAdd` fails on duplicate; succeeds and appends on new path.
- `RunProjectRemove` fails on missing path; succeeds and removes on existing path.
- Output JSON has `ok: true` on success and `ok: false` on error for all four commands.
