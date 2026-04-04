package cmd

import (
	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/analysis"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// runAuditOnPage runs contrast, touch-target, and typography checks on page
// and returns a summary suitable for embedding in a snapshot manifest.
func runAuditOnPage(page *rod.Page) (snapshot.AuditSummary, error) {
	// Contrast check
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

	touchViolations := 0
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
					touchViolations++
				}
			}
		}
	}

	// Typography check
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
	return snapshot.AuditSummary{
		TotalIssues:        totalIssues,
		ContrastViolations: contrastViolations,
		TouchViolations:    touchViolations,
		TypographyWarnings: typoWarnings,
	}, nil
}
