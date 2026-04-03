package cmd

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunDiff(args []string) {
	local := hasFlag(args, "--local")
	dir := snapshot.KaleidoscopeDir(local)

	// Determine target snapshot ID
	var targetID string
	for _, a := range args {
		if len(a) > 0 && a[0] != '-' {
			targetID = a
			break
		}
	}

	// Load baselines
	baselines, err := snapshot.LoadBaselines(dir)
	if err != nil {
		output.Fail("diff", err, "Run 'ks snapshot --set-baseline' to set a baseline first.")
		os.Exit(2)
	}

	baselineID := baselines.Default
	if baselineID == "" {
		fmt.Fprintln(os.Stderr, "diff: no default baseline set in baselines.json")
		os.Exit(2)
	}

	// Resolve target ID
	if targetID == "" {
		targetID, err = snapshot.LatestID(dir)
		if err != nil {
			output.Fail("diff", fmt.Errorf("no snapshots found: %w", err), "Take a snapshot first.")
			os.Exit(2)
		}
	}

	// Load both snapshots
	baseSnap, err := snapshot.Load(dir, baselineID)
	if err != nil {
		output.Fail("diff", fmt.Errorf("loading baseline %q: %w", baselineID, err), "")
		os.Exit(2)
	}

	targetSnap, err := snapshot.Load(dir, targetID)
	if err != nil {
		output.Fail("diff", fmt.Errorf("loading snapshot %q: %w", targetID, err), "")
		os.Exit(2)
	}

	// Run diff
	result := diff.Run(baseSnap, targetSnap, diff.DefaultThresholds())

	output.Success("diff", result)

	if result.HasRegressions {
		os.Exit(1)
	}
}
