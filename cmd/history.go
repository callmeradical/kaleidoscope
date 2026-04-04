package cmd

import (
	"os"
	"time"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

type snapshotSummary struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	CommitHash string    `json:"commitHash,omitempty"`
	URLCount   int       `json:"urlCount"`
	TotalIssues int      `json:"totalIssues"`
	IsBaseline bool      `json:"isBaseline"`
}

func RunHistory(args []string) {
	manifests, err := snapshot.List()
	if err != nil {
		output.Fail("history", err, "Run: ks snapshot")
		os.Exit(2)
	}

	baseline, _ := snapshot.LoadBaseline()
	baselineID := ""
	if baseline != nil {
		baselineID = baseline.SnapshotID
	}

	summaries := make([]snapshotSummary, 0, len(manifests))
	for _, m := range manifests {
		totalIssues := 0
		for _, u := range m.URLs {
			totalIssues += u.AuditSummary.TotalIssues
		}
		summaries = append(summaries, snapshotSummary{
			ID:          m.ID,
			Timestamp:   m.Timestamp,
			CommitHash:  m.CommitHash,
			URLCount:    len(m.URLs),
			TotalIssues: totalIssues,
			IsBaseline:  m.ID == baselineID,
		})
	}

	output.Success("history", map[string]any{
		"count":     len(summaries),
		"baseline":  baselineID,
		"snapshots": summaries,
	})
}
