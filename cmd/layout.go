package cmd

import (
	"os"
	"strconv"

	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunLayout(args []string) {
	selector := getArg(args)
	if selector == "" {
		selector = "body"
	}
	depthStr := getFlagValue(args, "--depth")
	maxDepth := 4
	if depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil {
			maxDepth = d
		}
	}

	err := browser.WithPage(func(page *rod.Page) error {
		result, err := page.Eval(`(selector, maxDepth) => {
			function walk(el, depth) {
				if (depth > maxDepth) return null;
				const rect = el.getBoundingClientRect();
				if (rect.width === 0 && rect.height === 0) return null;

				const cs = window.getComputedStyle(el);
				if (cs.display === 'none' || cs.visibility === 'hidden') return null;

				const node = {
					tag: el.tagName.toLowerCase(),
					id: el.id || undefined,
					classes: el.className ? el.className.split(' ').filter(Boolean) : undefined,
					box: {
						x: Math.round(rect.x),
						y: Math.round(rect.y),
						width: Math.round(rect.width),
						height: Math.round(rect.height),
					},
					display: cs.display,
				};

				const children = [];
				for (const child of el.children) {
					const c = walk(child, depth + 1);
					if (c) children.push(c);
				}
				if (children.length > 0) node.children = children;

				return node;
			}

			const root = document.querySelector(selector);
			if (!root) return { error: 'Element not found: ' + selector };
			return walk(root, 0);
		}`, selector, maxDepth)

		if err != nil {
			return err
		}

		output.Success("layout", map[string]any{
			"selector": selector,
			"depth":    maxDepth,
			"tree":     result.Value.Val(),
		})
		return nil
	})

	if err != nil {
		output.Fail("layout", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
