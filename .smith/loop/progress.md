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

**Remaining (future iterations):**
- Phase 3: cmd/breakpoints_common.go, cmd/audit_internal.go refactoring
- Phase 4: cmd/snapshot.go (RunSnapshot command)
- Phase 5: cmd/history.go (RunHistory command)
- Phase 6: Wire into main.go
