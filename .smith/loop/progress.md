# Progress Log

## US-006: Side-by-Side HTML Diff Report

### Run: kal-42e0c-autofix-kal-12c91-github-callmer-us-006 | Iteration 1 of 10

**Date:** 2026-04-04
**Status:** in_progress (tests-only iteration)

#### Files Created
| File | Purpose |
|------|---------|
| `snapshot/store.go` | Data types (Rect, AXElement, AuditSummary, BreakpointCapture, URLSnapshot, Snapshot) + Store/BaselineManager interfaces |
| `diff/engine.go` | Data types (AuditDelta, ElementChange, PixelDiff, URLDiff, DiffResult) + stub Compare function |
| `report/diff_report.go` | DiffData + support types, stub GenerateDiffReport (minimal template), WriteDiffFile |
| `cmd/diff_report.go` | RunDiffReport + stub buildDiffData + stub store/baseline implementations |
| `report/diff_report_test.go` | 17 tests covering HTML generation, CSS classes, escaping, file writing |
| `cmd/diff_report_test.go` | 11 tests covering buildDiffData mapping, URL count, audit delta, element changes, pixel diff |

#### Files Modified
| File | Change |
|------|--------|
| `main.go` | Added `case "diff-report": cmd.RunDiffReport(cmdArgs)` and usage entry |

#### Commands Run
```
go build ./...   → PASS (compiles cleanly)
go test ./...    → FAIL (expected for tests-only iteration)
```

#### Test Results
- `report` package: 4 pass, 13 fail (stub template missing all dynamic content)
- `cmd` package: 0 pass, 11 fail (buildDiffData stub returns empty DiffData)

**Passing:** `TestGenerateDiffReport_EmptyData`, `TestGenerateDiffReport_ElementChanges_Hidden`,
`TestWriteDiffFile_CreatesFile`, `TestWriteDiffFile_ContainsHTML`

**Failing (expected):** All tests that assert specific HTML structure, CSS classes, audit delta
mapping, element change mapping, and PixelDiff → HasDiff propagation.

#### Key Learnings
- Stub template with `<p>stub</p>` correctly forces test failures for all content assertions.
- `buildDiffData` returning `&report.DiffData{}` causes all mapping tests to fail — correct red state.
- `go test ./...` exit code is 1; compilation is clean.
- `cmd/util.go` `getNonFlagArgs` already handles `--output` flag, no changes needed there.

---

### Run: kal-42e0c-autofix-kal-12c91-github-callmer-us-006 | Iteration 2 of 10

**Date:** 2026-04-04
**Status:** done

#### Files Modified
| File | Change |
|------|--------|
| `report/diff_report.go` | Replaced stub template with full `diffHTMLTemplate` (side-by-side layout, audit delta table, element changes table, CSS variables) |
| `report/diff_report.go` | Changed `signedDelta` return type to `template.HTML` to prevent `+` being escaped as `&#43;` |
| `cmd/diff_report.go` | Implemented full `buildDiffData` (URL/breakpoint/audit-delta/element-change/PixelDiff mapping) |
| `cmd/diff_report.go` | Added `"time"` import |

#### Commands Run
```
go build ./...  → PASS
go test ./...   → PASS (all packages)
go vet ./...    → PASS
```

#### Test Results
- `report` package: 17/17 pass
- `cmd` package: 11/11 pass
- All other packages: no test files (expected)

#### Key Learnings
- `html/template` escapes `+` as `&#43;` when returned from a FuncMap function that returns `string`. Return `template.HTML` for numeric-only outputs with sign characters to avoid this.
- CSS class names used in Go template must NOT appear in the `<style>` block unconditionally if tests assert their absence. Solution: place class definitions inside the `{{if .ElementChanges}}` conditional block.
- `report.LoadScreenshot("")` returns `("", error)` — safe to call with empty path (error discarded) to get zero-value `template.URL`.

#### Story Status: DONE
