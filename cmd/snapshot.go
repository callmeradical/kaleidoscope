package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunSnapshot(args []string) {
	// Load project config
	config, err := ReadProjectConfig()
	if err != nil {
		output.Fail("snapshot", err, "Run: echo '{\"version\":1,\"urls\":[\"https://...\"]}' > .ks-project.json")
		os.Exit(2)
	}

	// Validate URLs
	if len(config.URLs) == 0 {
		output.Fail("snapshot", fmt.Errorf("no URLs configured"), "Add at least one URL to .ks-project.json")
		os.Exit(2)
	}

	// Build snapshot ID
	now := time.Now().UTC()
	id := now.Format("20060102T150405Z")
	shortHash := snapshot.ShortCommitHash()
	if shortHash != "" {
		id = id + "-" + shortHash
	}

	// Create snapshot root dir
	snapshotPath, err := snapshot.SnapshotPath(id)
	if err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}
	if err := os.MkdirAll(snapshotPath, 0755); err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}

	var urlEntries []snapshot.URLEntry
	promotedToBaseline := false

	for _, rawURL := range config.URLs {
		// Validate scheme
		u, parseErr := url.Parse(rawURL)
		if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") {
			urlEntries = append(urlEntries, snapshot.URLEntry{
				URL:       rawURL,
				Dir:       snapshot.URLToKey(rawURL),
				Reachable: false,
				Error:     "unsupported scheme",
			})
			continue
		}

		// Create URL subdirectory
		urlDir, err := snapshot.URLDir(snapshotPath, rawURL)
		if err != nil {
			urlEntries = append(urlEntries, snapshot.URLEntry{
				URL:       rawURL,
				Dir:       snapshot.URLToKey(rawURL),
				Reachable: false,
				Error:     err.Error(),
			})
			continue
		}

		var entry snapshot.URLEntry
		entry.URL = rawURL
		entry.Dir = snapshot.URLToKey(rawURL)

		captureErr := browser.WithPage(func(page *rod.Page) error {
			// Navigate to URL
			if err := page.Navigate(rawURL); err != nil {
				entry.Reachable = false
				entry.Error = err.Error()
				return nil
			}
			if err := page.WaitLoad(); err != nil {
				entry.Reachable = false
				entry.Error = err.Error()
				return nil
			}

			// Capture breakpoints
			_, bpErr := captureBreakpointsToDir(page, urlDir)
			if bpErr != nil {
				entry.Reachable = false
				entry.Error = bpErr.Error()
				return nil
			}

			// Gather audit data
			auditResult, auditErr := gatherAuditData(page, "")
			if auditErr != nil {
				entry.Reachable = false
				entry.Error = auditErr.Error()
				return nil
			}

			// Gather ax-tree data
			axResult, axErr := gatherAxTreeData(page)
			if axErr != nil {
				entry.Reachable = false
				entry.Error = axErr.Error()
				return nil
			}

			// Write audit.json
			auditData, _ := json.MarshalIndent(auditResult, "", "  ")
			if err := os.WriteFile(filepath.Join(urlDir, "audit.json"), auditData, 0644); err != nil {
				return err
			}

			// Write ax-tree.json
			axData, _ := json.MarshalIndent(axResult, "", "  ")
			if err := os.WriteFile(filepath.Join(urlDir, "ax-tree.json"), axData, 0644); err != nil {
				return err
			}

			// Extract stats
			totalIssues := 0
			if summary, ok := auditResult["summary"].(map[string]any); ok {
				if n, ok := summary["totalIssues"].(int); ok {
					totalIssues = n
				}
			}
			nodeCount := 0
			if n, ok := axResult["nodeCount"].(int); ok {
				nodeCount = n
			}

			entry.TotalIssues = totalIssues
			entry.AXNodeCount = nodeCount
			entry.Breakpoints = 4
			entry.CapturedAt = time.Now().UTC()
			entry.Reachable = true
			return nil
		})

		if captureErr != nil {
			output.Fail("snapshot", captureErr, "Is the browser running? Run: ks start")
			os.Exit(2)
		}

		urlEntries = append(urlEntries, entry)
	}

	// Compute summary
	var summary snapshot.Summary
	summary.TotalURLs = len(urlEntries)
	for _, e := range urlEntries {
		if e.Reachable {
			summary.ReachableURLs++
			summary.TotalIssues += e.TotalIssues
			summary.TotalAXNodes += e.AXNodeCount
		}
	}

	// Assemble and write Manifest
	manifest := snapshot.Manifest{
		ID:            id,
		Timestamp:     now,
		CommitHash:    shortHash,
		ProjectConfig: config,
		URLs:          urlEntries,
		Summary:       summary,
	}
	if err := snapshot.WriteManifest(snapshotPath, &manifest); err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}

	// Auto-promote to baseline if none exists
	baseline, err := snapshot.ReadBaselineManifest()
	if err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}
	if baseline == nil {
		bm := &snapshot.BaselineManifest{
			BaselineID: id,
			SetAt:      time.Now().UTC(),
			CommitHash: shortHash,
		}
		if err := snapshot.WriteBaselineManifest(bm); err != nil {
			output.Fail("snapshot", err, "")
			os.Exit(2)
		}
		promotedToBaseline = true
	}

	// Compute relative path
	cwd, _ := os.Getwd()
	relPath, _ := filepath.Rel(cwd, snapshotPath)
	if relPath == "" {
		relPath = snapshotPath
	}

	output.Success("snapshot", map[string]any{
		"id":                 id,
		"path":               relPath,
		"urls":               summary.TotalURLs,
		"reachable":          summary.ReachableURLs,
		"totalIssues":        summary.TotalIssues,
		"promotedToBaseline": promotedToBaseline,
	})
}
