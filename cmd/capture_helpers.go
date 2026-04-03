package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/callmeradical/kaleidoscope/analysis"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// validateURL rejects non-http/https URLs to prevent file:// and javascript: navigation.
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme %q not allowed (only http/https): %s", u.Scheme, rawURL)
	}
	return nil
}

// captureAuditData runs the full UX/a11y audit on page and returns a summary and raw result map.
func captureAuditData(page *rod.Page) (snapshot.AuditSummary, map[string]any, error) {
	selector := ""

	// 1. Accessibility audit via CDP
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

	// 2. Contrast check
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

	// 3. Touch targets
	touchJS := `(selector) => {
		const root = selector ? document.querySelector(selector) : document.body;
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

	// 4. Typography check
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
	sum := snapshot.AuditSummary{
		TotalIssues:        totalIssues,
		ContrastViolations: contrastViolations,
		TouchViolations:    touchViolations,
		TypographyWarnings: typoWarnings,
	}
	raw := map[string]any{
		"selector": selector,
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
	return sum, raw, nil
}

// captureAxTreeData queries the CDP accessibility tree and returns nodes and count.
func captureAxTreeData(page *rod.Page) ([]map[string]any, int, error) {
	tree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return nil, 0, err
	}

	nodes := make([]map[string]any, 0)
	for _, node := range tree.Nodes {
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
		if node.Ignored {
			continue
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
	return nodes, len(nodes), nil
}

// captureBreakpointsData captures screenshots at each breakpoint, writing PNGs to destDir.
// Returns the list of written filenames.
func captureBreakpointsData(page *rod.Page, destDir string, fullPage bool) ([]string, error) {
	// Save current viewport to restore later
	state, _ := browser.ReadState()
	var origW, origH int
	if state != nil && state.Viewport != nil {
		origW = state.Viewport.Width
		origH = state.Viewport.Height
	}

	var filenames []string
	for _, bp := range defaultBreakpoints {
		err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
			Width:  bp.Width,
			Height: bp.Height,
		})
		if err != nil {
			return filenames, fmt.Errorf("setting viewport for %s: %w", bp.Name, err)
		}

		page.MustWaitStable()

		filename := bp.Name + ".png"
		path := filepath.Join(destDir, filename)

		data, err := page.Screenshot(fullPage, nil)
		if err != nil {
			return filenames, fmt.Errorf("screenshot at %s: %w", bp.Name, err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return filenames, err
		}
		filenames = append(filenames, filename)
	}

	// Restore original viewport
	if origW > 0 && origH > 0 {
		_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
			Width:  origW,
			Height: origH,
		})
	}

	return filenames, nil
}

// writeJSON marshals data with indentation and writes it to path with 0644 permissions.
func writeJSON(path string, data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
