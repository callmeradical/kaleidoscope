package cmd

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunAccept implements `ks accept [snapshot-id] [--url <path>]`.
func RunAccept(args []string) {
	// Load the snapshot index.
	index, err := snapshot.LoadIndex()
	if err != nil {
		output.Fail("accept", err, "")
		os.Exit(2)
	}
	if len(index.Snapshots) == 0 {
		output.Fail("accept", fmt.Errorf("no snapshots exist"), "Run: ks snapshot")
		os.Exit(2)
	}

	// Resolve snapshot ID (positional arg or latest).
	snapshotID := getArg(args)
	var meta *snapshot.SnapshotMeta
	if snapshotID == "" {
		m := index.Snapshots[len(index.Snapshots)-1]
		meta = &m
	} else {
		for i := range index.Snapshots {
			if index.Snapshots[i].ID == snapshotID {
				meta = &index.Snapshots[i]
				break
			}
		}
		if meta == nil {
			output.Fail("accept", fmt.Errorf("snapshot not found: %s", snapshotID), "")
			os.Exit(2)
		}
	}

	// Resolve --url filter.
	urlFilter := getFlagValue(args, "--url")
	var urls []string
	if urlFilter != "" {
		found := false
		for _, u := range meta.URLs {
			if u == urlFilter {
				found = true
				break
			}
		}
		if !found {
			output.Fail("accept", fmt.Errorf("url %q not in snapshot %s", urlFilter, meta.ID), "")
			os.Exit(2)
		}
		urls = []string{urlFilter}
	}

	// Load baselines, apply accept, save.
	baselines, err := snapshot.LoadBaselines()
	if err != nil {
		output.Fail("accept", err, "")
		os.Exit(2)
	}

	updated, changed := snapshot.Accept(baselines, meta, urls)

	if err := snapshot.SaveBaselines(updated); err != nil {
		output.Fail("accept", err, "")
		os.Exit(2)
	}

	output.Success("accept", map[string]any{
		"snapshotId": meta.ID,
		"changed":    changed,
		"noOp":       len(changed) == 0,
		"baselines":  updated.Entries,
	})
}
