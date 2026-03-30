package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-rod/rod"
	"github.com/lars/kaleidoscope/browser"
	"github.com/lars/kaleidoscope/output"
)

func RunJS(args []string) {
	expr := strings.Join(getNonFlagArgs(args), " ")
	if expr == "" {
		output.Fail("js", fmt.Errorf("missing expression"), "Usage: ks js <expression>")
		os.Exit(2)
	}

	err := browser.WithPage(func(page *rod.Page) error {
		// Wrap in arrow function since rod's Eval expects a function body
		result, err := page.Eval(`() => ` + expr)
		if err != nil {
			return err
		}

		output.Success("js", map[string]any{
			"value": result.Value.Val(),
		})
		return nil
	})

	if err != nil {
		output.Fail("js", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
