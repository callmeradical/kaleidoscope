package cmd

import (
	"os"
	"syscall"
	"time"

	"github.com/lars/kaleidoscope/browser"
	"github.com/lars/kaleidoscope/output"
)

func RunStatus(_ []string) {
	state, err := browser.ReadState()
	if err != nil {
		output.Success("status", map[string]any{
			"running": false,
		})
		return
	}

	result := map[string]any{
		"running":    true,
		"pid":        state.PID,
		"wsEndpoint": state.WSEndpoint,
		"startedAt":  state.StartedAt.Format(time.RFC3339),
		"uptime":     time.Since(state.StartedAt).Round(time.Second).String(),
	}

	if state.CurrentURL != "" {
		result["currentUrl"] = state.CurrentURL
	}
	if state.Viewport != nil {
		result["viewport"] = map[string]int{
			"width":  state.Viewport.Width,
			"height": state.Viewport.Height,
		}
	}

	// Check if process is actually alive
	proc, findErr := os.FindProcess(state.PID)
	if findErr != nil {
		result["running"] = false
		result["stale"] = true
	} else if proc.Signal(syscall.Signal(0)) != nil {
		result["running"] = false
		result["stale"] = true
	}

	output.Success("status", result)
}
