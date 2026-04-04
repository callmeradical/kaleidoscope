# Tech Spec: Project Config Commands (US-001)

## Overview

Implement four CLI commands that manage a `.ks-project.json` file in the current working directory. This file defines the set of URLs a project tracks for snapshot and diff workflows.

**Commands:** `ks init`, `ks project-add`, `ks project-remove`, `ks project-show`

---

## Architecture Overview

```
main.go                    — add 4 new case branches
cmd/project.go             — new file: RunInit, RunProjectAdd, RunProjectRemove, RunProjectShow
cmd/project_config.go      — new file: ProjectConfig struct, load/save helpers
cmd/util.go                — extend getFlagValue to handle --name, --base-url, --paths flags
cmd/usage.go               — add usage strings for all 4 new commands
```

No new packages. No browser dependency. All four commands are pure file I/O operations.

---

## Data Model

### `.ks-project.json`

Stored in the current working directory. Committed to the repository.

```json
{
  "name": "my-app",
  "baseUrl": "https://example.com",
  "paths": ["/", "/dashboard", "/settings"],
  "breakpoints": [
    { "name": "mobile",  "width": 375,  "height": 812  },
    { "name": "tablet",  "width": 768,  "height": 1024 },
    { "name": "desktop", "width": 1280, "height": 720  },
    { "name": "wide",    "width": 1920, "height": 1080 }
  ]
}
```

### Go struct (`cmd/project_config.go`)

```go
package cmd

type ProjectBreakpoint struct {
    Name   string `json:"name"`
    Width  int    `json:"width"`
    Height int    `json:"height"`
}

type ProjectConfig struct {
    Name        string              `json:"name"`
    BaseURL     string              `json:"baseUrl"`
    Paths       []string            `json:"paths"`
    Breakpoints []ProjectBreakpoint `json:"breakpoints"`
}

const projectConfigFile = ".ks-project.json"

var defaultProjectBreakpoints = []ProjectBreakpoint{
    {"mobile",  375,  812},
    {"tablet",  768,  1024},
    {"desktop", 1280, 720},
    {"wide",    1920, 1080},
}
```

---

## Component Design

### `cmd/project_config.go` — Config helpers

**`loadProjectConfig() (*ProjectConfig, error)`**
- Reads `.ks-project.json` from the current working directory via `os.ReadFile`.
- Returns a typed error if the file does not exist (callers check for this to distinguish "not initialized" from other IO errors).
- JSON-unmarshals into `ProjectConfig`.

**`saveProjectConfig(cfg *ProjectConfig) error`**
- JSON-marshals with `json.MarshalIndent` (2-space indent for human readability).
- Writes to `.ks-project.json` in CWD with mode `0644`.

---

### `cmd/project.go` — Command handlers

#### `RunInit(args []string)`

```
ks init --name <name> --base-url <url> --paths <comma-separated-paths>
```

Logic:
1. Parse `--name`, `--base-url`, `--paths` from args via `getFlagValue`.
2. Validate: `--name` and `--base-url` are required; exit 2 with `output.Fail` if missing.
3. Check if `.ks-project.json` already exists via `os.Stat`; if it does, call `output.Fail` and exit 2 (error: "project already initialized").
4. Split `--paths` on `,` to get a `[]string` of paths. If `--paths` is empty, default to `["/"]`.
5. Construct `ProjectConfig` with `Breakpoints` set to `defaultProjectBreakpoints`.
6. Call `saveProjectConfig`.
7. Call `output.Success("init", map[string]any{...})` with the full config as result.

#### `RunProjectAdd(args []string)`

```
ks project-add <path>
```

Logic:
1. Get first non-flag arg as `path`. Validate non-empty; exit 2 if missing.
2. Load config via `loadProjectConfig`; exit 2 with `output.Fail` if not found (hint: run `ks init`).
3. Check for duplicate: if `path` already exists in `cfg.Paths`, call `output.Fail` ("path already exists") and exit 2.
4. Append path to `cfg.Paths`.
5. Call `saveProjectConfig`.
6. Call `output.Success("project-add", map[string]any{"path": path, "paths": cfg.Paths})`.

#### `RunProjectRemove(args []string)`

```
ks project-remove <path>
```

Logic:
1. Get first non-flag arg as `path`. Validate non-empty; exit 2 if missing.
2. Load config; exit 2 if not found.
3. Search for `path` in `cfg.Paths`. If not found, call `output.Fail` ("path not found") and exit 2.
4. Remove matching element from the slice (preserve order, rebuild slice without the element).
5. Call `saveProjectConfig`.
6. Call `output.Success("project-remove", map[string]any{"path": path, "paths": cfg.Paths})`.

#### `RunProjectShow(args []string)`

```
ks project-show
```

Logic:
1. Load config; exit 2 if not found (hint: run `ks init`).
2. Call `output.Success("project-show", cfg)` — marshals the full `ProjectConfig` struct directly.

---

## API Definitions

All commands use the existing `output.Result` envelope:

```json
{ "ok": true,  "command": "<cmd>", "result": { ... } }
{ "ok": false, "command": "<cmd>", "error": "...", "hint": "..." }
```

### `ks init` success result
```json
{
  "name": "my-app",
  "baseUrl": "https://example.com",
  "paths": ["/", "/dashboard"],
  "breakpoints": [
    { "name": "mobile",  "width": 375,  "height": 812  },
    { "name": "tablet",  "width": 768,  "height": 1024 },
    { "name": "desktop", "width": 1280, "height": 720  },
    { "name": "wide",    "width": 1920, "height": 1080 }
  ]
}
```

### `ks project-add` / `ks project-remove` success result
```json
{ "path": "/settings", "paths": ["/", "/dashboard", "/settings"] }
```

### `ks project-show` success result
Full `ProjectConfig` struct (same shape as `ks init`).

### Error cases
| Command          | Condition                     | Error message                |
|------------------|-------------------------------|------------------------------|
| `init`           | `.ks-project.json` exists     | "project already initialized"|
| `init`           | `--name` missing              | "missing --name"             |
| `init`           | `--base-url` missing          | "missing --base-url"         |
| `project-add`    | no path argument              | "missing path argument"      |
| `project-add`    | path already exists           | "path already exists"        |
| `project-add`    | config not found              | "project not initialized"    |
| `project-remove` | no path argument              | "missing path argument"      |
| `project-remove` | path not in list              | "path not found"             |
| `project-remove` | config not found              | "project not initialized"    |
| `project-show`   | config not found              | "project not initialized"    |

---

## Changes to Existing Files

### `main.go`

Add four cases to the switch statement:

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

Update the `usage` string to document the four new commands under a new "Project" section:

```
Project:
  init --name <n> --base-url <u> --paths <p>  Initialize project config
  project-add <path>                           Add a path to the project
  project-remove <path>                        Remove a path from the project
  project-show                                 Show project config as JSON
```

### `cmd/util.go`

Extend `getNonFlagArgs` to skip values for the new flags `--name`, `--base-url`, `--paths` so they are not mistakenly returned as positional args. Add these to the existing skip list:

```go
a == "--name" || a == "--base-url" || a == "--paths" ||
```

### `cmd/usage.go`

Add four entries to `CommandUsage` map for `"init"`, `"project-add"`, `"project-remove"`, `"project-show"` following the same format as existing entries.

---

## File Layout After Implementation

```
cmd/
  project.go          (new) RunInit, RunProjectAdd, RunProjectRemove, RunProjectShow
  project_config.go   (new) ProjectConfig, ProjectBreakpoint, loadProjectConfig, saveProjectConfig
  util.go             (modified) extend skip list in getNonFlagArgs
  usage.go            (modified) 4 new CommandUsage entries
main.go               (modified) 4 new case branches + usage string update
```

---

## Security Considerations

- **Path traversal**: `.ks-project.json` is always read/written in the current working directory using the constant `projectConfigFile = ".ks-project.json"`. No user-controlled path components are used in file I/O — the `<path>` argument in `project-add`/`project-remove` is a URL path stored as data, not used as a filesystem path.
- **JSON marshaling**: Standard `encoding/json` is used; no shell execution or templating involved.
- **File permissions**: Written with mode `0644` (owner read/write, world readable), consistent with other config files in the project.
- **No secrets**: `.ks-project.json` contains only project name, base URL, and URL paths — no credentials or tokens.

---

## Quality Gate

All changes must pass: `go test ./...`

No new test files are strictly required for this story (no existing test infrastructure found), but the pure-function helpers `loadProjectConfig` and `saveProjectConfig` are structured to be easily unit-testable with `t.TempDir()` if tests are added later.
