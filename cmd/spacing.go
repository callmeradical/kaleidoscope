package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/analysis"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunSpacing(args []string) {
	selector := getArg(args)

	err := browser.WithPage(func(page *rod.Page) error {
		jsExpr := `(selector) => {
			const root = selector ? document.querySelector(selector) : document.body;
			if (!root) return { error: 'Element not found' };

			// Find sibling groups and measure gaps
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

		result, err := page.Eval(jsExpr, selector)
		if err != nil {
			return err
		}

		rawGroups := result.Value.Val()
		groupList, ok := rawGroups.([]interface{})
		if !ok {
			output.Success("spacing", map[string]any{
				"groups": []any{},
			})
			return nil
		}

		var analyzed []map[string]any
		totalInconsistencies := 0

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
				switch v := g.(type) {
				case float64:
					gaps[i] = v
				}
			}

			spacingResult := analysis.AnalyzeSpacing(gaps)
			totalInconsistencies += len(spacingResult.Inconsistencies)

			entry := map[string]any{
				"container":       group["container"],
				"childCount":      group["childCount"],
				"gaps":            spacingResult.Gaps,
				"detectedScale":   spacingResult.DetectedScale,
				"inconsistencies": spacingResult.Inconsistencies,
			}
			analyzed = append(analyzed, entry)
		}

		output.Success("spacing", map[string]any{
			"selector": selector,
			"groups":   analyzed,
			"summary": map[string]int{
				"groupsAnalyzed":     len(analyzed),
				"totalInconsistencies": totalInconsistencies,
			},
		})
		return nil
	})

	if err != nil {
		output.Fail("spacing", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
