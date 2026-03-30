package cmd

import (
	"fmt"
	"os"

	"github.com/go-rod/rod"
	"github.com/lars/kaleidoscope/browser"
	"github.com/lars/kaleidoscope/output"
)

func RunOpen(args []string) {
	url := getArg(args)
	if url == "" {
		output.Fail("open", fmt.Errorf("missing URL argument"), "Usage: ks open <url>")
		os.Exit(2)
	}

	err := browser.WithPage(func(page *rod.Page) error {
		if err := page.Navigate(url); err != nil {
			return err
		}
		if err := page.WaitLoad(); err != nil {
			return err
		}

		title := ""
		titleEl, err := page.Element("title")
		if err == nil {
			title, _ = titleEl.Text()
		}

		// Update state with current URL
		state, _ := browser.ReadState()
		if state != nil {
			state.CurrentURL = url
			_ = browser.WriteState(state)
		}

		output.Success("open", map[string]any{
			"url":   page.MustInfo().URL,
			"title": title,
		})
		return nil
	})

	if err != nil {
		output.Fail("open", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
