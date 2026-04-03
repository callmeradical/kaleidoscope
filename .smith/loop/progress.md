# Progress Log

## US-004: Screenshot Pixel Diff

### Run kal-a29c7-autofix-kal-9f4f2-github-callmer-us-004 — Iteration 1

**Status:** in_progress (tests-only iteration)

**Files Created:**
- `snapshot/snapshot.go` — types: Snapshot, SnapshotScreenshot, ScreenshotDiff, DiffOutput; LoadSnapshot/SaveSnapshot helpers
- `analysis/pixeldiff.go` — stub: ImageDiffResult type + CompareImages stub (returns zero value so tests fail)
- `analysis/pixeldiff_writer.go` — WriteDiffImage + sanitizeFilename helpers
- `analysis/pixeldiff_test.go` — 9 tests covering all acceptance criteria

**Test Run Result:**
```
go test ./...
FAIL github.com/callmeradical/kaleidoscope/analysis  (7 tests fail, 2 pass)
```

Failing tests (expected — TDD red phase):
- TestCompareImages_Identical: stub returns Similarity=0, DiffImage=nil
- TestCompareImages_TotallyDifferent: stub returns zero pixel counts
- TestCompareImages_PartialDiff: stub returns Similarity=0
- TestCompareImages_MismatchedDimensions: stub returns MismatchedDimensions=false
- TestCompareImages_ChannelThreshold (threshold=9 case): stub returns 0 different pixels
- TestCompareImages_DiffImageIsRed: stub returns nil DiffImage
- TestCompareImages_DiffImageIsAttenuated: stub returns nil DiffImage

Passing tests (stubs sufficient):
- TestCompareImages_ChannelThreshold (threshold=10 case): zero pixels also satisfies DifferentPixels==0
- TestWriteDiffImage_Roundtrip: WriteDiffImage has real implementation

---

### Run kal-a29c7-autofix-kal-9f4f2-github-callmer-us-004 — Iteration 2

**Status:** done

**Files Changed:**
- `analysis/pixeldiff.go` — Implemented CompareImages: pixel-by-pixel diff, threshold comparison, diff image generation (red for changed, attenuated original for unchanged), mismatched-dimensions guard.

**Test Run Result:**
```
go test ./...
ok  github.com/callmeradical/kaleidoscope/analysis  (9/9 tests pass)
```

All acceptance criteria satisfied:
- Identical images → Similarity=1.0, DifferentPixels=0
- Totally different → Similarity=0.0, DifferentPixels=totalPixels
- Partial diff → Similarity≈0.5
- Mismatched dimensions → MismatchedDimensions=true, Similarity=0.0, DiffImage=nil (no panic)
- Channel threshold → strictly > threshold flags pixel
- DiffImage: changed pixels red (255,0,0,255), unchanged pixels original color at alpha=128

**Key Patterns:**
- `image/color.NRGBAModel.Convert(img.At(x,y)).(color.NRGBA)` is the correct way to decode pixels
- Pixel iteration must use `a.Bounds()` range, not assume 0,0 origin
- Diff image uses relative coordinates (dx=x-boundsA.Min.X) for non-zero-origin images


### Run kal-a29c7-autofix-kal-9f4f2-github-callmer-us-004 — Iteration 3

**Status:** done

**Files Created:**
- `cmd/diff.go` — RunDiff command: loadProjectConfig, loadPNG, findMatchingScreenshot, sanitizeDiffName, diffScreenshots, RunDiff

**Files Modified:**
- `main.go` — added `diff` case to switch, added Snapshots section to usage string
- `cmd/usage.go` — added `"diff"` entry with full flag/output/example documentation

**Test Run Result:**
```
go test ./...
ok  github.com/callmeradical/kaleidoscope/analysis  (cached — 9/9 pass)
go build ./...  (clean)
```

**Acceptance Criteria Satisfied:**
- ✅ Given two PNGs of same dimensions → diff PNG with red highlighted pixels (Phase 2 + WriteDiffImage)
- ✅ Returns similarity score 0.0–1.0 per screenshot pair
- ✅ Diff images written to snapshot directory alongside source screenshots (diffScreenshots writes to filepath.Dir(currentPath))
- ✅ `ks diff` JSON output includes screenshotDiffs (paths, similarity, per URL per breakpoint)
- ✅ Mismatched dimensions: MismatchedDimensions=true, Regressed=true, DiffPath="", no panic
- ✅ Configurable threshold: .ks-project.json screenshotThreshold (default 0.99)

**Key Patterns:**
- `getFlagValue(args, "--baseline")` pattern reused from other cmd files
- `filepath.Dir(currentPath)` determines snapshotDir so diffs land alongside source screenshots
- `sanitizeDiffName` strips http(s):// then applies same [a-zA-Z0-9._-] whitelist as WriteDiffImage
