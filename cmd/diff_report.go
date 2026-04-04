package cmd

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/report"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunDiffReport generates a side-by-side HTML diff report comparing the
// baseline snapshot against the latest (or a specified) snapshot.
func RunDiffReport(args []string) {
	// Optional positional snapshot ID.
	var snapshotID string
	for _, a := range args {
		if len(a) > 0 && a[0] != '-' {
			snapshotID = a
			break
		}
	}

	outputPath := getFlagValue(args, "--output")

	dir, err := browser.StateDir()
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	// Load baseline.
	baseline, err := snapshot.LoadBaseline(dir)
	if err != nil {
		output.Fail("diff-report", err, "Run: ks baseline set")
		os.Exit(2)
	}

	// Load current snapshot.
	var current *snapshot.Snapshot
	if snapshotID != "" {
		current, err = snapshot.Load(dir, snapshotID)
		if err != nil {
			output.Fail("diff-report", err, "Run: ks snapshot list to see available IDs")
			os.Exit(2)
		}
	} else {
		current, err = snapshot.Latest(dir)
		if err != nil {
			output.Fail("diff-report", err, "Run: ks snapshot to capture one")
			os.Exit(2)
		}
	}

	// Run diff engine.
	d, err := diff.Compare(baseline, current)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	// Run pixel diff for each breakpoint (non-fatal on error).
	for pi := range d.Pages {
		for bi := range d.Pages[pi].BreakpointDiffs {
			bd := &d.Pages[pi].BreakpointDiffs[bi]
			if bd.BaselinePath == "" || bd.CurrentPath == "" {
				continue
			}
			score, overlay, pixErr := diff.PixelDiff(bd.BaselinePath, bd.CurrentPath)
			if pixErr != nil {
				fmt.Fprintf(os.Stderr, "warning: pixel diff failed for %s/%s: %v\n", d.Pages[pi].URL, bd.Name, pixErr)
				continue
			}
			bd.DiffScore = score
			bd.DiffImageBytes = overlay
		}
	}

	// Build view model.
	data, err := report.BuildDiffData(d, baseline, current)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	// Write HTML report.
	reportPath, err := report.WriteDiffFile(outputPath, dir, data)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	output.Success("diff-report", map[string]any{
		"path":        reportPath,
		"baselineId":  baseline.ID,
		"currentId":   current.ID,
		"pages":       len(d.Pages),
		"regressions": diff.CountRegressions(d),
	})
}
