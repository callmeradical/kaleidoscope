# Progress Log

## US-002: Snapshot Capture and History

### Run: kal-7e88f-github-callmeradical-kaleidoscop-us-002 | Iteration 1 of 10

**Status:** in_progress (tests-first gate)

**Files Created:**
- `snapshot/model.go` — Manifest, URLEntry, AuditSummary, Baseline, ProjectConfig types
- `snapshot/urldir.go` — URLToDir with path traversal prevention
- `snapshot/store.go` — SnapshotsDir, SnapshotPath, Save, Load, List
- `snapshot/project.go` — LoadProjectConfig
- `snapshot/baseline.go` — BaselinePath, LoadBaseline, SaveBaseline
- `snapshot/urldir_test.go` — Table-driven tests for URLToDir (6 cases)
- `snapshot/store_test.go` — TestSaveLoad, TestListSortOrder, TestListEmptyDir
- `snapshot/baseline_test.go` — TestLoadBaselineMissing, TestSaveLoadBaseline
- `snapshot/project_test.go` — TestLoadProjectConfigMissing, TestLoadProjectConfigValid, TestLoadProjectConfigEmptyURLs

**Commands Run:**
- `go test ./snapshot/... -v` → PASS (9 tests)
- `go test ./...` → PASS

**Key Patterns:**
- Tests use `t.Chdir(tmpDir)` + create `.kaleidoscope/` dir so `browser.StateDir()` resolves to temp location
- `URLToDir` strips scheme, replaces non-alphanumeric with `-`, collapses dashes, trims, then safety-checks for path separators
- `List()` returns empty slice (not error) when snapshots dir doesn't exist
- `LoadBaseline()` returns (nil, nil) when baselines.json missing — callers treat nil as "no baseline yet"

### Run: kal-7e88f-github-callmeradical-kaleidoscop-us-002 | Iteration 2 of 10

**Status:** done

**Files Created/Modified:**
- `cmd/breakpoints_common.go` — shared `breakpoint` struct + `defaultBreakpoints`
- `cmd/breakpoints.go` — removed duplicate struct/var (now in breakpoints_common.go)
- `cmd/audit_internal.go` — `runAudit` + `runAxTree` helpers
- `cmd/audit.go` — refactored to thin wrapper calling `runAudit`
- `cmd/snapshot.go` — `RunSnapshot` command (full capture loop, manifest, baseline auto-promote)
- `cmd/history.go` — `RunHistory` command (list snapshots with summary stats)
- `main.go` — added `snapshot` and `history` cases + usage text

**Commands Run:**
- `go build ./...` → PASS
- `go test ./...` → PASS (snapshot package: all 9 tests cached)
- `go vet ./...` → PASS (no output)

**Key Patterns:**
- `runAudit` includes ax-tree CDP call to maintain backward-compatible output (accessibility summary in result map)
- `runAxTree` makes a separate CDP call for the full node list written to ax-tree.json
- Viewport restored after breakpoint screenshots using state read before the URL loop
- URL navigation errors recorded per-entry (non-fatal); only browser/filesystem errors abort the run
