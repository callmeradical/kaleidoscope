package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunSnapshot captures full interface state for all project URLs.
func RunSnapshot(args []string) {
	fullPage := hasFlag(args, "--full-page")

	// Load project config
	cfg, err := project.FindConfig()
	if err != nil || cfg == nil {
		msg := "Create a project config first. Run: ks project init"
		if err == nil {
			err = fmt.Errorf("no .ks-project.json found")
		}
		output.Fail("snapshot", err, msg)
		os.Exit(2)
	}

	// Verify project-local state exists
	local := filepath.Join(".", ".kaleidoscope")
	if info, statErr := os.Stat(local); statErr != nil || !info.IsDir() {
		output.Fail("snapshot", fmt.Errorf("project-local state not found"), "Run 'ks start --local' first to create a project-local state directory")
		os.Exit(2)
	}

	// Generate snapshot ID and create directory
	id := snapshot.GenerateID()
	snapshotDir, err := snapshot.CreateDir(id)
	if err != nil {
		output.Fail("snapshot", err, "Failed to create snapshot directory")
		os.Exit(2)
	}

	var urlEntries []snapshot.URLEntry

	browserErr := browser.WithPage(func(page *rod.Page) error {
		for _, rawURL := range cfg.URLs {
			entry := snapshot.URLEntry{URL: rawURL}

			// Validate URL scheme
			if err := validateURL(rawURL); err != nil {
				entry.Error = err.Error()
				urlEntries = append(urlEntries, entry)
				continue
			}

			// Compute safe directory name for this URL
			entry.Dir = snapshot.URLDir(rawURL)
			urlDir := filepath.Join(snapshotDir, entry.Dir)
			if err := os.MkdirAll(urlDir, 0755); err != nil {
				entry.Error = err.Error()
				urlEntries = append(urlEntries, entry)
				continue
			}

			// Navigate to URL
			if err := page.Navigate(rawURL); err != nil {
				entry.Error = fmt.Sprintf("navigate: %s", err.Error())
				urlEntries = append(urlEntries, entry)
				continue
			}
			page.MustWaitLoad()

			// Capture breakpoints
			filenames, bpErr := captureBreakpointsData(page, urlDir, fullPage)
			if bpErr != nil {
				// Non-fatal: log but continue
				entry.Error = fmt.Sprintf("breakpoints: %s", bpErr.Error())
			}
			entry.Screenshots = filenames

			// Capture audit
			auditSummary, auditRaw, auditErr := captureAuditData(page)
			if auditErr == nil {
				entry.AuditSummary = auditSummary
				_ = writeJSON(filepath.Join(urlDir, "audit.json"), auditRaw)
			}

			// Capture ax-tree
			nodes, nodeCount, axErr := captureAxTreeData(page)
			if axErr == nil {
				entry.AxNodeCount = nodeCount
				axData := map[string]any{
					"nodeCount": nodeCount,
					"nodes":     nodes,
				}
				_ = writeJSON(filepath.Join(urlDir, "ax-tree.json"), axData)
			}

			urlEntries = append(urlEntries, entry)
		}
		return nil
	})

	if browserErr != nil {
		output.Fail("snapshot", browserErr, "Is the browser running? Run: ks start")
		os.Exit(2)
	}

	// Extract commit hash from ID (format: <ts>-<hash> or just <ts>)
	commitHash := ""
	parts := strings.SplitN(id, "-", 2)
	if len(parts) == 2 {
		commitHash = parts[1]
	}

	// Build manifest
	manifest := &snapshot.Manifest{
		ID:         id,
		CommitHash: commitHash,
		Timestamp:  time.Now(),
		ProjectConfig: snapshot.ProjectConfig{
			Name:        cfg.Name,
			URLs:        cfg.URLs,
			Breakpoints: cfg.Breakpoints,
		},
		URLs: urlEntries,
	}

	if err := snapshot.WriteManifest(snapshotDir, manifest); err != nil {
		output.Fail("snapshot", err, "Failed to write snapshot manifest")
		os.Exit(2)
	}

	promoted, _ := snapshot.EnsureBaseline(id)

	output.Success("snapshot", map[string]any{
		"id":               id,
		"snapshotDir":      snapshotDir,
		"urlCount":         len(urlEntries),
		"baselinePromoted": promoted,
		"urls":             urlEntries,
	})
}
