package cmd

import (
	"os"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// osExit is a variable so tests can override it to avoid process termination.
var osExit = os.Exit

// RunAccept implements `ks accept [snapshot-id] [--url <path>]`.
func RunAccept(args []string) {
	snapshotID := getArg(args)
	urlPath := getFlagValue(args, "--url")

	// Resolve target snapshot.
	var snap *snapshot.Snapshot
	var err error
	if snapshotID != "" {
		snap, err = snapshot.GetSnapshot(snapshotID)
	} else {
		snap, err = snapshot.LatestSnapshot()
	}
	if err != nil {
		output.Fail("accept", err, "Run 'ks snapshot' to create a snapshot first.")
		osExit(2)
		return
	}

	// Load current baselines.
	current, err := snapshot.ReadBaselines()
	if err != nil {
		output.Fail("accept", err, "")
		osExit(2)
		return
	}

	// Determine scope: empty urlPath means all URLs.
	scope := urlPath
	if scope == "" {
		scope = "*"
	}

	updated, wasNoOp, err := snapshot.AcceptSnapshot(current, snap, scope)
	if err != nil {
		output.Fail("accept", err, "")
		osExit(2)
		return
	}

	if !wasNoOp {
		if err := snapshot.WriteBaselines(updated); err != nil {
			output.Fail("accept", err, "")
			osExit(2)
			return
		}
	}

	output.Success("accept", map[string]any{
		"snapshotId": snap.ID,
		"noOp":       wasNoOp,
		"updated":    updated,
		"url":        urlPath,
	})
}
