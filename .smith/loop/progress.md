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

**Remaining for next iteration:**
- Phase 2: Extract runAuditOnPage (cmd/audit_core.go) and runAxTreeOnPage (cmd/axtree_core.go)
- Phase 3: cmd/snapshot.go (RunSnapshot)
- Phase 4: cmd/history.go (RunHistory)
- Phase 5: wire up main.go, cmd/util.go, cmd/usage.go

