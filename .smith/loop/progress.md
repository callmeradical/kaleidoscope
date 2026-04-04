# Progress Log

## US-002: Snapshot Capture and History

### Run kal-1abe9-autofix-kal-7e88f-github-callmer-us-002 — Iteration 1 (tests-only)

**Status:** in_progress

**Files created:**
- `project/project.go` — Config struct, ErrNotFound sentinel, Load(), Save() with validation
- `project/project_test.go` — 6 tests: ErrNotFound, invalid version, empty URLs, malformed URL, happy path, round-trip
- `snapshot/snapshot.go` — All types (AuditSummary, BreakpointEntry, URLEntry, Manifest, Baselines, ListEntry), NewID(), UniqueSlug(), SnapshotRoot(), Store(), List(), LoadBaselines(), SaveBaselines()
- `snapshot/snapshot_test.go` — 8 tests: slug standard URL, slug sub-path, slug truncation, collision resolution, NewID format, SnapshotRoot creates dir, Store writes manifest, List sorted descending, LoadBaselines absent, SaveBaselines round-trip

**Commands run:**
- `go build ./...` — PASS
- `go test ./...` — PASS (project: ok, snapshot: ok)

**Key learnings:**
- slugify must replace `:` (port separator) as well as `/` path separators with dashes
- `url.Parse` is very permissive; only control characters reliably trigger an error for the malformed URL test
- Tests use `os.Chdir` into temp dirs to isolate file system side effects

**Remaining for next iteration:** None — story complete.

---

### Run kal-1abe9-autofix-kal-7e88f-github-callmer-us-002 — Iteration 2 (production implementation)

**Status:** done

**Files created:**
- `cmd/audit_core.go` — `runAuditOnPage(page) (snapshot.AuditSummary, error)` extracting contrast/touch/typo checks
- `cmd/axtree_core.go` — `AXNode` struct + `runAxTreeOnPage(page) ([]AXNode, error)` extracting ax-tree logic
- `cmd/snapshot.go` — `RunSnapshot`: loads project, generates ID/dir, captures per-URL breakpoints + audit + ax-tree, stores manifest, auto-promotes baseline
- `cmd/history.go` — `RunHistory`: lists snapshots reverse-chronologically with baseline marker and --limit support

**Files modified:**
- `cmd/audit.go` — delegates to `runAuditOnPage`; keeps ax-tree CDP call and full output shape
- `cmd/axtree.go` — delegates to `runAxTreeOnPage`; output shape unchanged
- `cmd/util.go` — added `--limit` to flag-value consuming set in `getNonFlagArgs`
- `cmd/usage.go` — added `snapshot` and `history` entries to `CommandUsage` map
- `main.go` — added `case "snapshot"` and `case "history"`; added Snapshots section to usage string

**Commands run:**
- `go build ./...` — PASS
- `go test ./...` — PASS (project: ok, snapshot: ok)

**Key learnings:**
- `node.Role.Value` and `node.Name.Value` in rod are `gson.JSON` structs; use `.Val().(string)` to extract the string value for typed structs
- The original `axtree.go` assigned to `map[string]any` which accepts any type, masking the type mismatch
- `browser.WithPage` callback + `browser.ReadState()` pattern for saving/restoring viewport reused from breakpoints.go

