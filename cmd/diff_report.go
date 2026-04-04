package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pixeldiff "github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/callmeradical/kaleidoscope/report"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

// RunDiffReport generates a side-by-side HTML diff report comparing baseline vs current snapshot.
func RunDiffReport(args []string) {
	snapshotID := getArg(args)
	outputPath := getFlagValue(args, "--output")
	if outputPath == "" {
		stateDir, err := browser.StateDir()
		if err != nil {
			output.Fail("diff-report", err, "could not determine state directory")
			return
		}
		outputPath = filepath.Join(stateDir, "diff-report.html")
	}

	// Load baseline.
	baseline, err := snapshot.LoadBaseline()
	if err != nil {
		output.Fail("diff-report", err, "no baseline set; run: ks snapshot baseline <id>")
		return
	}

	// Load current snapshot.
	var current *snapshot.Snapshot
	if snapshotID != "" {
		current, err = snapshot.Load(snapshotID)
		if err != nil {
			output.Fail("diff-report", err, "snapshot not found: "+snapshotID)
			return
		}
	} else {
		current, err = snapshot.LoadLatest()
		if err != nil {
			output.Fail("diff-report", err, "no snapshots found; run: ks snapshot")
			return
		}
	}

	if baseline.ID == current.ID {
		output.Fail("diff-report", fmt.Errorf("same snapshot"), "no diff: baseline and current are the same snapshot")
		return
	}

	diffData := buildDiffData(baseline, current)

	outputPath = filepath.Clean(outputPath)
	absPath, err := report.WriteDiffFile(outputPath, &diffData)
	if err != nil {
		output.Fail("diff-report", err, "failed to write diff report")
		return
	}

	output.Success("diff-report", map[string]any{
		"path":       absPath,
		"baselineId": baseline.ID,
		"currentId":  current.ID,
		"pages":      len(diffData.Pages),
	})
}

func buildDiffData(baseline, current *snapshot.Snapshot) report.DiffData {
	data := report.DiffData{
		GeneratedAt: current.TakenAt,
		BaselineID:  baseline.ID,
		CurrentID:   current.ID,
		BaselineAt:  baseline.TakenAt,
		CurrentAt:   current.TakenAt,
	}

	breakpointNames := []string{"mobile", "tablet", "desktop", "wide"}

	for i := range current.Pages {
		curPage := &current.Pages[i]
		basePage := findPage(baseline.Pages, curPage.URL)

		var breakpoints []report.DiffBreakpoint
		for _, bpName := range breakpointNames {
			baseShot := findShot(basePage, bpName)
			curShot := findShot(curPage, bpName)

			if baseShot == nil && curShot == nil {
				continue
			}

			var baseURI, curURI, overlayURI template.URL
			var score float64
			var img1, img2 image.Image

			if baseShot != nil {
				if uri, img, err := loadImageB64(baseShot.ScreenshotPath); err == nil {
					baseURI = uri
					img1 = img
				}
			}
			if curShot != nil {
				if uri, img, err := loadImageB64(curShot.ScreenshotPath); err == nil {
					curURI = uri
					img2 = img
				}
			}
			if img1 != nil && img2 != nil {
				overlay := pixeldiff.Overlay(img1, img2, nil)
				score = pixeldiff.Score(img1, img2, 10)
				if uri, err := encodeOverlayB64(overlay); err == nil {
					overlayURI = uri
				}
			}

			var width, height int
			if curShot != nil {
				width, height = curShot.Width, curShot.Height
			} else if baseShot != nil {
				width, height = baseShot.Width, baseShot.Height
			}

			breakpoints = append(breakpoints, report.DiffBreakpoint{
				Name:        bpName,
				Width:       width,
				Height:      height,
				BaselineURI: baseURI,
				CurrentURI:  curURI,
				OverlayURI:  overlayURI,
				DiffScore:   score,
			})
		}

		data.Pages = append(data.Pages, report.DiffPage{
			URL:            curPage.URL,
			Breakpoints:    breakpoints,
			AuditDelta:     computeAuditDelta(basePage, curPage),
			ElementChanges: computeElementChanges(basePage, curPage),
		})
	}
	return data
}

// findPage returns the PageSnapshot matching url, or nil if not found.
func findPage(pages []snapshot.PageSnapshot, url string) *snapshot.PageSnapshot {
	for i := range pages {
		if pages[i].URL == url {
			return &pages[i]
		}
	}
	return nil
}

// findShot returns the BreakpointShot matching name, or nil.
func findShot(page *snapshot.PageSnapshot, name string) *snapshot.BreakpointShot {
	if page == nil {
		return nil
	}
	for i := range page.Breakpoints {
		if page.Breakpoints[i].Name == name {
			return &page.Breakpoints[i]
		}
	}
	return nil
}

// loadImageB64 reads a PNG file and returns a base64 data URI and the decoded image.
func loadImageB64(path string) (template.URL, image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", nil, err
	}
	uri := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(data))
	return uri, img, nil
}

// encodeOverlayB64 encodes an image to PNG bytes and returns a base64 data URI.
func encodeOverlayB64(img image.Image) (template.URL, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())), nil
}

// computeAuditDelta computes before/after/delta counts for each audit category.
func computeAuditDelta(base, cur *snapshot.PageSnapshot) report.AuditDelta {
	var b, c snapshot.AuditSummary
	if base != nil && base.Audit != nil {
		b = *base.Audit
	}
	if cur != nil && cur.Audit != nil {
		c = *cur.Audit
	}
	return report.AuditDelta{
		Contrast:   report.CategoryDelta{Before: b.ContrastViolations, After: c.ContrastViolations, Delta: c.ContrastViolations - b.ContrastViolations},
		Touch:      report.CategoryDelta{Before: b.TouchViolations, After: c.TouchViolations, Delta: c.TouchViolations - b.TouchViolations},
		Typography: report.CategoryDelta{Before: b.TypographyWarnings, After: c.TypographyWarnings, Delta: c.TypographyWarnings - b.TypographyWarnings},
		Spacing:    report.CategoryDelta{Before: b.SpacingIssues, After: c.SpacingIssues, Delta: c.SpacingIssues - b.SpacingIssues},
	}
}

// computeElementChanges detects appeared/disappeared/moved/resized elements.
func computeElementChanges(base, cur *snapshot.PageSnapshot) []report.ElementChangeRow {
	var changes []report.ElementChangeRow

	baseMap := make(map[string]snapshot.ElementRecord)
	if base != nil {
		for _, e := range base.Elements {
			baseMap[e.Selector] = e
		}
	}
	curMap := make(map[string]snapshot.ElementRecord)
	if cur != nil {
		for _, e := range cur.Elements {
			curMap[e.Selector] = e
		}
	}

	for sel, be := range baseMap {
		if _, ok := curMap[sel]; !ok {
			changes = append(changes, report.ElementChangeRow{
				Selector:   sel,
				ChangeType: "disappeared",
				Details:    fmt.Sprintf("role=%s name=%s", be.Role, be.Name),
			})
		}
	}
	for sel, ce := range curMap {
		if _, ok := baseMap[sel]; !ok {
			changes = append(changes, report.ElementChangeRow{
				Selector:   sel,
				ChangeType: "appeared",
				Details:    fmt.Sprintf("role=%s name=%s", ce.Role, ce.Name),
			})
		}
	}
	for sel, be := range baseMap {
		if ce, ok := curMap[sel]; ok {
			var diffs []string
			if be.X != ce.X || be.Y != ce.Y {
				diffs = append(diffs, fmt.Sprintf("position: (%.0f,%.0f)→(%.0f,%.0f)", be.X, be.Y, ce.X, ce.Y))
			}
			if be.Width != ce.Width || be.Height != ce.Height {
				diffs = append(diffs, fmt.Sprintf("size: %.0fx%.0f→%.0fx%.0f", be.Width, be.Height, ce.Width, ce.Height))
			}
			if len(diffs) > 0 {
				changeType := "moved"
				if be.Width != ce.Width || be.Height != ce.Height {
					changeType = "resized"
				}
				changes = append(changes, report.ElementChangeRow{
					Selector:   sel,
					ChangeType: changeType,
					Details:    strings.Join(diffs, "; "),
				})
			}
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Selector < changes[j].Selector
	})
	return changes
}
