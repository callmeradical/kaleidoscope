package cmd

import (
	"errors"
	"os"

	diffpkg "github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunDiff compares a snapshot against the baseline and emits structured JSON.
// If args[0] is provided it is used as the snapshot ID; otherwise the latest
// snapshot is used. Exits with code 1 when regressions are detected, 2 on
// any other error.
func RunDiff(args []string) {
	// Load baseline.
	baseline, err := snapshot.LoadBaseline()
	if err != nil {
		hint := ""
		if errors.Is(err, snapshot.ErrNoBaseline) {
			hint = "Run: ks snapshot --set-baseline"
		}
		output.Fail("diff", err, hint)
		os.Exit(2)
	}

	// Load target snapshot.
	var id string
	if len(args) > 0 && args[0] != "" {
		id = args[0]
	}

	var target *snapshot.Snapshot
	if id != "" {
		target, err = snapshot.LoadSnapshot(id)
	} else {
		target, err = snapshot.LoadLatestSnapshot()
	}
	if err != nil {
		output.Fail("diff", err, "")
		os.Exit(2)
	}

	result := diffpkg.Compare(baseline, target)
	output.Success("diff", result)

	if result.Regressions {
		os.Exit(1)
	}
}
