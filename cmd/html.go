package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunHTML(args []string) {
	selector := getArg(args)
	outer := hasFlag(args, "--outer")

	err := browser.WithPage(func(page *rod.Page) error {
		var html string
		var err error

		if selector == "" {
			html, err = page.HTML()
		} else {
			el, findErr := page.Element(selector)
			if findErr != nil {
				return findErr
			}
			if outer {
				html, err = el.HTML()
			} else {
				result, evalErr := el.Eval(`() => this.innerHTML`)
				if evalErr != nil {
					// Fallback to outer HTML
					html, err = el.HTML()
				} else {
					html = result.Value.Str()
				}
			}
		}

		if err != nil {
			return err
		}

		output.Success("html", map[string]any{
			"html":     html,
			"selector": selector,
		})
		return nil
	})

	if err != nil {
		output.Fail("html", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
