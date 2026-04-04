package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/gitutil"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/project"
	"github.com/callmeradical/kaleidoscope/snapshot"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func RunSnapshot(args []string) {
	cfg, err := project.Load()
	if err != nil {
		output.Fail("snapshot", err, "Create a .ks-project.json with 'ks init'")
		os.Exit(2)
	}

	hash := gitutil.ShortHash()
	id := snapshot.NewID(hash)

	_, err = snapshot.SnapshotPath(id)
	if err != nil {
		output.Fail("snapshot", err, "Could not create snapshot directory")
		os.Exit(2)
	}

	type urlError struct {
		URL   string `json:"url"`
		Error string `json:"error"`
	}

	var summaries []snapshot.URLSummary
	var urlErrors []urlError

	err = browser.WithPage(func(page *rod.Page) error {
		for _, rawURL := range cfg.URLs {
			resolvedURL := rawURL
			if cfg.BaseURL != "" {
				base, parseErr := url.Parse(cfg.BaseURL)
				ref, refErr := url.Parse(rawURL)
				if parseErr == nil && refErr == nil {
					resolvedURL = base.ResolveReference(ref).String()
				}
			}

			// Validate scheme
			parsed, parseErr := url.Parse(resolvedURL)
			if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				urlErrors = append(urlErrors, urlError{URL: rawURL, Error: fmt.Sprintf("invalid URL scheme: %s", resolvedURL)})
				continue
			}

			// Navigate
			if navErr := page.Navigate(resolvedURL); navErr != nil {
				urlErrors = append(urlErrors, urlError{URL: rawURL, Error: navErr.Error()})
				continue
			}
			page.MustWaitLoad()

			slug := snapshot.URLSlug(resolvedURL)
			urlDir, dirErr := snapshot.URLDir(id, slug)
			if dirErr != nil {
				urlErrors = append(urlErrors, urlError{URL: rawURL, Error: dirErr.Error()})
				continue
			}

			// Screenshots at 4 breakpoints
			for _, bp := range defaultBreakpoints {
				vpErr := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
					Width:             bp.Width,
					Height:            bp.Height,
					DeviceScaleFactor: 1,
				})
				if vpErr != nil {
					urlErrors = append(urlErrors, urlError{URL: rawURL, Error: fmt.Sprintf("viewport %s: %v", bp.Name, vpErr)})
					continue
				}
				page.MustWaitStable()
				data, ssErr := page.Screenshot(false, nil)
				if ssErr != nil {
					urlErrors = append(urlErrors, urlError{URL: rawURL, Error: fmt.Sprintf("screenshot %s: %v", bp.Name, ssErr)})
					continue
				}
				pngName := fmt.Sprintf("%s-%dx%d.png", bp.Name, bp.Width, bp.Height)
				if writeErr := os.WriteFile(filepath.Join(urlDir, pngName), data, 0644); writeErr != nil {
					urlErrors = append(urlErrors, urlError{URL: rawURL, Error: fmt.Sprintf("write screenshot %s: %v", bp.Name, writeErr)})
				}
			}

			// Restore desktop viewport
			_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
				Width:             1280,
				Height:            720,
				DeviceScaleFactor: 1,
			})

			// Run audit
			auditResult, auditErr := auditPage(page)
			if auditErr != nil {
				urlErrors = append(urlErrors, urlError{URL: rawURL, Error: fmt.Sprintf("audit: %v", auditErr)})
			} else {
				auditData, _ := json.MarshalIndent(auditResult, "", "  ")
				_ = os.WriteFile(filepath.Join(urlDir, "audit.json"), auditData, 0644)
			}

			// Run AX-tree
			axNodes, axErr := axTreePage(page)
			if axErr != nil {
				urlErrors = append(urlErrors, urlError{URL: rawURL, Error: fmt.Sprintf("ax-tree: %v", axErr)})
			} else {
				axData, _ := json.MarshalIndent(axNodes, "", "  ")
				_ = os.WriteFile(filepath.Join(urlDir, "ax-tree.json"), axData, 0644)
			}

			summaries = append(summaries, snapshot.URLSummary{
				URL:                rawURL,
				Slug:               slug,
				ContrastViolations: auditResult.ContrastViolations,
				TouchViolations:    auditResult.TouchViolations,
				TypographyWarnings: auditResult.TypographyWarnings,
				AXActiveNodes:      auditResult.AXActiveNodes,
				AXTotalNodes:       auditResult.AXTotalNodes,
			})
		}
		return nil
	})

	if err != nil {
		output.Fail("snapshot", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}

	// If all URLs failed, abort
	if len(summaries) == 0 && len(urlErrors) > 0 {
		output.Fail("snapshot", fmt.Errorf("all URLs failed"), "Check that URLs in .ks-project.json are reachable")
		os.Exit(2)
	}

	// Write manifest
	now := time.Now().UTC()
	manifest := snapshot.Manifest{
		ID:           id,
		Timestamp:    now,
		CommitHash:   hash,
		ProjectURLs:  cfg.URLs,
		ProjectName:  cfg.Name,
		BaseURL:      cfg.BaseURL,
		URLSummaries: summaries,
	}
	if writeErr := snapshot.WriteManifest(id, &manifest); writeErr != nil {
		output.Fail("snapshot", writeErr, "Could not write snapshot manifest")
		os.Exit(2)
	}

	// Baseline auto-promotion
	baselines, _ := snapshot.ReadBaselines()
	baselinePromoted := false
	if baselines == nil || baselines.DefaultBaseline == "" {
		_ = snapshot.WriteBaselines(&snapshot.BaselinesFile{DefaultBaseline: id})
		baselinePromoted = true
	}

	// Build snapshot path for output
	snapshotPath, _ := snapshot.SnapshotPath(id)

	// Build URL summaries for output
	urlSummariesOut := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		urlSummariesOut = append(urlSummariesOut, map[string]any{
			"url":                s.URL,
			"slug":               s.Slug,
			"contrastViolations": s.ContrastViolations,
			"touchViolations":    s.TouchViolations,
			"typographyWarnings": s.TypographyWarnings,
			"axActiveNodes":      s.AXActiveNodes,
			"axTotalNodes":       s.AXTotalNodes,
		})
	}

	errorsOut := make([]map[string]any, 0, len(urlErrors))
	for _, e := range urlErrors {
		errorsOut = append(errorsOut, map[string]any{
			"url":   e.URL,
			"error": e.Error,
		})
	}

	output.Success("snapshot", map[string]any{
		"id":               id,
		"path":             snapshotPath,
		"timestamp":        now.Format(time.RFC3339),
		"commitHash":       hash,
		"urlCount":         len(cfg.URLs),
		"baselinePromoted": baselinePromoted,
		"urls":             urlSummariesOut,
		"errors":           errorsOut,
	})
}
