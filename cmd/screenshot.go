package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/lars/kaleidoscope/browser"
	"github.com/lars/kaleidoscope/output"
)

func RunScreenshot(args []string) {
	selector := getFlagValue(args, "--selector")
	fullPage := hasFlag(args, "--full-page")
	outputPath := getFlagValue(args, "--output")

	err := browser.WithPage(func(page *rod.Page) error {
		// Determine output path
		if outputPath == "" {
			dir, err := browser.ScreenshotDir()
			if err != nil {
				return fmt.Errorf("creating screenshot directory: %w", err)
			}
			outputPath = filepath.Join(dir, fmt.Sprintf("%d.png", time.Now().UnixMilli()))
		}

		if selector != "" {
			// Element screenshot
			el, err := page.Element(selector)
			if err != nil {
				return fmt.Errorf("element not found: %s: %w", selector, err)
			}
			data, err := el.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)
			if err != nil {
				return fmt.Errorf("taking element screenshot: %w", err)
			}
			if err := os.WriteFile(outputPath, data, 0644); err != nil {
				return err
			}
		} else if fullPage {
			data, err := page.Screenshot(true, nil)
			if err != nil {
				return fmt.Errorf("taking full page screenshot: %w", err)
			}
			if err := os.WriteFile(outputPath, data, 0644); err != nil {
				return err
			}
		} else {
			data, err := page.Screenshot(false, nil)
			if err != nil {
				return fmt.Errorf("taking screenshot: %w", err)
			}
			if err := os.WriteFile(outputPath, data, 0644); err != nil {
				return err
			}
		}

		info := page.MustInfo()
		state, _ := browser.ReadState()
		vp := map[string]int{"width": 1280, "height": 720}
		if state != nil && state.Viewport != nil {
			vp["width"] = state.Viewport.Width
			vp["height"] = state.Viewport.Height
		}

		output.Success("screenshot", map[string]any{
			"path":     outputPath,
			"url":      info.URL,
			"viewport": vp,
			"fullPage": fullPage,
			"selector": selector,
		})
		return nil
	})

	if err != nil {
		output.Fail("screenshot", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
