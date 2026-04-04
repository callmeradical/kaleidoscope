package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunSnapshot(args []string) {
	fullPage := hasFlag(args, "--full-page")

	// Load project config
	cfg, err := project.Load()
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			output.Fail("snapshot", err, "Create .ks-project.json with a list of URLs. See 'ks snapshot --usage'.")
		} else {
			output.Fail("snapshot", err, "Failed to load project config.")
		}
		os.Exit(2)
	}

	// Generate snapshot ID and directory
	id := snapshot.NewID()
	root, err := snapshot.SnapshotRoot()
	if err != nil {
		output.Fail("snapshot", err, "Failed to create snapshot root directory.")
		os.Exit(2)
	}
	snapshotDir := filepath.Join(root, id)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		output.Fail("snapshot", err, "Failed to create snapshot directory.")
		os.Exit(2)
	}

	var urlEntries []snapshot.URLEntry
	captureTime := time.Now().UTC()

	// Connect to browser and capture each URL
	err = browser.WithPage(func(page *rod.Page) error {
		// Save original viewport
		state, _ := browser.ReadState()
		var origW, origH int
		if state != nil && state.Viewport != nil {
			origW = state.Viewport.Width
			origH = state.Viewport.Height
		}

		seen := make(map[string]int)

		for _, rawURL := range cfg.URLs {
			entry := snapshot.URLEntry{URL: rawURL}

			// Navigate
			if err := page.Navigate(rawURL); err != nil {
				entry.Error = err.Error()
				fmt.Fprintf(os.Stderr, "snapshot: error navigating to %s: %v\n", rawURL, err)
				urlEntries = append(urlEntries, entry)
				continue
			}
			page.MustWaitStable()

			slug := snapshot.UniqueSlug(rawURL, seen)
			entry.Slug = slug

			urlDir := filepath.Join(snapshotDir, slug)
			if err := os.MkdirAll(urlDir, 0755); err != nil {
				entry.Error = err.Error()
				fmt.Fprintf(os.Stderr, "snapshot: error creating dir for %s: %v\n", rawURL, err)
				urlEntries = append(urlEntries, entry)
				continue
			}

			// Capture breakpoints
			var bpEntries []snapshot.BreakpointEntry
			for _, bp := range defaultBreakpoints {
				if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
					Width:  bp.Width,
					Height: bp.Height,
				}); err != nil {
					fmt.Fprintf(os.Stderr, "snapshot: error setting viewport %s: %v\n", bp.Name, err)
					continue
				}
				page.MustWaitStable()

				data, err := page.Screenshot(fullPage, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "snapshot: error capturing screenshot at %s: %v\n", bp.Name, err)
					continue
				}

				filename := fmt.Sprintf("%s-%dx%d.png", bp.Name, bp.Width, bp.Height)
				if err := os.WriteFile(filepath.Join(urlDir, filename), data, 0644); err != nil {
					fmt.Fprintf(os.Stderr, "snapshot: error writing screenshot %s: %v\n", filename, err)
					continue
				}

				bpEntries = append(bpEntries, snapshot.BreakpointEntry{
					Name:   bp.Name,
					Width:  bp.Width,
					Height: bp.Height,
					File:   filepath.Join(slug, filename),
				})
			}
			entry.Breakpoints = bpEntries

			// Restore viewport before running audit
			if origW > 0 && origH > 0 {
				_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
					Width:  origW,
					Height: origH,
				})
			}
			page.MustWaitStable()

			// Audit
			auditSummary, err := runAuditOnPage(page)
			if err != nil {
				fmt.Fprintf(os.Stderr, "snapshot: audit error for %s: %v\n", rawURL, err)
			}
			entry.AuditSummary = auditSummary

			// Write audit.json
			if auditData, err := json.MarshalIndent(auditSummary, "", "  "); err == nil {
				_ = os.WriteFile(filepath.Join(urlDir, "audit.json"), auditData, 0644)
			}

			// AX-tree
			nodes, err := runAxTreeOnPage(page)
			if err != nil {
				fmt.Fprintf(os.Stderr, "snapshot: ax-tree error for %s: %v\n", rawURL, err)
			}

			// Write ax-tree.json
			if axData, err := json.MarshalIndent(nodes, "", "  "); err == nil {
				_ = os.WriteFile(filepath.Join(urlDir, "ax-tree.json"), axData, 0644)
			}

			urlEntries = append(urlEntries, entry)
		}

		// Restore original viewport
		if origW > 0 && origH > 0 {
			_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
				Width:  origW,
				Height: origH,
			})
		}

		return nil
	})

	if err != nil {
		output.Fail("snapshot", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}

	// Build manifest
	manifest := snapshot.Manifest{
		ID:         id,
		Timestamp:  captureTime,
		CommitHash: snapshot.GitShortHash(),
		Project:    *cfg,
		URLs:       urlEntries,
	}

	// Store manifest
	if err := snapshot.Store(&manifest); err != nil {
		output.Fail("snapshot", err, "Failed to write snapshot manifest.")
		os.Exit(2)
	}

	// Auto-promote baseline if none exists
	autoPromotedBaseline := false
	bl, err := snapshot.LoadBaselines()
	if err == nil && bl == nil {
		if err := snapshot.SaveBaselines(&snapshot.Baselines{SnapshotID: id}); err == nil {
			autoPromotedBaseline = true
		}
	}

	// Build per-URL result slice
	urlResults := make([]map[string]any, 0, len(urlEntries))
	for _, u := range urlEntries {
		m := map[string]any{
			"url":  u.URL,
			"slug": u.Slug,
		}
		if u.Error != "" {
			m["error"] = u.Error
		}
		urlResults = append(urlResults, m)
	}

	output.Success("snapshot", map[string]any{
		"id":                   id,
		"snapshotDir":          snapshotDir,
		"timestamp":            manifest.Timestamp,
		"commitHash":           manifest.CommitHash,
		"autoPromotedBaseline": autoPromotedBaseline,
		"urls":                 urlResults,
	})
}
