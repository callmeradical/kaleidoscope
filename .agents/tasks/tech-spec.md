# Tech Spec: Project Config Commands (US-001)

## Overview

Introduce a `.ks-project.json` config file and four CLI commands (`ks init`, `ks project-add`, `ks project-remove`, `ks project-show`) that define a Kaleidoscope project as a named set of URL paths and breakpoints to track.

---

## Architecture Overview

```
main.go                    ← add case branches for new commands
cmd/init.go                ← RunInit
cmd/project_add.go         ← RunProjectAdd
cmd/project_remove.go      ← RunProjectRemove
cmd/project_show.go        ← RunProjectShow
project/config.go          ← ProjectConfig type, Load, Save, DefaultBreakpoints
```

A new `project` package encapsulates all config I/O so that future snapshot/diff commands can import it without depending on `cmd`. The four `cmd/` files are thin handlers that parse flags, delegate to `project`, and emit `output.Success`/`output.Fail`.

---

## Component Design

### `project` Package (`project/config.go`)

Responsible for:
- Defining the canonical `ProjectConfig` and `Breakpoint` types
- Reading / writing `.ks-project.json` in the current working directory
- Providing `DefaultBreakpoints()`

```go
package project

const ConfigFile = ".ks-project.json"

type Breakpoint struct {
    Name   string `json:"name"`
    Width  int    `json:"width"`
    Height int    `json:"height"`
}

type ProjectConfig struct {
    Name        string       `json:"name"`
    BaseURL     string       `json:"baseURL"`
    Paths       []string     `json:"paths"`
    Breakpoints []Breakpoint `json:"breakpoints"`
}

func DefaultBreakpoints() []Breakpoint {
    return []Breakpoint{
        {"mobile",  375,  812},
        {"tablet",  768, 1024},
        {"desktop", 1280, 720},
        {"wide",   1920, 1080},
    }
}

// Load reads .ks-project.json from CWD. Returns error if file does not exist.
func Load() (*ProjectConfig, error)

// Save writes cfg to .ks-project.json in CWD with 2-space indentation.
func Save(cfg *ProjectConfig) error
```

`Load` and `Save` operate on `filepath.Join(".", ConfigFile)`. No home-directory fallback — the project config is always local.

---

### `cmd/init.go` — `RunInit`

**Flags parsed:**
- `--name <name>` (required)
- `--base-url <url>` (required)
- `--paths <comma-separated-paths>` (optional; defaults to `["/"]`)
- `--breakpoints` — not exposed in this story; breakpoints always default to the four presets

**Logic:**
1. Check whether `.ks-project.json` already exists → `output.Fail` with hint if so.
2. Parse `--name`, `--base-url`, `--paths` (split on `,`).
3. Construct `ProjectConfig{Name, BaseURL, Paths, Breakpoints: project.DefaultBreakpoints()}`.
4. `project.Save(cfg)`.
5. `output.Success("init", cfg)`.

---

### `cmd/project_add.go` — `RunProjectAdd`

**Positional arg:** path string (e.g. `/settings`).

**Logic:**
1. `project.Load()` → `output.Fail` if config not found.
2. Check for duplicate path → `output.Fail("project-add", err, "path already exists")`.
3. Append path, `project.Save(cfg)`.
4. `output.Success("project-add", cfg)`.

---

### `cmd/project_remove.go` — `RunProjectRemove`

**Positional arg:** path string.

**Logic:**
1. `project.Load()` → `output.Fail` if config not found.
2. Search for path; if not found → `output.Fail("project-remove", err, "path not found")`.
3. Remove path, `project.Save(cfg)`.
4. `output.Success("project-remove", cfg)`.

---

### `cmd/project_show.go` — `RunProjectShow`

**Logic:**
1. `project.Load()` → `output.Fail` if config not found.
2. `output.Success("project-show", cfg)`.

---

### `main.go` Updates

Add four `case` branches in the `switch command` block:

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

Add entries to the `usage` string under a new `Project Config:` section.

---

## API Definitions (CLI)

| Command | Flags / Args | Success result payload |
|---|---|---|
| `ks init` | `--name <s>` `--base-url <u>` `[--paths /,/dash]` | Full `ProjectConfig` object |
| `ks project-add <path>` | positional path | Updated `ProjectConfig` object |
| `ks project-remove <path>` | positional path | Updated `ProjectConfig` object |
| `ks project-show` | none | Full `ProjectConfig` object |

All commands emit JSON via `output.Result` (`output.Success` / `output.Fail`). Exit code `2` on error, consistent with the rest of the CLI.

---

## Data Model

### `.ks-project.json` (committed to repo)

```json
{
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
```

**No schema versioning needed at this stage** — the file is simple and the full PRD covers a single linear evolution.

### `.gitignore` note

`.kaleidoscope/snapshots/` must be gitignored. This story does not create snapshot infrastructure, but `ks init` should append `.kaleidoscope/snapshots/` to `.gitignore` if not already present (or document this as a manual step). Given the AC only requires `.ks-project.json` creation, appending to `.gitignore` is a nice-to-have — defer to implementor judgement.

---

## Flag Parsing

The existing `getFlagValue` / `hasFlag` helpers in `cmd/util.go` cover all needed parsing. The `--paths` flag value is split on `,` using `strings.Split`, with each element trimmed of whitespace.

`getNonFlagArgs` in `util.go` must be updated to recognise `--name`, `--base-url`, and `--paths` as value-taking flags (so their values are not misidentified as positional args).

---

## Security Considerations

- **Path traversal**: `--paths` values are stored as-is and used only as URL path segments later (in snapshot commands). No filesystem operations are performed on them here. No sanitization risk in this story.
- **Base URL**: Stored as a string; no HTTP requests are made in this story. Validation (e.g., must start with `http://` or `https://`) is recommended to provide early feedback but is not a security gate at this layer.
- **File write scope**: `project.Save` writes only to `filepath.Join(".", ".ks-project.json")` — the current working directory. No path traversal is possible because the filename is a constant.
- **Config file permissions**: Written with `0644` (owner rw, group/other r), consistent with the rest of the project.

---

## Testing

Quality gate: `go test ./...`

Recommended test cases in `project/config_test.go`:
- `Load` returns error when file absent
- `Save` + `Load` roundtrip preserves all fields
- `DefaultBreakpoints` returns exactly 4 presets

Recommended test cases in `cmd/` (table-driven, using `os.MkdirTemp` + `os.Chdir`):
- `RunInit` creates `.ks-project.json` with correct content
- `RunInit` fails if file already exists
- `RunProjectAdd` appends path
- `RunProjectAdd` fails on duplicate
- `RunProjectRemove` removes path
- `RunProjectRemove` fails on missing path
- `RunProjectShow` outputs config JSON

Each test that writes files should use `t.TempDir()` and change the working directory for the duration of the test, restoring it in a `t.Cleanup` callback.
