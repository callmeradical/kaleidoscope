# Tech Spec: US-001 — Project Config Commands

## Overview

Introduce `.ks-project.json` as a project-level configuration file and four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) that create and manage it. This enables snapshot and diff commands (future stories) to know which pages to monitor.

---

## Architecture Overview

```
main.go                        (add cases: init, project-add, project-remove, project-show)
cmd/
  project.go                   (new file: all four command handlers + shared project helpers)
cmd/usage.go                   (add usage strings for the four new commands)
.ks-project.json               (new artifact: committed to repo, written by ks init)
```

No new packages. No browser dependency. No Chrome required for any of these commands. All operations are pure file I/O on `.ks-project.json` in the current working directory.

---

## Data Model

### `.ks-project.json`

```json
{
  "name": "my-app",
  "baseURL": "https://example.com",
  "paths": ["/", "/dashboard", "/settings"],
  "breakpoints": [
    { "name": "mobile",   "width": 375,  "height": 812  },
    { "name": "tablet",   "width": 768,  "height": 1024 },
    { "name": "desktop",  "width": 1280, "height": 720  },
    { "name": "wide",     "width": 1920, "height": 1080 }
  ]
}
```

**Fields:**

| Field        | Type             | Description                                      |
|--------------|------------------|--------------------------------------------------|
| `name`       | `string`         | Human-readable project name                      |
| `baseURL`    | `string`         | Base URL (no trailing slash enforced on write)   |
| `paths`      | `[]string`       | URL paths to monitor; each must start with `/`   |
| `breakpoints`| `[]Breakpoint`   | Viewport presets; defaults to the four standard  |

### Go Struct (in `cmd/project.go`)

```go
// ProjectConfig represents .ks-project.json
type ProjectConfig struct {
    Name        string       `json:"name"`
    BaseURL     string       `json:"baseURL"`
    Paths       []string     `json:"paths"`
    Breakpoints []Breakpoint `json:"breakpoints"`
}

// Breakpoint is a named viewport size.
type Breakpoint struct {
    Name   string `json:"name"`
    Width  int    `json:"width"`
    Height int    `json:"height"`
}
```

`Breakpoint` is self-contained in `cmd/project.go`. The existing `breakpoint` struct in `cmd/breakpoints.go` is unexported and stays separate — no refactor needed.

---

## Component Design

### File: `cmd/project.go`

#### Constants / defaults

```go
const projectConfigFile = ".ks-project.json"

var defaultProjectBreakpoints = []Breakpoint{
    {"mobile",  375,  812},
    {"tablet",  768,  1024},
    {"desktop", 1280, 720},
    {"wide",    1920, 1080},
}
```

#### Helper: `readProjectConfig() (*ProjectConfig, error)`

- Reads and JSON-decodes `.ks-project.json` from CWD.
- Returns a typed error if the file does not exist (used by all sub-commands).

#### Helper: `writeProjectConfig(cfg *ProjectConfig) error`

- JSON-marshals with `json.MarshalIndent` (2-space indent) and writes to `.ks-project.json`.

---

#### `RunInit(args []string)`

**CLI:** `ks init --name <name> --base-url <url> --paths /,/dashboard`

Logic:
1. Parse `--name`, `--base-url`, `--paths` from `args`. All three are required.
2. Check if `.ks-project.json` already exists via `os.Stat`. If it does, call `output.Fail` and `os.Exit(2)`.
3. Split `--paths` value on `,` to produce `[]string`.
4. Build `ProjectConfig` with `defaultProjectBreakpoints`.
5. Call `writeProjectConfig`.
6. Call `output.Success("init", map[string]any{ "path": projectConfigFile, "name": cfg.Name, "baseURL": cfg.BaseURL, "paths": cfg.Paths, "breakpoints": cfg.Breakpoints })`.

Flag parsing uses a local loop over `args` (consistent with other commands in this codebase — no `flag` package).

---

#### `RunProjectAdd(args []string)`

**CLI:** `ks project-add <path>`

Logic:
1. `path := getArg(args)` — first positional argument.
2. If empty, `output.Fail` + exit.
3. `readProjectConfig()` — fail if not found.
4. Check for duplicate: if `path` already in `cfg.Paths`, `output.Fail("project-add", fmt.Errorf("path already exists: %s", path), "")` + exit.
5. Append path, `writeProjectConfig`.
6. `output.Success("project-add", map[string]any{ "path": path, "paths": cfg.Paths })`.

---

#### `RunProjectRemove(args []string)`

**CLI:** `ks project-remove <path>`

Logic:
1. `path := getArg(args)`.
2. If empty, `output.Fail` + exit.
3. `readProjectConfig()`.
4. Linear scan for `path`. If not found, `output.Fail("project-remove", fmt.Errorf("path not found: %s", path), "")` + exit.
5. Remove element, `writeProjectConfig`.
6. `output.Success("project-remove", map[string]any{ "path": path, "paths": cfg.Paths })`.

---

#### `RunProjectShow(args []string)`

**CLI:** `ks project-show`

Logic:
1. `readProjectConfig()` — fail if not found, with hint `"Run: ks init --name <name> --base-url <url> --paths /"`.
2. `output.Success("project-show", cfg)` — marshals the full struct as the result.

---

### File: `main.go` changes

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

Add the four commands to the `usage` string under a new section:

```
Project Config:
  init --name <n> --base-url <u> --paths <p>   Create .ks-project.json
  project-add <path>                            Add a path to the project
  project-remove <path>                         Remove a path from the project
  project-show                                  Show project config as JSON
```

---

### File: `cmd/usage.go` changes

Add four entries to `CommandUsage`:

```go
"init": `ks init --name <name> --base-url <url> --paths <paths>

Create a .ks-project.json in the current directory.

Options:
  --name <name>       Project name (required)
  --base-url <url>    Base URL to monitor (required)
  --paths <paths>     Comma-separated URL paths (required), e.g. /,/dashboard

Output:
  { "ok": true, "result": { "path": ".ks-project.json", "name": "...", "baseURL": "...", "paths": [...], "breakpoints": [...] } }

Notes:
  Breakpoints default to mobile (375x812), tablet (768x1024), desktop (1280x720), wide (1920x1080).
  Returns an error if .ks-project.json already exists.`,

"project-add": `ks project-add <path>

Append a URL path to the project config.

Arguments:
  path    URL path to add, e.g. /settings (required)

Output:
  { "ok": true, "result": { "path": "/settings", "paths": [...] } }

Notes:
  Returns an error if the path already exists.`,

"project-remove": `ks project-remove <path>

Remove a URL path from the project config.

Arguments:
  path    URL path to remove (required)

Output:
  { "ok": true, "result": { "path": "/settings", "paths": [...] } }

Notes:
  Returns an error if the path does not exist.`,

"project-show": `ks project-show

Print the full project config as structured JSON.

Output:
  { "ok": true, "result": { "name": "...", "baseURL": "...", "paths": [...], "breakpoints": [...] } }

Notes:
  Returns an error if .ks-project.json does not exist. Run ks init first.`,
```

---

## API Definitions

No HTTP API. All four commands are pure CLI → stdout JSON via `output.Result`.

### Output shapes

| Command          | `result` payload                                             |
|------------------|--------------------------------------------------------------|
| `init`           | `{ path, name, baseURL, paths[], breakpoints[] }`           |
| `project-add`    | `{ path, paths[] }`                                          |
| `project-remove` | `{ path, paths[] }`                                          |
| `project-show`   | Full `ProjectConfig` struct                                  |

All errors follow the existing `output.Fail` convention: `{ "ok": false, "command": "...", "error": "...", "hint": "..." }`.

---

## File Location Conventions

| File                            | Committed? | Notes                                  |
|---------------------------------|------------|----------------------------------------|
| `.ks-project.json`              | Yes        | Shared project definition              |
| `.kaleidoscope/`                | No         | Runtime state; gitignored              |
| `.kaleidoscope/snapshots/`      | No         | Future story                           |
| `.kaleidoscope/baselines.json`  | Yes        | Future story                           |

These commands only read/write `.ks-project.json` in the current working directory (no `StateDir` or `~/.kaleidoscope` involvement).

---

## Flag Parsing Pattern

Consistent with existing commands (no `flag` package). Use a local helper:

```go
func getFlagValue(args []string, flag string) string {
    for i, a := range args {
        if a == flag && i+1 < len(args) {
            return args[i+1]
        }
    }
    return ""
}
```

`--paths` is split with `strings.Split(rawPaths, ",")` and each element trimmed with `strings.TrimSpace`.

---

## Security Considerations

- **Path traversal:** `.ks-project.json` is always written to `"."` (CWD). No user-controlled path is used as a file path — it is stored as data only.
- **URL validation:** `baseURL` and path values are stored as strings without executing or fetching them in these commands. No validation of URL structure is required at this layer (future snapshot commands will use them with the browser).
- **File permissions:** Written with `0644`, consistent with all other files in the project.
- **No secrets:** The config file contains no credentials. Safe to commit.

---

## Testing

Quality gate: `go test ./...`

Test file: `cmd/project_test.go`

Scenarios to cover:
1. `RunInit` creates `.ks-project.json` with correct content and default breakpoints.
2. `RunInit` returns error if file already exists.
3. `RunProjectAdd` appends a new path and persists it.
4. `RunProjectAdd` returns error on duplicate path.
5. `RunProjectRemove` removes an existing path and persists it.
6. `RunProjectRemove` returns error on non-existent path.
7. `RunProjectShow` outputs full config.
8. `RunProjectShow` returns error if file is missing.

Each test should create a temp directory, `os.Chdir` into it, run the command, and assert on the written file content and/or stdout JSON.

---

## Implementation Checklist

- [ ] `cmd/project.go` — `ProjectConfig`, `Breakpoint` structs, helpers, four `Run*` functions
- [ ] `main.go` — four switch cases + usage string section
- [ ] `cmd/usage.go` — four `CommandUsage` entries
- [ ] `cmd/project_test.go` — unit tests for all eight scenarios
