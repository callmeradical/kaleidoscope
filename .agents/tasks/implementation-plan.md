# Implementation Plan: US-002 — Snapshot Capture and History

**Story**: As an AI agent, I want to automatically capture full interface state for every URL in a project, so that I can compare state before and after code changes.

**Depends on**: US-001 (project config, `.ks-project.json`)

---

## Phase 1: Project Config Package (`project/`)

Introduce the `project` package that reads `.ks-project.json`. US-002 only reads this file; US-001 writes it. A minimal stub implementation is sufficient if US-001 is not yet merged.

### Task 1.1 — Create `project/config.go`

- **Sub-task 1.1.1**: Define `ProjectConfig` struct with fields `Name string`, `URLs []string`, `Breakpoints []string` (with JSON tags matching `.ks-project.json`).
- **Sub-task 1.1.2**: Implement `LoadConfig(path string) (*ProjectConfig, error)` — reads and JSON-decodes the file at `path`.
- **Sub-task 1.1.3**: Implement `FindConfig() (*ProjectConfig, error)` — walks up from `os.Getwd()`, stopping at filesystem root or first `.git` directory; returns `(nil, nil)` if not found (no error).

### Task 1.2 — Write unit tests for `project/config.go`

- **Sub-task 1.2.1**: Test `LoadConfig` with a valid JSON fixture — assert all fields parse correctly.
- **Sub-task 1.2.2**: Test `LoadConfig` with a missing file — assert error is returned.
- **Sub-task 1.2.3**: Test `FindConfig` when `.ks-project.json` exists in CWD — assert non-nil result.
- **Sub-task 1.2.4**: Test `FindConfig` when no config file is found — assert `(nil, nil)` is returned.

---

## Phase 2: Snapshot Data Model (`snapshot/model.go`)

Define the in-memory data structures for snapshot manifests. No I/O in this file.

### Task 2.1 — Create `snapshot/model.go`

- **Sub-task 2.1.1**: Define `AuditSummary` struct (`TotalIssues`, `ContrastViolations`, `TouchViolations`, `TypographyWarnings int`) with JSON tags.
- **Sub-task 2.1.2**: Define `URLEntry` struct (`URL`, `Dir string`, `AuditSummary AuditSummary`, `AxNodeCount int`, `Screenshots []string`, `Error string`) with JSON tags (`omitempty` on `Error`).
- **Sub-task 2.1.3**: Define `ProjectConfig` snapshot-local struct (`Name string`, `URLs []string`, `Breakpoints []string`) — a snapshot-in-time copy, separate from `project.ProjectConfig`.
- **Sub-task 2.1.4**: Define `Manifest` struct (`ID`, `CommitHash string`, `Timestamp time.Time`, `ProjectConfig ProjectConfig`, `URLs []URLEntry`) with JSON tags (`omitempty` on `CommitHash`).

---

## Phase 3: Snapshot Store (`snapshot/store.go`)

Filesystem persistence layer for snapshot directories and manifests.

### Task 3.1 — Create `snapshot/store.go`

- **Sub-task 3.1.1**: Implement `SnapshotsDir() (string, error)` — calls `browser.StateDir()` (project-local `.kaleidoscope/`), appends `"snapshots"`, creates directory with `os.MkdirAll(…, 0755)`, returns path.
- **Sub-task 3.1.2**: Implement `GenerateID() string` — gets `time.Now().UnixMilli()` for prefix; runs `exec.Command("git", "rev-parse", "--short", "HEAD")` with a 2-second timeout; if command succeeds, appends `-<hash>`; if it fails, uses timestamp-only format.
- **Sub-task 3.1.3**: Implement `CreateDir(id string) (string, error)` — joins `SnapshotsDir()` with `id`, calls `os.MkdirAll`, returns the resulting path.
- **Sub-task 3.1.4**: Implement `URLDir(rawURL string) string` — parses URL with `url.Parse`, takes only the path component (strips query/fragment), strips leading `/`, replaces all characters not matching `[a-zA-Z0-9_-]` with `_`, trims leading/trailing underscores, collapses consecutive underscores, returns `"root"` if result is empty (handles `/` path).
- **Sub-task 3.1.5**: Implement `WriteManifest(snapshotDir string, m *Manifest) error` — JSON-encodes `m` with `json.MarshalIndent`, writes to `<snapshotDir>/snapshot.json` with permissions `0644`.
- **Sub-task 3.1.6**: Implement `ReadManifest(snapshotDir string) (*Manifest, error)` — reads and JSON-decodes `<snapshotDir>/snapshot.json`.
- **Sub-task 3.1.7**: Implement `List() ([]*Manifest, error)` — reads `SnapshotsDir()`, iterates subdirectories, calls `ReadManifest` on each (skipping non-directory entries and entries without `snapshot.json`), sorts results by `Manifest.Timestamp` descending using `sort.Slice`, returns the slice.

### Task 3.2 — Write unit tests for `snapshot/store.go`

- **Sub-task 3.2.1**: Test `URLDir` with `/` → `"root"`.
- **Sub-task 3.2.2**: Test `URLDir` with `/about` → `"about"`.
- **Sub-task 3.2.3**: Test `URLDir` with `/products/items` → `"products_items"`.
- **Sub-task 3.2.4**: Test `URLDir` with query strings (e.g., `/page?q=1`) → query stripped, result `"page"`.
- **Sub-task 3.2.5**: Test `URLDir` with special characters in path → all non-`[a-zA-Z0-9_-]` become `_`, no leading/trailing underscores.
- **Sub-task 3.2.6**: Test `GenerateID` format — assert timestamp prefix is numeric string of length ≥13, optional `-` hash suffix is alphanumeric.
- **Sub-task 3.2.7**: Test `WriteManifest` / `ReadManifest` round-trip — write a `Manifest`, read it back, assert all fields match (using `t.TempDir()` for isolation).
- **Sub-task 3.2.8**: Test `List` sort order — write two manifests with different timestamps, assert `List()` returns them in descending order.
- **Sub-task 3.2.9**: Test `List` with empty directory — assert returns empty slice and no error.

---

## Phase 4: Baseline Manager (`snapshot/baseline.go`)

Manages `.kaleidoscope/baselines.json` for tracking which snapshot is the current baseline.

### Task 4.1 — Create `snapshot/baseline.go`

- **Sub-task 4.1.1**: Define `Baseline` struct (`SnapshotID string`, `PromotedAt time.Time`, `PromotedBy string`) with JSON tags.
- **Sub-task 4.1.2**: Implement `BaselinePath() (string, error)` — calls `browser.StateDir()` for the project-local `.kaleidoscope/` directory, returns `<stateDir>/baselines.json`. Must use project-local state only (not `~/.kaleidoscope/`).
- **Sub-task 4.1.3**: Implement `ReadBaseline() (*Baseline, error)` — gets `BaselinePath()`, returns `(nil, nil)` if file does not exist (`os.IsNotExist`), otherwise JSON-decodes and returns.
- **Sub-task 4.1.4**: Implement `WriteBaseline(b *Baseline) error` — JSON-encodes `b` with `json.MarshalIndent`, writes to `BaselinePath()` with permissions `0644`.
- **Sub-task 4.1.5**: Implement `EnsureBaseline(snapshotID string) (bool, error)` — calls `ReadBaseline()`; if baseline already exists, returns `(false, nil)` without modification; if not found, creates a new `Baseline{SnapshotID: snapshotID, PromotedAt: time.Now(), PromotedBy: "auto"}` and calls `WriteBaseline`, returns `(true, nil)`.

### Task 4.2 — Write unit tests for `snapshot/baseline.go`

- **Sub-task 4.2.1**: Test `ReadBaseline` when no file exists — assert `(nil, nil)`.
- **Sub-task 4.2.2**: Test `WriteBaseline` / `ReadBaseline` round-trip — write, read back, assert all fields equal.
- **Sub-task 4.2.3**: Test `EnsureBaseline` idempotency — call twice with different IDs; assert second call returns `false` and does NOT overwrite the first baseline.
- **Sub-task 4.2.4**: Test `EnsureBaseline` on first run — assert returns `true` and `baselines.json` is created with `PromotedBy: "auto"`.

---

## Phase 5: `browser/state.go` — Add `SnapshotsDir` Helper

### Task 5.1 — Extend `browser/state.go`

- **Sub-task 5.1.1**: Add `SnapshotsDir() (string, error)` function — calls `StateDir()`, appends `"snapshots"` to the result, creates directory with `os.MkdirAll(ssDir, 0755)`, returns the path. This is the authoritative function that `snapshot.SnapshotsDir()` delegates to.

> **Note**: Decide during implementation whether `snapshot.SnapshotsDir()` calls `browser.StateDir()` directly or calls `browser.SnapshotsDir()`. Either is acceptable; avoid creating a circular import.

---

## Phase 6: Internal Capture Helpers (`cmd/capture_helpers.go`)

Refactor existing inline command logic into reusable, data-returning helper functions. **Zero functional change** to existing `Run*` commands.

### Task 6.1 — Audit existing commands to locate inline logic

- **Sub-task 6.1.1**: Read `cmd/audit.go` — identify the JavaScript evaluation, data extraction, and summary-building logic that produces the audit result.
- **Sub-task 6.1.2**: Read `cmd/axtree.go` — identify the CDP `AccessibilityGetFullAXTree` call, node filtering, and count logic.
- **Sub-task 6.1.3**: Read `cmd/breakpoints.go` — identify the viewport iteration, screenshot capture, and file-writing logic.

### Task 6.2 — Create `cmd/capture_helpers.go`

- **Sub-task 6.2.1**: Extract `captureAuditData(page *rod.Page) (summary snapshot.AuditSummary, raw map[string]any, err error)` — move all JS evaluation, analysis, and summary construction out of `RunAudit`; return `AuditSummary` (counts only) and full raw result map. Do not call `output.Success` inside this helper.
- **Sub-task 6.2.2**: Extract `captureAxTreeData(page *rod.Page) (nodes []map[string]any, nodeCount int, err error)` — move CDP tree query and node filtering out of `RunAxTree`; return the node slice and count. Do not call `output.Success` inside this helper.
- **Sub-task 6.2.3**: Extract `captureBreakpointsData(page *rod.Page, destDir string) (filenames []string, err error)` — move breakpoint iteration, viewport resize, screenshot capture, and file writing out of `RunBreakpoints`; write PNGs to `destDir` (named `mobile.png`, `tablet.png`, `desktop.png`, `wide.png`); restore original viewport after completion; return slice of written filenames. Do not call `output.Success` inside this helper.

### Task 6.3 — Update existing `Run*` commands to delegate to helpers

- **Sub-task 6.3.1**: Update `cmd/audit.go` — replace inline logic with a call to `captureAuditData`; pass raw result to `output.Success("audit", raw)`. Assert output is identical to pre-refactor.
- **Sub-task 6.3.2**: Update `cmd/axtree.go` — replace inline logic with a call to `captureAxTreeData`; reconstruct the output map that `output.Success` receives. Assert output is identical to pre-refactor.
- **Sub-task 6.3.3**: Update `cmd/breakpoints.go` — replace inline logic with a call to `captureBreakpointsData`; pass filenames into `output.Success`. Assert output structure is identical to pre-refactor.

> **Risk**: These refactors must be behavioral no-ops. Verify by running `go test ./...` after each individual file change.

---

## Phase 7: URL Validation (Security)

Add input validation before any URL is used for navigation, per the security requirements in the tech spec.

### Task 7.1 — Add `validateURL` helper in `cmd/capture_helpers.go` or a new `cmd/util_url.go`

- **Sub-task 7.1.1**: Implement `validateURL(rawURL string) error` — calls `url.Parse(rawURL)`; rejects any URL whose scheme is not `"http"` or `"https"` (i.e., rejects `file://`, `javascript:`, empty scheme, etc.); returns a descriptive error.
- **Sub-task 7.1.2**: Call `validateURL` in `RunSnapshot` before each URL is passed to `page.Navigate`.

---

## Phase 8: `ks snapshot` Command (`cmd/snapshot.go`)

Orchestrate the full capture loop for all project URLs.

### Task 8.1 — Create `cmd/snapshot.go`

- **Sub-task 8.1.1**: Implement `RunSnapshot(args []string)`:
  1. Parse optional `--full-page` flag from `args`.
  2. Call `project.FindConfig()` — if `cfg` is `nil`, call `output.Fail("snapshot", err, "Create a project config first. Run: ks project init")` and `os.Exit(2)`.
  3. Verify project-local state exists — check for `.kaleidoscope/` in CWD; if absent, call `output.Fail("snapshot", err, "Run 'ks start --local' first to create a project-local state directory")` and `os.Exit(2)`.
  4. Call `snapshot.GenerateID()` to produce the snapshot ID.
  5. Call `snapshot.CreateDir(id)` to create the snapshot root directory.
  6. Call `browser.WithPage(func(page *rod.Page) error { … })` — if this returns an error (browser not running), call `output.Fail("snapshot", err, "Is the browser running? Run: ks start")` and `os.Exit(2)`.
  7. Inside the `WithPage` callback, iterate `cfg.URLs`:
     a. Call `validateURL(rawURL)` — if invalid, set `entry.Error` and `continue`.
     b. Compute `entry.Dir = snapshot.URLDir(rawURL)`.
     c. Call `os.MkdirAll(filepath.Join(snapshotDir, entry.Dir), 0755)`.
     d. Call `page.Navigate(rawURL)` — if error, set `entry.Error` and `continue`.
     e. Call `page.WaitLoad()`.
     f. Call `captureBreakpointsData(page, urlDir)` — assign `entry.Screenshots`; log non-fatal errors.
     g. Call `captureAuditData(page)` — assign `entry.AuditSummary`; write `audit.json` to `urlDir` using the raw result.
     h. Call `captureAxTreeData(page)` — assign `entry.AxNodeCount`; write `ax-tree.json` to `urlDir`.
     i. Append `entry` to `urlEntries`.
  8. Extract short commit hash from `id` (split on `-`, take second part if present).
  9. Construct `snapshot.Manifest` with `ID`, `Timestamp: time.Now()`, `CommitHash`, `ProjectConfig` (copied from `cfg`), `URLs: urlEntries`.
  10. Call `snapshot.WriteManifest(snapshotDir, manifest)`.
  11. Call `snapshot.EnsureBaseline(id)` — capture `promoted bool`.
  12. Call `output.Success("snapshot", map[string]any{ "id", "snapshotDir", "urlCount", "baselinePromoted", "urls" })`.

- **Sub-task 8.1.2**: Implement `writeJSON(path string, data any) error` helper (or reuse existing `cmd/util.go` if a JSON-write utility already exists) — marshals data with `json.MarshalIndent` and writes to `path` with `0644` permissions.

---

## Phase 9: `ks history` Command (`cmd/history.go`)

List all snapshots in reverse chronological order.

### Task 9.1 — Create `cmd/history.go`

- **Sub-task 9.1.1**: Implement `RunHistory(args []string)`:
  1. Parse optional `--limit N` flag from `args` (default `0` = all).
  2. Call `snapshot.List()` — if error, call `output.Fail("history", err, "No snapshots found. Run: ks snapshot")` and `os.Exit(2)`.
  3. Apply limit: if `limit > 0`, truncate the manifests slice to `manifests[:limit]`.
  4. Build `summaries` slice — for each manifest, construct `map[string]any` with `"id"`, `"timestamp"`, `"commitHash"`, `"urlCount"`, `"urls"` (using `urlSummaries` helper below).
  5. Call `output.Success("history", map[string]any{ "count": len(summaries), "snapshots": summaries })`.
- **Sub-task 9.1.2**: Implement `urlSummaries(entries []snapshot.URLEntry) []map[string]any` helper — returns a slice of maps with only `"url"`, `"dir"`, and `"auditSummary"` (omits screenshot paths and ax-tree details for brevity in history output).

---

## Phase 10: Wire Up Commands in `main.go`

### Task 10.1 — Update `main.go`

- **Sub-task 10.1.1**: Add `case "snapshot": cmd.RunSnapshot(cmdArgs)` to the command switch statement.
- **Sub-task 10.1.2**: Add `case "history": cmd.RunHistory(cmdArgs)` to the command switch statement.
- **Sub-task 10.1.3**: Add a new `"Snapshot & History:"` section to the usage/help string with entries:
  - `snapshot` — `Capture full interface state for all project URLs`
  - `history` — `List snapshots in reverse chronological order`

---

## Phase 11: `.gitignore` Updates

### Task 11.1 — Update project `.gitignore`

- **Sub-task 11.1.1**: Check if `.kaleidoscope/snapshots/` is already in `.gitignore`; if not, add it.
- **Sub-task 11.1.2**: Check if `.kaleidoscope/state.json` is already in `.gitignore`; if not, add it.
- **Sub-task 11.1.3**: Check if `.kaleidoscope/screenshots/` is already in `.gitignore`; if not, add it.
- **Sub-task 11.1.4**: Confirm `.kaleidoscope/baselines.json` is NOT in `.gitignore` (it must be committed).

---

## Phase 12: Quality Gates — Run and Fix Tests

### Task 12.1 — Verify all tests pass

- **Sub-task 12.1.1**: Run `go build ./...` — fix any compilation errors.
- **Sub-task 12.1.2**: Run `go test ./...` — fix any failing tests.
- **Sub-task 12.1.3**: Verify `go vet ./...` passes with no warnings.

### Task 12.2 — Verify acceptance criteria manually (checklist)

- **Sub-task 12.2.1**: Verify `ks snapshot` creates `.kaleidoscope/snapshots/<id>/` with one subdirectory per URL.
- **Sub-task 12.2.2**: Verify each URL subdirectory contains 4 PNG files, `audit.json`, and `ax-tree.json`.
- **Sub-task 12.2.3**: Verify root `snapshot.json` manifest has `id`, `timestamp`, `commitHash` (when in git), and `projectConfig`.
- **Sub-task 12.2.4**: Verify snapshot ID format is `<unix-ms>-<short-hash>` in git, or `<unix-ms>` outside git.
- **Sub-task 12.2.5**: Verify first `ks snapshot` auto-creates `.kaleidoscope/baselines.json` with `promotedBy: "auto"`.
- **Sub-task 12.2.6**: Verify `ks history` returns snapshots sorted by timestamp descending with summary stats.
- **Sub-task 12.2.7**: Verify `ks snapshot` fails gracefully (no panic, `ok: false` JSON output) when a URL is unreachable.

---

## Implementation Order (Recommended)

```
Phase 1  → Phase 2  → Phase 3  → Phase 4   (data/persistence foundation)
Phase 5                                     (browser helper, independent)
Phase 6  → Phase 7                          (capture refactor + security, depends on 1-4)
Phase 8  → Phase 9  → Phase 10             (commands, depends on all above)
Phase 11                                    (gitignore, independent)
Phase 12                                    (validation, last)
```

Phases 1–5 can be parallelized since they introduce new files with no modifications to existing code. Phase 6 (capture helpers refactor) carries the highest risk and should be done carefully with intermediate `go test` runs after each file change.

---

## File Inventory

| File | Status | Phase |
|------|--------|-------|
| `project/config.go` | New | 1 |
| `project/config_test.go` | New | 1 |
| `snapshot/model.go` | New | 2 |
| `snapshot/store.go` | New | 3 |
| `snapshot/store_test.go` | New | 3 |
| `snapshot/baseline.go` | New | 4 |
| `snapshot/baseline_test.go` | New | 4 |
| `browser/state.go` | Modified (add `SnapshotsDir`) | 5 |
| `cmd/capture_helpers.go` | New | 6 |
| `cmd/audit.go` | Modified (delegate to helper) | 6 |
| `cmd/axtree.go` | Modified (delegate to helper) | 6 |
| `cmd/breakpoints.go` | Modified (delegate to helper) | 6 |
| `cmd/snapshot.go` | New | 8 |
| `cmd/history.go` | New | 9 |
| `main.go` | Modified (add cases + usage) | 10 |
| `.gitignore` | Modified | 11 |
