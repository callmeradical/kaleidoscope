package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunAudit(args []string) {
	selector := getArg(args)

	err := browser.WithPage(func(page *rod.Page) error {
		resultMap, _, err := runAudit(page, selector)
		if err != nil {
			return err
		}
		output.Success("audit", resultMap)
		return nil
	})

	if err != nil {
		output.Fail("audit", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
