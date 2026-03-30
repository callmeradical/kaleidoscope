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
	"github.com/callmeradical/kaleidoscope/report"
)

func RunReport(args []string) {
	outputPath := getFlagValue(args, "--output")
	fullPage := hasFlag(args, "--full-page")
	selector := getFlagValue(args, "--selector")

	err := browser.WithPage(func(page *rod.Page) error {
		info := page.MustInfo()
		state, _ := browser.ReadState()
		vp := "1280x720"
		if state != nil && state.Viewport != nil {
			vp = fmt.Sprintf("%dx%d", state.Viewport.Width, state.Viewport.Height)
		}

		data := &report.Data{
			URL:         info.URL,
			Title:       info.Title,
			GeneratedAt: time.Now(),
			Viewport:    vp,
		}

		// --- Screenshots at each breakpoint ---
		ssDir, err := browser.ScreenshotDir()
		if err != nil {
			return fmt.Errorf("creating screenshot dir: %w", err)
		}

		var origW, origH int
		if state != nil && state.Viewport != nil {
			origW = state.Viewport.Width
			origH = state.Viewport.Height
		}

		ts := time.Now().UnixMilli()
		breakpoints := []struct {
			Name   string
			Width  int
			Height int
		}{
			{"mobile", 375, 812},
			{"tablet", 768, 1024},
			{"desktop", 1280, 720},
			{"wide", 1920, 1080},
		}

		for _, bp := range breakpoints {
			if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
				Width:  bp.Width,
				Height: bp.Height,
			}); err != nil {
				return fmt.Errorf("setting viewport for %s: %w", bp.Name, err)
			}
			page.MustWaitStable()

			filename := fmt.Sprintf("%d-report-%s-%dx%d.png", ts, bp.Name, bp.Width, bp.Height)
			path := filepath.Join(ssDir, filename)

			ssData, err := page.Screenshot(fullPage, nil)
			if err != nil {
				return fmt.Errorf("screenshot at %s: %w", bp.Name, err)
			}
			if err := os.WriteFile(path, ssData, 0644); err != nil {
				return err
			}

			dataURI, err := report.LoadScreenshot(path)
			if err != nil {
				return err
			}

			data.Screenshots = append(data.Screenshots, report.Screenshot{
				Breakpoint: bp.Name,
				Width:      bp.Width,
				Height:     bp.Height,
				Path:       path,
				DataURI:    dataURI,
			})
		}

		// Restore viewport
		if origW > 0 && origH > 0 {
			_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
				Width:  origW,
				Height: origH,
			})
		}

		// --- Contrast analysis ---
		contrastJS := `(selector) => {
			const root = selector ? document.querySelector(selector) : document.body;
			if (!root) return [];
			const els = root.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, a, li, td, th, label, button, input, textarea');
			const results = [];
			for (const el of els) {
				const cs = window.getComputedStyle(el);
				if (cs.display === 'none' || cs.visibility === 'hidden') continue;
				const text = el.textContent.trim();
				if (!text) continue;
				let bgColor = 'rgba(0, 0, 0, 0)';
				let cur = el;
				while (cur && cur !== document.documentElement) {
					const bg = window.getComputedStyle(cur).backgroundColor;
					if (bg && bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent') { bgColor = bg; break; }
					cur = cur.parentElement;
				}
				if (bgColor === 'rgba(0, 0, 0, 0)') bgColor = 'rgb(255, 255, 255)';
				results.push({
					selector: el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().split(/\s+/).join('.') : ''),
					text: text.substring(0, 50),
					color: cs.color,
					backgroundColor: bgColor,
					fontSize: parseFloat(cs.fontSize),
					fontWeight: cs.fontWeight,
				});
			}
			return results;
		}`
		contrastResult, _ := page.Eval(contrastJS, selector)
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
					if err != nil {
						continue
					}
					if !check.MeetsMinimum {
						data.ContrastIssues = append(data.ContrastIssues, report.ContrastIssue{
							Selector:   stringVal(el["selector"]),
							Text:       stringVal(el["text"]),
							Ratio:      check.Ratio,
							Foreground: fg,
							Background: bg,
							IsLarge:    check.IsLargeText,
							AA:         check.AANormal,
							AAA:        check.AAANormal,
						})
					}
				}
			}
		}
		data.ContrastViolations = len(data.ContrastIssues)

		// --- Touch targets ---
		touchJS := `(selector) => {
			const root = selector ? document.querySelector(selector) : document.body;
			if (!root) return [];
			const interactive = root.querySelectorAll('a, button, input, select, textarea, [role="button"], [role="link"], [tabindex]');
			const results = [];
			for (const el of interactive) {
				const rect = el.getBoundingClientRect();
				if (rect.width === 0 || rect.height === 0) continue;
				results.push({
					tag: el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().split(/\s+/).join('.') : ''),
					width: rect.width,
					height: rect.height,
				});
			}
			return results;
		}`
		touchResult, _ := page.Eval(touchJS, selector)
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
						data.TouchIssues = append(data.TouchIssues, report.TouchIssue{
							Tag:       tag,
							Width:     w,
							Height:    h,
							Violation: check.Violation,
						})
					}
				}
			}
		}
		data.TouchViolations = len(data.TouchIssues)

		// --- Typography ---
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
					tag, _ := el["tag"].(string)

					lineHeight := analysis.ParseLineHeight(lineHeightStr, fontSize)
					check := analysis.CheckTypography(fontSize, lineHeight, fontFamily, isHeading)
					for _, w := range check.Warnings {
						data.TypographyIssues = append(data.TypographyIssues, report.TypographyIssue{
							Tag:        tag,
							FontSize:   fontSize,
							LineHeight: lineHeight,
							FontFamily: fontFamily,
							Warning:    w,
						})
					}
				}
			}
		}
		data.TypographyWarnings = len(data.TypographyIssues)

		// --- Spacing ---
		spacingJS := `(selector) => {
			const root = selector ? document.querySelector(selector) : document.body;
			if (!root) return [];
			const groups = [];
			const containers = root.querySelectorAll('div, section, main, article, ul, ol, nav, form, fieldset');
			for (const container of containers) {
				const children = Array.from(container.children).filter(child => {
					const cs = window.getComputedStyle(child);
					return cs.display !== 'none' && cs.visibility !== 'hidden';
				});
				if (children.length < 2) continue;
				const rects = children.map(child => child.getBoundingClientRect());
				const gaps = [];
				for (let i = 1; i < rects.length; i++) {
					const gap = rects[i].top - rects[i-1].bottom;
					if (gap >= 0) gaps.push(Math.round(gap * 10) / 10);
				}
				if (gaps.length > 0) {
					groups.push({
						container: container.tagName.toLowerCase() + (container.id ? '#' + container.id : ''),
						childCount: children.length,
						gaps: gaps,
					});
				}
			}
			return groups;
		}`
		spacingResult, _ := page.Eval(spacingJS, selector)
		if spacingResult != nil {
			if groupList, ok := spacingResult.Value.Val().([]interface{}); ok {
				for _, item := range groupList {
					group, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					rawGaps, ok := group["gaps"].([]interface{})
					if !ok {
						continue
					}
					gaps := make([]float64, len(rawGaps))
					for i, g := range rawGaps {
						if v, ok := g.(float64); ok {
							gaps[i] = v
						}
					}
					containerName, _ := group["container"].(string)
					spacingRes := analysis.AnalyzeSpacing(gaps)
					for _, inc := range spacingRes.Inconsistencies {
						data.SpacingIssueList = append(data.SpacingIssueList, report.SpacingIssue{
							Container: containerName,
							Index:     inc.Index,
							Gap:       inc.Gap,
							Expected:  inc.Expected,
						})
					}
				}
			}
		}
		data.SpacingIssues = len(data.SpacingIssueList)

		// --- Accessibility tree summary ---
		axTree, err := proto.AccessibilityGetFullAXTree{}.Call(page)
		if err == nil {
			data.AXTotalNodes = len(axTree.Nodes)
			for _, n := range axTree.Nodes {
				if !n.Ignored {
					data.AXActiveNodes++
				}
			}
		}

		// --- Total ---
		data.TotalIssues = data.ContrastViolations + data.TouchViolations + data.TypographyWarnings + data.SpacingIssues

		// --- Generate report ---
		reportDir := filepath.Join(ssDir, "..")
		if outputPath != "" {
			reportDir = filepath.Dir(outputPath)
		}

		var reportPath string
		if outputPath != "" {
			// Write to specific path
			if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
				return err
			}
			f, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer f.Close()
			if err := report.Generate(f, data); err != nil {
				return err
			}
			reportPath, _ = filepath.Abs(outputPath)
		} else {
			reportPath, err = report.WriteFile(reportDir, data)
			if err != nil {
				return fmt.Errorf("writing report: %w", err)
			}
		}

		output.Success("report", map[string]any{
			"path":        reportPath,
			"url":         info.URL,
			"title":       info.Title,
			"totalIssues": data.TotalIssues,
			"summary": map[string]any{
				"contrastViolations": data.ContrastViolations,
				"touchViolations":    data.TouchViolations,
				"typographyWarnings": data.TypographyWarnings,
				"spacingIssues":      data.SpacingIssues,
				"axActiveNodes":      data.AXActiveNodes,
			},
			"screenshots": len(data.Screenshots),
		})
		return nil
	})

	if err != nil {
		output.Fail("report", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}

func stringVal(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
