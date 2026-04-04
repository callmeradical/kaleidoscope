package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunSnapshot(args []string) {
	projectConfig, err := snapshot.LoadProjectConfig()
	if err != nil {
		output.Fail("snapshot", err, "Create a .ks-project.json file with a 'urls' array")
		os.Exit(2)
	}

	// Resolve git commit hash (best-effort)
	commitHash := ""
	if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		commitHash = strings.TrimSpace(string(out))
	}

	// Build snapshot ID
	ts := time.Now().UTC().Format("20060102T150405Z")
	id := ts
	if commitHash != "" {
		id = ts + "-" + commitHash
	}

	// Get snapshot directory path
	snapshotDir, err := snapshot.SnapshotPath(id)
	if err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}

	// Read original viewport before starting
	state, _ := browser.ReadState()
	var origW, origH int
	if state != nil && state.Viewport != nil {
		origW = state.Viewport.Width
		origH = state.Viewport.Height
	}

	var entries []snapshot.URLEntry

	err = browser.WithPage(func(page *rod.Page) error {
		for _, u := range projectConfig.URLs {
			urlDir := snapshot.URLToDir(u)
			urlPath := filepath.Join(snapshotDir, urlDir)
			if err := os.MkdirAll(urlPath, 0755); err != nil {
				return err
			}

			// Navigate to URL
			if err := page.Navigate(u); err != nil {
				entries = append(entries, snapshot.URLEntry{URL: u, Dir: urlDir, Error: err.Error()})
				continue
			}
			if err := page.WaitLoad(); err != nil {
				entries = append(entries, snapshot.URLEntry{URL: u, Dir: urlDir, Error: err.Error()})
				continue
			}

			// Screenshots at each breakpoint
			var breakpointFiles []string
			for _, bp := range defaultBreakpoints {
				if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
					Width:             bp.Width,
					Height:            bp.Height,
					DeviceScaleFactor: 1,
				}); err != nil {
					return fmt.Errorf("setting viewport for %s: %w", bp.Name, err)
				}
				page.MustWaitStable()

				data, err := page.Screenshot(true, nil)
				if err != nil {
					return fmt.Errorf("screenshot at %s: %w", bp.Name, err)
				}

				filename := fmt.Sprintf("%s-%dx%d.png", bp.Name, bp.Width, bp.Height)
				if err := os.WriteFile(filepath.Join(urlPath, filename), data, 0644); err != nil {
					return err
				}
				breakpointFiles = append(breakpointFiles, filename)
			}

			// Restore viewport after screenshots
			if origW > 0 && origH > 0 {
				_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
					Width:  origW,
					Height: origH,
				})
			}

			// Audit
			auditMap, auditSummary, err := runAudit(page, "")
			if err != nil {
				return err
			}
			auditJSON, err := json.MarshalIndent(auditMap, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(urlPath, "audit.json"), auditJSON, 0644); err != nil {
				return err
			}

			// Ax-tree
			axNodes, axCount, err := runAxTree(page)
			if err != nil {
				return err
			}
			axJSON, err := json.MarshalIndent(axNodes, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(urlPath, "ax-tree.json"), axJSON, 0644); err != nil {
				return err
			}

			entries = append(entries, snapshot.URLEntry{
				URL:          u,
				Dir:          urlDir,
				Breakpoints:  breakpointFiles,
				AuditSummary: auditSummary,
				AxNodeCount:  axCount,
			})
		}
		return nil
	})

	if err != nil {
		output.Fail("snapshot", err, "Run: ks start")
		os.Exit(2)
	}

	// Write manifest
	manifest := snapshot.Manifest{
		ID:            id,
		Timestamp:     time.Now().UTC(),
		CommitHash:    commitHash,
		ProjectConfig: *projectConfig,
		URLs:          entries,
	}
	if err := snapshot.Save(&manifest); err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}

	// Auto-promote baseline on first run
	baseline, err := snapshot.LoadBaseline()
	if err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}
	autoBaseline := false
	if baseline == nil {
		if err := snapshot.SaveBaseline(&snapshot.Baseline{SnapshotID: id, SetAt: time.Now().UTC()}); err != nil {
			output.Fail("snapshot", err, "")
			os.Exit(2)
		}
		autoBaseline = true
	}

	output.Success("snapshot", map[string]any{
		"id":           id,
		"snapshotDir":  snapshotDir,
		"urlCount":     len(entries),
		"autoBaseline": autoBaseline,
		"urls":         entries,
	})
}
