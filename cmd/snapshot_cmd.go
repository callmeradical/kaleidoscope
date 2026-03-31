package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/callmeradical/kaleidoscope/analysis"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func RunSnapshot(args []string) {
	proj, err := snapshot.LoadProject()
	if err != nil {
		output.Fail("snapshot", err, "Run 'ks init' to create a project first")
		os.Exit(2)
	}

	stateDir, err := browser.StateDir()
	if err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}

	commitHash := gitCommitHash()
	now := time.Now()
	snapID := snapshot.GenerateID(commitHash, now)
	snapDir := snapshot.SnapshotPath(stateDir, snapID)

	if err := os.MkdirAll(snapDir, 0755); err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}

	var urlResults []map[string]any

	err = browser.WithPage(func(page *rod.Page) error {
		for _, urlPath := range proj.Paths {
			fullURL := strings.TrimRight(proj.BaseURL, "/") + urlPath

			if err := page.Navigate(fullURL); err != nil {
				return fmt.Errorf("navigating to %s: %w", fullURL, err)
			}
			page.MustWaitLoad()

			var screenshotPaths []map[string]any
			for _, bp := range proj.Breakpoints {
				if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
					Width:  bp.Width,
					Height: bp.Height,
				}); err != nil {
					return fmt.Errorf("setting viewport for %s at %s: %w", urlPath, bp.Name, err)
				}
				page.MustWaitStable()

				data, err := page.Screenshot(false, nil)
				if err != nil {
					return fmt.Errorf("screenshot for %s at %s: %w", urlPath, bp.Name, err)
				}
				ssPath, err := snapshot.WriteScreenshot(snapDir, urlPath, bp.Name, bp.Width, bp.Height, data)
				if err != nil {
					return err
				}
				screenshotPaths = append(screenshotPaths, map[string]any{
					"breakpoint": bp.Name,
					"width":      bp.Width,
					"height":     bp.Height,
					"path":       ssPath,
				})
			}

			_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1280, Height: 720})

			auditData := captureAudit(page, "")
			auditJSON, _ := json.Marshal(auditData)
			if err := snapshot.WriteAuditJSON(snapDir, urlPath, auditJSON); err != nil {
				return err
			}

			axData := captureAxTree(page)
			axJSON, _ := json.Marshal(axData)
			if err := snapshot.WriteAxTreeJSON(snapDir, urlPath, axJSON); err != nil {
				return err
			}

			urlResults = append(urlResults, map[string]any{
				"path":        urlPath,
				"url":         fullURL,
				"screenshots": screenshotPaths,
			})
		}
		return nil
	})

	if err != nil {
		output.Fail("snapshot", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}

	manifest := &snapshot.Manifest{
		ID:         snapID,
		Timestamp:  now,
		CommitHash: commitHash,
		Project:    *proj,
	}
	if err := snapshot.WriteManifest(snapDir, manifest); err != nil {
		output.Fail("snapshot", err, "")
		os.Exit(2)
	}

	baselines, _ := snapshot.LoadBaselines(stateDir)
	if baselines == nil {
		baselines = snapshot.Baselines{}
	}
	needsBaseline := false
	for _, urlPath := range proj.Paths {
		if _, ok := baselines[urlPath]; !ok {
			needsBaseline = true
			baselines[urlPath] = snapID
		}
	}
	if needsBaseline {
		_ = snapshot.SaveBaselines(stateDir, baselines)
	}

	output.Success("snapshot", map[string]any{
		"id":        snapID,
		"timestamp": now,
		"commit":    commitHash,
		"directory": snapDir,
		"urls":      urlResults,
		"baseline":  needsBaseline,
	})
}

func RunHistory(args []string) {
	stateDir, err := browser.StateDir()
	if err != nil {
		output.Fail("history", err, "")
		os.Exit(2)
	}

	manifests, err := snapshot.ListSnapshots(stateDir)
	if err != nil {
		output.Fail("history", err, "")
		os.Exit(2)
	}

	var items []map[string]any
	for _, m := range manifests {
		items = append(items, map[string]any{
			"id":        m.ID,
			"timestamp": m.Timestamp,
			"commit":    m.CommitHash,
			"project":   m.Project.Name,
			"urls":      len(m.Project.Paths),
		})
	}

	output.Success("history", map[string]any{
		"count":     len(items),
		"snapshots": items,
	})
}

// captureAudit runs the audit checks on the current page and returns structured data.
func captureAudit(page *rod.Page, selector string) map[string]any {
	contrastJS := `(selector) => {
		const root = selector ? document.querySelector(selector) : document.body;
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
			results.push({ color: cs.color, backgroundColor: bgColor, fontSize: parseFloat(cs.fontSize), fontWeight: cs.fontWeight, selector: el.tagName.toLowerCase() });
		}
		return results;
	}`
	contrastResult, _ := page.Eval(contrastJS, selector)
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

	touchJS := `(selector) => {
		const root = selector ? document.querySelector(selector) : document.body;
		if (!root) return [];
		const interactive = root.querySelectorAll('a, button, input, select, textarea, [role="button"], [role="link"], [tabindex]');
		const results = [];
		for (const el of interactive) {
			const rect = el.getBoundingClientRect();
			if (rect.width === 0 || rect.height === 0) continue;
			results.push({ tag: el.tagName.toLowerCase(), width: rect.width, height: rect.height });
		}
		return results;
	}`
	touchResult, _ := page.Eval(touchJS, selector)
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

	typoJS := `(selector) => {
		const root = selector ? document.querySelector(selector) : document.body;
		if (!root) return [];
		const els = root.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, li, td, th, label');
		const results = [];
		for (const el of els) {
			const cs = window.getComputedStyle(el);
			if (cs.display === 'none') continue;
			if (!el.textContent.trim()) continue;
			const isHeading = /^h[1-6]$/i.test(el.tagName);
			results.push({ tag: el.tagName.toLowerCase(), fontSize: parseFloat(cs.fontSize), lineHeight: cs.lineHeight, fontFamily: cs.fontFamily, isHeading: isHeading });
		}
		return results;
	}`
	typoResult, _ := page.Eval(typoJS, selector)
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
		"touchTargets": map[string]any{
			"total":      touchTotal,
			"violations": touchViolations,
		},
	}
}

// captureAxTree captures the accessibility tree from the current page.
func captureAxTree(page *rod.Page) map[string]any {
	tree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return map[string]any{"nodes": []any{}}
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
		nodes = append(nodes, n)
	}
	return map[string]any{
		"nodeCount": len(nodes),
		"nodes":     nodes,
	}
}

// gitCommitHash returns the current HEAD commit hash, or empty string if not in a git repo.
func gitCommitHash() string {
	out, err := exec.Command("git", "rev-parse", "--short=8", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
