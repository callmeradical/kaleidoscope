package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/callmeradical/kaleidoscope/baseline"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunAccept is the entry point for `ks accept`. It resolves the kaleidoscope
// state directory and delegates to acceptCmd for testability.
func RunAccept(args []string) {
	if PrintUsage("accept", args) {
		return
	}
	ksDir, err := browser.StateDir()
	if err != nil {
		output.Fail("accept", err, "")
		os.Exit(2)
	}
	if code := acceptCmd(args, ksDir); code != 0 {
		os.Exit(code)
	}
}

// acceptCmd implements the accept logic and returns an exit code. Using an
// explicit ksDir parameter makes the function testable without a real browser.
func acceptCmd(args []string, ksDir string) int {
	snapshotID := getArg(args)
	urlFilter := getFlagValue(args, "--url")

	if urlFilter != "" && !strings.HasPrefix(urlFilter, "/") {
		output.Fail("accept", fmt.Errorf("url path must start with /: %s", urlFilter), "")
		return 2
	}

	store, err := snapshot.OpenStore(filepath.Join(ksDir, "snapshots"))
	if err != nil {
		output.Fail("accept", err, "")
		return 2
	}

	var snap *snapshot.Snapshot
	if snapshotID != "" {
		snap, err = store.ByID(snapshotID)
		if err != nil {
			output.Fail("accept", err, "")
			return 2
		}
	} else {
		snap, err = store.Latest()
		if err != nil {
			output.Fail("accept", err, "Run: ks snapshot")
			return 2
		}
	}

	var paths []string
	if urlFilter != "" {
		found := false
		for _, u := range snap.URLs {
			if u.Path == urlFilter {
				found = true
				break
			}
		}
		if !found {
			output.Fail("accept", errors.New("snapshot does not contain URL path: "+urlFilter), "")
			return 2
		}
		paths = []string{urlFilter}
	} else {
		for _, u := range snap.URLs {
			paths = append(paths, u.Path)
		}
	}

	mgr := baseline.NewManager(ksDir)
	updated, err := mgr.Accept(snap.ID, paths)
	if err != nil {
		output.Fail("accept", err, "")
		return 2
	}

	output.Success("accept", map[string]any{
		"snapshotId": snap.ID,
		"paths":      paths,
		"baselines":  updated.Baselines,
	})
	return 0
}
