# Progress Log

## US-004: Screenshot Pixel Diff

### Run: kal-f500d-autofix-kal-9f4f2-github-callmer-us-004 | Iteration 1

**Status:** in_progress (tests-first iteration)

**Files Created:**
- `pixeldiff/diff.go` — Core engine: `Result`, `Options`, `DefaultOptions()`, `Compare()`, `WriteDiffPNG()`
- `pixeldiff/diff_test.go` — 7 unit tests covering all acceptance criteria

**Commands Run:**
- `go test ./pixeldiff/... -v` → PASS (7/7 tests)
- `go test ./...` → PASS (no regressions)

**Tests Coverage:**
- TestIdenticalImages: score=1.0, diffPixels=0, regressed=false
- TestCompletelyDifferent: score=0.0, all pixels diff, regressed=true
- TestPartialDiff: 100/10000 pixels diff, score=0.99, regressed=false at 1% threshold
- TestDimensionMismatch: returns DimensionMismatch=true, Regressed=true, diffImg=nil (no panic)
- TestThresholdBoundary: verifies threshold=0.005 (regressed), 0.02 (not), 0.01 (boundary, not)
- TestPixelTolerance: diff exactly at tolerance (=10) is NOT flagged (uses `>` not `>=`)
- TestWriteDiffPNG: writes PNG to temp file, decodes and verifies 50×50 dimensions

**Key Learnings:**
- Phases 1-2 (pixeldiff package) are fully independent of US-002
- Phase 3 (cmd/diff.go integration) requires US-002 snapshot types — deferred
- `similarityScore < (1.0 - threshold)` is strict less-than (boundary is not regressed)
- Per-channel diff uses `> opts.PixelTolerance` (strict, not >=)

---

### Run: kal-f500d-autofix-kal-9f4f2-github-callmer-us-004 | Iteration 2

**Status:** done

**Files Created/Modified:**
- `cmd/diff.go` — Phase 3: `RunDiff`, `ScreenshotDiffEntry`, `ScreenshotSummary`, `slugifyURL`, `buildDiffPath`, `resolveThreshold`
- `cmd/diff_test.go` — 7 tests for cmd/diff.go (slugify, compareScreenshotPair, buildDiffPath, RunDiff)
- `cmd/util.go` — Added `--screenshot-threshold`, `--baseline`, `--current`, `--output-dir`, `--url`, `--breakpoint` to flag skip list
- `main.go` — Added `diff` case routing to `cmd.RunDiff`

**Commands Run:**
- `go build ./...` → PASS
- `go test ./cmd/... -v` → PASS (7/7 tests)
- `go test ./...` → PASS (14/14 tests: 7 cmd + 7 pixeldiff)

**Acceptance Criteria Verification:**
- AC1: pixeldiff.Compare produces diff PNG highlighting changed pixels ✅
- AC2: Returns similarity score 0.0-1.0 ✅
- AC3: WriteDiffPNG writes to snapshot dir alongside source screenshots ✅ (output-dir or alongside current path)
- AC4: `ks diff` JSON output includes screenshot diff results ✅ (ScreenshotDiffEntry with paths, scores)
- AC5: Dimension mismatch handled gracefully, no panic ✅
- AC6: `--screenshot-threshold` flag controls regression detection ✅

**Implementation Notes:**
- Phase 3 implemented as standalone `ks diff --baseline <path> --current <path>` without US-002 snapshot system
- Designed to integrate with US-002 snapshot types when available (ScreenshotDiffEntry has URL/Breakpoint fields)
- `ks-project.json` `screenshotThreshold` field supported as config fallback

