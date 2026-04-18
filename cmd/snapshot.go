package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/callmeradical/kaleidoscope/analysis"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunSnapshot implements `ks snapshot`.
func RunSnapshot(args []string) {
	cfg, err := loadProjectConfig()
	if err != nil {
		output.Fail("snapshot", err, "Run 'ks init' first to create a project config")
		os.Exit(2)
	}

	if len(cfg.Paths) == 0 {
		output.Fail("snapshot", fmt.Errorf("no paths configured"), "Add paths with: ks project-add /path")
		os.Exit(2)
	}

	breakpoints := cfg.Breakpoints
	if len(breakpoints) == 0 {
		breakpoints = DefaultBreakpoints
	}

	ts := time.Now()
	commitHash := snapshot.GitShortHash()

	var results []snapshot.PathResult

	err = browser.WithPage(func(page *rod.Page) error {
		baseURL := cfg.BaseURL

		// Save original viewport to restore later
		state, _ := browser.ReadState()
		var origW, origH int
		if state != nil && state.Viewport != nil {
			origW = state.Viewport.Width
			origH = state.Viewport.Height
		}

		for _, urlPath := range cfg.Paths {
			fullURL := baseURL + urlPath

			// Navigate
			if err := page.Navigate(fullURL); err != nil {
				return fmt.Errorf("navigating to %s: %w", fullURL, err)
			}
			page.MustWaitStable()

			pr := snapshot.PathResult{
				Path:        urlPath,
				Screenshots: make(map[string][]byte),
			}

			// Screenshots at each breakpoint
			for _, bp := range breakpoints {
				if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
					Width:  bp.Width,
					Height: bp.Height,
				}); err != nil {
					return fmt.Errorf("setting viewport for %s at %s: %w", bp.Name, urlPath, err)
				}
				page.MustWaitStable()

				data, err := page.Screenshot(false, nil)
				if err != nil {
					return fmt.Errorf("screenshot %s at %s: %w", bp.Name, urlPath, err)
				}
				pr.Screenshots[bp.Name] = data
			}

			// Audit (reusing audit logic inline)
			pr.Audit = captureAudit(page)

			// Accessibility tree
			pr.AxTree = captureAxTree(page)

			results = append(results, pr)
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

	// Determine store root
	storeRoot, err := snapshotStoreRoot()
	if err != nil {
		output.Fail("snapshot", err, "Cannot determine state directory")
		os.Exit(2)
	}

	store := snapshot.NewStore(storeRoot)

	manifest, err := store.Create(ts, commitHash, cfg, results)
	if err != nil {
		output.Fail("snapshot", err, "Failed to create snapshot")
		os.Exit(2)
	}

	// Auto-promote to baseline if no baseline exists
	isNewBaseline := false
	if !store.HasBaseline() {
		if err := store.PromoteBaseline(manifest.ID); err != nil {
			output.Fail("snapshot", err, "Failed to set baseline")
			os.Exit(2)
		}
		isNewBaseline = true
	}

	output.Success("snapshot", map[string]any{
		"id":          manifest.ID,
		"timestamp":   manifest.Timestamp,
		"commitHash":  manifest.CommitHash,
		"paths":       manifest.Paths,
		"stats":       manifest.Stats,
		"newBaseline": isNewBaseline,
		"directory":   filepath.Join(storeRoot, "snapshots", manifest.ID),
	})
}

// RunHistory implements `ks history`.
func RunHistory(args []string) {
	storeRoot, err := snapshotStoreRoot()
	if err != nil {
		output.Fail("history", err, "Cannot determine state directory")
		os.Exit(2)
	}

	store := snapshot.NewStore(storeRoot)
	manifests, err := store.List()
	if err != nil {
		output.Fail("history", err, "Failed to list snapshots")
		os.Exit(2)
	}

	// Check current baseline
	var currentBaseline string
	if bf, err := store.LoadBaseline(); err == nil {
		currentBaseline = bf.Current
	}

	entries := make([]map[string]any, 0, len(manifests))
	for _, m := range manifests {
		entry := map[string]any{
			"id":         m.ID,
			"timestamp":  m.Timestamp,
			"commitHash": m.CommitHash,
			"stats":      m.Stats,
			"isBaseline": m.ID == currentBaseline,
		}
		entries = append(entries, entry)
	}

	output.Success("history", map[string]any{
		"snapshots": entries,
		"count":     len(entries),
		"baseline":  currentBaseline,
	})
}

// snapshotStoreRoot returns the .kaleidoscope directory path.
func snapshotStoreRoot() (string, error) {
	// Try project-local first
	local := filepath.Join(".", ".kaleidoscope")
	if info, err := os.Stat(local); err == nil && info.IsDir() {
		return local, nil
	}
	// Create project-local
	if err := os.MkdirAll(local, 0755); err != nil {
		return "", err
	}
	return local, nil
}

// captureAudit runs the audit analysis on the current page and returns the result map.
func captureAudit(page *rod.Page) map[string]any {
	// Accessibility summary
	axTree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	var axSummary map[string]any
	if err == nil {
		ignored := 0
		total := len(axTree.Nodes)
		for _, n := range axTree.Nodes {
			if n.Ignored {
				ignored++
			}
		}
		axSummary = map[string]any{
			"totalNodes":   total,
			"ignoredNodes": ignored,
			"activeNodes":  total - ignored,
		}
	}

	// Contrast check
	contrastJS := `() => {
		const root = document.body;
		if (!root) return [];
		const els = root.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, a, li, td, th, label, button');
		const results = [];
		for (const el of els) {
			const cs = window.getComputedStyle(el);
			if (cs.display === 'none' || cs.visibility === 'hidden') continue;
			if (!el.textContent.trim()) continue;
			let bgColor = 'rgba(0, 0, 0, 0)';
			let cur = el;
			while (cur && cur !== document.documentElement) {
				const bg = window.getComputedStyle(cur).backgroundColor;
				if (bg && bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent') { bgColor = bg; break; }
				cur = cur.parentElement;
			}
			if (bgColor === 'rgba(0, 0, 0, 0)') bgColor = 'rgb(255, 255, 255)';
			results.push({
				color: cs.color,
				backgroundColor: bgColor,
				fontSize: parseFloat(cs.fontSize),
				fontWeight: cs.fontWeight,
				selector: el.tagName.toLowerCase(),
			});
		}
		return results;
	}`
	contrastResult, _ := page.Eval(contrastJS)

	contrastViolations := 0
	if contrastResult != nil {
		if elList, ok := contrastResult.Value.Val().([]interface{}); ok {
			for _, item := range elList {
				el, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				fg, _ := el["color"].(string)
				bg, _ := el["backgroundColor"].(string)
				fontSize, _ := el["fontSize"].(float64)
				fontWeight, _ := el["fontWeight"].(string)
				check, err := analysis.CheckContrast(fg, bg, fontSize, fontWeight)
				if err == nil && !check.MeetsMinimum {
					contrastViolations++
				}
			}
		}
	}

	// Touch targets
	touchJS := `() => {
		const root = document.body;
		if (!root) return [];
		const interactive = root.querySelectorAll('a, button, input, select, textarea, [role="button"], [role="link"], [tabindex]');
		const results = [];
		for (const el of interactive) {
			const rect = el.getBoundingClientRect();
			if (rect.width === 0 || rect.height === 0) continue;
			results.push({
				tag: el.tagName.toLowerCase(),
				width: rect.width,
				height: rect.height,
			});
		}
		return results;
	}`
	touchResult, _ := page.Eval(touchJS)

	touchViolations := 0
	touchTotal := 0
	if touchResult != nil {
		if elList, ok := touchResult.Value.Val().([]interface{}); ok {
			touchTotal = len(elList)
			for _, item := range elList {
				el, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				w, _ := el["width"].(float64)
				h, _ := el["height"].(float64)
				tag, _ := el["tag"].(string)
				check := analysis.CheckTouchTarget(tag, w, h)
				if !check.Passes {
					touchViolations++
				}
			}
		}
	}

	// Typography check
	typoJS := `() => {
		const root = document.body;
		if (!root) return [];
		const els = root.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, li, td, th, label');
		const results = [];
		for (const el of els) {
			const cs = window.getComputedStyle(el);
			if (cs.display === 'none') continue;
			if (!el.textContent.trim()) continue;
			const isHeading = /^h[1-6]$/i.test(el.tagName);
			results.push({
				tag: el.tagName.toLowerCase(),
				fontSize: parseFloat(cs.fontSize),
				lineHeight: cs.lineHeight,
				fontFamily: cs.fontFamily,
				isHeading: isHeading,
			});
		}
		return results;
	}`
	typoResult, _ := page.Eval(typoJS)

	typoWarnings := 0
	if typoResult != nil {
		if elList, ok := typoResult.Value.Val().([]interface{}); ok {
			for _, item := range elList {
				el, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				fontSize, _ := el["fontSize"].(float64)
				lineHeightStr, _ := el["lineHeight"].(string)
				fontFamily, _ := el["fontFamily"].(string)
				isHeading, _ := el["isHeading"].(bool)

				lineHeight := analysis.ParseLineHeight(lineHeightStr, fontSize)
				check := analysis.CheckTypography(fontSize, lineHeight, fontFamily, isHeading)
				typoWarnings += len(check.Warnings)
			}
		}
	}

	totalIssues := contrastViolations + touchViolations + typoWarnings

	return map[string]any{
		"summary": map[string]any{
			"totalIssues":        totalIssues,
			"contrastViolations": contrastViolations,
			"touchViolations":    touchViolations,
			"typographyWarnings": typoWarnings,
		},
		"accessibility": axSummary,
		"contrast": map[string]any{
			"violations": contrastViolations,
		},
		"touchTargets": map[string]any{
			"total":      touchTotal,
			"violations": touchViolations,
		},
		"typography": map[string]any{
			"warnings": typoWarnings,
		},
	}
}

// captureAxTree dumps the accessibility tree for the current page.
func captureAxTree(page *rod.Page) map[string]any {
	tree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	nodes := make([]map[string]any, 0)
	for _, node := range tree.Nodes {
		if node.Ignored {
			continue
		}
		n := map[string]any{
			"nodeId": node.NodeID,
			"role":   "",
			"name":   "",
		}
		if node.Role != nil {
			n["role"] = node.Role.Value
		}
		if node.Name != nil {
			n["name"] = node.Name.Value
		}
		if len(node.ChildIDs) > 0 {
			children := make([]string, len(node.ChildIDs))
			for i, id := range node.ChildIDs {
				children[i] = string(id)
			}
			n["children"] = children
		}
		if len(node.Properties) > 0 {
			props := make(map[string]any)
			for _, p := range node.Properties {
				props[string(p.Name)] = p.Value.Value
			}
			n["properties"] = props
		}
		nodes = append(nodes, n)
	}

	return map[string]any{
		"nodeCount": len(nodes),
		"nodes":     nodes,
	}
}
