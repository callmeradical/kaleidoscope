# Progress Log

## US-004: Screenshot Pixel Diff

### Run: kal-29905-autofix-kal-9f4f2-github-callmer-us-004 | Iteration 2

**Status**: DONE

**Commands run**:
- `go test ./...` → PASS (all packages: cmd OK, diff OK)

**Files modified**:
- `diff/pixeldiff.go` — implemented `pixelDistance`, `renderDiff`, `CompareBytes`, `CompareFiles`

**Test results**:
- `diff` package: 7 PASS (TestIdenticalImages, TestCompletelyDifferent, TestPartialChange, TestDimensionMismatch, TestThresholdFlag, TestDiffImageOutput, TestCompareFiles)
- `cmd` package: 3 PASS (unchanged from iteration 1)

**Key learnings**:
- `color.RGBAModel.Convert(c).RGBA()` returns 0-65535; shift right 8 for 0-255 range
- Zero-value `color.RGBA{}` == `{0,0,0,0}`; use that to detect unset HighlightColor option

---

### Run: kal-29905-autofix-kal-9f4f2-github-callmer-us-004 | Iteration 1

**Status**: in_progress (tests-only iteration)

**Commands run**:
- `go build ./...` → PASS (compiles clean)
- `go test ./...` → FAIL (expected: 7 failing tests in diff package, 3 passing in cmd)

**Files created/modified**:
- `diff/pixeldiff.go` — new stub package with types and empty CompareBytes/CompareFiles
- `diff/pixeldiff_test.go` — 7 failing tests: TestIdenticalImages, TestCompletelyDifferent, TestPartialChange, TestDimensionMismatch, TestThresholdFlag, TestDiffImageOutput, TestCompareFiles
- `cmd/diff.go` — new stub RunDiff command (flag parsing, snapshot resolution, JSON output scaffold)
- `cmd/diff_test.go` — 3 tests: TestDiffJSONOutput (PASS), TestDiffMissingBaseline (PASS), TestDiffThresholdFlag (PASS)
- `cmd/util.go` — added --snapshot, --baseline, --threshold to getNonFlagArgs skip list
- `main.go` — added `case "diff": cmd.RunDiff(cmdArgs)` and usage entry

**Test results**:
- `diff` package: 7 FAIL (stubs return zero values — implementation needed next iteration)
- `cmd` package: 3 PASS (command scaffolding, flag parsing, JSON output format verified)

**Key learnings**:
- `os.Exit` in commands kills test processes; use subprocess pattern (KS_TEST_SUBPROCESS env var) for RunDiff error path testing
- `cmd` tests test the command routing/JSON structure; `diff` tests test the actual pixel logic
- Next iteration: implement CompareBytes/CompareFiles in diff/pixeldiff.go

