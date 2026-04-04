package diffreport_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/callmeradical/kaleidoscope/diff"
	"github.com/callmeradical/kaleidoscope/diffreport"
	"github.com/callmeradical/kaleidoscope/snapshot"
)

func makeResult(baseID, curID string, urls ...string) *diff.Result {
	r := &diff.Result{
		BaselineID:  baseID,
		CurrentID:   curID,
		GeneratedAt: time.Now(),
	}
	for _, u := range urls {
		r.URLs = append(r.URLs, diff.URLDiff{
			URL: u,
			AuditDeltas: []diff.AuditDelta{
				{Category: "Contrast", Before: 1, After: 3, Delta: 2},
				{Category: "Touch", Before: 2, After: 1, Delta: -1},
				{Category: "Typography", Before: 0, After: 0, Delta: 0},
				{Category: "Spacing", Before: 1, After: 1, Delta: 0},
			},
			ElementChanges: []diff.ElementChange{
				{Role: "button", Name: "Submit", Selector: "button#submit", Type: "appeared"},
			},
			HasRegression: true,
		})
	}
	r.HasRegressions = len(r.URLs) > 0 && r.URLs[0].HasRegression
	return r
}

func makeSnapshot(id string, urls ...string) *snapshot.Snapshot {
	s := &snapshot.Snapshot{
		ID:        id,
		CreatedAt: time.Now(),
		CommitSHA: "deadbeef",
	}
	for _, u := range urls {
		s.URLs = append(s.URLs, snapshot.URLSnapshot{URL: u})
	}
	return s
}

// --- Build ---

func TestBuild_CorrectURLCount(t *testing.T) {
	result := makeResult("base", "cur", "https://example.com", "https://example.com/about")
	base := makeSnapshot("base", "https://example.com", "https://example.com/about")
	cur := makeSnapshot("cur", "https://example.com", "https://example.com/about")

	data, err := diffreport.Build(result, base, cur)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(data.URLs) != 2 {
		t.Errorf("expected 2 URLSections, got %d", len(data.URLs))
	}
}

func TestBuild_AuditDeltaRowFields(t *testing.T) {
	result := makeResult("base", "cur", "https://example.com")
	base := makeSnapshot("base", "https://example.com")
	cur := makeSnapshot("cur", "https://example.com")

	data, err := diffreport.Build(result, base, cur)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(data.URLs) == 0 || len(data.URLs[0].AuditDeltas) == 0 {
		t.Fatal("no audit delta rows returned")
	}

	var contrast *diffreport.AuditDeltaRow
	for i := range data.URLs[0].AuditDeltas {
		if data.URLs[0].AuditDeltas[i].Category == "Contrast" {
			contrast = &data.URLs[0].AuditDeltas[i]
			break
		}
	}
	if contrast == nil {
		t.Fatal("Contrast row not found")
	}
	if !contrast.IsWorse {
		t.Error("expected IsWorse=true for Contrast delta=2")
	}
	if contrast.IsBetter {
		t.Error("expected IsBetter=false for Contrast delta=2")
	}

	var touch *diffreport.AuditDeltaRow
	for i := range data.URLs[0].AuditDeltas {
		if data.URLs[0].AuditDeltas[i].Category == "Touch" {
			touch = &data.URLs[0].AuditDeltas[i]
			break
		}
	}
	if touch == nil {
		t.Fatal("Touch row not found")
	}
	if touch.IsWorse {
		t.Error("expected IsWorse=false for Touch delta=-1")
	}
	if !touch.IsBetter {
		t.Error("expected IsBetter=true for Touch delta=-1")
	}
}

func TestBuild_NeutralDeltaFlags(t *testing.T) {
	result := makeResult("base", "cur", "https://example.com")
	base := makeSnapshot("base", "https://example.com")
	cur := makeSnapshot("cur", "https://example.com")
	data, _ := diffreport.Build(result, base, cur)

	for _, row := range data.URLs[0].AuditDeltas {
		if row.Category == "Spacing" {
			if row.IsWorse || row.IsBetter {
				t.Errorf("Spacing delta=0: expected IsWorse=false, IsBetter=false; got IsWorse=%v, IsBetter=%v", row.IsWorse, row.IsBetter)
			}
		}
	}
}

func TestBuild_HasRegressionsPropagate(t *testing.T) {
	result := makeResult("base", "cur", "https://example.com")
	base := makeSnapshot("base", "https://example.com")
	cur := makeSnapshot("cur", "https://example.com")
	data, _ := diffreport.Build(result, base, cur)
	if !data.HasRegressions {
		t.Error("expected HasRegressions=true")
	}
}

func TestBuild_ElementChangeRowsMapped(t *testing.T) {
	result := makeResult("base", "cur", "https://example.com")
	base := makeSnapshot("base", "https://example.com")
	cur := makeSnapshot("cur", "https://example.com")
	data, _ := diffreport.Build(result, base, cur)
	if len(data.URLs[0].ElementChanges) != 1 {
		t.Fatalf("expected 1 element change, got %d", len(data.URLs[0].ElementChanges))
	}
	ec := data.URLs[0].ElementChanges[0]
	if ec.Type != "appeared" {
		t.Errorf("Type: got %q want %q", ec.Type, "appeared")
	}
	if ec.Name != "Submit" {
		t.Errorf("Name: got %q want %q", ec.Name, "Submit")
	}
}

// --- Generate ---

func TestGenerate_ValidHTMLOutput(t *testing.T) {
	result := makeResult("base", "cur", "https://example.com")
	base := makeSnapshot("base", "https://example.com")
	cur := makeSnapshot("cur", "https://example.com")
	data, err := diffreport.Build(result, base, cur)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var buf bytes.Buffer
	if err := diffreport.Generate(&buf, data); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	html := buf.String()

	for _, needle := range []string{
		"<!DOCTYPE html>",
		"Kaleidoscope Diff Report",
		"screenshot-trio",
		"https://example.com",
	} {
		if !strings.Contains(html, needle) {
			t.Errorf("generated HTML missing expected string: %q", needle)
		}
	}
}

func TestGenerate_ContainsRegressionBadge(t *testing.T) {
	result := makeResult("base", "cur", "https://example.com")
	base := makeSnapshot("base", "https://example.com")
	cur := makeSnapshot("cur", "https://example.com")
	data, _ := diffreport.Build(result, base, cur)

	var buf bytes.Buffer
	_ = diffreport.Generate(&buf, data)
	if !strings.Contains(buf.String(), "badge-regression") {
		t.Error("expected badge-regression in output when HasRegressions=true")
	}
}

func TestGenerate_NoRegressionBadge(t *testing.T) {
	result := &diff.Result{
		BaselineID:     "base",
		CurrentID:      "cur",
		GeneratedAt:    time.Now(),
		HasRegressions: false,
		URLs: []diff.URLDiff{
			{URL: "https://example.com", HasRegression: false},
		},
	}
	base := makeSnapshot("base", "https://example.com")
	cur := makeSnapshot("cur", "https://example.com")
	data, _ := diffreport.Build(result, base, cur)

	var buf bytes.Buffer
	_ = diffreport.Generate(&buf, data)
	if !strings.Contains(buf.String(), "badge-ok") {
		t.Error("expected badge-ok in output when HasRegressions=false")
	}
}

// --- WriteFile ---

func TestWriteFile_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	outputPath := filepath.Join(tmp, "subdir", "report.html")

	result := makeResult("base", "cur", "https://example.com")
	base := makeSnapshot("base", "https://example.com")
	cur := makeSnapshot("cur", "https://example.com")
	data, err := diffreport.Build(result, base, cur)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	absPath, err := diffreport.WriteFile(outputPath, data)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		t.Errorf("expected file to exist at %s: %v", absPath, err)
	}
	content, _ := os.ReadFile(absPath)
	if !strings.Contains(string(content), "<!DOCTYPE html>") {
		t.Error("written file does not contain valid HTML")
	}
}

func TestWriteFile_CreatesParentDirs(t *testing.T) {
	tmp := t.TempDir()
	deepPath := filepath.Join(tmp, "a", "b", "c", "report.html")

	result := makeResult("base", "cur")
	base := makeSnapshot("base")
	cur := makeSnapshot("cur")
	data, _ := diffreport.Build(result, base, cur)

	_, err := diffreport.WriteFile(deepPath, data)
	if err != nil {
		t.Fatalf("WriteFile with deep path: %v", err)
	}
	if _, err := os.Stat(deepPath); err != nil {
		t.Errorf("file not created at deep path: %v", err)
	}
}
