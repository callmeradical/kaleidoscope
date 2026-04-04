package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunAudit(args []string) {
	selector := getArg(args)

	err := browser.WithPage(func(page *rod.Page) error {
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

		// 2-4. Contrast, touch-target, typography via shared core
		summary, err := runAuditOnPage(page)
		if err != nil {
			return err
		}

		// Count touch total separately for the detailed output
		touchJS := `() => {
			const interactive = document.body.querySelectorAll('a, button, input, select, textarea, [role="button"], [role="link"], [tabindex]');
			let count = 0;
			for (const el of interactive) {
				const rect = el.getBoundingClientRect();
				if (rect.width > 0 && rect.height > 0) count++;
			}
			return count;
		}`
		touchTotal := 0
		if tr, err2 := page.Eval(touchJS); err2 == nil && tr != nil {
			if v, ok := tr.Value.Val().(float64); ok {
				touchTotal = int(v)
			}
		}

		output.Success("audit", map[string]any{
			"selector": selector,
			"summary": map[string]any{
				"totalIssues":        summary.TotalIssues,
				"contrastViolations": summary.ContrastViolations,
				"touchViolations":    summary.TouchViolations,
				"typographyWarnings": summary.TypographyWarnings,
			},
			"accessibility": axSummary,
			"contrast": map[string]any{
				"violations": summary.ContrastViolations,
			},
			"touchTargets": map[string]any{
				"total":      touchTotal,
				"violations": summary.TouchViolations,
			},
			"typography": map[string]any{
				"warnings": summary.TypographyWarnings,
			},
		})
		return nil
	})

	if err != nil {
		output.Fail("audit", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
