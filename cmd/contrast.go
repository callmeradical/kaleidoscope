package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/lars/kaleidoscope/analysis"
	"github.com/lars/kaleidoscope/browser"
	"github.com/lars/kaleidoscope/output"
)

func RunContrast(args []string) {
	selector := getArg(args)

	err := browser.WithPage(func(page *rod.Page) error {
		// JS to collect text elements and their colors
		jsExpr := `(selector) => {
			const root = selector ? document.querySelector(selector) : document.body;
			if (!root) return { error: 'Element not found' };

			const textEls = root.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, a, li, td, th, label, button, input, textarea');
			const results = [];

			for (const el of textEls) {
				const cs = window.getComputedStyle(el);
				if (cs.display === 'none' || cs.visibility === 'hidden') continue;

				// Walk up to find effective background color
				let bgColor = 'rgba(0, 0, 0, 0)';
				let current = el;
				while (current && current !== document.documentElement) {
					const bg = window.getComputedStyle(current).backgroundColor;
					if (bg && bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent') {
						bgColor = bg;
						break;
					}
					current = current.parentElement;
				}
				if (bgColor === 'rgba(0, 0, 0, 0)') bgColor = 'rgb(255, 255, 255)';

				const text = el.textContent.trim();
				if (!text) continue;

				results.push({
					selector: el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (el.className ? '.' + el.className.split(' ').join('.') : ''),
					text: text.substring(0, 50),
					color: cs.color,
					backgroundColor: bgColor,
					fontSize: parseFloat(cs.fontSize),
					fontWeight: cs.fontWeight,
				});
			}
			return results;
		}`

		result, err := page.Eval(jsExpr, selector)
		if err != nil {
			return err
		}

		// Process results through our contrast analysis
		elements := result.Value.Val()
		elList, ok := elements.([]interface{})
		if !ok {
			output.Success("contrast", map[string]any{
				"elements": []any{},
				"summary":  map[string]int{"total": 0},
			})
			return nil
		}

		var checks []map[string]any
		violations := 0
		passes := 0

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

			entry := map[string]any{
				"selector":     el["selector"],
				"text":         el["text"],
				"ratio":        check.Ratio,
				"meetsMinimum": check.MeetsMinimum,
				"isLargeText":  check.IsLargeText,
				"foreground":   fg,
				"background":   bg,
				"aa":           check.AANormal,
				"aaa":          check.AAANormal,
			}
			checks = append(checks, entry)

			if check.MeetsMinimum {
				passes++
			} else {
				violations++
			}
		}

		output.Success("contrast", map[string]any{
			"selector": selector,
			"elements": checks,
			"summary": map[string]int{
				"total":      len(checks),
				"passes":     passes,
				"violations": violations,
			},
		})
		return nil
	})

	if err != nil {
		output.Fail("contrast", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
