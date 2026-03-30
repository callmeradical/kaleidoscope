package cmd

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
)

func RunStop(_ []string) {
	if err := browser.Stop(); err != nil {
		output.Fail("stop", err, "Is the browser running? Check: ks status")
		os.Exit(2)
	}

	output.Success("stop", map[string]any{
		"message": fmt.Sprintf("browser stopped"),
	})
}
