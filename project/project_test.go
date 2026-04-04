package project_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/callmeradical/kaleidoscope/project"
)

// TestLoad_NotFound verifies Load returns fs.ErrNotExist when the file is absent.
func TestLoad_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := project.Load(dir)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
}

// TestSaveLoad_RoundTrip verifies that Save followed by Load returns identical data.
func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &project.Config{
		Name:    "myproject",
		BaseURL: "http://localhost:3000",
		Paths:   []string{"/", "/dashboard"},
		Breakpoints: []project.Breakpoint{
			{Name: "mobile", Width: 375, Height: 812},
		},
	}

	if err := project.Save(dir, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := project.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", loaded.Name, original.Name)
	}
	if loaded.BaseURL != original.BaseURL {
		t.Errorf("BaseURL: got %q, want %q", loaded.BaseURL, original.BaseURL)
	}
	if len(loaded.Paths) != len(original.Paths) {
		t.Fatalf("Paths length: got %d, want %d", len(loaded.Paths), len(original.Paths))
	}
	for i, p := range original.Paths {
		if loaded.Paths[i] != p {
			t.Errorf("Paths[%d]: got %q, want %q", i, loaded.Paths[i], p)
		}
	}
	if len(loaded.Breakpoints) != len(original.Breakpoints) {
		t.Fatalf("Breakpoints length: got %d, want %d", len(loaded.Breakpoints), len(original.Breakpoints))
	}
	if loaded.Breakpoints[0].Name != original.Breakpoints[0].Name {
		t.Errorf("Breakpoints[0].Name: got %q, want %q", loaded.Breakpoints[0].Name, original.Breakpoints[0].Name)
	}
}

// TestValidate_EmptyName verifies Validate rejects an empty Name.
func TestValidate_EmptyName(t *testing.T) {
	cfg := &project.Config{BaseURL: "http://localhost", Paths: []string{"/"}}
	if err := project.Validate(cfg); err == nil {
		t.Fatal("expected error for empty Name, got nil")
	}
}

// TestValidate_EmptyBaseURL verifies Validate rejects an empty BaseURL.
func TestValidate_EmptyBaseURL(t *testing.T) {
	cfg := &project.Config{Name: "myproject", Paths: []string{"/"}}
	if err := project.Validate(cfg); err == nil {
		t.Fatal("expected error for empty BaseURL, got nil")
	}
}

// TestValidate_EmptyPaths verifies Validate rejects an empty Paths slice.
func TestValidate_EmptyPaths(t *testing.T) {
	cfg := &project.Config{Name: "myproject", BaseURL: "http://localhost"}
	if err := project.Validate(cfg); err == nil {
		t.Fatal("expected error for empty Paths, got nil")
	}
}

// TestValidate_Valid verifies Validate accepts a fully populated Config.
func TestValidate_Valid(t *testing.T) {
	cfg := &project.Config{
		Name:    "myproject",
		BaseURL: "http://localhost:3000",
		Paths:   []string{"/", "/dashboard"},
	}
	if err := project.Validate(cfg); err != nil {
		t.Fatalf("expected no error for valid config, got %v", err)
	}
}

// TestDefaultBreakpoints verifies all four standard presets are present.
func TestDefaultBreakpoints(t *testing.T) {
	bps := project.DefaultBreakpoints
	if len(bps) != 4 {
		t.Fatalf("expected 4 default breakpoints, got %d", len(bps))
	}

	want := []project.Breakpoint{
		{Name: "mobile", Width: 375, Height: 812},
		{Name: "tablet", Width: 768, Height: 1024},
		{Name: "desktop", Width: 1280, Height: 720},
		{Name: "wide", Width: 1920, Height: 1080},
	}
	for i, w := range want {
		if bps[i] != w {
			t.Errorf("DefaultBreakpoints[%d]: got %+v, want %+v", i, bps[i], w)
		}
	}
}
