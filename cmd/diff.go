package cmd

import (
	"os"
	"strconv"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// parseFlagFloat reads a float flag value, returning defaultVal if absent or unparseable.
func parseFlagFloat(args []string, flag string, defaultVal float64) float64 {
	raw := getFlagValue(args, flag)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

// RunDiff compares a snapshot against the baseline and outputs structured JSON.
func RunDiff(args []string) {
	if PrintUsage("diff", args) {
		return
	}

	snapshotID := getArg(args)
	posThreshold := parseFlagFloat(args, "--pos-threshold", 4.0)
	sizeThreshold := parseFlagFloat(args, "--size-threshold", 4.0)

	baseline, err := snapshot.LoadBaseline()
	if err != nil {
		output.Fail("diff", err, "Run: ks baseline set <snapshot-id>")
		os.Exit(2)
	}

	var target *snapshot.Snapshot
	if snapshotID == "" {
		target, err = snapshot.LoadLatest()
	} else {
		target, err = snapshot.LoadByID(snapshotID)
	}
	if err != nil {
		output.Fail("diff", err, "")
		os.Exit(2)
	}

	auditDiff := diff.ComputeAuditDiff(baseline.AuditData, target.AuditData)
	elemDiff := diff.ComputeElementDiff(baseline.Elements, target.Elements, posThreshold, sizeThreshold)

	result := diff.DiffResult{
		SnapshotID:    target.ID,
		BaselineID:    baseline.ID,
		Audit:         auditDiff,
		Elements:      elemDiff,
		HasRegression: auditDiff.HasRegression || elemDiff.HasRegression,
	}

	output.Success("diff", result)

	if result.HasRegression {
		os.Exit(1)
	}
}
