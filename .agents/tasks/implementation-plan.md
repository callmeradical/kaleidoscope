# Implementation Plan: Snapshot Capture and History (US-002)

## Overview

Introduces `ks snapshot` and `ks history` commands. A snapshot captures screenshots at 4 breakpoints, an accessibility tree dump, and a full audit result for every URL defined in `.ks-project.json`. On first run the snapshot is auto-promoted as the baseline in `.kaleidoscope/baselines.json`.

**Module:** `github.com/callmeradical/kaleidoscope`
**Quality gate:** `go test ./...`
**Depends on:** US-001 (`.ks-project.json` project config)

---

## Phase 1 — Snapshot Package Foundation

> Pure data and I/O helpers. No Chrome dependency, no cmd dependency. Implement and test these first so that later phases can import them.

### Task 1.1 — `snapshot/urlkey.go`

Converts a raw URL to a filesystem-safe directory name used inside each snapshot.

**Sub-tasks:**
1. Create directory `snapshot/` at the repo root.
2. Create file `snapshot/urlkey.go` with `package snapshot`.
3. Implement `URLToKey(rawURL string) string`:
   - Parse the URL using `net/url`.
   - Strip scheme (`https://`, `http://`).
   - Drop query string and fragment.
   - Replace all `/` characters with `-`.
   - Collapse consecutive `-` into a single `-`.
   - Strip any leading or trailing `-`.
   - Remove any `..` path components that survived sanitization.
   - Truncate to 128 characters.
   - Return the resulting string (always a flat name with no path separators).

### Task 1.2 — `snapshot/urlkey_test.go`

**Sub-tasks:**
1. Create `snapshot/urlkey_test.go`.
2. Write table-driven tests covering:
   - Root URL: `https://example.com/` → `"example.com"`
   - Deep path: `https://example.com/about/team` → `"example.com-about-team"`
   - Query string stripped: `https://example.com/about?foo=bar` → `"example.com-about"`
   - Fragment stripped: `https://example.com/page#section` → `"example.com-page"`
   - Path traversal blocked: `https://example.com/../secret` → no `..` in output
   - Special characters replaced: spaces, `#`, `?` in the path
   - Long URL truncated to ≤ 128 chars
   - Consecutive slashes/dashes collapsed

---

### Task 1.3 — `snapshot/git.go`

Returns the 7-char short commit hash of HEAD; gracefully falls back to `""`.

**Sub-tasks:**
1. Create `snapshot/git.go` with `package snapshot`.
2. Implement `ShortCommitHash() string`:
   - Run `exec.Command("git", "rev-parse", "--short", "HEAD")`.
   - Capture stdout, trim whitespace.
   - On any error (not a git repo, no git binary, non-zero exit) return `""`.
   - Do NOT return any error; the function is always safe to call.

### Task 1.4 — `snapshot/git_test.go`

**Sub-tasks:**
1. Create `snapshot/git_test.go`.
2. Write test `TestShortCommitHash_InRepo`:
   - Working directory is `/workspace` (a git repo).
   - Assert returned string is non-empty and 7 chars long.
3. Write test `TestShortCommitHash_NotRepo`:
   - Create a temporary directory with `os.MkdirTemp`.
   - `os.Chdir` into it (restore with `t.Cleanup`).
   - Assert returned string is `""`.

---

### Task 1.5 — `snapshot/snapshot.go`

Data types only; no I/O. Referenced by all other snapshot sub-packages.

**Sub-tasks:**
1. Create `snapshot/snapshot.go` with `package snapshot`.
2. Define `type SnapshotID = string`.
3. Define `Manifest` struct:
   ```go
   type Manifest struct {
       ID            SnapshotID    `json:"id"`
       Timestamp     time.Time     `json:"timestamp"`
       CommitHash    string        `json:"commitHash,omitempty"`
       ProjectConfig interface{}   `json:"projectConfig"`
       URLs          []URLEntry    `json:"urls"`
       Summary       Summary       `json:"summary"`
   }
   ```
4. Define `URLEntry` struct:
   ```go
   type URLEntry struct {
       URL         string    `json:"url"`
       Dir         string    `json:"dir"`
       TotalIssues int       `json:"totalIssues"`
       AXNodeCount int       `json:"axNodeCount"`
       Breakpoints int       `json:"breakpoints"`
       CapturedAt  time.Time `json:"capturedAt"`
       Reachable   bool      `json:"reachable"`
       Error       string    `json:"error,omitempty"`
   }
   ```
5. Define `Summary` struct:
   ```go
   type Summary struct {
       TotalURLs     int `json:"totalURLs"`
       ReachableURLs int `json:"reachableURLs"`
       TotalIssues   int `json:"totalIssues"`
       TotalAXNodes  int `json:"totalAXNodes"`
   }
   ```
6. Define `BaselineManifest` struct:
   ```go
   type BaselineManifest struct {
       BaselineID SnapshotID `json:"baselineId"`
       SetAt      time.Time  `json:"setAt"`
       CommitHash string     `json:"commitHash,omitempty"`
   }
   ```

---

### Task 1.6 — `snapshot/store.go`

All disk I/O for snapshot directories and manifest files.

**Sub-tasks:**
1. Create `snapshot/store.go` with `package snapshot`.
2. Add constant `snapshotsDir = ".kaleidoscope/snapshots"` and `baselineFile = ".kaleidoscope/baselines.json"`.
3. Implement `SnapshotsDir() (string, error)`:
   - Compute absolute path: `filepath.Join(cwd, ".kaleidoscope/snapshots")`.
   - Call `os.MkdirAll(path, 0755)`.
   - Return the path.
4. Implement `SnapshotPath(id SnapshotID) (string, error)`:
   - Call `SnapshotsDir()`.
   - Return `filepath.Join(dir, id)` (does not create directory).
5. Implement `URLDir(snapshotPath, rawURL string) (string, error)`:
   - Compute key via `URLToKey(rawURL)`.
   - Compute full path: `filepath.Join(snapshotPath, key)`.
   - Call `os.MkdirAll(path, 0755)`.
   - Return the path.
6. Implement `WriteManifest(snapshotPath string, m *Manifest) error`:
   - Marshal `m` to indented JSON.
   - Write to `filepath.Join(snapshotPath, "snapshot.json")` with mode `0644`.
7. Implement `ReadManifest(snapshotPath string) (*Manifest, error)`:
   - Read `filepath.Join(snapshotPath, "snapshot.json")`.
   - Unmarshal JSON into `*Manifest` and return.
8. Implement `ListSnapshotIDs() ([]SnapshotID, error)`:
   - Call `SnapshotsDir()`.
   - Read directory entries with `os.ReadDir`.
   - Collect entry names that are directories (skip files).
   - Sort descending (newest first) — lexicographic reverse on the ISO timestamp prefix.
   - Return the sorted slice.
9. Implement `ReadBaselineManifest() (*BaselineManifest, error)`:
   - Compute path: `filepath.Join(cwd, ".kaleidoscope/baselines.json")`.
   - If file does not exist (`os.IsNotExist`), return `nil, nil`.
   - Unmarshal JSON into `*BaselineManifest` and return.
10. Implement `WriteBaselineManifest(b *BaselineManifest) error`:
    - Ensure `.kaleidoscope/` directory exists via `os.MkdirAll`.
    - Marshal `b` to indented JSON.
    - Write to `filepath.Join(cwd, ".kaleidoscope/baselines.json")` with mode `0644`.

### Task 1.7 — `snapshot/store_test.go`

**Sub-tasks:**
1. Create `snapshot/store_test.go`.
2. For each test, use `t.TempDir()` and `os.Chdir` (restored with `t.Cleanup`) to isolate filesystem state.
3. Write `TestSnapshotPath`: verify returned path is `<tmpdir>/.kaleidoscope/snapshots/<id>`.
4. Write `TestURLDir`: verify directory is created and path ends with the sanitized key.
5. Write `TestWriteReadManifest` (round-trip):
   - Create a `Manifest` with known values.
   - Call `WriteManifest`, then `ReadManifest`.
   - Assert all fields are equal.
6. Write `TestListSnapshotIDs_Order`:
   - Create three snapshot directories with lexicographically ordered names.
   - Assert `ListSnapshotIDs` returns them newest-first.
7. Write `TestBaselineRoundTrip`:
   - Call `WriteBaselineManifest`, then `ReadBaselineManifest`.
   - Assert `BaselineID` and `CommitHash` match.
8. Write `TestReadBaselineManifest_Missing`: assert returns `nil, nil` when file absent.

---

## Phase 2 — Project Config

> Defines how to load `.ks-project.json`. Coordinate with US-001 owner — if the struct and function already exist in `cmd/project.go`, skip this phase and import from there.

### Task 2.1 — `cmd/project.go`

**Sub-tasks:**
1. Create `cmd/project.go` with `package cmd`.
2. Define `ProjectConfig` struct:
   ```go
   type ProjectConfig struct {
       Version int      `json:"version"`
       URLs    []string `json:"urls"`
   }
   ```
3. Implement `ReadProjectConfig() (*ProjectConfig, error)`:
   - Read `.ks-project.json` from the current working directory.
   - If the file does not exist, return a descriptive error: `".ks-project.json not found in current directory"`.
   - Unmarshal JSON; if malformed, return a descriptive error.
   - Return the parsed config.

---

## Phase 3 — Refactor Existing Commands

> Extract unexported internal helpers from three existing commands so that `cmd/snapshot.go` can reuse them without duplication. The public `Run*` functions must remain unchanged in behavior and signature.

### Task 3.1 — Refactor `cmd/breakpoints.go`

**Sub-tasks:**
1. Read the current implementation of `RunBreakpoints` in `cmd/breakpoints.go`.
2. Extract the core capture loop into a new unexported function:
   ```go
   // captureBreakpointsToDir saves 4 breakpoint PNGs to destDir, returns metadata.
   func captureBreakpointsToDir(page *rod.Page, destDir string) ([]map[string]any, error)
   ```
   - The function sets each viewport, waits for layout to settle, takes a screenshot, and writes a PNG named `<breakpoint-name>.png` (e.g., `mobile.png`, `tablet.png`, `desktop.png`, `wide.png`) into `destDir`.
   - Returns a slice of `map[string]any` with keys `"breakpoint"`, `"width"`, `"height"`, `"path"`.
   - Restores original viewport before returning.
3. Rewrite `RunBreakpoints` as a thin wrapper:
   - Resolve `destDir` via `browser.ScreenshotDir()` (with timestamped sub-path as before for backwards compatibility).
   - Call `captureBreakpointsToDir(page, destDir)`.
   - Call `output.Success("breakpoints", ...)` with the results.
4. Verify `RunBreakpoints` output shape is identical to the pre-refactor version.

---

### Task 3.2 — Refactor `cmd/audit.go`

**Sub-tasks:**
1. Read the current implementation of `RunAudit` in `cmd/audit.go`.
2. Extract the data-gathering logic into a new unexported function:
   ```go
   // gatherAuditData runs all audit checks on the current page and returns the result map.
   // Does NOT call output.Success or os.Exit.
   func gatherAuditData(page *rod.Page, selector string) (map[string]any, error)
   ```
   - Move all JS evaluation, contrast checking, touch target checking, and typography checking into this function.
   - Return the assembled `map[string]any` (same shape as the `result` field in the existing `output.Success` call).
3. Rewrite `RunAudit` as a thin wrapper:
   - Call `gatherAuditData(page, selector)`.
   - On success, call `output.Success("audit", result)`.
   - On error, call `output.Fail` and `os.Exit(2)` as before.
4. Verify `RunAudit` output shape is identical to the pre-refactor version.

---

### Task 3.3 — Refactor `cmd/axtree.go`

**Sub-tasks:**
1. Read the current implementation of `RunAxTree` in `cmd/axtree.go`.
2. Extract the data-gathering logic into a new unexported function:
   ```go
   // gatherAxTreeData dumps the full ax-tree from the current page and returns the result map.
   // Does NOT call output.Success or os.Exit.
   func gatherAxTreeData(page *rod.Page) (map[string]any, error)
   ```
   - Move the CDP call, node iteration, and result assembly into this function.
   - Return `map[string]any{"nodeCount": len(nodes), "nodes": nodes}`.
3. Rewrite `RunAxTree` as a thin wrapper:
   - Call `gatherAxTreeData(page)`.
   - On success, call `output.Success("ax-tree", result)`.
   - On error, call `output.Fail` and `os.Exit(2)` as before.
4. Verify `RunAxTree` output shape is identical to the pre-refactor version.

---

## Phase 4 — New Commands

### Task 4.1 — `cmd/snapshot.go`

**Sub-tasks:**
1. Create `cmd/snapshot.go` with `package cmd`.
2. Add imports: `snapshot` package, `browser`, `output`, `encoding/json`, `os`, `path/filepath`, `time`.
3. Implement `RunSnapshot(args []string)`:
   a. **Load project config:** call `ReadProjectConfig()`. On error, call `output.Fail` with hint `"Run: echo '{\"version\":1,\"urls\":[\"https://...\"]}' > .ks-project.json"` and `os.Exit(2)`.
   b. **Validate URLs:** if `len(config.URLs) == 0`, call `output.Fail` with hint `"Add at least one URL to .ks-project.json"` and `os.Exit(2)`.
   c. **Reject non-http(s) URLs:** iterate config.URLs; for any URL whose scheme is not `http` or `https`, add a URLEntry with `Reachable: false` and `Error: "unsupported scheme"`.
   d. **Build snapshot ID:** format current time as `"20060102T150405Z"` (UTC); append `-<short-hash>` from `snapshot.ShortCommitHash()` if non-empty. Result: `"20060102T150405Z-abc1234"` or `"20060102T150405Z"`.
   e. **Create snapshot root dir:** call `snapshot.SnapshotPath(id)`, then `os.MkdirAll(snapshotPath, 0755)`.
   f. **For each URL** in `config.URLs`:
      - Call `snapshot.URLDir(snapshotPath, url)` to get/create the URL subdirectory.
      - Inside `browser.WithPage`:
        - Navigate to the URL via `page.Navigate(url)` and `page.WaitLoad()`.
        - On navigate error: record `URLEntry{Reachable: false, Error: err.Error()}`, continue.
        - Call `captureBreakpointsToDir(page, urlDir)` — saves 4 PNGs, returns metadata.
        - Call `gatherAuditData(page, "")` — returns audit result map.
        - Call `gatherAxTreeData(page)` — returns ax-tree result map.
        - Write `audit.json`: marshal audit result to `filepath.Join(urlDir, "audit.json")` with mode `0644`.
        - Write `ax-tree.json`: marshal ax-tree result to `filepath.Join(urlDir, "ax-tree.json")` with mode `0644`.
        - Extract `totalIssues` from audit result summary.
        - Extract `nodeCount` from ax-tree result.
        - Record `URLEntry{URL: url, Dir: key, TotalIssues: n, AXNodeCount: m, Breakpoints: 4, CapturedAt: time.Now().UTC(), Reachable: true}`.
      - If `browser.WithPage` itself fails (browser not running): call `output.Fail` with hint `"Is the browser running? Run: ks start"` and `os.Exit(2)`.
   g. **Compute summary:** iterate URLEntries, sum issues, AX nodes, count reachable.
   h. **Assemble and write Manifest:**
      - Set `ID`, `Timestamp` (UTC now), `CommitHash`, `ProjectConfig` (verbatim `config`), `URLs`, `Summary`.
      - Call `snapshot.WriteManifest(snapshotPath, &manifest)`.
   i. **Auto-promote to baseline:** call `snapshot.ReadBaselineManifest()`. If result is `nil` (no existing baseline), call `snapshot.WriteBaselineManifest(&BaselineManifest{BaselineID: id, SetAt: time.Now().UTC(), CommitHash: shortHash})`.
   j. **Output success:**
      ```json
      {
        "ok": true,
        "command": "snapshot",
        "result": {
          "id": "<id>",
          "path": "<relative-path>",
          "urls": <total>,
          "reachable": <reachable-count>,
          "totalIssues": <n>,
          "promotedToBaseline": <bool>
        }
      }
      ```
4. Exit code 0 on success (even if some URLs were unreachable). Exit code 2 on fatal errors only.

---

### Task 4.2 — `cmd/history.go`

**Sub-tasks:**
1. Create `cmd/history.go` with `package cmd`.
2. Add imports: `snapshot` package, `output`.
3. Implement `RunHistory(args []string)`:
   a. Call `snapshot.ListSnapshotIDs()`. On error, call `output.Fail` and `os.Exit(2)`.
   b. If the list is empty (or directory doesn't exist), return success with `{"snapshots": [], "baselineId": ""}` — do NOT error.
   c. Call `snapshot.ReadBaselineManifest()` to get the current `baselineId` (empty string if nil).
   d. For each snapshot ID, call `snapshot.ReadManifest(snapshotPath)`:
      - On error reading a specific manifest: include the ID with `null` summary fields and continue.
      - On success: extract `timestamp`, `commitHash`, `summary.totalURLs`, `summary.reachableURLs`, `summary.totalIssues`, `summary.totalAXNodes`.
      - Set `isBaseline: id == baselineId`.
   e. Call `output.Success("history", result)` where result is:
      ```json
      {
        "snapshots": [
          {
            "id": "...",
            "timestamp": "...",
            "commitHash": "...",
            "isBaseline": true,
            "urls": 3,
            "reachableUrls": 3,
            "totalIssues": 12,
            "totalAXNodes": 148
          }
        ],
        "baselineId": "..."
      }
      ```

---

## Phase 5 — Wiring and Repository Integration

### Task 5.1 — Update `main.go`

**Sub-tasks:**
1. Read `main.go`.
2. Add two new `case` statements to the `switch` block:
   ```go
   case "snapshot":
       cmd.RunSnapshot(cmdArgs)
   case "history":
       cmd.RunHistory(cmdArgs)
   ```
3. Add a new section to the `usage` string:
   ```
   Snapshot & History:
     snapshot               Capture full interface state for all project URLs
     history                List snapshots in reverse chronological order
   ```
   Insert this section after the existing `UX Evaluation:` block and before `Design System Catalog:`.

---

### Task 5.2 — Update `.gitignore`

**Sub-tasks:**
1. Check if a `.gitignore` exists at the repo root.
2. If it exists, append the following lines (only if not already present):
   ```
   .kaleidoscope/snapshots/
   .kaleidoscope/state.json
   .kaleidoscope/screenshots/
   ```
3. If no `.gitignore` exists, create one with those three lines.
4. Note: `.kaleidoscope/baselines.json` is intentionally NOT gitignored — it is committed to the repo.

---

## Phase 6 — Quality Gate

### Task 6.1 — Run `go test ./...`

**Sub-tasks:**
1. Run `go build ./...` first to catch compile errors.
2. Run `go test ./...` and capture output.
3. If any tests fail:
   - Read the failing test file and the source file it tests.
   - Fix the root cause (type mismatch, logic error, missing import, etc.).
   - Re-run `go test ./...` until all tests pass.
4. If `go vet ./...` produces warnings, fix them.

---

## File Manifest

| File | Action | Notes |
|------|--------|-------|
| `snapshot/urlkey.go` | **Create** | URL → filesystem key |
| `snapshot/urlkey_test.go` | **Create** | Edge-case tests |
| `snapshot/git.go` | **Create** | Short commit hash helper |
| `snapshot/git_test.go` | **Create** | In-repo and non-repo tests |
| `snapshot/snapshot.go` | **Create** | Data types: Manifest, URLEntry, Summary, BaselineManifest |
| `snapshot/store.go` | **Create** | Disk I/O: dirs, manifests, baseline |
| `snapshot/store_test.go` | **Create** | Round-trip and ordering tests |
| `cmd/project.go` | **Create** | ProjectConfig + ReadProjectConfig (if US-001 hasn't created it) |
| `cmd/breakpoints.go` | **Modify** | Extract `captureBreakpointsToDir`; `RunBreakpoints` becomes wrapper |
| `cmd/audit.go` | **Modify** | Extract `gatherAuditData`; `RunAudit` becomes wrapper |
| `cmd/axtree.go` | **Modify** | Extract `gatherAxTreeData`; `RunAxTree` becomes wrapper |
| `cmd/snapshot.go` | **Create** | `RunSnapshot` — main orchestrator |
| `cmd/history.go` | **Create** | `RunHistory` — lists snapshots |
| `main.go` | **Modify** | Add `snapshot` and `history` cases + usage entries |
| `.gitignore` | **Create/Modify** | Gitignore `.kaleidoscope/snapshots/` and friends |

---

## Implementation Order

Follow this order to avoid forward-reference compile errors:

1. `snapshot/urlkey.go` + `snapshot/urlkey_test.go`
2. `snapshot/git.go` + `snapshot/git_test.go`
3. `snapshot/snapshot.go` (types only — no deps)
4. `snapshot/store.go` + `snapshot/store_test.go`
5. `cmd/project.go` (coordinate with US-001 owner)
6. `cmd/breakpoints.go` refactor (extract `captureBreakpointsToDir`)
7. `cmd/audit.go` refactor (extract `gatherAuditData`)
8. `cmd/axtree.go` refactor (extract `gatherAxTreeData`)
9. `cmd/snapshot.go` (depends on all above)
10. `cmd/history.go` (depends on `snapshot/store`)
11. `main.go` (wire up new commands)
12. `.gitignore` update
13. `go test ./...`

---

## Key Constraints and Rules

- All commands use `output.Success` / `output.Fail` JSON convention — no direct `fmt.Println` in new code.
- No external dependencies beyond the existing `go.mod` (uses only `os`, `encoding/json`, `path/filepath`, `net/url`, `os/exec`, `time`, `sort`).
- Screenshot diffing is pure Go (`image`/`image/png`) — no ImageMagick.
- `.ks-project.json` is committed; `.kaleidoscope/snapshots/` is gitignored; `.kaleidoscope/baselines.json` is committed.
- No integration tests (require a running browser) — unit tests only, consistent with existing test surface.
- `URLToKey` must never produce a path with `..` or leading/trailing path separators.
- `ShortCommitHash` must never return an error; always safe to call.
- `RunSnapshot` exits 0 even if individual URLs are unreachable (graceful degradation).
- File permissions: `0755` for directories, `0644` for files.
