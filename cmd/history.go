package cmd

import (
	"os"
	"strconv"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunHistory(args []string) {
	// Parse --limit flag
	limit := 0
	if limitStr := getFlagValue(args, "--limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			output.Fail("history", nil, "--limit must be a non-negative integer")
			os.Exit(2)
		}
	}

	entries, err := snapshot.List()
	if err != nil {
		output.Fail("history", err, "Failed to list snapshots.")
		os.Exit(2)
	}

	// Mark baseline
	bl, _ := snapshot.LoadBaselines()
	for i := range entries {
		entries[i].IsBaseline = bl != nil && bl.SnapshotID == entries[i].ID
	}

	// Apply limit
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}

	output.Success("history", map[string]any{
		"snapshots": entries,
	})
}
