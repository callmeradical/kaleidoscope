package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelperProcess is invoked as a subprocess to exercise commands that call os.Exit.
// It is not a real test on its own.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(1)
	}
	switch args[0] {
	case "init":
		RunInit(args[1:])
	case "project-add":
		RunProjectAdd(args[1:])
	case "project-remove":
		RunProjectRemove(args[1:])
	case "project-show":
		RunProjectShow(args[1:])
	default:
		os.Exit(2)
	}
}

// runProjectCmd runs a project subcommand in a subprocess with dir as working directory.
// Returns stdout output and exit code.
func runProjectCmd(t *testing.T, dir string, cmdName string, args ...string) (string, int) {
	t.Helper()
	cs := []string{"-test.run=TestHelperProcess", "--"}
	cs = append(cs, cmdName)
	cs = append(cs, args...)
	c := exec.Command(os.Args[0], cs...)
	c.Env = append(os.Environ(), "GO_TEST_HELPER_PROCESS=1")
	c.Dir = dir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return out.String(), exitCode
}

// parseResult parses the JSON output from a command into a map.
func parseResult(t *testing.T, output string) map[string]any {
	t.Helper()
	// Find the first JSON line
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			var m map[string]any
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				t.Fatalf("failed to parse output as JSON: %v\noutput: %s", err, output)
			}
			return m
		}
	}
	t.Fatalf("no JSON found in output: %s", output)
	return nil
}

// --- Data model tests ---

func TestDefaultBreakpoints(t *testing.T) {
	if len(defaultProjectBreakpoints) != 4 {
		t.Fatalf("expected 4 default breakpoints, got %d", len(defaultProjectBreakpoints))
	}
	names := []string{"mobile", "tablet", "desktop", "wide"}
	widths := []int{375, 768, 1280, 1920}
	heights := []int{812, 1024, 720, 1080}
	for i, bp := range defaultProjectBreakpoints {
		if bp.Name != names[i] {
			t.Errorf("breakpoint[%d].Name = %q, want %q", i, bp.Name, names[i])
		}
		if bp.Width != widths[i] {
			t.Errorf("breakpoint[%d].Width = %d, want %d", i, bp.Width, widths[i])
		}
		if bp.Height != heights[i] {
			t.Errorf("breakpoint[%d].Height = %d, want %d", i, bp.Height, heights[i])
		}
	}
}

func TestProjectConfigJSONRoundtrip(t *testing.T) {
	cfg := ProjectConfig{
		Name:    "myproject",
		BaseURL: "http://localhost:3000",
		Paths:   []string{"/", "/dashboard"},
		Breakpoints: []ProjectBreakpoint{
			{Name: "mobile", Width: 375, Height: 812},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var got ProjectConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.Name != cfg.Name {
		t.Errorf("Name = %q, want %q", got.Name, cfg.Name)
	}
	if got.BaseURL != cfg.BaseURL {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, cfg.BaseURL)
	}
	if len(got.Paths) != len(cfg.Paths) {
		t.Errorf("Paths length = %d, want %d", len(got.Paths), len(cfg.Paths))
	}
}

func TestProjectConfigJSONFieldNames(t *testing.T) {
	cfg := ProjectConfig{
		Name:    "test",
		BaseURL: "http://example.com",
		Paths:   []string{"/"},
		Breakpoints: []ProjectBreakpoint{
			{Name: "mobile", Width: 375, Height: 812},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	for _, key := range []string{"name", "baseUrl", "paths", "breakpoints"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON field %q not found in marshaled ProjectConfig", key)
		}
	}
}

// --- Helper function tests ---

func TestSaveAndLoadProjectConfig(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	cfg := &ProjectConfig{
		Name:        "testproject",
		BaseURL:     "http://localhost:8080",
		Paths:       []string{"/", "/about"},
		Breakpoints: defaultProjectBreakpoints,
	}
	if err := saveProjectConfig(cfg); err != nil {
		t.Fatalf("saveProjectConfig error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(projectConfigFile); os.IsNotExist(err) {
		t.Fatalf("expected %s to exist after saveProjectConfig", projectConfigFile)
	}

	loaded, err := loadProjectConfig()
	if err != nil {
		t.Fatalf("loadProjectConfig error: %v", err)
	}
	if loaded.Name != cfg.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, cfg.Name)
	}
	if loaded.BaseURL != cfg.BaseURL {
		t.Errorf("BaseURL = %q, want %q", loaded.BaseURL, cfg.BaseURL)
	}
	if len(loaded.Paths) != len(cfg.Paths) {
		t.Errorf("Paths length = %d, want %d", len(loaded.Paths), len(cfg.Paths))
	}
	if len(loaded.Breakpoints) != len(cfg.Breakpoints) {
		t.Errorf("Breakpoints length = %d, want %d", len(loaded.Breakpoints), len(cfg.Breakpoints))
	}
}

func TestLoadProjectConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	_, err = loadProjectConfig()
	if err == nil {
		t.Fatal("expected error when config file does not exist, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got: %v", err)
	}
}

// --- Command tests via subprocess ---

func TestRunInit_CreatesConfig(t *testing.T) {
	dir := t.TempDir()
	out, code := runProjectCmd(t, dir, "init",
		"--name", "myapp",
		"--base-url", "http://localhost:3000",
		"--paths", "/,/dashboard",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
	}

	// Config file must exist
	configPath := filepath.Join(dir, ".ks-project.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected .ks-project.json to exist: %v", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON in config: %v", err)
	}
	if cfg.Name != "myapp" {
		t.Errorf("Name = %q, want %q", cfg.Name, "myapp")
	}
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "http://localhost:3000")
	}
	if len(cfg.Paths) != 2 || cfg.Paths[0] != "/" || cfg.Paths[1] != "/dashboard" {
		t.Errorf("Paths = %v, want [/ /dashboard]", cfg.Paths)
	}
	if len(cfg.Breakpoints) != 4 {
		t.Errorf("Breakpoints length = %d, want 4 (default presets)", len(cfg.Breakpoints))
	}

	// Output must be valid JSON with ok=true
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); !ok {
		t.Errorf("expected ok=true in output, got: %s", out)
	}
	if cmd, _ := result["command"].(string); cmd != "init" {
		t.Errorf("expected command=init in output, got: %s", out)
	}
}

func TestRunInit_DefaultBreakpoints(t *testing.T) {
	dir := t.TempDir()
	_, code := runProjectCmd(t, dir, "init",
		"--name", "myapp",
		"--base-url", "http://localhost:3000",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	configPath := filepath.Join(dir, ".ks-project.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected .ks-project.json to exist: %v", err)
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(cfg.Breakpoints) != 4 {
		t.Errorf("expected 4 default breakpoints, got %d", len(cfg.Breakpoints))
	}
}

func TestRunInit_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	// Create config first
	runProjectCmd(t, dir, "init", "--name", "first", "--base-url", "http://localhost")

	// Second init must fail
	out, code := runProjectCmd(t, dir, "init", "--name", "second", "--base-url", "http://localhost")
	if code == 0 {
		t.Fatalf("expected non-zero exit when config already exists, got 0\noutput: %s", out)
	}
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("expected ok=false in output, got: %s", out)
	}
}

func TestRunInit_MissingName(t *testing.T) {
	dir := t.TempDir()
	out, code := runProjectCmd(t, dir, "init", "--base-url", "http://localhost")
	if code == 0 {
		t.Fatalf("expected non-zero exit when --name is missing, got 0\noutput: %s", out)
	}
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("expected ok=false in output")
	}
}

func TestRunInit_MissingBaseURL(t *testing.T) {
	dir := t.TempDir()
	out, code := runProjectCmd(t, dir, "init", "--name", "myapp")
	if code == 0 {
		t.Fatalf("expected non-zero exit when --base-url is missing, got 0\noutput: %s", out)
	}
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("expected ok=false in output")
	}
}

func TestRunProjectAdd_AppendPath(t *testing.T) {
	dir := t.TempDir()
	runProjectCmd(t, dir, "init", "--name", "app", "--base-url", "http://localhost", "--paths", "/")

	out, code := runProjectCmd(t, dir, "project-add", "/settings")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
	}

	// Config must contain /settings
	configPath := filepath.Join(dir, ".ks-project.json")
	data, _ := os.ReadFile(configPath)
	var cfg ProjectConfig
	json.Unmarshal(data, &cfg)

	found := false
	for _, p := range cfg.Paths {
		if p == "/settings" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected /settings in paths, got %v", cfg.Paths)
	}

	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); !ok {
		t.Errorf("expected ok=true, got: %s", out)
	}
}

func TestRunProjectAdd_DuplicatePath(t *testing.T) {
	dir := t.TempDir()
	runProjectCmd(t, dir, "init", "--name", "app", "--base-url", "http://localhost", "--paths", "/,/settings")

	out, code := runProjectCmd(t, dir, "project-add", "/settings")
	if code == 0 {
		t.Fatalf("expected non-zero exit for duplicate path, got 0\noutput: %s", out)
	}
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("expected ok=false for duplicate path")
	}
}

func TestRunProjectAdd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	out, code := runProjectCmd(t, dir, "project-add", "/settings")
	if code == 0 {
		t.Fatalf("expected non-zero exit when project not initialized, got 0\noutput: %s", out)
	}
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("expected ok=false when project not initialized")
	}
}

func TestRunProjectRemove_RemovesPath(t *testing.T) {
	dir := t.TempDir()
	runProjectCmd(t, dir, "init", "--name", "app", "--base-url", "http://localhost", "--paths", "/,/settings")

	out, code := runProjectCmd(t, dir, "project-remove", "/settings")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
	}

	configPath := filepath.Join(dir, ".ks-project.json")
	data, _ := os.ReadFile(configPath)
	var cfg ProjectConfig
	json.Unmarshal(data, &cfg)

	for _, p := range cfg.Paths {
		if p == "/settings" {
			t.Errorf("expected /settings to be removed from paths, got %v", cfg.Paths)
		}
	}

	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); !ok {
		t.Errorf("expected ok=true, got: %s", out)
	}
}

func TestRunProjectRemove_NotFound(t *testing.T) {
	dir := t.TempDir()
	runProjectCmd(t, dir, "init", "--name", "app", "--base-url", "http://localhost", "--paths", "/")

	out, code := runProjectCmd(t, dir, "project-remove", "/nonexistent")
	if code == 0 {
		t.Fatalf("expected non-zero exit for non-existent path, got 0\noutput: %s", out)
	}
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("expected ok=false for non-existent path")
	}
}

func TestRunProjectRemove_NoConfig(t *testing.T) {
	dir := t.TempDir()
	out, code := runProjectCmd(t, dir, "project-remove", "/settings")
	if code == 0 {
		t.Fatalf("expected non-zero exit when project not initialized, got 0\noutput: %s", out)
	}
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("expected ok=false when project not initialized")
	}
}

func TestRunProjectShow_OutputsConfig(t *testing.T) {
	dir := t.TempDir()
	runProjectCmd(t, dir, "init", "--name", "myapp", "--base-url", "http://localhost:3000", "--paths", "/,/dashboard")

	out, code := runProjectCmd(t, dir, "project-show")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
	}

	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); !ok {
		t.Errorf("expected ok=true, got: %s", out)
	}
	if cmd, _ := result["command"].(string); cmd != "project-show" {
		t.Errorf("expected command=project-show, got: %s", out)
	}
	if result["result"] == nil {
		t.Errorf("expected result field in output, got: %s", out)
	}
}

func TestRunProjectShow_NoConfig(t *testing.T) {
	dir := t.TempDir()
	out, code := runProjectCmd(t, dir, "project-show")
	if code == 0 {
		t.Fatalf("expected non-zero exit when project not initialized, got 0\noutput: %s", out)
	}
	result := parseResult(t, out)
	if ok, _ := result["ok"].(bool); ok {
		t.Errorf("expected ok=false when project not initialized")
	}
}

// --- getNonFlagArgs skip-list tests ---

func TestGetNonFlagArgs_SkipsNewProjectFlags(t *testing.T) {
	args := []string{"--name", "myapp", "--base-url", "http://localhost", "--paths", "/,/dashboard", "/extra"}
	got := getNonFlagArgs(args)
	if len(got) != 1 || got[0] != "/extra" {
		t.Errorf("getNonFlagArgs with project flags = %v, want [\"/extra\"]", got)
	}
}

func TestGetNonFlagArgs_NameValueSkipped(t *testing.T) {
	// --name value should not appear as a positional arg
	args := []string{"--name", "somevalue"}
	got := getNonFlagArgs(args)
	if len(got) != 0 {
		t.Errorf("expected no positional args, got %v", got)
	}
}

func TestGetNonFlagArgs_BaseURLValueSkipped(t *testing.T) {
	args := []string{"--base-url", "http://example.com"}
	got := getNonFlagArgs(args)
	if len(got) != 0 {
		t.Errorf("expected no positional args, got %v", got)
	}
}

func TestGetNonFlagArgs_PathsValueSkipped(t *testing.T) {
	args := []string{"--paths", "/,/dashboard"}
	got := getNonFlagArgs(args)
	if len(got) != 0 {
		t.Errorf("expected no positional args, got %v", got)
	}
}
