package cmd

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunAxTree(args []string) {
	err := browser.WithPage(func(page *rod.Page) error {
		nodes, err := runAxTreeOnPage(page)
		if err != nil {
			return err
		}
		output.Success("ax-tree", map[string]any{
			"nodeCount": len(nodes),
			"nodes":     nodes,
		})
		return nil
	})

	if err != nil {
		output.Fail("ax-tree", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
