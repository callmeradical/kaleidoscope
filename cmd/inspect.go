package cmd

import (
	"fmt"
	"os"

	"github.com/go-rod/rod"
	"github.com/lars/kaleidoscope/browser"
	"github.com/lars/kaleidoscope/output"
)

func RunInspect(args []string) {
	selector := getArg(args)
	if selector == "" {
		output.Fail("inspect", fmt.Errorf("missing selector"), "Usage: ks inspect <selector>")
		os.Exit(2)
	}

	err := browser.WithPage(func(page *rod.Page) error {
		el, err := page.Element(selector)
		if err != nil {
			return fmt.Errorf("element not found: %s: %w", selector, err)
		}

		// Bounding box
		box, err := el.Shape()
		if err != nil {
			return fmt.Errorf("getting element shape: %w", err)
		}

		// Get bounding box from the shape quads
		var boundingBox map[string]any
		if box != nil && len(box.Quads) > 0 {
			// Calculate bounding box from first quad
			quad := box.Quads[0]
			if len(quad) >= 8 {
				x := quad[0]
				y := quad[1]
				w := quad[2] - quad[0]
				h := quad[5] - quad[1]
				boundingBox = map[string]any{
					"x": x, "y": y, "width": w, "height": h,
				}
			}
		}

		// Computed styles via JS
		styles, err := el.Eval(`() => {
			const cs = window.getComputedStyle(this);
			return {
				color: cs.color,
				backgroundColor: cs.backgroundColor,
				fontSize: cs.fontSize,
				fontFamily: cs.fontFamily,
				fontWeight: cs.fontWeight,
				lineHeight: cs.lineHeight,
				padding: cs.padding,
				margin: cs.margin,
				display: cs.display,
				position: cs.position,
				zIndex: cs.zIndex,
				opacity: cs.opacity,
				visibility: cs.visibility,
				overflow: cs.overflow,
				width: cs.width,
				height: cs.height,
				borderRadius: cs.borderRadius,
			};
		}`)
		if err != nil {
			return fmt.Errorf("getting computed styles: %w", err)
		}

		visible, _ := el.Visible()

		result := map[string]any{
			"selector":    selector,
			"visible":     visible,
			"boundingBox": boundingBox,
			"styles":      styles.Value.Val(),
		}

		// Try to get tag name
		tagResult, err := el.Eval(`() => this.tagName.toLowerCase()`)
		if err == nil {
			result["tagName"] = tagResult.Value.Str()
		}

		output.Success("inspect", result)
		return nil
	})

	if err != nil {
		output.Fail("inspect", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
