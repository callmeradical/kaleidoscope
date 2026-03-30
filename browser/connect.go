package browser

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// WithBrowser connects to the running browser, executes fn, then disconnects.
// We do NOT call browser.Close() as that sends BrowserClose and kills Chrome.
// Instead we just let the WebSocket connection drop when the process exits.
func WithBrowser(fn func(browser *rod.Browser) error) error {
	state, err := ReadState()
	if err != nil {
		return fmt.Errorf("no browser running (run: ks start): %w", err)
	}

	browser := rod.New().ControlURL(state.WSEndpoint)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("connecting to browser: %w", err)
	}
	// Do NOT defer browser.Close() — that kills Chrome.
	// The WebSocket connection drops naturally when the CLI process exits.

	return fn(browser)
}

// WithPage connects to the browser and provides the first available page,
// or creates a new one if none exist.
func WithPage(fn func(page *rod.Page) error) error {
	return WithBrowser(func(browser *rod.Browser) error {
		pages, err := browser.Pages()
		if err != nil {
			return fmt.Errorf("listing pages: %w", err)
		}

		var page *rod.Page
		if len(pages) > 0 {
			page = pages.First()
		} else {
			page, err = browser.Page(proto.TargetCreateTarget{})
			if err != nil {
				return fmt.Errorf("creating page: %w", err)
			}
		}

		return fn(page)
	})
}
