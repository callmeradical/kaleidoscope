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

