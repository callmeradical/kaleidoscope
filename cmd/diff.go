package cmd

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunDiff implements `ks diff [snapshot-id]`.
func RunDiff(args []string) {
	storeRoot, err := snapshotStoreRoot()
	if err != nil {
		output.Fail("diff", err, "Cannot determine state directory")
		os.Exit(2)
	}

	store := snapshot.NewStore(storeRoot)

	// Load baseline
	bf, err := store.LoadBaseline()
	if err != nil {
		output.Fail("diff", fmt.Errorf("no baseline found"), "Run 'ks snapshot' to capture a baseline first")
		os.Exit(2)
	}

	baselineID := bf.Current

	// Determine target snapshot ID
	targetID := getArg(args)
	if targetID == "" {
		// Default to latest snapshot
		manifests, err := store.List()
		if err != nil || len(manifests) == 0 {
			output.Fail("diff", fmt.Errorf("no snapshots found"), "Run 'ks snapshot' first")
			os.Exit(2)
		}
		targetID = manifests[0].ID
	}

	if targetID == baselineID {
		// Comparing baseline to itself — no regressions by definition
		result := &diff.DiffResult{
			HasRegressions: false,
			AuditDeltas:    make(map[string]*diff.AuditDelta),
		}
		output.Success("diff", map[string]any{
			"baselineID": baselineID,
			"snapshotID": targetID,
			"sameSnapshot": true,
			"diff":       result,
		})
		return
	}

	// Load both manifests to get paths
	baselineManifest, err := store.Get(baselineID)
	if err != nil {
		output.Fail("diff", err, "Baseline snapshot not found")
		os.Exit(2)
	}

	targetManifest, err := store.Get(targetID)
	if err != nil {
		output.Fail("diff", err, fmt.Sprintf("Snapshot %q not found", targetID))
		os.Exit(2)
	}

	// Aggregate diff across all paths
	aggregated := &diff.DiffResult{
		AuditDeltas: make(map[string]*diff.AuditDelta),
	}

	// Use union of paths from both snapshots
	pathSet := make(map[string]bool)
	for _, p := range baselineManifest.Paths {
		pathSet[p] = true
	}
	for _, p := range targetManifest.Paths {
		pathSet[p] = true
	}

	for urlPath := range pathSet {
		baseAudit, baseAxTree, _ := store.LoadPathData(baselineID, urlPath)
		currAudit, currAxTree, _ := store.LoadPathData(targetID, urlPath)

		baseData := diff.SnapshotData{Audit: baseAudit, AxTree: baseAxTree}
		currData := diff.SnapshotData{Audit: currAudit, AxTree: currAxTree}

		pathDiff := diff.Compare(baseData, currData)

		// Merge audit deltas
		for cat, delta := range pathDiff.AuditDeltas {
			if existing, ok := aggregated.AuditDeltas[cat]; ok {
				existing.BaselineCount += delta.BaselineCount
				existing.CurrentCount += delta.CurrentCount
				existing.Delta += delta.Delta
			} else {
				cp := *delta
				aggregated.AuditDeltas[cat] = &cp
			}
		}

		// Merge element changes
		aggregated.ElementChanges = append(aggregated.ElementChanges, pathDiff.ElementChanges...)

		// Merge summary
		aggregated.Summary.NewAuditIssues += pathDiff.Summary.NewAuditIssues
		aggregated.Summary.ResolvedAuditIssues += pathDiff.Summary.ResolvedAuditIssues
		aggregated.Summary.ElementsAppeared += pathDiff.Summary.ElementsAppeared
		aggregated.Summary.ElementsDisappeared += pathDiff.Summary.ElementsDisappeared
		aggregated.Summary.ElementsMoved += pathDiff.Summary.ElementsMoved
		aggregated.Summary.ElementsResized += pathDiff.Summary.ElementsResized

		if pathDiff.HasRegressions {
			aggregated.HasRegressions = true
		}
	}

	output.Success("diff", map[string]any{
		"baselineID": baselineID,
		"snapshotID": targetID,
		"diff":       aggregated,
	})

	if aggregated.HasRegressions {
		os.Exit(1)
	}
}
