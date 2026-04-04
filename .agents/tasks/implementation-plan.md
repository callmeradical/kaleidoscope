# Implementation Plan: Snapshot Capture and History (US-002)

## Overview

Introduce `ks snapshot` and `ks history` commands that capture full interface state (screenshots at 4 breakpoints, audit results, accessibility tree) for every URL in a project config, persist them under `.kaleidoscope/snapshots/`, and auto-promote the first snapshot as a baseline.

**Depends on:** US-001 (`.ks-project.json` project config must exist)
**Quality Gate:** `go test ./...` must pass

---

## Phase 1: `snapshot/` Package — Pure Data Model and Storage

No browser dependency. All functions are pure (data in → data out) or do simple filesystem I/O.

### Task 1.1 — `snapshot/model.go`

Define the core types used throughout the feature.

**Sub-tasks:**
1. Create `/workspace/snapshot/model.go` with package declaration `package snapshot`.
2. Define `ProjectConfig` struct:
   - `Name string` (json: `"name"`)
   - `URLs []string` (json: `"urls"`)
3. Define `AuditSummary` struct:
   - `TotalIssues int` (json: `"totalIssues"`)
   - `ContrastViolations int` (json: `"contrastViolations"`)
   - `TouchViolations int` (json: `"touchViolations"`)
   - `TypographyWarnings int` (json: `"typographyWarnings"`)
4. Define `URLEntry` struct:
   - `URL string` (json: `"url"`)
   - `Dir string` (json: `"dir"`)
   - `Breakpoints []string` (json: `"breakpoints"`)
   - `AuditSummary AuditSummary` (json: `"auditSummary"`)
   - `AxNodeCount int` (json: `"axNodeCount"`)
   - `Error string` (json: `"error,omitempty"`)
5. Define `Manifest` struct:
   - `ID string` (json: `"id"`)
   - `Timestamp time.Time` (json: `"timestamp"`)
   - `CommitHash string` (json: `"commitHash,omitempty"`)
   - `ProjectConfig ProjectConfig` (json: `"projectConfig"`)
   - `URLs []URLEntry` (json: `"urls"`)
6. Define `Baseline` struct:
   - `SnapshotID string` (json: `"snapshotId"`)
   - `SetAt time.Time` (json: `"setAt"`)
7. Add `import "time"`.

---

### Task 1.2 — `snapshot/urldir.go`

URL-to-directory-name sanitization with path traversal prevention.

**Sub-tasks:**
1. Create `/workspace/snapshot/urldir.go`.
2. Import `"net/url"`, `"regexp"`, `"strings"`.
3. Implement `URLToDir(rawURL string) string`:
   - Parse the URL; if parsing fails, treat the full string as the path.
   - Strip the scheme (e.g., `http://`, `https://`).
   - Take `host + path` (host includes port, e.g., `localhost:3000`).
   - Replace every character that is NOT `[a-zA-Z0-9]` with `-`.
   - Collapse consecutive dashes into a single dash using a regex (`-{2,}` → `-`).
   - Trim leading and trailing dashes.
   - Verify output contains no `/`, `\`, or `..` (return `"invalid"` if it does as a safety net).
   - Return the sanitized string.

**Examples to implement correctly:**
- `"http://localhost:3000"` → `"localhost-3000"`
- `"http://localhost:3000/about"` → `"localhost-3000-about"`
- `"https://example.com/foo/bar"` → `"example-com-foo-bar"`
- `"http://localhost:3000/../etc"` → safe form with no `..`

---

### Task 1.3 — `snapshot/store.go`

Filesystem storage: directories, save, load, list.

**Sub-tasks:**
1. Create `/workspace/snapshot/store.go`.
2. Import `"encoding/json"`, `"os"`, `"path/filepath"`, `"sort"`, `"github.com/callmeradical/kaleidoscope/browser"`.
3. Implement `SnapshotsDir() (string, error)`:
   - Call `browser.StateDir()` to get the `.kaleidoscope/` root.
   - Return `filepath.Join(stateDir, "snapshots")`.
   - Do NOT create it here (callers are responsible for `os.MkdirAll`).
4. Implement `SnapshotPath(id string) (string, error)`:
   - Call `SnapshotsDir()`.
   - Return `filepath.Join(snapshotsDir, id)`.
5. Implement `Save(m *Manifest) error`:
   - Call `SnapshotPath(m.ID)` to get the directory.
   - Call `os.MkdirAll(snapshotDir, 0755)`.
   - Marshal `m` to JSON with `json.MarshalIndent` (2-space indent).
   - Write to `filepath.Join(snapshotDir, "snapshot.json")` with `os.WriteFile(..., 0644)`.
6. Implement `Load(id string) (*Manifest, error)`:
   - Call `SnapshotPath(id)`.
   - Read `filepath.Join(snapshotPath, "snapshot.json")` with `os.ReadFile`.
   - Unmarshal JSON into `*Manifest`.
   - Return manifest or error.
7. Implement `List() ([]*Manifest, error)`:
   - Call `SnapshotsDir()`.
   - Call `os.ReadDir(snapshotsDir)` — if dir doesn't exist, return empty slice (not an error).
   - For each directory entry (skip files), attempt `Load(entry.Name())`.
   - On load error: write warning to `os.Stderr`, skip the entry (do not fail).
   - Sort resulting `[]*Manifest` by `Timestamp` descending (newest first).
   - Return sorted slice.

---

### Task 1.4 — `snapshot/project.go`

Load the project config from `.ks-project.json`.

**Sub-tasks:**
1. Create `/workspace/snapshot/project.go`.
2. Import `"encoding/json"`, `"fmt"`, `"os"`.
3. Implement `LoadProjectConfig() (*ProjectConfig, error)`:
   - Read `".ks-project.json"` from the current working directory using `os.ReadFile`.
   - On `os.IsNotExist` error: return descriptive error `fmt.Errorf(".ks-project.json not found: %w — create it with a 'name' and 'urls' array", err)`.
   - Unmarshal JSON into `*ProjectConfig`.
   - Validate that `len(config.URLs) > 0`; if not, return error `"no URLs defined in .ks-project.json"`.
   - Return config.

---

### Task 1.5 — `snapshot/baseline.go`

Read/write the baseline pointer file.

**Sub-tasks:**
1. Create `/workspace/snapshot/baseline.go`.
2. Import `"encoding/json"`, `"os"`, `"path/filepath"`, `"github.com/callmeradical/kaleidoscope/browser"`.
3. Implement `BaselinePath() (string, error)`:
   - Call `browser.StateDir()`.
   - Return `filepath.Join(stateDir, "baselines.json")`.
4. Implement `LoadBaseline() (*Baseline, error)`:
   - Call `BaselinePath()`.
   - Read the file with `os.ReadFile`.
   - If `os.IsNotExist(err)`: return `nil, nil` (no baseline is not an error).
   - Unmarshal JSON into `*Baseline`.
   - Return baseline.
5. Implement `SaveBaseline(b *Baseline) error`:
   - Call `BaselinePath()`.
   - Marshal `b` to JSON with `json.MarshalIndent` (2-space indent).
   - Write with `os.WriteFile(..., 0644)`.

---

## Phase 2: Unit Tests for `snapshot/` Package

Tests must pass with `go test ./...`. These tests have no browser dependency.

### Task 2.1 — `snapshot/urldir_test.go`

**Sub-tasks:**
1. Create `/workspace/snapshot/urldir_test.go`.
2. Add table-driven test `TestURLToDir` covering:
   - `"http://localhost:3000"` → `"localhost-3000"`
   - `"http://localhost:3000/about"` → `"localhost-3000-about"`
   - `"https://example.com/foo/bar"` → `"example-com-foo-bar"`
   - `"http://localhost:3000/../etc"` → output must not contain `..` or `/`
   - `"http://evil.com/%2F%2F"` → safe sanitized output
   - Empty string → handled gracefully (no panic)

---

### Task 2.2 — `snapshot/store_test.go`

Uses a temp directory via `t.TempDir()` to avoid touching real state.

**Sub-tasks:**
1. Create `/workspace/snapshot/store_test.go`.
2. Add test `TestSaveLoad`:
   - Construct a `Manifest` with known values.
   - Set up environment so `browser.StateDir()` resolves to `t.TempDir()` (use `os.Setenv` or create `.kaleidoscope/` in test temp dir and `os.Chdir`).
   - Call `Save(&m)`.
   - Call `Load(m.ID)`.
   - Assert all fields match (ID, Timestamp, CommitHash, URLs, etc.).
3. Add test `TestListSortOrder`:
   - Save two manifests with different timestamps (newer second).
   - Call `List()`.
   - Assert first result has the newer timestamp.
4. Add test `TestListEmptyDir`:
   - Call `List()` against a non-existent snapshots dir.
   - Assert returns empty slice and no error.

---

### Task 2.3 — `snapshot/baseline_test.go`

**Sub-tasks:**
1. Create `/workspace/snapshot/baseline_test.go`.
2. Add test `TestLoadBaselineMissing`:
   - Set up state dir without baselines.json.
   - Call `LoadBaseline()`.
   - Assert result is `nil` and error is `nil`.
3. Add test `TestSaveLoadBaseline`:
   - Construct a `Baseline` with known `SnapshotID` and `SetAt`.
   - Call `SaveBaseline(&b)`.
   - Call `LoadBaseline()`.
   - Assert fields match.

---

### Task 2.4 — `snapshot/project_test.go`

**Sub-tasks:**
1. Create `/workspace/snapshot/project_test.go`.
2. Add test `TestLoadProjectConfigMissing`:
   - Run from a temp dir without `.ks-project.json`.
   - Call `LoadProjectConfig()`.
   - Assert error is non-nil and message contains actionable guidance.
3. Add test `TestLoadProjectConfigValid`:
   - Write a valid `.ks-project.json` to temp dir.
   - Call `LoadProjectConfig()`.
   - Assert `Name` and `URLs` are populated correctly.

---

## Phase 3: Shared `cmd/` Helpers (Refactoring)

Extract duplicated logic into shared helpers within the `cmd` package.

### Task 3.1 — `cmd/breakpoints_common.go` (new file)

Move the breakpoint type and slice to a shared location.

**Sub-tasks:**
1. Read `/workspace/cmd/breakpoints.go` to find the existing `breakpoint` struct and `defaultBreakpoints` definition.
2. Create `/workspace/cmd/breakpoints_common.go` with:
   ```go
   package cmd

   type breakpoint struct {
       Name   string
       Width  int
       Height int
   }

   var defaultBreakpoints = []breakpoint{
       {"mobile", 375, 812},
       {"tablet", 768, 1024},
       {"desktop", 1280, 720},
       {"wide", 1920, 1080},
   }
   ```
3. Remove the duplicate `breakpoint` struct and `defaultBreakpoints` from `cmd/breakpoints.go` (leave all other logic intact).
4. Verify `go build ./...` still compiles.

---

### Task 3.2 — `cmd/audit_internal.go` (new file)

Extract audit and ax-tree logic into internal helpers callable from both `cmd/audit.go` and `cmd/snapshot.go`.

**Sub-tasks:**
1. Read `/workspace/cmd/audit.go` in full to understand the exact JS evaluation and analysis calls.
2. Read `/workspace/cmd/axtree.go` to understand the ax-tree dump logic.
3. Create `/workspace/cmd/audit_internal.go` with:
   - `func runAudit(page *rod.Page, selector string) (map[string]any, snapshot.AuditSummary, error)`
     - Move the JS eval + `analysis.*` calls from `cmd/audit.go` here.
     - Return the full result map (same structure currently passed to `output.Success`) and an `AuditSummary`.
   - `func runAxTree(page *rod.Page) ([]map[string]any, int, error)`
     - Move/replicate the `proto.AccessibilityGetFullAXTree{}` call + node counting here.
     - Return the node list (as `[]map[string]any` for JSON marshaling) and non-ignored node count.
4. Refactor `cmd/audit.go`'s `RunAudit` to become a thin wrapper:
   - Call `runAudit(page, selector)`.
   - Call `output.Success("audit", resultMap)`.
5. Verify `go build ./...` still compiles and behavior is unchanged.

---

## Phase 4: `cmd/snapshot.go` — The Snapshot Command

**Sub-tasks:**

### Task 4.1 — File scaffold and imports
1. Create `/workspace/cmd/snapshot.go` with `package cmd`.
2. Add imports: `"encoding/json"`, `"fmt"`, `"os"`, `"os/exec"`, `"path/filepath"`, `"strings"`, `"time"`, `"github.com/go-rod/rod/lib/proto"`, `"github.com/callmeradical/kaleidoscope/browser"`, `"github.com/callmeradical/kaleidoscope/output"`, `"github.com/callmeradical/kaleidoscope/snapshot"`.
3. Define `func RunSnapshot(args []string)`.

### Task 4.2 — Load project config
1. Call `snapshot.LoadProjectConfig()`.
2. On error: call `output.Fail("snapshot", err, "Create a .ks-project.json file with a 'urls' array")` and `os.Exit(2)`.

### Task 4.3 — Resolve git commit hash
1. Run `exec.Command("git", "rev-parse", "--short", "HEAD")`.
2. Capture stdout with `.Output()`.
3. On any error (not in git repo, git not found, etc.): set `commitHash = ""`.
4. On success: `commitHash = strings.TrimSpace(string(out))`.

### Task 4.4 — Build snapshot ID and create directory
1. Format current UTC time: `time.Now().UTC().Format("20060102T150405Z")`.
2. If `commitHash != ""`: `id = ts + "-" + commitHash`, else `id = ts`.
3. Call `snapshot.SnapshotPath(id)` to get `snapshotDir`.
4. Call `os.MkdirAll(snapshotDir, 0755)`. On error: `output.Fail` + exit 2.

### Task 4.5 — Per-URL capture loop
1. Initialize `var entries []snapshot.URLEntry`.
2. Use `browser.WithPage(func(page *rod.Page) error { ... })` for the entire URL loop (open one browser session).
3. On `WithPage` error (browser not running): `output.Fail("snapshot", err, "Run: ks start")` + exit 2.
4. For each `url` in `projectConfig.URLs`:

   **a. Setup:**
   - `urlDir := snapshot.URLToDir(url)`
   - `urlPath := filepath.Join(snapshotDir, urlDir)`
   - `os.MkdirAll(urlPath, 0755)` — on error, return the error (triggers WithPage error handling).

   **b. Navigate:**
   - `page.Navigate(url)` — on error: append `snapshot.URLEntry{URL: url, Dir: urlDir, Error: err.Error()}` to entries, `continue`.
   - `page.WaitLoad()` — on error: append URLEntry with error, `continue`.

   **c. Screenshots:**
   - Iterate `defaultBreakpoints`.
   - For each bp: call `page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: bp.Width, Height: bp.Height, DeviceScaleFactor: 1})`.
   - Call `page.MustWaitStable()`.
   - Call `page.Screenshot(true, nil)` to get PNG bytes.
   - Filename: `fmt.Sprintf("%s-%dx%d.png", bp.Name, bp.Width, bp.Height)`.
   - Write to `filepath.Join(urlPath, filename)` with `os.WriteFile(..., 0644)` — on error, return it.
   - Append filename to `breakpointFiles []string`.
   - After all breakpoints: restore original viewport (read from state via `browser.ReadState()` before the loop).

   **d. Audit:**
   - Call `runAudit(page, "")` (from `cmd/audit_internal.go`).
   - Marshal result map to JSON, write to `filepath.Join(urlPath, "audit.json")` with `os.WriteFile(..., 0644)`.

   **e. Ax-tree:**
   - Call `runAxTree(page)` (from `cmd/audit_internal.go`).
   - Marshal node list to JSON, write to `filepath.Join(urlPath, "ax-tree.json")`.

   **f. Record URLEntry:**
   - Append `snapshot.URLEntry{URL: url, Dir: urlDir, Breakpoints: breakpointFiles, AuditSummary: auditSummary, AxNodeCount: axCount}`.

### Task 4.6 — Write manifest
1. Construct `snapshot.Manifest{ID: id, Timestamp: time.Now().UTC(), CommitHash: commitHash, ProjectConfig: *projectConfig, URLs: entries}`.
2. Call `snapshot.Save(&manifest)` — on error: `output.Fail` + exit 2.

### Task 4.7 — Auto-promote baseline
1. Call `snapshot.LoadBaseline()` — on error: `output.Fail` + exit 2.
2. If returned baseline is `nil`:
   - Call `snapshot.SaveBaseline(&snapshot.Baseline{SnapshotID: id, SetAt: time.Now().UTC()})`.
   - Set `autoBaseline = true`.
3. Else: `autoBaseline = false`.

### Task 4.8 — Output success
1. Build result map:
   ```go
   map[string]any{
       "id":           id,
       "snapshotDir":  snapshotDir,
       "urlCount":     len(entries),
       "autoBaseline": autoBaseline,
       "urls":         entries,
   }
   ```
2. Call `output.Success("snapshot", result)`.

---

## Phase 5: `cmd/history.go` — The History Command

**Sub-tasks:**

### Task 5.1 — File scaffold
1. Create `/workspace/cmd/history.go` with `package cmd`.
2. Add imports: `"github.com/callmeradical/kaleidoscope/output"`, `"github.com/callmeradical/kaleidoscope/snapshot"`.
3. Define `func RunHistory(args []string)`.

### Task 5.2 — Load snapshots and baseline
1. Call `snapshot.List()` — on error: `output.Fail("history", err, "Run: ks snapshot")` + `os.Exit(2)`.
2. Call `snapshot.LoadBaseline()` — ignore error (baseline may not exist); extract `baselineID` from result or use `""`.

### Task 5.3 — Build output list
1. Initialize `type snapshotSummary struct` (anonymous or named) with fields: `ID`, `Timestamp`, `CommitHash`, `URLCount`, `TotalIssues`, `IsBaseline`.
2. For each manifest in the list:
   - Sum `totalIssues` across all `m.URLs[i].AuditSummary.TotalIssues`.
   - Set `isBaseline = (m.ID == baselineID)`.
   - Append summary to list.

### Task 5.4 — Output
1. Call `output.Success("history", map[string]any{"count": len(summaries), "baseline": baselineID, "snapshots": summaries})`.
2. For empty snapshots: same call with `count: 0`, `snapshots: []` — NOT an error.

---

## Phase 6: Wire Commands into `main.go`

**Sub-tasks:**

### Task 6.1 — Add switch cases
1. Read `/workspace/main.go` to find the `switch` statement.
2. Add two new cases:
   ```go
   case "snapshot":
       cmd.RunSnapshot(cmdArgs)
   case "history":
       cmd.RunHistory(cmdArgs)
   ```
3. Place them logically near other project-level commands.

### Task 6.2 — Update usage string
1. Find the usage/help text in `main.go` (or `cmd/usage.go`).
2. Add a `Project:` section (or append to existing relevant section):
   ```
   Project:
     snapshot                Capture full interface state for all project URLs
     history                 List snapshots in reverse chronological order
   ```

---

## Phase 7: Verification

**Sub-tasks:**

### Task 7.1 — Build verification
1. Run `go build ./...` — must succeed with no errors or warnings.

### Task 7.2 — Test execution
1. Run `go test ./...` — all tests must pass.
2. Check specifically that `snapshot/urldir_test.go`, `snapshot/store_test.go`, `snapshot/baseline_test.go`, and `snapshot/project_test.go` all pass.

### Task 7.3 — Vet and lint
1. Run `go vet ./...` — must produce no output.

---

## File Change Summary

| File | Action | Notes |
|------|--------|-------|
| `snapshot/model.go` | Create | Core types: Manifest, URLEntry, AuditSummary, Baseline, ProjectConfig |
| `snapshot/urldir.go` | Create | URLToDir with path traversal prevention |
| `snapshot/store.go` | Create | SnapshotsDir, SnapshotPath, Save, Load, List |
| `snapshot/project.go` | Create | LoadProjectConfig |
| `snapshot/baseline.go` | Create | BaselinePath, LoadBaseline, SaveBaseline |
| `snapshot/urldir_test.go` | Create | Table-driven tests for URLToDir |
| `snapshot/store_test.go` | Create | Save/Load roundtrip + List sort order |
| `snapshot/baseline_test.go` | Create | Missing file + roundtrip |
| `snapshot/project_test.go` | Create | Missing file + valid config |
| `cmd/breakpoints_common.go` | Create | Shared breakpoint type + defaultBreakpoints |
| `cmd/breakpoints.go` | Modify | Remove duplicate struct/var |
| `cmd/audit_internal.go` | Create | runAudit + runAxTree helpers |
| `cmd/audit.go` | Modify | Thin wrapper calling runAudit |
| `cmd/snapshot.go` | Create | RunSnapshot command |
| `cmd/history.go` | Create | RunHistory command |
| `main.go` | Modify | Add "snapshot" and "history" cases + usage text |

---

## Key Constraints and Design Decisions

1. **No browser dependency in `snapshot/` package** — all browser interaction stays in `cmd/`.
2. **Pure functions for diff-readiness** — `snapshot/` functions take data in, return data out; no side effects beyond filesystem I/O.
3. **Graceful URL failures** — a single unreachable URL records an error in `URLEntry.Error` but does not abort the entire snapshot.
4. **Path traversal prevention** — `URLToDir` must be verified by unit tests against adversarial inputs.
5. **Screenshot format** — pure Go `image/png` via rod's built-in screenshot; no ImageMagick.
6. **Git hash via exec** — `exec.Command("git", "rev-parse", "--short", "HEAD")` with separate args; no shell interpolation.
7. **Baseline auto-promotion** — only if no `baselines.json` exists (nil return from LoadBaseline).
8. **List resilience** — `List()` skips corrupt snapshot.json files with a stderr warning rather than failing entirely.
9. **State dir priority** — `browser.StateDir()` resolves `./.kaleidoscope/` (project-local) first, then `~/.kaleidoscope/`.
10. **File permissions** — directories `0755`, files `0644` (matches existing codebase conventions).
