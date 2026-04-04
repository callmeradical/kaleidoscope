package cmd

import (
	"errors"
	"os"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/diffreport"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunDiffReport implements the `ks diff-report` command.
func RunDiffReport(args []string) {
	// Parse optional snapshot ID (first non-flag positional arg).
	snapshotID := ""
	for _, a := range args {
		if len(a) > 0 && a[0] != '-' {
			snapshotID = a
			break
		}
	}

	// Parse --output flag; default to .kaleidoscope/diff-report.html.
	outputPath := getFlagValue(args, "--output")
	if outputPath == "" {
		outputPath = ".kaleidoscope/diff-report.html"
	}

	// Step 1: Load baselines.
	baselines, err := snapshot.LoadBaselines()
	if err != nil {
		output.Fail("diff-report", err, "failed to load baselines")
		os.Exit(2)
	}

	// Step 2: Load current snapshot.
	var currentSnap *snapshot.Snapshot
	if snapshotID != "" {
		currentSnap, err = snapshot.Load(snapshotID)
	} else {
		currentSnap, err = snapshot.Latest()
	}
	if err != nil {
		output.Fail("diff-report", err, "No snapshots found. Run `ks snapshot` first")
		os.Exit(2)
	}

	// Step 3: Find baseline snapshot ID.
	baselineIDs := make(map[string]struct{})
	for _, us := range currentSnap.URLs {
		if id, ok := baselines.BaselineFor(us.URL); ok {
			baselineIDs[id] = struct{}{}
		}
	}
	if len(baselineIDs) == 0 {
		output.Fail("diff-report", errors.New("no baseline set"), "No baseline set. Run `ks snapshot --set-baseline <id>`")
		os.Exit(2)
	}

	// Use the first baseline ID found (common case: one baseline per project).
	var baselineID string
	for id := range baselineIDs {
		baselineID = id
		break
	}

	baselineSnap, err := snapshot.Load(baselineID)
	if err != nil {
		output.Fail("diff-report", err, "failed to load baseline snapshot")
		os.Exit(2)
	}

	// Step 4: Compute diff.
	result, err := diff.Compute(baselineSnap, currentSnap)
	if err != nil {
		output.Fail("diff-report", err, "diff computation failed")
		os.Exit(2)
	}

	// Step 5: Build HTML data model.
	data, err := diffreport.Build(result, baselineSnap, currentSnap)
	if err != nil {
		output.Fail("diff-report", err, "failed to build report data")
		os.Exit(2)
	}

	// Step 6: Write file.
	absPath, err := diffreport.WriteFile(outputPath, data)
	if err != nil {
		output.Fail("diff-report", err, "failed to write report file")
		os.Exit(2)
	}

	// Step 7: Emit success JSON.
	output.Success("diff-report", map[string]any{
		"path":           absPath,
		"baselineId":     result.BaselineID,
		"currentId":      result.CurrentID,
		"hasRegressions": result.HasRegressions,
		"urlCount":       len(result.URLs),
	})
}
