package project_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/callmeradical/kaleidoscope/project"
)

// chdir changes to dir and restores the original on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// tempDir creates a temp directory, changes into it, and restores on cleanup.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ks-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	chdir(t, dir)
	return dir
}

func TestExists_False(t *testing.T) {
	tempDir(t)
	if project.Exists() {
		t.Error("Exists() = true, want false when file does not exist")
	}
}

func TestExists_TrueAfterWrite(t *testing.T) {
	tempDir(t)
	cfg := &project.Config{
		Name:        "test",
		BaseURL:     "http://localhost",
		Paths:       []string{"/"},
		Breakpoints: project.DefaultBreakpoints,
	}
	if err := project.Write(cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !project.Exists() {
		t.Error("Exists() = false, want true after Write()")
	}
}

func TestWriteRead_RoundTrip(t *testing.T) {
	tempDir(t)
	want := &project.Config{
		Name:        "my-app",
		BaseURL:     "http://localhost:3000",
		Paths:       []string{"/", "/dashboard", "/settings"},
		Breakpoints: project.DefaultBreakpoints,
	}
	if err := project.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := project.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if got.BaseURL != want.BaseURL {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, want.BaseURL)
	}
	if len(got.Paths) != len(want.Paths) {
		t.Errorf("len(Paths) = %d, want %d", len(got.Paths), len(want.Paths))
	} else {
		for i, p := range want.Paths {
			if got.Paths[i] != p {
				t.Errorf("Paths[%d] = %q, want %q", i, got.Paths[i], p)
			}
		}
	}
	if len(got.Breakpoints) != len(want.Breakpoints) {
		t.Errorf("len(Breakpoints) = %d, want %d", len(got.Breakpoints), len(want.Breakpoints))
	} else {
		for i, bp := range want.Breakpoints {
			if got.Breakpoints[i] != bp {
				t.Errorf("Breakpoints[%d] = %+v, want %+v", i, got.Breakpoints[i], bp)
			}
		}
	}
}

func TestWrite_JSONIndentation(t *testing.T) {
	dir := tempDir(t)
	cfg := &project.Config{
		Name:        "indent-test",
		BaseURL:     "http://example.com",
		Paths:       []string{"/"},
		Breakpoints: project.DefaultBreakpoints,
	}
	if err := project.Write(cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, project.Filename))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Verify it is valid JSON
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	// Verify 2-space indentation by checking first line after opening brace
	lines := splitLines(string(data))
	if len(lines) < 2 {
		t.Fatalf("expected multi-line JSON, got: %s", data)
	}
	// Second line should start with exactly two spaces
	if len(lines[1]) < 2 || lines[1][0] != ' ' || lines[1][1] != ' ' || (len(lines[1]) > 2 && lines[1][2] == ' ') {
		t.Errorf("expected 2-space indentation on line 2, got: %q", lines[1])
	}
}

func TestWrite_FieldNames(t *testing.T) {
	tempDir(t)
	cfg := &project.Config{
		Name:        "field-test",
		BaseURL:     "http://example.com",
		Paths:       []string{"/"},
		Breakpoints: project.DefaultBreakpoints,
	}
	if err := project.Write(cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(project.Filename)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"name", "baseURL", "paths", "breakpoints"} {
		if _, ok := out[key]; !ok {
			t.Errorf("JSON missing field %q", key)
		}
	}
}

func TestRead_MissingFile(t *testing.T) {
	tempDir(t)
	_, err := project.Read()
	if err == nil {
		t.Error("Read() on missing file returned nil error, want error")
	}
}

func TestRead_MalformedJSON(t *testing.T) {
	tempDir(t)
	if err := os.WriteFile(project.Filename, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := project.Read()
	if err == nil {
		t.Error("Read() on malformed JSON returned nil error, want error")
	}
}

func TestDefaultBreakpoints(t *testing.T) {
	bps := project.DefaultBreakpoints
	if len(bps) != 4 {
		t.Fatalf("len(DefaultBreakpoints) = %d, want 4", len(bps))
	}
	expected := []project.Breakpoint{
		{Name: "mobile", Width: 375, Height: 812},
		{Name: "tablet", Width: 768, Height: 1024},
		{Name: "desktop", Width: 1280, Height: 720},
		{Name: "wide", Width: 1920, Height: 1080},
	}
	for i, bp := range expected {
		if bps[i] != bp {
			t.Errorf("DefaultBreakpoints[%d] = %+v, want %+v", i, bps[i], bp)
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
