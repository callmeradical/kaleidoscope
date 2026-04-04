package cmd

import (
	"github.com/callmeradical/kaleidoscope/analysis"
	"github.com/callmeradical/kaleidoscope/snapshot"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// auditPage runs a full UX/a11y audit on page and returns the result.
// It does not produce any output — call sites are responsible for writing results.
func auditPage(page *rod.Page) (snapshot.AuditResult, error) {
	var result snapshot.AuditResult

	// 1. Accessibility audit via CDP
	axTree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err == nil {
		total := len(axTree.Nodes)
		ignored := 0
		for _, n := range axTree.Nodes {
			if n.Ignored {
				ignored++
			}
		}
		result.AXTotalNodes = total
		result.AXActiveNodes = total - ignored
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
	contrastResult, _ := page.Eval(contrastJS, "")
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
					result.ContrastViolations++
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
	touchResult, _ := page.Eval(touchJS, "")
	if touchResult != nil {
		if elList, ok := touchResult.Value.Val().([]interface{}); ok {
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
					result.TouchViolations++
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
	typoResult, _ := page.Eval(typoJS, "")
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
				result.TypographyWarnings += len(check.Warnings)
			}
		}
	}

	result.TotalIssues = result.ContrastViolations + result.TouchViolations + result.TypographyWarnings
	return result, nil
}

// axTreePage extracts the full accessibility tree from page as a slice of AXNode.
func axTreePage(page *rod.Page) ([]snapshot.AXNode, error) {
	tree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return nil, err
	}

	var nodes []snapshot.AXNode
	for _, node := range tree.Nodes {
		if node.Ignored {
			continue
		}
		n := snapshot.AXNode{
			NodeID: string(node.NodeID),
		}
		if node.Role != nil {
			n.Role = node.Role.Value.Str()
		}
		if node.Name != nil {
			n.Name = node.Name.Value.Str()
		}
		if len(node.ChildIDs) > 0 {
			n.Children = make([]string, len(node.ChildIDs))
			for i, id := range node.ChildIDs {
				n.Children[i] = string(id)
			}
		}
		if len(node.Properties) > 0 {
			n.Properties = make(map[string]any)
			for _, p := range node.Properties {
				n.Properties[string(p.Name)] = p.Value.Value
			}
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}
