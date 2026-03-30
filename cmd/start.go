package cmd

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunStart(args []string) {
	local := hasFlag(args, "--local")

	state, err := browser.Start(local)
	if err != nil {
		output.Fail("start", err, "")
		os.Exit(2)
	}

	output.Success("start", map[string]any{
		"pid":        state.PID,
		"wsEndpoint": state.WSEndpoint,
		"viewport":   fmt.Sprintf("%dx%d", state.Viewport.Width, state.Viewport.Height),
	})
}
