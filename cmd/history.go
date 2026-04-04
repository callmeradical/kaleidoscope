package cmd

import (
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunHistory(args []string) {
	ids, err := snapshot.ListSnapshotIDs()
	if err != nil {
		output.Fail("history", err, "")
		os.Exit(2)
	}

	baselineID := ""
	baseline, _ := snapshot.ReadBaselineManifest()
	if baseline != nil {
		baselineID = baseline.BaselineID
	}

	if len(ids) == 0 {
		output.Success("history", map[string]any{
			"snapshots":  []any{},
			"baselineId": baselineID,
		})
		return
	}

	snapshotsDir, err := snapshot.SnapshotsDir()
	if err != nil {
		output.Fail("history", err, "")
		os.Exit(2)
	}

	var snapshots []map[string]any
	for _, id := range ids {
		snapshotPath := filepath.Join(snapshotsDir, id)
		m, readErr := snapshot.ReadManifest(snapshotPath)
		entry := map[string]any{
			"id":         id,
			"isBaseline": id == baselineID,
		}
		if readErr != nil || m == nil {
			entry["timestamp"] = nil
			entry["commitHash"] = nil
			entry["urls"] = nil
			entry["reachableUrls"] = nil
			entry["totalIssues"] = nil
			entry["totalAXNodes"] = nil
		} else {
			entry["timestamp"] = m.Timestamp
			entry["commitHash"] = m.CommitHash
			entry["urls"] = m.Summary.TotalURLs
			entry["reachableUrls"] = m.Summary.ReachableURLs
			entry["totalIssues"] = m.Summary.TotalIssues
			entry["totalAXNodes"] = m.Summary.TotalAXNodes
		}
		snapshots = append(snapshots, entry)
	}

	output.Success("history", map[string]any{
		"snapshots":  snapshots,
		"baselineId": baselineID,
	})
}
