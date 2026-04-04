# Progress Log

## US-002: Snapshot Capture and History

### Run: kal-29860-autofix-kal-7e88f-github-callmer-us-002 | Iteration 1 (tests-only)

**Date:** 2026-04-04

**Commands run:**
- `go build ./snapshot/...` → PASS (package compiles with stubs)
- `go test ./snapshot/...` → FAIL (expected — stub implementations return wrong values)
- `go build ./...` → PASS (rest of codebase unaffected)

**Files created:**
- `snapshot/urlkey.go` — stub (returns "")
- `snapshot/urlkey_test.go` — table-driven tests for URLToKey
- `snapshot/git.go` — stub (returns "")
- `snapshot/git_test.go` — in-repo and non-repo tests for ShortCommitHash
- `snapshot/snapshot.go` — full type definitions (Manifest, URLEntry, Summary, BaselineManifest)
- `snapshot/store.go` — stub implementations for all store functions
- `snapshot/store_test.go` — round-trip and ordering tests for store functions

**Failing tests (expected):**
- `TestShortCommitHash_InRepo` — stub returns "" inside a git repo
- `TestWriteReadManifest` — WriteManifest is a no-op stub
- `TestListSnapshotIDs_Order` — stub returns nil
- `TestBaselineRoundTrip` — WriteBaselineManifest is a no-op stub
- `TestURLToKey/root_URL`, `deep_path`, `query_string_stripped`, `fragment_stripped`, `consecutive_slashes_collapsed` — stub returns ""

**Key patterns:**
- Stubs have correct signatures so tests compile; tests fail due to missing logic.
- Next iteration: implement URLToKey, ShortCommitHash, and all store functions.
- Store tests use `os.Chdir` + `t.Cleanup` for temp-dir isolation.

## US-002: Snapshot Capture and History

### Run: kal-29860-autofix-kal-7e88f-github-callmer-us-002 | Iteration 2 (production-code)

**Date:** 2026-04-04

**Commands run:**
- `go build ./...` → PASS
- `go test ./...` → PASS (snapshot package: ok 0.010s)
- `go vet ./...` → PASS

**Files implemented:**
- `snapshot/urlkey.go` — URLToKey: parse URL, strip scheme/query/fragment, replace `/` with `-`, sanitize, collapse dashes, strip `..", truncate to 128
- `snapshot/git.go` — ShortCommitHash: runs `git rev-parse --short HEAD`, returns "" on any error
- `snapshot/store.go` — all store functions: WriteManifest, ReadManifest, ListSnapshotIDs (descending), ReadBaselineManifest, WriteBaselineManifest
- `cmd/project.go` — ProjectConfig struct + ReadProjectConfig
- `cmd/audit.go` — extracted gatherAuditData; RunAudit is thin wrapper
- `cmd/axtree.go` — extracted gatherAxTreeData; RunAxTree is thin wrapper
- `cmd/breakpoints.go` — extracted captureBreakpointsToDir; RunBreakpoints keeps original output shape
- `cmd/snapshot.go` — RunSnapshot: loads config, builds ID, captures per URL, writes manifest, auto-promotes baseline
- `cmd/history.go` — RunHistory: lists snapshots with metadata
- `main.go` — added snapshot/history cases and usage entries
- `.gitignore` — updated to specific .kaleidoscope/ subdirs (keeping baselines.json committable)

**Status:** US-002 DONE

**Key patterns:**
- URLToKey uses `net/url.Parse` to cleanly strip query/fragment before string manipulation.
- captureBreakpointsToDir uses `<name>.png` filenames (not timestamped) for deterministic snapshot paths.
- RunBreakpoints preserves original timestamped filename format for backwards compatibility.

