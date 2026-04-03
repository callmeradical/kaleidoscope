package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/report"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

var snapshotIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// RunDiffReport implements the `ks diff-report` command.
func RunDiffReport(args []string) {
	snapshotID := getArg(args)
	if snapshotID == "" {
		snapshotID = "latest"
	}

	outputPath := getFlagValue(args, "--output")

	if !snapshotIDPattern.MatchString(snapshotID) {
		output.Fail("diff-report", errors.New("invalid snapshot ID: "+snapshotID),
			"Snapshot IDs may only contain alphanumeric characters, underscores, and hyphens")
		os.Exit(2)
	}

	stateDir, err := browser.StateDir()
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	current, err := snapshot.Load(stateDir, snapshotID)
	if err != nil {
		output.Fail("diff-report", err, "Run 'ks snapshot' first")
		os.Exit(2)
	}

	baselines, err := snapshot.LoadBaseline(stateDir)
	if err != nil {
		output.Fail("diff-report", err, "Run 'ks baseline set' first")
		os.Exit(2)
	}

	baselineID := resolveBaselineID(baselines, current)
	if baselineID == "" {
		output.Fail("diff-report",
			errors.New("no baseline found for any URL in current snapshot"),
			"Run 'ks baseline set' first")
		os.Exit(2)
	}

	baselineSnap, err := snapshot.Load(stateDir, baselineID)
	if err != nil {
		output.Fail("diff-report",
			fmt.Errorf("snapshot '%s' not found: %w", baselineID, err),
			"Run 'ks snapshot list' to see available snapshots")
		os.Exit(2)
	}

	snapshotDiff := diff.Compare(baselineSnap, current)

	data, err := report.BuildDiffData(snapshotDiff)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	if outputPath == "" {
		outputPath = filepath.Join(stateDir, "diff-report.html")
	}

	absPath, err := report.WriteDiffFile(outputPath, data)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	output.Success("diff-report", map[string]any{
		"path":       absPath,
		"baselineId": snapshotDiff.BaselineID,
		"currentId":  snapshotDiff.CurrentID,
		"urlCount":   len(snapshotDiff.URLs),
	})
}

// resolveBaselineID picks the baseline snapshot ID from the baselines map.
// It checks for a global baseline (single entry) or per-URL entries matching the current snapshot.
func resolveBaselineID(baselines map[string]string, current *snapshot.Snapshot) string {
	if len(baselines) == 1 {
		for _, id := range baselines {
			return id
		}
	}
	for _, u := range current.URLs {
		if id, ok := baselines[u.URL]; ok {
			return id
		}
	}
	return ""
}
