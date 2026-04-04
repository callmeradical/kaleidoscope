package cmd

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunDiff implements the `ks diff [snapshot-id]` command.
// It compares a snapshot (default: latest) against the project baseline and
// outputs structured JSON. Exits with code 1 if regressions are detected,
// code 2 on errors.
func RunDiff(args []string) {
	snapshotID := getArg(args)

	baseline, err := snapshot.LoadBaseline()
	if err != nil {
		output.Fail("diff", fmt.Errorf("no baseline set: %w", err), "Run: ks snapshot set-baseline <snapshot-id>")
		os.Exit(2)
	}

	var current *snapshot.Snapshot
	if snapshotID == "" {
		current, err = snapshot.LoadLatest()
	} else {
		current, err = snapshot.Load(snapshotID)
	}
	if err != nil {
		output.Fail("diff", err, "Run: ks snapshot list")
		os.Exit(2)
	}

	auditDelta := diff.ComputeAuditDelta(baseline.AuditData, current.AuditData)
	elementDelta := diff.ComputeElementDelta(baseline.AXNodes, current.AXNodes)

	result := diff.DiffResult{
		SnapshotID:  current.ID,
		BaselineID:  baseline.ID,
		Regressions: auditDelta.HasRegression || elementDelta.HasRegression,
		Audit:       auditDelta,
		Elements:    elementDelta,
	}

	output.Success("diff", result)

	if result.Regressions {
		os.Exit(1)
	}
}
