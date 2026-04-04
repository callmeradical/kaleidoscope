package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/report"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunDiffReport implements the `ks diff-report [snapshot-id] [--output path]` command.
// It compares the latest (or specified) snapshot against the active baseline and
// generates a self-contained HTML diff report.
func RunDiffReport(args []string) {
	snapshotID := getArg(args)
	outputPath := getFlagValue(args, "--output")
	if outputPath == "" {
		outputPath = filepath.Join(".kaleidoscope", "diff-report.html")
	}

	// Load baseline manager — stub: always returns error (no concrete implementation yet).
	mgr := newBaselineManager(args)
	baselineID, err := mgr.ActiveBaselineID()
	if err != nil || baselineID == "" {
		if err == nil {
			err = errors.New("no active baseline; run: ks baseline set")
		}
		output.Fail("diff-report", err, "run: ks baseline set")
		os.Exit(2)
	}

	// Load snapshot store — stub: always returns error (no concrete implementation yet).
	store := newSnapshotStore(args)

	baselineSnap, err := store.LoadByID(baselineID)
	if err != nil {
		output.Fail("diff-report", err, "baseline snapshot not found in store")
		os.Exit(2)
	}

	var currentSnap *snapshot.Snapshot
	if snapshotID != "" {
		currentSnap, err = store.LoadByID(snapshotID)
	} else {
		currentSnap, err = store.Latest()
	}
	if err != nil || currentSnap == nil {
		if err == nil {
			err = errors.New("no snapshots found; run: ks snapshot")
		}
		output.Fail("diff-report", err, "no snapshots found; run: ks snapshot")
		os.Exit(2)
	}

	result, err := diff.Compare(baselineSnap, currentSnap)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	data := buildDiffData(baselineSnap, currentSnap, result)

	absPath, err := report.WriteDiffFile(outputPath, data)
	if err != nil {
		output.Fail("diff-report", err, "")
		os.Exit(2)
	}

	// Aggregate summary counts.
	var contrastDelta, touchDelta, typographyDelta, spacingDelta, elementChanges int
	breakpointsDiffed := 0
	for _, u := range result.URLs {
		contrastDelta += u.AuditDelta.ContrastDelta
		touchDelta += u.AuditDelta.TouchDelta
		typographyDelta += u.AuditDelta.TypographyDelta
		spacingDelta += u.AuditDelta.SpacingDelta
		elementChanges += len(u.ElementChanges)
	}
	for _, u := range data.URLs {
		breakpointsDiffed += len(u.Breakpoints)
	}

	output.Success("diff-report", map[string]any{
		"path":       absPath,
		"baselineID": data.BaselineID,
		"currentID":  data.CurrentID,
		"urlCount":   len(result.URLs),
		"summary": map[string]any{
			"contrastDelta":    contrastDelta,
			"touchDelta":       touchDelta,
			"typographyDelta":  typographyDelta,
			"spacingDelta":     spacingDelta,
			"elementChanges":   elementChanges,
			"breakpointsDiffed": breakpointsDiffed,
		},
	})
}

// buildDiffData assembles a report.DiffData from snapshot and diff data.
func buildDiffData(baseline, current *snapshot.Snapshot, result *diff.DiffResult) *report.DiffData {
	// Build lookup maps for O(1) access.
	diffByURL := make(map[string]*diff.URLDiff, len(result.URLs))
	for i := range result.URLs {
		u := &result.URLs[i]
		diffByURL[u.URL] = u
	}

	baselineByURL := make(map[string]*snapshot.URLSnapshot, len(baseline.URLs))
	for i := range baseline.URLs {
		u := &baseline.URLs[i]
		baselineByURL[u.URL] = u
	}

	currentByURL := make(map[string]*snapshot.URLSnapshot, len(current.URLs))
	for i := range current.URLs {
		u := &current.URLs[i]
		currentByURL[u.URL] = u
	}

	// Iterate over baseline URLs in order to build sections.
	sections := make([]report.URLDiffSection, 0, len(baseline.URLs))
	for _, baseURL := range baseline.URLs {
		ud := diffByURL[baseURL.URL]
		currURL := currentByURL[baseURL.URL]

		// Map audit delta.
		var deltaRow report.AuditDeltaRow
		if ud != nil {
			d := ud.AuditDelta
			deltaRow = report.AuditDeltaRow{
				ContrastBefore:   d.ContrastBefore,
				ContrastAfter:    d.ContrastAfter,
				ContrastDelta:    d.ContrastDelta,
				TouchBefore:      d.TouchBefore,
				TouchAfter:       d.TouchAfter,
				TouchDelta:       d.TouchDelta,
				TypographyBefore: d.TypographyBefore,
				TypographyAfter:  d.TypographyAfter,
				TypographyDelta:  d.TypographyDelta,
				SpacingBefore:    d.SpacingBefore,
				SpacingAfter:     d.SpacingAfter,
				SpacingDelta:     d.SpacingDelta,
			}
		}

		// Map element changes.
		var changeRows []report.ElementChangeRow
		if ud != nil {
			for _, ec := range ud.ElementChanges {
				changeRows = append(changeRows, report.ElementChangeRow{
					Role:     ec.Role,
					Name:     ec.Name,
					Selector: ec.Selector,
					Type:     ec.Type,
					Details:  ec.Details,
				})
			}
		}

		// Build a lookup map for current breakpoints by name.
		currBPByName := make(map[string]*snapshot.BreakpointCapture)
		if currURL != nil {
			for i := range currURL.Breakpoints {
				bp := &currURL.Breakpoints[i]
				currBPByName[bp.Name] = bp
			}
		}

		// Determine PixelDiff info (per URLDiff, applied to all breakpoints).
		var pixelDiffPercent float64
		var pixelDiffPath string
		var hasDiff bool
		if ud != nil && ud.PixelDiff != nil {
			pixelDiffPercent = ud.PixelDiff.DiffPercent
			pixelDiffPath = ud.PixelDiff.DiffPath
			hasDiff = pixelDiffPercent > 0.0
		}

		// Build breakpoint rows from baseline breakpoints.
		var bpRows []report.BreakpointDiffRow
		for _, bp := range baseURL.Breakpoints {
			baselineURI, _ := report.LoadScreenshot(bp.ScreenshotPath)

			currentURI, _ := report.LoadScreenshot("") // empty = no screenshot
			if currBP, ok := currBPByName[bp.Name]; ok {
				currentURI, _ = report.LoadScreenshot(currBP.ScreenshotPath)
			}

			overlayURI, _ := report.LoadScreenshot("") // empty = no overlay
			if hasDiff && pixelDiffPath != "" {
				overlayURI, _ = report.LoadScreenshot(pixelDiffPath)
			}

			bpRows = append(bpRows, report.BreakpointDiffRow{
				Name:           bp.Name,
				Width:          bp.Width,
				Height:         bp.Height,
				BaselineURI:    baselineURI,
				CurrentURI:     currentURI,
				DiffOverlayURI: overlayURI,
				DiffPercent:    pixelDiffPercent,
				HasDiff:        hasDiff,
			})
		}

		sections = append(sections, report.URLDiffSection{
			URL:            baseURL.URL,
			Breakpoints:    bpRows,
			AuditDelta:     deltaRow,
			ElementChanges: changeRows,
		})
	}

	return &report.DiffData{
		BaselineID:  baseline.ID,
		CurrentID:   current.ID,
		GeneratedAt: time.Now(),
		URLs:        sections,
	}
}

// newBaselineManager constructs the active baseline manager.
// Stub: always returns an error so that RunDiffReport fails gracefully.
func newBaselineManager(_ []string) snapshot.BaselineManager {
	return &stubBaselineManager{}
}

// newSnapshotStore constructs the snapshot store.
// Stub: always returns errors so that RunDiffReport fails gracefully.
func newSnapshotStore(_ []string) snapshot.Store {
	return &stubSnapshotStore{}
}

// stubBaselineManager is a placeholder that always reports no active baseline.
type stubBaselineManager struct{}

func (s *stubBaselineManager) ActiveBaselineID() (string, error) {
	return "", errors.New("baseline manager not implemented; depends on US-003")
}

// stubSnapshotStore is a placeholder that always returns errors.
type stubSnapshotStore struct{}

func (s *stubSnapshotStore) Latest() (*snapshot.Snapshot, error) {
	return nil, errors.New("snapshot store not implemented; depends on US-003")
}

func (s *stubSnapshotStore) LoadByID(_ string) (*snapshot.Snapshot, error) {
	return nil, errors.New("snapshot store not implemented; depends on US-003")
}
