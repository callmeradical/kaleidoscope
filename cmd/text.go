package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunText(args []string) {
	selector := getArg(args)

	err := browser.WithPage(func(page *rod.Page) error {
		var text string
		var err error

		if selector == "" {
			el, findErr := page.Element("body")
			if findErr != nil {
				return findErr
			}
			text, err = el.Text()
		} else {
			el, findErr := page.Element(selector)
			if findErr != nil {
				return findErr
			}
			text, err = el.Text()
		}

		if err != nil {
			return err
		}

		output.Success("text", map[string]any{
			"text":     text,
			"selector": selector,
		})
		return nil
	})

	if err != nil {
		output.Fail("text", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
