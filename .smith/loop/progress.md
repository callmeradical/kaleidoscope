# Progress Log

## US-004: Screenshot Pixel Diff


## Run: kal-9f4f2-github-callmeradical-kaleidoscop-us-004 | Iteration 1

### Actions
- Created `snapshot/pixeldiff.go`: pure-Go pixel diff engine (DiffConfig, ScreenshotDiffResult, LoadPNG, SavePNG, DiffImages, DiffScreenshotFiles, DefaultDiffConfig, ErrDimensionMismatch)
- Created `snapshot/pixeldiff_test.go`: 10 unit tests covering all acceptance criteria (identical, fully-different, single-pixel, dimension mismatch, noise floor, threshold regressed/not-regressed, corrupt PNG, diff PNG written, default config)
- Fixed test bug: base image colour must differ from default red highlight colour to avoid false positives

### Quality Gate
- `go test ./...` → PASS (snapshot: ok)

### Key Learnings
- When the base image colour matches the highlight colour, identical-image and single-pixel tests falsely count every pixel as highlighted — use a different base colour (e.g. green)
- `image.Image.RGBA()` returns [0,65535]; shift right 8 to get [0,255] before computing delta

### Status
- US-004: in_progress (tests written, implementation in snapshot/pixeldiff.go; cmd/diff.go integration pending next iteration)

## Run: kal-9f4f2-github-callmeradical-kaleidoscop-us-004 | Iteration 2

### Actions
- Created `snapshot/manifest.go`: `SnapshotManifest`, `ScreenshotEntry` types with `LoadManifest` helper
- Created `cmd/diff.go`: `RunDiff` function with `--baseline`, `--current`, `--threshold` flags; `DiffOutput` and `ScreenshotDiffs` structs; screenshot pair matching loop calling `snapshot.DiffScreenshotFiles`
- Registered `case "diff": cmd.RunDiff(cmdArgs)` in `main.go`
- Added `--baseline`, `--current`, `--threshold` to `getNonFlagArgs` skip list in `cmd/util.go`

### Quality Gate
- `go build ./...` → PASS
- `go test ./...` → PASS (snapshot: ok, all 10 tests)

### Files Changed
- `snapshot/manifest.go` (created)
- `cmd/diff.go` (created)
- `main.go` (added diff case)
- `cmd/util.go` (added --baseline, --current, --threshold to flag skip list)

### Status
- US-004: done
