# Progress Log

## US-006: Side-by-Side HTML Diff Report

### Run kal-b4ba8-autofix-kal-12c91-github-callmer-us-006 · Iteration 1

**Status:** in_progress

**Goal:** Tests-only iteration — write failing tests before implementation.

**Files Created:**
- `diff/pixel.go` — stub package `diff` with `Overlay()` and `Score()` returning zero values
- `diff/pixel_test.go` — 4 tests: `TestOverlay_identical`, `TestOverlay_fullyDifferent`, `TestOverlay_differentSizes`, `TestScore_threshold`
- `report/diff_report.go` — stub types (`DiffData`, `DiffPage`, `DiffBreakpoint`, `AuditDelta`, `CategoryDelta`, `ElementChangeRow`) and no-op `GenerateDiff()`/`WriteDiffFile()`
- `report/diff_report_test.go` — 3 tests: `TestGenerateDiff_smoke`, `TestGenerateDiff_missingImages`, `TestGenerateDiff_deltaColors`

**Quality Gate Result:** `go test ./...` — 7 tests FAIL as expected (stubs not implemented)

```
FAIL github.com/callmeradical/kaleidoscope/diff      (4 failures)
FAIL github.com/callmeradical/kaleidoscope/report    (3 failures)
```

**Key Learnings:**
- Go binary at `/usr/local/go/bin/go` (not in default PATH; use `PATH=/usr/local/go/bin:$PATH`)
- `diff/` package uses package name `diff` (not `pixeldiff`); imported with alias in tests as `pixeldiff "...kaleidoscope/diff"`
- Stub stubs compile clean (`go build ./...` passes)
- Next iteration: implement `Overlay()`, `Score()`, and `GenerateDiff()` to make tests pass

---

### Run kal-b4ba8-autofix-kal-12c91-github-callmer-us-006 · Iteration 2

**Status:** done

**Goal:** Implement production code to make all 7 failing tests pass.

**Files Modified/Created:**
- `diff/pixel.go` — full implementation of `Overlay()`, `Score()`, helper functions (`pixelAt`, `absDiff`, `maxChannelDelta`, `unionBounds`, `defaultOptions`)
- `report/diff_report.go` — full `GenerateDiff()` with HTML template (`diffReportTmpl`), helper funcs (`deltaClass`, `deltaSign`, `scoreClass`, `pct`); `WriteDiffFile()` unchanged
- `snapshot/snapshot.go` — NEW: minimal snapshot package with `Snapshot`/`PageSnapshot`/`BreakpointShot`/`AuditSummary`/`ElementRecord` types and `LoadBaseline()`/`Load()`/`LoadLatest()` functions
- `cmd/diff_report.go` — NEW: `RunDiffReport()` command handler with `buildDiffData()`, `loadImageB64()`, `encodeOverlayB64()`, `computeAuditDelta()`, `computeElementChanges()`
- `main.go` — added `"diff-report"` case + help text
- `cmd/usage.go` — added `"diff-report"` usage entry

**Quality Gate Result:** `go test ./...` — ALL PASS

```
ok  github.com/callmeradical/kaleidoscope/diff    0.003s  (4 tests)
ok  github.com/callmeradical/kaleidoscope/report  0.006s  (3 tests incl. 3 subtests)
```

**Key Learnings:**
- `pixelAt` uses `image.Point.In(bounds)` for bounds check; `uint8(r32>>8)` converts 16-bit premultiplied RGBA back to 8-bit
- `unionBounds` handles images of different sizes; extra rows of shorter image treated as zero (always different from img2's pixel)
- snapshot package created as US-003 stub to allow cmd/diff_report.go to compile; returns graceful errors when no snapshots/baseline exist
- HTML template uses `{{if .BaselineURI}}` — `template.URL("")` is falsy in Go templates ✓
- `deltaClass` func in FuncMap generates CSS classes used directly in template; tested via `TestGenerateDiff_deltaColors`

