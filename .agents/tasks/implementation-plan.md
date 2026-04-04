# Implementation Plan: US-002 — Snapshot Capture and History

## Overview

Introduces `ks snapshot` and `ks history` commands, backed by two new Go packages (`project/`, `snapshot/`) and internal refactors that extract reusable page-level audit and ax-tree logic.

---

## Phase 1: New Package Foundations

### Task 1.1 — `project` Package (`project/project.go`)

**Goal:** Load and validate `.ks-project.json`; save it back.

Sub-tasks:
- 1.1.1 Create `project/` directory and `project/project.go` with package declaration.
- 1.1.2 Define `Config` struct with `Version int`, `URLs []string`, `Breakpoints []string` (omitempty).
- 1.1.3 Define sentinel `ErrNotFound` error (e.g., `errors.New("project config not found")`).
- 1.1.4 Implement `Load() (*Config, error)`:
  - Read `.ks-project.json` from `os.Getwd()`.
  - Return `ErrNotFound` (using `errors.Is`-compatible wrapping) if the file does not exist (`os.IsNotExist`).
  - Unmarshal JSON into `Config`.
  - Validate: `version` must equal 1; return descriptive error otherwise.
  - Validate: `urls` must be non-empty; return descriptive error otherwise.
  - Validate: each URL must parse without error via `url.Parse`; return descriptive error naming the bad URL.
- 1.1.5 Implement `Save(cfg *Config) error`:
  - Marshal `cfg` to indented JSON.
  - Write to `.ks-project.json` in CWD with permission `0644`.

---

### Task 1.2 — `snapshot` Package (`snapshot/snapshot.go`)

**Goal:** All on-disk snapshot state (manifest types, storage, baseline management).

Sub-tasks:

- 1.2.1 Create `snapshot/` directory and `snapshot/snapshot.go` with package declaration and imports.

- 1.2.2 Define types:
  - `AuditSummary` — `TotalIssues`, `ContrastViolations`, `TouchViolations`, `TypographyWarnings` (all `int`, JSON snake-camelCase).
  - `BreakpointEntry` — `Name string`, `Width int`, `Height int`, `File string` (relative path).
  - `URLEntry` — `URL string`, `Slug string`, `Breakpoints []BreakpointEntry`, `AuditSummary AuditSummary`, `Error string` (omitempty).
  - `Manifest` — `ID string`, `Timestamp time.Time`, `CommitHash string` (omitempty), `Project project.Config`, `URLs []URLEntry`.
  - `Baselines` — `SnapshotID string` (json: `"snapshotId"`).
  - `ListEntry` — `ID string`, `Timestamp time.Time`, `CommitHash string` (omitempty), `URLCount int`, `Summary AuditSummary`, `IsBaseline bool`.

- 1.2.3 Implement internal `gitShortHash() string`:
  - Run `exec.Command("git", "rev-parse", "--short", "HEAD").Output()`.
  - Return trimmed output on success, empty string on error.
  - Arguments passed as string slice (no shell interpolation).

- 1.2.4 Implement `NewID() string`:
  - Format: `time.Now().UTC().Format("20060102T150405Z")`.
  - Append `-<hash>` when `gitShortHash()` returns non-empty.

- 1.2.5 Implement internal `slugify(rawURL string) string`:
  - Parse with `url.Parse`.
  - Combine `host` + `path`, replacing `/` with `-`.
  - Strip leading/trailing dashes.
  - Truncate to 80 characters.
  - Examples: `http://localhost:3000` → `localhost-3000`; `http://localhost:3000/about/team` → `localhost-3000-about-team`.

- 1.2.6 Implement internal `uniqueSlug(rawURL string, seen map[string]int) string`:
  - Call `slugify`.
  - If `slug` already in `seen`, append `-2`, `-3`, etc., incrementing until unique.
  - Record in `seen`; return final slug.

- 1.2.7 Implement `SnapshotRoot() (string, error)`:
  - Path: `.kaleidoscope/snapshots/` relative to CWD.
  - Create with `os.MkdirAll(..., 0755)` if not present.
  - Return the path.

- 1.2.8 Implement `Store(manifest *Manifest) error`:
  - Call `SnapshotRoot()` to get base dir.
  - Create `<base>/<manifest.ID>/` directory (`os.MkdirAll`, `0755`).
  - Marshal `manifest` to indented JSON; write as `snapshot.json` (`0644`).
  - Return nil (files written by the caller per URL sub-directory; `Store` only writes the manifest).

- 1.2.9 Implement `List() ([]ListEntry, error)`:
  - Call `SnapshotRoot()`; read directory entries with `os.ReadDir`.
  - For each entry (dir only), read `snapshot.json` inside it, unmarshal into `Manifest`.
  - Aggregate `AuditSummary` across all `URLs` into `Summary`.
  - Build `ListEntry`; collect into slice.
  - Sort descending by `Timestamp`.
  - `IsBaseline` set to false initially (caller sets it after loading baselines).
  - Return slice.

- 1.2.10 Implement `LoadBaselines() (*Baselines, error)`:
  - Path: `.kaleidoscope/baselines.json` relative to CWD.
  - Return `nil, nil` if file does not exist (`os.IsNotExist`).
  - Unmarshal and return `*Baselines`.

- 1.2.11 Implement `SaveBaselines(b *Baselines) error`:
  - Ensure `.kaleidoscope/` directory exists (`os.MkdirAll`, `0755`).
  - Marshal `b` to indented JSON; write to `.kaleidoscope/baselines.json` (`0644`).

---

## Phase 2: Internal Refactors (Audit & AX-Tree Core)

### Task 2.1 — Extract `runAuditOnPage` (`cmd/audit_core.go`)

**Goal:** Move the page-level audit logic into a reusable function; update `cmd/audit.go` to call it.

Sub-tasks:

- 2.1.1 Create `cmd/audit_core.go`:
  - Declare `func runAuditOnPage(page *rod.Page) (snapshot.AuditSummary, error)`.
  - Move the contrast-check block (JS eval + `analysis.CheckContrast` loop), touch-target block, and typography block from `RunAudit` into this function.
  - Return `snapshot.AuditSummary{TotalIssues, ContrastViolations, TouchViolations, TypographyWarnings}` and `nil` error on success.
  - Keep the ax-tree call (`proto.AccessibilityGetFullAXTree`) in `RunAudit` only (not needed in the core audit summary).

- 2.1.2 Update `cmd/audit.go` — `RunAudit`:
  - Call `runAuditOnPage(page)` to get `summary`.
  - Keep ax-tree call and `axSummary` assembly as-is.
  - Keep existing `output.Success("audit", ...)` structure intact; wire `summary` fields through.
  - No change to public API or JSON output shape.

---

### Task 2.2 — Extract `runAxTreeOnPage` (`cmd/axtree_core.go`)

**Goal:** Move the page-level ax-tree logic into a typed, reusable function; update `cmd/axtree.go` to call it.

Sub-tasks:

- 2.2.1 Create `cmd/axtree_core.go`:
  - Define `AXNode` struct: `NodeID string`, `Role string`, `Name string`, `Children []string` (omitempty), `Properties map[string]any` (omitempty) — all JSON-tagged.
  - Declare `func runAxTreeOnPage(page *rod.Page) ([]AXNode, error)`.
  - Move the `proto.AccessibilityGetFullAXTree` call and node-conversion loop from `RunAxTree` into this function.
  - Skip ignored nodes as currently done.
  - Return `[]AXNode` and error.

- 2.2.2 Update `cmd/axtree.go` — `RunAxTree`:
  - Call `runAxTreeOnPage(page)`.
  - Build the existing `output.Success("ax-tree", ...)` result from the returned `[]AXNode`.
  - No change to JSON output shape.

---

## Phase 3: `ks snapshot` Command (`cmd/snapshot.go`)

**Goal:** Iterate project URLs, capture breakpoint screenshots + audit + ax-tree, persist to disk, auto-promote baseline.

Sub-tasks:

- 3.1 Create `cmd/snapshot.go` with `func RunSnapshot(args []string)`.

- 3.2 Parse flags:
  - `fullPage := hasFlag(args, "--full-page")`.

- 3.3 Load project config:
  - Call `project.Load()`.
  - On `errors.Is(err, project.ErrNotFound)`: call `output.Fail("snapshot", err, "Create .ks-project.json with a list of URLs. See 'ks snapshot --help'.")` and `os.Exit(2)`.
  - On any other error: `output.Fail(...)` and `os.Exit(2)`.

- 3.4 Generate snapshot ID and directory:
  - `id := snapshot.NewID()`.
  - `root, err := snapshot.SnapshotRoot()` — fail on error.
  - `snapshotDir := filepath.Join(root, id)`.
  - `os.MkdirAll(snapshotDir, 0755)`.

- 3.5 Connect to browser via `browser.WithPage`:
  - On error: `output.Fail("snapshot", err, "Is the browser running? Run: ks start")` and `os.Exit(2)`.

- 3.6 Per-URL capture loop (inside `browser.WithPage` callback):
  - Maintain `seen map[string]int` for slug uniqueness.
  - For each `url` in `cfg.URLs`:
    - **a. Navigate:** `page.Navigate(url)`. On error: append `URLEntry{URL: url, Error: err.Error()}` to manifest URLs, log to stderr, `continue`.
    - **b. Wait stable:** `page.MustWaitStable()`.
    - **c. Slug:** `slug := snapshot.UniqueSlug(url, seen)` (or unexported helper — expose as needed).
    - **d. URL sub-directory:** `urlDir := filepath.Join(snapshotDir, slug)`; `os.MkdirAll(urlDir, 0755)`.
    - **e. Breakpoints loop** (over `defaultBreakpoints` from `cmd/breakpoints.go` — reuse the existing slice):
      - `page.SetViewport(...)`.
      - `page.MustWaitStable()`.
      - `data, err := page.Screenshot(fullPage, nil)` — on error log and skip this breakpoint.
      - Filename: `<name>-<W>x<H>.png` (e.g., `mobile-375x812.png`).
      - Write to `filepath.Join(urlDir, filename)`.
      - Append `BreakpointEntry{Name, Width, Height, File: filepath.Join(slug, filename)}` to local slice.
    - **f. Restore viewport** to original (read original from `browser.ReadState()` before loop starts).
    - **g. Audit:** `summary, err := runAuditOnPage(page)` — on error log; use zero-value summary.
    - **h. Write `audit.json`:** marshal `summary` to `filepath.Join(urlDir, "audit.json")` (`0644`).
    - **i. AX-tree:** `nodes, err := runAxTreeOnPage(page)` — on error log; use nil slice.
    - **j. Write `ax-tree.json`:** marshal `nodes` to `filepath.Join(urlDir, "ax-tree.json")` (`0644`).
    - **k. Append URLEntry** (with `Slug`, `Breakpoints`, `AuditSummary`, `Error` if any) to manifest.

- 3.7 Build `Manifest`:
  - `ID`, `Timestamp: time.Now().UTC()`, `CommitHash: snapshot.GitShortHash()` (exported or obtained via `NewID` side-channel — keep internal; derive from ID string or re-call).
  - `Project: *cfg`.
  - `URLs`: collected entries.

- 3.8 Store manifest:
  - Call `snapshot.Store(&manifest)` — writes `snapshot.json`.

- 3.9 Auto-promote baseline:
  - `bl, err := snapshot.LoadBaselines()`.
  - If `err == nil && bl == nil`: call `snapshot.SaveBaselines(&snapshot.Baselines{SnapshotID: id})`. Set `autoPromotedBaseline = true`.
  - Otherwise `autoPromotedBaseline = false`.

- 3.10 Output:
  - `output.Success("snapshot", map[string]any{ "id": id, "snapshotDir": snapshotDir, "timestamp": manifest.Timestamp, "commitHash": manifest.CommitHash, "autoPromotedBaseline": autoPromotedBaseline, "urls": <per-url result slice> })`.

- 3.11 Error handling summary (from spec):
  - Missing `.ks-project.json` → `output.Fail` + exit 2.
  - Browser not running → `output.Fail` + exit 2.
  - Individual URL unreachable → record per-URL `error` field, log to stderr, continue; final result `ok: true`.

---

## Phase 4: `ks history` Command (`cmd/history.go`)

**Goal:** List snapshots in reverse chronological order; mark baseline.

Sub-tasks:

- 4.1 Create `cmd/history.go` with `func RunHistory(args []string)`.

- 4.2 Parse flags:
  - `limitStr := getFlagValue(args, "--limit")`.
  - Parse to `int`; default `0` (unlimited). Invalid value: `output.Fail(...)` exit 2.

- 4.3 Load snapshots:
  - `entries, err := snapshot.List()` — on error `output.Fail(...)` exit 2.

- 4.4 Load baselines and mark:
  - `bl, _ := snapshot.LoadBaselines()`.
  - For each `ListEntry`, set `IsBaseline = (bl != nil && bl.SnapshotID == entry.ID)`.

- 4.5 Apply `--limit`:
  - If `limit > 0 && limit < len(entries)`, truncate to `entries[:limit]`.

- 4.6 Output:
  - `output.Success("history", map[string]any{"snapshots": entries})`.

---

## Phase 5: Wire Up in `main.go` and Supporting Files

### Task 5.1 — `cmd/util.go` — Add `--limit` to flag-value parser

Sub-tasks:
- 5.1.1 In `getNonFlagArgs`, add `"--limit"` to the set of flags that consume a following value (the `skip = true` branch), alongside existing flags like `--selector`, `--output`, etc.

---

### Task 5.2 — `main.go` — New `case` entries and usage string

Sub-tasks:
- 5.2.1 Add to `switch` block:
  ```go
  case "snapshot":
      cmd.RunSnapshot(cmdArgs)
  case "history":
      cmd.RunHistory(cmdArgs)
  ```
- 5.2.2 Add a **Snapshots** section to the `usage` string:
  ```
  Snapshots:
    snapshot [--full-page]  Capture all project URLs; auto-promote first as baseline
    history [--limit N]     List snapshots in reverse chronological order
  ```

---

### Task 5.3 — `cmd/usage.go` — Register command usage strings

Sub-tasks:
- 5.3.1 Add `"snapshot"` entry to `CommandUsage` map:
  - Document flags: `--full-page`.
  - Document output shape (id, snapshotDir, timestamp, commitHash, autoPromotedBaseline, urls[]).
  - Document error conditions (missing `.ks-project.json`, browser not running, per-URL errors).
- 5.3.2 Add `"history"` entry to `CommandUsage` map:
  - Document flags: `--limit N`.
  - Document output shape (snapshots[], isBaseline).

---

## Phase 6: Testing

### Task 6.1 — `project` Package Tests (`project/project_test.go`)

Sub-tasks:
- 6.1.1 Test `Load()` returns `ErrNotFound` when `.ks-project.json` absent (temp dir).
- 6.1.2 Test `Load()` returns error for `version != 1`.
- 6.1.3 Test `Load()` returns error for empty `urls`.
- 6.1.4 Test `Load()` returns error for malformed URL in `urls`.
- 6.1.5 Test `Load()` happy path with valid config.
- 6.1.6 Test `Save()` + `Load()` round-trip.

---

### Task 6.2 — `snapshot` Package Tests (`snapshot/snapshot_test.go`)

Sub-tasks:
- 6.2.1 Test `slugify` with standard URL forms and edge cases (root path, sub-path, truncation at 80 chars).
- 6.2.2 Test `uniqueSlug` collision resolution (same slug generates `-2`, `-3` suffix).
- 6.2.3 Test `NewID` format — matches `^\d{8}T\d{6}Z(-[a-f0-9]+)?$`.
- 6.2.4 Test `SnapshotRoot` creates the directory if absent (temp dir).
- 6.2.5 Test `Store` writes `snapshot.json` with correct content (temp dir).
- 6.2.6 Test `List` returns entries sorted descending by timestamp (temp dir with 2 fake snapshots).
- 6.2.7 Test `LoadBaselines` returns `nil, nil` when file absent.
- 6.2.8 Test `SaveBaselines` + `LoadBaselines` round-trip.

---

### Task 6.3 — Verify full test suite passes

Sub-tasks:
- 6.3.1 Run `go test ./...` — all tests must pass with no compilation errors.
- 6.3.2 Fix any import cycle or missing import issues introduced by the refactors.

---

## Dependency & Sequencing Notes

| Phase | Depends on |
|---|---|
| Phase 1 (project + snapshot packages) | Independent; can start first |
| Phase 2 (audit/axtree refactor) | Phase 1 (needs `snapshot.AuditSummary` type) |
| Phase 3 (cmd/snapshot.go) | Phase 1, Phase 2 |
| Phase 4 (cmd/history.go) | Phase 1 (`snapshot.List`, `snapshot.LoadBaselines`) |
| Phase 5 (main.go, util.go, usage.go) | Phase 3, Phase 4 |
| Phase 6 (tests) | All prior phases |

Phases 1 and 2 can be implemented in parallel. Phase 3 and Phase 4 can be implemented in parallel once Phase 1 and 2 are done.

---

## File Manifest (new and modified files)

| File | Action |
|---|---|
| `project/project.go` | Create |
| `project/project_test.go` | Create |
| `snapshot/snapshot.go` | Create |
| `snapshot/snapshot_test.go` | Create |
| `cmd/audit_core.go` | Create |
| `cmd/axtree_core.go` | Create |
| `cmd/audit.go` | Modify (delegate to `runAuditOnPage`) |
| `cmd/axtree.go` | Modify (delegate to `runAxTreeOnPage`) |
| `cmd/snapshot.go` | Create |
| `cmd/history.go` | Create |
| `cmd/util.go` | Modify (add `--limit` to flag-value set) |
| `cmd/usage.go` | Modify (add `snapshot` and `history` entries) |
| `main.go` | Modify (add cases + usage section) |
