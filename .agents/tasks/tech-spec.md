# Tech Spec: Project Config Commands (US-001)

## Overview

Implements `.ks-project.json` and four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) that let users define a kaleidoscope project as a named set of URL paths and breakpoints. This is the foundation for subsequent snapshot and diff commands.

---

## Architecture Overview

The feature follows the existing codebase patterns exactly:

- A new `project` package owns the data model and file I/O (mirrors `browser/state.go`)
- Four new handler functions in `cmd/` dispatch the commands (mirrors `cmd/breakpoints.go`, etc.)
- `main.go` switch statement gains four new cases
- `cmd/usage.go` `CommandUsage` map gains four new entries
- `cmd/util.go` `getNonFlagArgs` skip list gains the new flags (`--name`, `--base-url`, `--paths`)
- All output goes through `output.Success` / `output.Fail` (no direct `fmt.Println`)

No new dependencies are required. All file I/O uses `encoding/json` and `os` from the standard library.

---

## Data Model

### `.ks-project.json` schema

```json
{
  "name": "my-app",
  "baseURL": "http://localhost:3000",
  "paths": ["/", "/dashboard", "/settings"],
  "breakpoints": [
    { "name": "mobile",  "width": 375,  "height": 812  },
    { "name": "tablet",  "width": 768,  "height": 1024 },
    { "name": "desktop", "width": 1280, "height": 720  },
    { "name": "wide",    "width": 1920, "height": 1080 }
  ]
}
```

### Rules

- The file is always written to the current working directory as `.ks-project.json`.
- It is committed to the repo (not gitignored); `.kaleidoscope/snapshots/` is gitignored (future story).
- `breakpoints` defaults to the four standard presets when not provided at `init` time. No `--breakpoints` flag is implemented in this story.
- `paths` entries are URL path strings (must begin with `/`).

---

## New Package: `project/`

**File**: `project/project.go`

```
package project

import (
    "encoding/json"
    "errors"
    "os"
)

// Breakpoint mirrors the existing breakpoint type in cmd/breakpoints.go.
type Breakpoint struct {
    Name   string `json:"name"`
    Width  int    `json:"width"`
    Height int    `json:"height"`
}

// Config is the in-memory representation of .ks-project.json.
type Config struct {
    Name        string       `json:"name"`
    BaseURL     string       `json:"baseURL"`
    Paths       []string     `json:"paths"`
    Breakpoints []Breakpoint `json:"breakpoints"`
}

const Filename = ".ks-project.json"

// DefaultBreakpoints are the four standard presets.
var DefaultBreakpoints = []Breakpoint{
    {"mobile",  375,  812},
    {"tablet",  768,  1024},
    {"desktop", 1280, 720},
    {"wide",    1920, 1080},
}

// Exists reports whether .ks-project.json exists in the current directory.
func Exists() bool { ... }

// Read loads and parses .ks-project.json from the current directory.
func Read() (*Config, error) { ... }

// Write serialises cfg and writes it to .ks-project.json (indent = 2 spaces).
func Write(cfg *Config) error { ... }
```

### Function contracts

#### `Exists() bool`
- `os.Stat(Filename)` — returns `true` only when the file exists and `os.Stat` returns no error.

#### `Read() (*Config, error)`
- `os.ReadFile(Filename)` → `json.Unmarshal` → return `*Config`.
- Returns a wrapped error that includes the filename if the file is absent or malformed.

#### `Write(cfg *Config) error`
- `json.MarshalIndent(cfg, "", "  ")` → `os.WriteFile(Filename, data, 0644)`.

---

## New Command Handlers

### File: `cmd/init.go`

**Function**: `RunInit(args []string)`

**Flag parsing**:
| Flag | Required | Description |
|---|---|---|
| `--name <name>` | yes | Project name |
| `--base-url <url>` | yes | Base URL (e.g. `http://localhost:3000`) |
| `--paths <csv>` | yes | Comma-separated path list (e.g. `/,/dashboard`) |

**Logic**:
1. Parse `--name`, `--base-url`, `--paths` with `getFlagValue`.
2. If any required flag is missing → `output.Fail("init", err, hint)` + `os.Exit(2)`.
3. Split `--paths` value on `,`; trim whitespace from each element.
4. If `project.Exists()` → `output.Fail("init", errors.New(".ks-project.json already exists"), "")` + `os.Exit(2)`.
5. Build `project.Config{Name, BaseURL, Paths, Breakpoints: project.DefaultBreakpoints}`.
6. `project.Write(cfg)` → on error `output.Fail(...)`.
7. On success: `output.Success("init", map[string]any{"path": project.Filename, "name": cfg.Name, "baseURL": cfg.BaseURL, "paths": cfg.Paths, "breakpoints": cfg.Breakpoints})`.

### File: `cmd/project.go`

Contains three functions: `RunProjectAdd`, `RunProjectRemove`, `RunProjectShow`.

#### `RunProjectAdd(args []string)`

**Argument**: first non-flag arg is the path to add.

**Logic**:
1. `getArg(args)` → path string; if empty → `output.Fail` with hint.
2. `project.Read()` → if error → `output.Fail`.
3. Check for duplicate: iterate `cfg.Paths`; if path already present → `output.Fail("project-add", errors.New("path already exists: "+path), "")` + `os.Exit(2)`.
4. Append path to `cfg.Paths`.
5. `project.Write(cfg)` → on error `output.Fail`.
6. `output.Success("project-add", map[string]any{"path": path, "paths": cfg.Paths})`.

#### `RunProjectRemove(args []string)`

**Argument**: first non-flag arg is the path to remove.

**Logic**:
1. `getArg(args)` → path string; if empty → `output.Fail`.
2. `project.Read()` → if error → `output.Fail`.
3. Search `cfg.Paths` for the path; if not found → `output.Fail("project-remove", errors.New("path not found: "+path), "")` + `os.Exit(2)`.
4. Remove the path (filter slice without it).
5. `project.Write(cfg)` → on error `output.Fail`.
6. `output.Success("project-remove", map[string]any{"removed": path, "paths": cfg.Paths})`.

#### `RunProjectShow(args []string)`

**Logic**:
1. `project.Read()` → if error → `output.Fail("project-show", err, "Run: ks init --name <name> --base-url <url> --paths <paths>")` + `os.Exit(2)`.
2. `output.Success("project-show", cfg)` — the entire `*Config` struct serialises as the result payload.

---

## Changes to Existing Files

### `main.go`

Add four cases to the `switch command` block:

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

Add a new section in the `usage` string under a "Project Config:" heading:

```
Project Config:
  init --name <n> --base-url <u> --paths <p>  Create .ks-project.json
  project-add <path>                           Add a path to the project
  project-remove <path>                        Remove a path from the project
  project-show                                 Show the project config as JSON
```

### `cmd/util.go` — `getNonFlagArgs`

Add `--name`, `--base-url`, `--paths` to the value-consuming flag skip list:

```go
if a == "--selector" || ... || a == "--name" || a == "--base-url" || a == "--paths" {
    skip = true
}
```

### `cmd/usage.go` — `CommandUsage`

Add four entries:

```
"init": `ks init --name <name> --base-url <url> --paths <csv-paths>

Create a .ks-project.json in the current directory.

Options:
  --name <name>       Project name (required)
  --base-url <url>    Base URL for the project (required)
  --paths <csv>       Comma-separated list of URL paths to track (required)

Output:
  { "ok": true, "result": { "path": ".ks-project.json", "name": "...", "baseURL": "...", "paths": [...], "breakpoints": [...] } }

Examples:
  ks init --name my-app --base-url http://localhost:3000 --paths /,/dashboard

Notes:
  Fails if .ks-project.json already exists.
  Breakpoints default to mobile, tablet, desktop, and wide presets.`,

"project-add": `ks project-add <path>

Append a URL path to the project config.

Arguments:
  path    URL path to add (e.g. /settings)

Output:
  { "ok": true, "result": { "path": "/settings", "paths": [...] } }

Notes:
  Fails if the path already exists in the project.
  Requires .ks-project.json in the current directory.`,

"project-remove": `ks project-remove <path>

Remove a URL path from the project config.

Arguments:
  path    URL path to remove (e.g. /settings)

Output:
  { "ok": true, "result": { "removed": "/settings", "paths": [...] } }

Notes:
  Fails if the path does not exist in the project.
  Requires .ks-project.json in the current directory.`,

"project-show": `ks project-show

Display the full project config as structured JSON.

Output:
  { "ok": true, "result": { "name": "...", "baseURL": "...", "paths": [...], "breakpoints": [...] } }

Notes:
  Requires .ks-project.json in the current directory.`,
```

---

## API Definitions

No HTTP API. All interaction is via the `ks` CLI. JSON output schema per command:

### `ks init` success result

```json
{
  "ok": true,
  "command": "init",
  "result": {
    "path": ".ks-project.json",
    "name": "my-app",
    "baseURL": "http://localhost:3000",
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

### `ks project-add` success result

```json
{ "ok": true, "command": "project-add", "result": { "path": "/settings", "paths": ["/", "/dashboard", "/settings"] } }
```

### `ks project-remove` success result

```json
{ "ok": true, "command": "project-remove", "result": { "removed": "/settings", "paths": ["/", "/dashboard"] } }
```

### `ks project-show` success result

```json
{
  "ok": true,
  "command": "project-show",
  "result": {
    "name": "my-app",
    "baseURL": "http://localhost:3000",
    "paths": ["/", "/dashboard"],
    "breakpoints": [...]
  }
}
```

### Error result (all commands)

```json
{ "ok": false, "command": "<cmd>", "error": "<message>", "hint": "<hint>" }
```

---

## File Structure After Implementation

```
project/
  project.go          # Config type, Exists/Read/Write, DefaultBreakpoints

cmd/
  init.go             # RunInit
  project.go          # RunProjectAdd, RunProjectRemove, RunProjectShow
  util.go             # +3 flags in getNonFlagArgs skip list (edited)
  usage.go            # +4 entries in CommandUsage map (edited)

main.go               # +4 switch cases, updated usage string (edited)
```

---

## Security Considerations

- **Path traversal**: The `project.Write` function always writes to the literal filename `.ks-project.json` in the current working directory. It does not accept any user-supplied file path, so directory traversal is not possible.
- **Arbitrary URL storage**: `baseURL` and `paths` are stored as plain strings and never executed by this package. Validation (scheme check, path prefix `/`) should be added as input guards in `RunInit` and `RunProjectAdd` to reject obviously invalid input early, though no network calls are made here.
- **File permissions**: Written with `0644` (owner rw, group/other r) — consistent with the rest of the codebase (`browser/state.go`).
- **JSON injection**: `encoding/json` handles marshalling; no manual string concatenation is used for JSON output.

---

## Testing Notes (quality gate: `go test ./...`)

- `project/project_test.go`: table-driven tests for `Read`, `Write`, `Exists` using `os.TempDir` / `os.Chdir` to isolate filesystem state.
- `cmd/init_test.go` and `cmd/project_test.go`: test each command's happy path and all documented error conditions (missing flags, duplicate path, nonexistent path, already-exists guard).
- No Chrome/browser dependency — all commands in this story are pure file I/O.
