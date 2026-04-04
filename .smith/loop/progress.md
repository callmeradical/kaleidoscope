# Progress Log

## US-004: Screenshot Pixel Diff

### Run kal-890c7-autofix-kal-9f4f2-github-callmer-us-004 — Iteration 1

**Status:** in_progress (tests-first gate)

**Files created:**
- `diff/pixel.go` — stubs: `ScreenshotDiffResult`, `Options`, `DefaultOptions()`, `CompareImages()` (returns zero values)
- `diff/io.go` — stubs: `LoadPNG()`, `SavePNG()`, `DiffImagePath()`
- `diff/pixel_test.go` — 11 tests covering all acceptance criteria

**go test ./... result:** FAIL (expected — stubs return zero values)

Failing tests (all expected at this stage):
- TestIdenticalImages, TestCompletelyDifferentImages, TestPartialDiff
- TestDimensionMismatch, TestThresholdTolerance, TestRegressionFlag
- TestCompareImagesZeroPixels, TestDiffImageWrittenToSnapshotDir

Passing tests: TestDefaultOptions, TestLoadSavePNG, TestDiffImagePath, TestLoadPNGMissingFile, TestSavePNGInvalidPath

**Key learnings:**
- `diff` package is isolated with no external deps (stdlib image/png only)
- Tests use white-box approach (`package diff`) for direct access to unexported helpers
- `go` binary available via `~/.local/share/mise/shims/go`

**Next iteration:** Implement `CompareImages` in `pixel.go` to make all tests pass.


### Run kal-890c7-autofix-kal-9f4f2-github-callmer-us-004 — Iteration 2

**Status:** done ✓

**Actions:**
- Implemented `CompareImages` in `diff/pixel.go` (dimension check, pixel iteration, similarity calculation, regression flag)
- Created `cmd/diff.go` with `RunDiff`, `DiffOutput`, `ScreenshotDiffEntry`, manifest loading, baseline/current resolution, path validation
- Added `case "diff": cmd.RunDiff(cmdArgs)` to `main.go`
- Added `"diff"` entry to `CommandUsage` in `cmd/usage.go`

**go test ./... result:** PASS — all 13 diff tests pass, full build clean

**Files changed:**
- `diff/pixel.go` — CompareImages fully implemented
- `cmd/diff.go` — new file, full CLI command
- `main.go` — added diff case
- `cmd/usage.go` — added diff usage entry

**Key learnings:**
- `image.Image.RGBA()` returns uint32 in 0–65535; shift right 8 bits for 8-bit values
- Zero-pixel edge case handled by returning similarity=1.0 (no pixels to compare = identical)
- `flag.ContinueOnError` needed for `flag.FlagSet` in non-fatal parse errors
