package cmd

import (
	"os"
	"strconv"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunHistory lists snapshots in reverse chronological order.
func RunHistory(args []string) {
	limit := 0
	if v := getFlagValue(args, "--limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	manifests, err := snapshot.List()
	if err != nil {
		output.Fail("history", err, "No snapshots found. Run: ks snapshot")
		os.Exit(2)
	}

	if limit > 0 && limit < len(manifests) {
		manifests = manifests[:limit]
	}

	summaries := make([]map[string]any, 0, len(manifests))
	for _, m := range manifests {
		summaries = append(summaries, map[string]any{
			"id":          m.ID,
			"timestamp":   m.Timestamp,
			"commitHash":  m.CommitHash,
			"urlCount":    len(m.URLs),
			"urls":        urlSummaries(m.URLs),
		})
	}

	output.Success("history", map[string]any{
		"count":     len(summaries),
		"snapshots": summaries,
	})
}

// urlSummaries returns a condensed view of URL entries for history output.
func urlSummaries(entries []snapshot.URLEntry) []map[string]any {
	result := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		result = append(result, map[string]any{
			"url":          e.URL,
			"dir":          e.Dir,
			"auditSummary": e.AuditSummary,
		})
	}
	return result
}
