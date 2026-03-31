package cmd

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunAccept(args []string) {
	snapshotID := getArg(args)
	urlFilter := getFlagValue(args, "--url")

	proj, err := snapshot.LoadProject()
	if err != nil {
		output.Fail("accept", err, "Run 'ks init' to create a project first")
		os.Exit(2)
	}

	stateDir, err := browser.StateDir()
	if err != nil {
		output.Fail("accept", err, "")
		os.Exit(2)
	}

	if snapshotID == "" {
		latest, err := snapshot.LatestSnapshot(stateDir)
		if err != nil || latest == nil {
			output.Fail("accept", fmt.Errorf("no snapshots found"), "Run 'ks snapshot' first")
			os.Exit(2)
		}
		snapshotID = latest.ID
	}

	if _, err := snapshot.ReadManifest(snapshot.SnapshotPath(stateDir, snapshotID)); err != nil {
		output.Fail("accept", fmt.Errorf("snapshot %q not found: %w", snapshotID, err), "")
		os.Exit(2)
	}

	baselines, err := snapshot.LoadBaselines(stateDir)
	if err != nil {
		output.Fail("accept", err, "")
		os.Exit(2)
	}

	var updated []string
	if urlFilter != "" {
		baselines[urlFilter] = snapshotID
		updated = append(updated, urlFilter)
	} else {
		for _, p := range proj.Paths {
			baselines[p] = snapshotID
			updated = append(updated, p)
		}
	}

	if err := snapshot.SaveBaselines(stateDir, baselines); err != nil {
		output.Fail("accept", err, "")
		os.Exit(2)
	}

	output.Success("accept", map[string]any{
		"snapshotId":  snapshotID,
		"updatedURLs": updated,
		"baselines":   baselines,
	})
}
