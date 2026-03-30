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

type breakpoint struct {
	Name   string
	Width  int
	Height int
}

var defaultBreakpoints = []breakpoint{
	{"mobile", 375, 812},
	{"tablet", 768, 1024},
	{"desktop", 1280, 720},
	{"wide", 1920, 1080},
}

func RunBreakpoints(args []string) {
	fullPage := hasFlag(args, "--full-page")

	err := browser.WithPage(func(page *rod.Page) error {
		dir, err := browser.ScreenshotDir()
		if err != nil {
			return err
		}

		ts := time.Now().UnixMilli()
		var results []map[string]any

		// Save current viewport to restore later
		state, _ := browser.ReadState()
		var origW, origH int
		if state != nil && state.Viewport != nil {
			origW = state.Viewport.Width
			origH = state.Viewport.Height
		}

		for _, bp := range defaultBreakpoints {
			err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
				Width:  bp.Width,
				Height: bp.Height,
			})
			if err != nil {
				return fmt.Errorf("setting viewport for %s: %w", bp.Name, err)
			}

			// Wait for layout to settle
			page.MustWaitStable()

			filename := fmt.Sprintf("%d-%s-%dx%d.png", ts, bp.Name, bp.Width, bp.Height)
			path := filepath.Join(dir, filename)

			data, err := page.Screenshot(fullPage, nil)
			if err != nil {
				return fmt.Errorf("screenshot at %s: %w", bp.Name, err)
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				return err
			}

			results = append(results, map[string]any{
				"breakpoint": bp.Name,
				"width":      bp.Width,
				"height":     bp.Height,
				"path":       path,
			})
		}

		// Restore original viewport
		if origW > 0 && origH > 0 {
			_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
				Width:  origW,
				Height: origH,
			})
		}

		output.Success("breakpoints", map[string]any{
			"screenshots": results,
			"url":         page.MustInfo().URL,
		})
		return nil
	})

	if err != nil {
		output.Fail("breakpoints", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
