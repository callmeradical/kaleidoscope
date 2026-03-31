package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/callmeradical/kaleidoscope/browser"
	diffpkg "github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunDiff(args []string) {
	snapshotID := getArg(args)

	proj, err := snapshot.LoadProject()
	if err != nil {
		output.Fail("diff", err, "Run 'ks init' to create a project first")
		os.Exit(2)
	}

	stateDir, err := browser.StateDir()
	if err != nil {
		output.Fail("diff", err, "")
		os.Exit(2)
	}

	if snapshotID == "" {
		latest, err := snapshot.LatestSnapshot(stateDir)
		if err != nil || latest == nil {
			output.Fail("diff", fmt.Errorf("no snapshots found"), "Run 'ks snapshot' first")
			os.Exit(2)
		}
		snapshotID = latest.ID
	}

	baselines, err := snapshot.LoadBaselines(stateDir)
	if err != nil {
		output.Fail("diff", err, "")
		os.Exit(2)
	}
	if len(baselines) == 0 {
		output.Fail("diff", fmt.Errorf("no baseline found"), "Run 'ks snapshot' to create a baseline first")
		os.Exit(2)
	}

	snapPath := snapshot.SnapshotPath(stateDir, snapshotID)

	var urlDiffs []diffpkg.URLDiff
	hasRegressions := false

	for _, urlPath := range proj.Paths {
		baselineID, ok := baselines[urlPath]
		if !ok {
			continue
		}

		baseAudit, _ := snapshot.ReadAuditJSON(stateDir, baselineID, urlPath)
		currAudit, _ := snapshot.ReadAuditJSON(stateDir, snapshotID, urlPath)

		var auditDeltas []diffpkg.AuditDelta
		if baseAudit != nil && currAudit != nil {
			auditDeltas = diffpkg.DiffAudit(baseAudit, currAudit)
		}

		baseAX, _ := snapshot.ReadAxTreeJSON(stateDir, baselineID, urlPath)
		currAX, _ := snapshot.ReadAxTreeJSON(stateDir, snapshotID, urlPath)

		var elemChanges []diffpkg.ElementChange
		if baseAX != nil && currAX != nil {
			elemChanges = diffpkg.DiffAxTree(baseAX, currAX)
		}

		for _, bp := range proj.Breakpoints {
			basePNG := snapshot.ScreenshotPath(stateDir, baselineID, urlPath, bp.Name, bp.Width, bp.Height)
			currPNG := snapshot.ScreenshotPath(stateDir, snapshotID, urlPath, bp.Name, bp.Width, bp.Height)

			if _, err := os.Stat(basePNG); os.IsNotExist(err) {
				continue
			}
			if _, err := os.Stat(currPNG); os.IsNotExist(err) {
				continue
			}

			diffPNG := filepath.Join(snapPath, snapshot.URLToPath(urlPath), fmt.Sprintf("diff-%s-%dx%d.png", bp.Name, bp.Width, bp.Height))
			_, _ = diffpkg.DiffScreenshots(basePNG, currPNG, diffPNG)
		}

		urlRegression := diffpkg.HasRegressions(auditDeltas, elemChanges)
		if urlRegression {
			hasRegressions = true
		}

		urlDiffs = append(urlDiffs, diffpkg.URLDiff{
			Path:           urlPath,
			AuditDeltas:    auditDeltas,
			ElementChanges: elemChanges,
			HasRegressions: urlRegression,
		})
	}

	result := diffpkg.Result{
		BaselineID:  getAnyBaselineID(baselines),
		CurrentID:   snapshotID,
		URLs:        urlDiffs,
		Regressions: hasRegressions,
	}

	output.Success("diff", result)

	if hasRegressions {
		os.Exit(1)
	}
}

func getAnyBaselineID(b snapshot.Baselines) string {
	for _, id := range b {
		return id
	}
	return ""
}


