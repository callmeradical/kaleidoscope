package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

var viewportPresets = map[string][2]int{
	"mobile":  {375, 812},
	"tablet":  {768, 1024},
	"desktop": {1280, 720},
	"wide":    {1920, 1080},
}

func parseViewport(arg string) (int, int, error) {
	if preset, ok := viewportPresets[strings.ToLower(arg)]; ok {
		return preset[0], preset[1], nil
	}

	parts := strings.SplitN(strings.ToLower(arg), "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid viewport: %s (use preset name or WxH)", arg)
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %s", parts[0])
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %s", parts[1])
	}
	return w, h, nil
}

func RunViewport(args []string) {
	arg := getArg(args)
	if arg == "" {
		// List presets
		presets := make(map[string]string)
		for name, size := range viewportPresets {
			presets[name] = fmt.Sprintf("%dx%d", size[0], size[1])
		}
		output.Success("viewport", map[string]any{
			"presets": presets,
			"usage":   "ks viewport <mobile|tablet|desktop|wide|WxH>",
		})
		return
	}

	width, height, err := parseViewport(arg)
	if err != nil {
		output.Fail("viewport", err, "Use: mobile, tablet, desktop, wide, or WxH")
		os.Exit(2)
	}

	err = browser.WithPage(func(page *rod.Page) error {
		err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
			Width:  width,
			Height: height,
		})
		if err != nil {
			return err
		}

		// Update state
		state, _ := browser.ReadState()
		if state != nil {
			state.Viewport = &browser.Viewport{Width: width, Height: height}
			_ = browser.WriteState(state)
		}

		output.Success("viewport", map[string]any{
			"width":  width,
			"height": height,
		})
		return nil
	})

	if err != nil {
		output.Fail("viewport", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}
