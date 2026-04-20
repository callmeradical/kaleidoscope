# Implementation Plan: Pre-Commit Hook Integration (US-007)

## Overview

Introduce `ks install-hook` — a command that writes an advisory git pre-commit shell script to `.git/hooks/pre-commit`. The hook runs `ks snapshot && ks diff` on every commit, emits structured JSON to stdout, and always exits 0 so it never blocks commits.

**Files to create:** `cmd/installhook.go`, `cmd/installhook_test.go`
**Files to modify:** `main.go`, `cmd/usage.go`

---

## Phase 1: Core Command Implementation (`cmd/installhook.go`)

### Task 1.1 — Create `cmd/installhook.go` scaffold

- **Sub-task 1.1.1** — Create `cmd/installhook.go` with `package cmd` declaration and necessary imports (`errors`, `os`, `path/filepath`).
- **Sub-task 1.1.2** — Define the exported `RunInstallHook(args []string)` function stub.

### Task 1.2 — Implement `findGitRoot` helper

- **Sub-task 1.2.1** — Implement `findGitRoot(dir string) (string, error)` that resolves `dir` to an absolute path via `filepath.Abs`.
- **Sub-task 1.2.2** — Add the upward-walking loop: at each level check for the existence of a `.git` subdirectory via `os.Stat`.
- **Sub-task 1.2.3** — Implement filesystem-root termination guard: when `filepath.Dir(abs) == abs`, return `errors.New("not a git repository")`.

### Task 1.3 — Define the `hookScript` constant

- **Sub-task 1.3.1** — Embed the POSIX shell script as a Go raw-string constant `hookScript`.
- **Sub-task 1.3.2** — Ensure the script body matches the spec exactly:
  - Shebang `#!/bin/sh` and advisory comment header.
  - `command -v ks` check; emit JSON error to stdout and `exit 0` if missing.
  - `ks status` liveness probe; emit informational JSON and call `ks start` if Chrome is not running; emit JSON error and `exit 0` on start failure.
  - Capture `ks snapshot` output; emit it and `exit 0` on non-zero exit.
  - Capture and emit `ks diff` output.
  - Final unconditional `exit 0`.
- **Sub-task 1.3.3** — Verify all intermediate diagnostic lines are JSON-compatible objects (not plain text).

### Task 1.4 — Implement `RunInstallHook` validation logic

- **Sub-task 1.4.1** — Parse `--force` flag from `args` using `hasFlag(args, "--force")`.
- **Sub-task 1.4.2** — Validate `.ks-project.json` existence via `os.Stat`; on `os.IsNotExist`, call `output.Fail("install-hook", err, "Run \`ks init\` to create a project config first")` and `os.Exit(2)`.
- **Sub-task 1.4.3** — Call `findGitRoot(".")` to locate the git root; on error, call `output.Fail("install-hook", err, "Run \`ks install-hook\` from inside a git repository")` and `os.Exit(2)`.
- **Sub-task 1.4.4** — Construct `hookPath` as `filepath.Join(gitRoot, ".git", "hooks", "pre-commit")`.

### Task 1.5 — Implement existing-hook guard

- **Sub-task 1.5.1** — Use `os.Stat(hookPath)` to detect whether a hook already exists; capture the boolean result as `existedBefore`.
- **Sub-task 1.5.2** — If `existedBefore && !force`: call `output.Fail("install-hook", errors.New("pre-commit hook already exists: "+hookPath), "Use --force to overwrite")` and `os.Exit(2)`.

### Task 1.6 — Write hook file to disk

- **Sub-task 1.6.1** — Write `hookScript` to `hookPath` using `os.WriteFile(hookPath, []byte(hookScript), 0755)`.
- **Sub-task 1.6.2** — On write error, call `output.Fail("install-hook", err, "")` and `os.Exit(2)`.

### Task 1.7 — Emit success output

- **Sub-task 1.7.1** — Call `output.Success("install-hook", map[string]any{"hookPath": hookPath, "overwrite": force && existedBefore})`.

---

## Phase 2: Wire Command into CLI Router (`main.go`)

### Task 2.1 — Register `install-hook` case in the switch statement

- **Sub-task 2.1.1** — Locate the existing `switch cmd` block in `main.go`.
- **Sub-task 2.1.2** — Add `case "install-hook": cmd.RunInstallHook(cmdArgs)` after the `install-skills` case (or in the appropriate alphabetical/logical position).

### Task 2.2 — Update the top-level usage string

- **Sub-task 2.2.1** — Locate the `usage` string constant in `main.go`.
- **Sub-task 2.2.2** — Add a `Hooks:` section (or append to an existing relevant section) with the line: `  install-hook [--force]  Install git pre-commit hook for regression detection`.

---

## Phase 3: Document Command in `cmd/usage.go`

### Task 3.1 — Add `"install-hook"` entry to the `CommandUsage` map

- **Sub-task 3.1.1** — Add the map key `"install-hook"` with:
  - `Summary`: `"Install a git pre-commit hook that runs snapshot and diff on every commit"`.
  - `Usage`: `"ks install-hook [--force]"`.
  - `Flags`: one entry `{Flag: "--force", Description: "Overwrite existing hook without warning"}`.
  - `Examples`: `["ks install-hook", "ks install-hook --force"]`.
  - `Output`: the example success JSON string from the spec (`{"ok":true,"command":"install-hook","result":{"hookPath":".git/hooks/pre-commit","overwrite":false}}`).

---

## Phase 4: Tests (`cmd/installhook_test.go`)

### Task 4.1 — Create test file scaffold

- **Sub-task 4.1.1** — Create `cmd/installhook_test.go` with `package cmd` (or `package cmd_test`) and required imports (`os`, `path/filepath`, `strings`, `testing`).
- **Sub-task 4.1.2** — Add a `setupTestRepo` helper that creates a temp directory with a `.git/hooks/` subdirectory and optionally a `.ks-project.json` file, returning the temp dir path and a cleanup function.

### Task 4.2 — Test: no `.ks-project.json`

- **Sub-task 4.2.1** — Create a temp git repo directory without `.ks-project.json`.
- **Sub-task 4.2.2** — Call `RunInstallHook([]string{})` (capturing `os.Exit` via a subprocess or exit-capture pattern) and assert the output JSON contains `"ok":false` and error mentions missing project config.

### Task 4.3 — Test: not inside a git repository

- **Sub-task 4.3.1** — Create a temp directory with `.ks-project.json` but no `.git/` subdirectory.
- **Sub-task 4.3.2** — Assert the output JSON contains `"ok":false` and error mentions "not a git repository".

### Task 4.4 — Test: first install (no existing hook)

- **Sub-task 4.4.1** — Create a temp directory with both `.ks-project.json` and `.git/hooks/`.
- **Sub-task 4.4.2** — Call `RunInstallHook([]string{})`.
- **Sub-task 4.4.3** — Assert the hook file now exists at `.git/hooks/pre-commit`.
- **Sub-task 4.4.4** — Assert the output JSON contains `"ok":true` and `"overwrite":false`.

### Task 4.5 — Test: hook already exists, no `--force`

- **Sub-task 4.5.1** — Pre-create `.git/hooks/pre-commit` with arbitrary content.
- **Sub-task 4.5.2** — Call `RunInstallHook([]string{})` without `--force`.
- **Sub-task 4.5.3** — Assert the output JSON contains `"ok":false` and the hint `"Use --force to overwrite"`.
- **Sub-task 4.5.4** — Assert the original hook file content is unchanged.

### Task 4.6 — Test: hook already exists, with `--force`

- **Sub-task 4.6.1** — Pre-create `.git/hooks/pre-commit` with arbitrary content.
- **Sub-task 4.6.2** — Call `RunInstallHook([]string{"--force"})`.
- **Sub-task 4.6.3** — Assert the output JSON contains `"ok":true` and `"overwrite":true`.
- **Sub-task 4.6.4** — Assert the hook file now contains the kaleidoscope hook script (not the original content).

### Task 4.7 — Test: written hook has executable permissions (`0755`)

- **Sub-task 4.7.1** — After a successful install, call `os.Stat` on the hook path.
- **Sub-task 4.7.2** — Assert `info.Mode()&0111 != 0` (execute bits are set).

### Task 4.8 — Test: written hook contains expected shebang and `ks snapshot` invocation

- **Sub-task 4.8.1** — Read the written hook file content.
- **Sub-task 4.8.2** — Assert content starts with `#!/bin/sh`.
- **Sub-task 4.8.3** — Assert content contains `ks snapshot`.
- **Sub-task 4.8.4** — Assert content contains `ks diff`.
- **Sub-task 4.8.5** — Assert content contains `exit 0`.

### Task 4.9 — Test: `findGitRoot` unit tests

- **Sub-task 4.9.1** — Test that `findGitRoot` returns the repo root when called from a subdirectory of a git repo.
- **Sub-task 4.9.2** — Test that `findGitRoot` returns an error when no `.git` directory exists in the tree.

---

## Phase 5: Verification

### Task 5.1 — Run the full test suite

- **Sub-task 5.1.1** — Execute `go test ./...` and confirm all tests pass (quality gate).

### Task 5.2 — Manual smoke test (optional)

- **Sub-task 5.2.1** — Run `ks install-hook` in a repo with `.ks-project.json` and verify the hook file is created at `.git/hooks/pre-commit` with mode `0755`.
- **Sub-task 5.2.2** — Run `ks install-hook` a second time without `--force` and verify the advisory error JSON is emitted.
- **Sub-task 5.2.3** — Run `ks install-hook --force` and verify the hook is overwritten and `"overwrite":true` is in the output.

---

## File Change Summary

| File | Action | Key Content |
|---|---|---|
| `cmd/installhook.go` | **Create** | `RunInstallHook`, `findGitRoot`, `hookScript` constant |
| `cmd/installhook_test.go` | **Create** | 7+ test cases covering all acceptance criteria |
| `main.go` | **Modify** | Add `case "install-hook"` switch case; add Hooks section to usage string |
| `cmd/usage.go` | **Modify** | Add `"install-hook"` entry to `CommandUsage` map |
