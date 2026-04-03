package cmd

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/callmeradical/kaleidoscope/baseline"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

var validSnapshotID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// RunAccept implements the `ks accept` command.
// It promotes a snapshot to baseline in .kaleidoscope/baselines.json.
func RunAccept(args []string) {
	dir, err := browser.StateDir()
	if err != nil {
		output.Fail("accept", err, "")
		return
	}

	snapshotID := getArg(args)
	urlFilter := getFlagValue(args, "--url")

	if snapshotID != "" && !validSnapshotID.MatchString(snapshotID) {
		output.Fail("accept", errors.New("invalid snapshot ID: contains invalid characters"), "")
		return
	}

	idx, err := snapshot.LoadIndex(dir)
	if err != nil {
		output.Fail("accept", err, "")
		return
	}
	if len(idx.Snapshots) == 0 {
		output.Fail("accept", errors.New("no snapshots found"), "run `ks snapshot` first")
		return
	}

	var snap *snapshot.SnapshotEntry
	if snapshotID != "" {
		snap = idx.ByID(snapshotID)
		if snap == nil {
			output.Fail("accept", fmt.Errorf("snapshot %q not found", snapshotID), "")
			return
		}
	} else {
		snap = idx.Latest()
	}

	b, err := baseline.Load(dir)
	if err != nil {
		output.Fail("accept", err, "")
		return
	}

	updated := []string{}
	skipped := []string{}

	for _, u := range snap.URLs {
		if urlFilter != "" && u.Path != urlFilter {
			continue
		}
		if b.Accept(u, snap.ID) {
			updated = append(updated, u.Path)
		} else {
			skipped = append(skipped, u.Path)
		}
	}

	if urlFilter != "" && len(updated)+len(skipped) == 0 {
		output.Fail("accept", fmt.Errorf("no URL with path %q in snapshot %s", urlFilter, snap.ID), "")
		return
	}

	if err := b.Save(dir); err != nil {
		output.Fail("accept", err, "")
		return
	}

	output.Success("accept", map[string]any{
		"snapshot_id": snap.ID,
		"updated":     updated,
		"skipped":     skipped,
	})
}
