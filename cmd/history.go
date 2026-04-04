package cmd

import (
	"strconv"
	"time"

	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunHistory(args []string) {
	limitStr := getFlagValue(args, "--limit")
	limit := 0
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	ids, err := snapshot.ListIDs()
	if err != nil || len(ids) == 0 {
		output.Success("history", map[string]any{
			"count":     0,
			"snapshots": []any{},
		})
		return
	}

	if limit > 0 && limit < len(ids) {
		ids = ids[:limit]
	}

	baselines, _ := snapshot.ReadBaselines()
	defaultBaseline := ""
	if baselines != nil {
		defaultBaseline = baselines.DefaultBaseline
	}

	entries := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		m, readErr := snapshot.ReadManifest(id)
		if readErr != nil {
			continue
		}

		totalContrast := 0
		totalTouch := 0
		totalTypo := 0
		totalAX := 0
		for _, s := range m.URLSummaries {
			totalContrast += s.ContrastViolations
			totalTouch += s.TouchViolations
			totalTypo += s.TypographyWarnings
			totalAX += s.AXActiveNodes
		}

		entries = append(entries, map[string]any{
			"id":                      m.ID,
			"timestamp":               m.Timestamp.UTC().Format(time.RFC3339),
			"commitHash":              m.CommitHash,
			"isBaseline":              m.ID == defaultBaseline,
			"urlCount":                len(m.URLSummaries),
			"totalContrastViolations": totalContrast,
			"totalTouchViolations":    totalTouch,
			"totalTypographyWarnings": totalTypo,
			"totalAXActiveNodes":      totalAX,
		})
	}

	output.Success("history", map[string]any{
		"count":     len(entries),
		"snapshots": entries,
	})
}
