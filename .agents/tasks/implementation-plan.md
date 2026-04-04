# Implementation Plan: Snapshot Capture and History (US-002)

## Overview

This plan implements `ks snapshot` and `ks history` commands for Kaleidoscope, enabling chronological tracking of interface state across commits. The work spans six new files, two refactored command handlers, and additions to `main.go` and `cmd/usage.go`.

---

## Phase 1: Foundation Packages (No Chrome Dependency)

These are pure-Go packages with no browser dependency. They can be implemented and unit-tested in isolation.

### Task 1.1 — `project` Package

**File:** `project/config.go`

- **Sub-task 1.1.1** — Create `project/` directory and `config.go` file.
- **Sub-task 1.1.2** — Define `Config` struct with `Name string`, `BaseURL string`, `URLs []string` and JSON tags matching `.ks-project.json` schema.
- **Sub-task 1.1.3** — Implement `Load() (*Config, error)`:
  - Read `.ks-project.json` from CWD via `os.ReadFile`.
  - Unmarshal JSON into `Config`.
  - Return descriptive error wrapping `"run 'ks init' first"` if file is missing.
  - Validate `len(c.URLs) > 0`; return error if empty.
- **Sub-task 1.1.4** — Write `project/config_test.go` with table-driven tests:
  - Valid config with `baseURL` and multiple URLs.
  - Missing file returns error with hint.
  - Empty `urls` array returns error.
  - Malformed JSON returns parse error.

---

### Task 1.2 — `gitutil` Package

**File:** `gitutil/gitutil.go`

- **Sub-task 1.2.1** — Create `gitutil/` directory and `gitutil.go` file.
- **Sub-task 1.2.2** — Implement `ShortHash() string`:
  - Run `exec.Command("git", "rev-parse", "--short", "HEAD").Output()`.
  - Return `strings.TrimSpace(string(out))` on success.
  - Return `""` on any error (not in git repo, git not available, no commits).
- **Sub-task 1.2.3** — Write `gitutil/gitutil_test.go`:
  - Test that calling in a git repo with commits returns a non-empty 7-char string.
  - Test behavior is graceful (no panic) when git is unavailable (can mock via `PATH`).

---

### Task 1.3 — `snapshot` Package

**File:** `snapshot/manager.go`

#### Sub-task 1.3.1 — Data Structures

Define all types in `snapshot/manager.go`:

- `type ID = string`
- `Manifest` struct:
  - `ID string` (json: `"id"`)
  - `Timestamp time.Time` (json: `"timestamp"`)
  - `CommitHash string` (json: `"commitHash,omitempty"`)
  - `ProjectURLs []string` (json: `"projectURLs"`)
  - `ProjectName string` (json: `"projectName,omitempty"`)
  - `BaseURL string` (json: `"baseURL,omitempty"`)
  - `URLSummaries []URLSummary` (json: `"urlSummaries"`)
- `URLSummary` struct with fields: `URL`, `Slug`, `ContrastViolations`, `TouchViolations`, `TypographyWarnings`, `AXActiveNodes`, `AXTotalNodes` (all with JSON tags).
- `AuditResult` struct: `URL`, `ContrastViolations`, `TouchViolations`, `TypographyWarnings`, `AXActiveNodes`, `AXTotalNodes`, `TotalIssues`.
- `AXNode` struct: `NodeID`, `Role`, `Name`, `Children []string`, `Properties map[string]any`.
- `BaselinesFile` struct: `DefaultBaseline string`, `URLBaselines map[string]string`.

#### Sub-task 1.3.2 — Path Helpers

Implement path helpers (all create directories as needed):

- `SnapshotsDir() (string, error)`:
  - Use `browser.StateDir()` to locate `.kaleidoscope/`.
  - Append `snapshots/`.
  - Call `os.MkdirAll` with mode `0755`.
  - Return full path.
- `SnapshotPath(id ID) (string, error)`:
  - Call `SnapshotsDir()`.
  - Return `filepath.Join(dir, id)` after creating the directory.
- `URLSlug(rawURL string) string`:
  - Parse with `url.Parse`.
  - Compute `slug = host + path`.
  - Replace all `/` with `_`.
  - Strip non-allowlist characters using `regexp.MustCompile(`[^a-zA-Z0-9._-]`).ReplaceAllString(slug, "_")`.
  - Trim trailing underscores.
  - Enforce max length of 200 chars.
  - Return slug.
- `URLDir(snapshotID ID, slug string) (string, error)`:
  - Call `SnapshotPath(snapshotID)`.
  - Return `filepath.Join(snapshotPath, slug)` after `os.MkdirAll`.

#### Sub-task 1.3.3 — Persistence Functions

- `NewID(hash string) ID`:
  - Format: `fmt.Sprintf("%d-%s", time.Now().Unix(), hash)` when `hash != ""`.
  - Format: `fmt.Sprintf("%d", time.Now().Unix())` when `hash == ""`.
- `WriteManifest(id ID, m *Manifest) error`:
  - Get snapshot path via `SnapshotPath(id)`.
  - Marshal `m` to JSON with `json.MarshalIndent`.
  - Write to `snapshot.json` in the snapshot dir with mode `0644`.
- `ReadManifest(id ID) (*Manifest, error)`:
  - Read `snapshot.json` from `SnapshotPath(id)`.
  - Unmarshal and return.
- `ListIDs() ([]ID, error)`:
  - Call `SnapshotsDir()`.
  - Read directory entries with `os.ReadDir`.
  - Filter to directories only.
  - Sort by name in **descending** order (newest first — works because epoch seconds are zero-padded within the millennium).
  - Return slice of directory names as IDs.
- `ReadBaselines() (*BaselinesFile, error)`:
  - Compute path: `filepath.Join(stateDir, "baselines.json")`.
  - If file does not exist (`os.IsNotExist`), return `nil, nil`.
  - Otherwise read, unmarshal, return.
- `WriteBaselines(b *BaselinesFile) error`:
  - Marshal with `json.MarshalIndent`.
  - Write to `.kaleidoscope/baselines.json` with mode `0644`.

#### Sub-task 1.3.4 — Unit Tests (`snapshot/manager_test.go`)

- `TestURLSlug`: table-driven tests covering:
  - Root path: `https://example.com/` → `example.com_`
  - Subpath: `https://example.com/about` → `example.com_about`
  - Query strings are excluded (only host+path used).
  - Unicode characters replaced with `_`.
  - URLs exceeding 200 chars are truncated.
- `TestNewID`: verify format with hash (`"1234567890-abc1234"`) and without (`"1234567890"`).
- `TestListIDs`: create temp directory, write fake snapshot dirs in random order, verify `ListIDs` returns them newest-first.
- `TestWriteReadManifest`: round-trip write and read of a `Manifest` struct.
- `TestReadBaselinesNoFile`: verify `ReadBaselines` returns `nil, nil` when file absent.
- `TestWriteReadBaselines`: round-trip write and read of `BaselinesFile`.

---

## Phase 2: Internal Audit & AX-Tree Helpers

**File:** `cmd/internal_audit.go`

These helpers extract the audit and AX-tree logic from existing command handlers so `RunSnapshot` can reuse them without calling `output.Success`.

### Task 2.1 — Extract `auditPage` Helper

- **Sub-task 2.1.1** — Read and understand the full body of `RunAudit` in `cmd/audit.go` to identify exactly which logic to extract.
- **Sub-task 2.1.2** — Create `cmd/internal_audit.go` with `auditPage(page *rod.Page) (snapshot.AuditResult, error)`:
  - Copy the JS evaluation logic, contrast checking, touch target checking, and typography checking from `RunAudit`.
  - Return a populated `snapshot.AuditResult` struct instead of calling `output.Success`.
  - Do not include any `output.Success` or `output.Fail` calls.
- **Sub-task 2.1.3** — Refactor `RunAudit` in `cmd/audit.go` to call `auditPage(page)` and wrap the result for `output.Success`. Verify the external behavior (JSON output fields) is unchanged.

### Task 2.2 — Extract `axTreePage` Helper

- **Sub-task 2.2.1** — Read `cmd/axtree.go` to understand the AX-tree extraction logic.
- **Sub-task 2.2.2** — Add `axTreePage(page *rod.Page) ([]snapshot.AXNode, error)` to `cmd/internal_audit.go`:
  - Call `proto.AccessibilityGetFullAXTree{}.Call(page)`.
  - Transform the CDP response into `[]snapshot.AXNode` (nodeId, role, name, children, properties).
  - Return the slice.
- **Sub-task 2.2.3** — Refactor `RunAxTree` in `cmd/axtree.go` to call `axTreePage(page)` and wrap the result for `output.Success`. Verify external behavior is unchanged.

---

## Phase 3: `ks snapshot` Command

**File:** `cmd/snapshot.go`

### Task 3.1 — Scaffold `RunSnapshot`

- **Sub-task 3.1.1** — Create `cmd/snapshot.go` with `func RunSnapshot(args []string)`.
- **Sub-task 3.1.2** — Load project config: call `project.Load()`. On error, call `output.Fail("snapshot", err, "Create a .ks-project.json with 'ks init'")` and `os.Exit(2)`.
- **Sub-task 3.1.3** — Generate snapshot ID: call `gitutil.ShortHash()` then `snapshot.NewID(hash)`.
- **Sub-task 3.1.4** — Create snapshot root directory via `snapshot.SnapshotPath(id)`.

### Task 3.2 — URL Validation

- **Sub-task 3.2.1** — Before processing each URL in `cfg.URLs`, call `url.Parse` and validate scheme is `"http"` or `"https"`. Reject `file://`, `javascript:`, and other schemes.
- **Sub-task 3.2.2** — Resolve relative URLs against `cfg.BaseURL` when `cfg.BaseURL` is set.
- **Sub-task 3.2.3** — If a URL is invalid, record a per-URL error and skip to next URL (do not abort).

### Task 3.3 — Per-URL Capture Loop

Inside `browser.WithPage`, iterate over `cfg.URLs`:

- **Sub-task 3.3.1** — Navigate: `page.Navigate(resolvedURL)` + `page.MustWaitLoad()`. On error, record per-URL error, continue.
- **Sub-task 3.3.2** — Compute `slug := snapshot.URLSlug(resolvedURL)`.
- **Sub-task 3.3.3** — Create URL subdirectory via `snapshot.URLDir(id, slug)`.
- **Sub-task 3.3.4** — Screenshot loop over 4 breakpoints (mobile 375×812, tablet 768×1024, desktop 1280×720, wide 1920×1080):
  - Set viewport: `page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: w, Height: h, DeviceScaleFactor: 1})`.
  - Wait for stability: `page.MustWaitStable()`.
  - Capture: `data, err := page.Screenshot(false, nil)`.
  - Write to `<urlDir>/<name>-<w>x<h>.png` with `os.WriteFile(..., 0644)`.
  - On error, record per-URL error for this breakpoint, continue to next breakpoint.
- **Sub-task 3.3.5** — Restore desktop viewport (1280×720) before audit.
- **Sub-task 3.3.6** — Run audit: call `auditPage(page)`. Write result to `<urlDir>/audit.json` as JSON with `os.WriteFile(..., 0644)`. On error, record per-URL error, continue.
- **Sub-task 3.3.7** — Run AX-tree: call `axTreePage(page)`. Write result to `<urlDir>/ax-tree.json`. On error, record per-URL error, continue.
- **Sub-task 3.3.8** — Build `snapshot.URLSummary` from audit result and append to summaries slice.

### Task 3.4 — Write Manifest

- **Sub-task 3.4.1** — Build `snapshot.Manifest` with: `ID`, `Timestamp` (UTC now), `CommitHash`, `ProjectURLs` (copy of `cfg.URLs`), `ProjectName`, `BaseURL`, `URLSummaries`.
- **Sub-task 3.4.2** — Call `snapshot.WriteManifest(id, &manifest)`. On error, call `output.Fail`.

### Task 3.5 — Baseline Auto-Promotion

- **Sub-task 3.5.1** — Call `snapshot.ReadBaselines()`.
- **Sub-task 3.5.2** — If result is `nil` or `DefaultBaseline == ""`, call `snapshot.WriteBaselines(&BaselinesFile{DefaultBaseline: id})`. Set `baselinePromoted = true`.
- **Sub-task 3.5.3** — Otherwise set `baselinePromoted = false`.

### Task 3.6 — Error Handling and Output

- **Sub-task 3.6.1** — If ALL URLs produced errors (none succeeded), call `output.Fail("snapshot", err, "Check that URLs in .ks-project.json are reachable")` and `os.Exit(2)`.
- **Sub-task 3.6.2** — Otherwise call `output.Success("snapshot", map[string]any{...})` with:
  - `"id"`: snapshot ID string.
  - `"path"`: absolute path to snapshot directory.
  - `"timestamp"`: ISO 8601 UTC timestamp string.
  - `"commitHash"`: git short hash or `""`.
  - `"urlCount"`: number of URLs in config.
  - `"baselinePromoted"`: bool.
  - `"urls"`: slice of per-URL summaries (url, slug, issue counts).
  - `"errors"`: slice of per-URL error objects (url, error string); empty array if none.

---

## Phase 4: `ks history` Command

**File:** `cmd/history.go`

### Task 4.1 — Scaffold `RunHistory`

- **Sub-task 4.1.1** — Create `cmd/history.go` with `func RunHistory(args []string)`.
- **Sub-task 4.1.2** — Parse `--limit N` flag using `getFlagValue(args, "--limit")`. Parse to int; default 0 (no limit). Invalid values silently ignored (treat as no limit).

### Task 4.2 — Load Snapshot List

- **Sub-task 4.2.1** — Call `snapshot.ListIDs()`. On error (e.g., no `.kaleidoscope/` directory), return `output.Success("history", map[string]any{"count": 0, "snapshots": []})`.
- **Sub-task 4.2.2** — If `ids` is empty, return success with empty list (not an error).
- **Sub-task 4.2.3** — Apply `--limit` truncation if limit > 0: `ids = ids[:min(limit, len(ids))]`.

### Task 4.3 — Load Baseline Info

- **Sub-task 4.3.1** — Call `snapshot.ReadBaselines()` to get `BaselinesFile`. If nil (no file), treat `defaultBaseline = ""`.

### Task 4.4 — Build Snapshot Summaries

- **Sub-task 4.4.1** — For each ID in `ids`, call `snapshot.ReadManifest(id)`. If error, skip this ID (don't abort).
- **Sub-task 4.4.2** — From manifest, compute aggregate stats across all URL summaries:
  - `totalContrastViolations`: sum of `URLSummary.ContrastViolations`.
  - `totalTouchViolations`: sum of `URLSummary.TouchViolations`.
  - `totalTypographyWarnings`: sum of `URLSummary.TypographyWarnings`.
  - `totalAXActiveNodes`: sum of `URLSummary.AXActiveNodes`.
- **Sub-task 4.4.3** — Set `isBaseline = (manifest.ID == baselines.DefaultBaseline)`.
- **Sub-task 4.4.4** — Build entry map with: `id`, `timestamp` (ISO 8601), `commitHash`, `isBaseline`, `urlCount` (`len(manifest.URLSummaries)`), and the four total fields.

### Task 4.5 — Output

- **Sub-task 4.5.1** — Call `output.Success("history", map[string]any{"count": len(entries), "snapshots": entries})`.

---

## Phase 5: Integration — Wire Commands into CLI

### Task 5.1 — `main.go` Command Registration

- **Sub-task 5.1.1** — Read `main.go` to identify the switch statement location (around line 80-128).
- **Sub-task 5.1.2** — Add case `"snapshot"` → `cmd.RunSnapshot(cmdArgs)` to the switch.
- **Sub-task 5.1.3** — Add case `"history"` → `cmd.RunHistory(cmdArgs)` to the switch.
- **Sub-task 5.1.4** — Add a `Project:` section to the usage string in `main.go` with entries for `snapshot` and `history`.

### Task 5.2 — `cmd/usage.go` Entries

- **Sub-task 5.2.1** — Add `"snapshot"` entry to `CommandUsage` map with full flag documentation (`--local`).
- **Sub-task 5.2.2** — Add `"history"` entry to `CommandUsage` map with full flag documentation (`--limit N`).

### Task 5.3 — `.gitignore` Additions

- **Sub-task 5.3.1** — Read the existing `.gitignore` (if present).
- **Sub-task 5.3.2** — Add the following lines if not already present:
  ```
  .kaleidoscope/snapshots/
  .kaleidoscope/state.json
  .kaleidoscope/screenshots/
  ```
  Note: `.kaleidoscope/baselines.json` must NOT be gitignored.

---

## Phase 6: Quality Gates

### Task 6.1 — Run Existing Tests

- **Sub-task 6.1.1** — Run `go test ./...` to verify all pre-existing tests still pass after refactoring `RunAudit` and `RunAxTree`.
- **Sub-task 6.1.2** — Fix any compile errors or test failures introduced by the refactor before proceeding.

### Task 6.2 — Run New Unit Tests

- **Sub-task 6.2.1** — Run `go test ./project/...` — all 4 table-driven cases pass.
- **Sub-task 6.2.2** — Run `go test ./gitutil/...` — graceful behavior verified.
- **Sub-task 6.2.3** — Run `go test ./snapshot/...` — all table-driven tests for `URLSlug`, `NewID`, `ListIDs`, `WriteManifest`/`ReadManifest`, `ReadBaselines`/`WriteBaselines` pass.

### Task 6.3 — Build Verification

- **Sub-task 6.3.1** — Run `go build ./...` to verify the binary compiles cleanly with no unused imports or undefined references.

---

## File Summary

| File | Action | Phase |
|------|--------|-------|
| `project/config.go` | Create | 1.1 |
| `project/config_test.go` | Create | 1.1 |
| `gitutil/gitutil.go` | Create | 1.2 |
| `gitutil/gitutil_test.go` | Create | 1.2 |
| `snapshot/manager.go` | Create | 1.3 |
| `snapshot/manager_test.go` | Create | 1.3 |
| `cmd/internal_audit.go` | Create | 2 |
| `cmd/audit.go` | Refactor | 2.1 |
| `cmd/axtree.go` | Refactor | 2.2 |
| `cmd/snapshot.go` | Create | 3 |
| `cmd/history.go` | Create | 4 |
| `main.go` | Modify | 5.1 |
| `cmd/usage.go` | Modify | 5.2 |
| `.gitignore` | Modify | 5.3 |

---

## Dependency Order

```
Phase 1 (project, gitutil, snapshot packages)
    └── Phase 2 (internal_audit.go — depends on snapshot types)
            └── Phase 3 (cmd/snapshot.go — depends on all of the above)
Phase 4 (cmd/history.go — depends on snapshot package only)
Phase 5 (main.go wiring — depends on Phase 3 and 4)
Phase 6 (tests — verify everything)
```

Phases 1 and 4 are independent and can be implemented in parallel. Phase 2 requires Phase 1 (for `snapshot.AuditResult` / `snapshot.AXNode` types). Phase 3 requires Phase 1 and 2. Phase 5 requires Phase 3 and 4.
