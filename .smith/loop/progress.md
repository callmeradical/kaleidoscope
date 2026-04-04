# Progress Log

## US-002: Snapshot Capture and History

### Run: kal-d6ce7-autofix-kal-7e88f-github-callmer-us-002 | Iteration 1 (tests-only)

**Status:** in_progress (tests written, failing; implementation pending)

**Commands run:**
- `go build ./...` → PASS (all packages compile)
- `go test ./...` → FAIL (expected: stubs panic, implementation absent)

**Files created:**
- `project/config.go` — stub: `Config` struct + `Load()` signature (panics)
- `project/config_test.go` — 4 table-driven tests: valid config, missing file, empty URLs, malformed JSON
- `gitutil/gitutil.go` — stub: `ShortHash()` signature (panics)
- `gitutil/gitutil_test.go` — 4 tests: in git repo, no git binary, not a git repo, no commits
- `snapshot/manager.go` — stub: all types (Manifest, URLSummary, AuditResult, AXNode, BaselinesFile) + all function signatures (panic)
- `snapshot/manager_test.go` — 9 tests: URLSlug (table-driven + max length), NewID (with/without hash), ListIDs (newest-first + empty), WriteManifest/ReadManifest round-trip, ReadBaselines (no file + write/read round-trip)

**Test failures observed (expected):**
- `gitutil` → `panic: not implemented` in TestShortHash_InGitRepo
- `project` → `panic: not implemented` in TestLoad_ValidConfig
- `snapshot` → `panic: not implemented` in TestURLSlug

**Key patterns for next iteration:**
- `browser.StateDir()` checks `./.kaleidoscope/` in CWD first; tests use `chdir(t, t.TempDir())` + `.kaleidoscope/` seeded to control path resolution
- `gitutil.ShortHash()` must use `exec.Command("git", "rev-parse", "--short", "HEAD")`; return `""` on any error
- `project.Load()` error for missing file must contain the word "init" (tested explicitly)
- `snapshot.ListIDs()` must return IDs in descending order (newest first by string sort of epoch-prefixed names)
- `snapshot.ReadBaselines()` must return `nil, nil` (not an error) when file is absent
