package project_test

import (
	"os"
	"testing"

	"github.com/callmeradical/kaleidoscope/project"
)

func TestDefaultBreakpoints(t *testing.T) {
	bps := project.DefaultBreakpoints()
	if len(bps) != 4 {
		t.Fatalf("expected 4 breakpoints, got %d", len(bps))
	}

	want := []project.Breakpoint{
		{Name: "mobile", Width: 375, Height: 812},
		{Name: "tablet", Width: 768, Height: 1024},
		{Name: "desktop", Width: 1280, Height: 720},
		{Name: "wide", Width: 1920, Height: 1080},
	}
	for i, bp := range bps {
		if bp != want[i] {
			t.Errorf("breakpoint[%d]: got %+v, want %+v", i, bp, want[i])
		}
	}
}

func TestLoad_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	cfg, err := project.Load()
	if err == nil {
		t.Fatalf("expected error when .ks-project.json absent, got cfg=%+v", cfg)
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	want := &project.ProjectConfig{
		Name:    "my-project",
		BaseURL: "http://localhost:3000",
		Paths:   []string{"/", "/dashboard", "/settings"},
		Breakpoints: []project.Breakpoint{
			{Name: "mobile", Width: 375, Height: 812},
		},
	}

	if err := project.Save(want); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := project.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got.Name != want.Name {
		t.Errorf("Name: got %q, want %q", got.Name, want.Name)
	}
	if got.BaseURL != want.BaseURL {
		t.Errorf("BaseURL: got %q, want %q", got.BaseURL, want.BaseURL)
	}
	if len(got.Paths) != len(want.Paths) {
		t.Errorf("Paths len: got %d, want %d", len(got.Paths), len(want.Paths))
	}
	for i, p := range want.Paths {
		if i < len(got.Paths) && got.Paths[i] != p {
			t.Errorf("Paths[%d]: got %q, want %q", i, got.Paths[i], p)
		}
	}
	if len(got.Breakpoints) != len(want.Breakpoints) {
		t.Errorf("Breakpoints len: got %d, want %d", len(got.Breakpoints), len(want.Breakpoints))
	}
	if len(got.Breakpoints) > 0 && got.Breakpoints[0] != want.Breakpoints[0] {
		t.Errorf("Breakpoints[0]: got %+v, want %+v", got.Breakpoints[0], want.Breakpoints[0])
	}
}
