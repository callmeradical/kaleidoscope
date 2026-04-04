# Implementation Plan: Pre-Commit Hook Integration (US-007)

## Overview

Implement `ks install-hook` — a command that writes a git pre-commit hook script
to `.git/hooks/pre-commit`. The hook runs `ks snapshot && ks diff` on every
`git commit`, outputs structured diff JSON to stdout, and always exits 0
(advisory/non-blocking).

---

## Phase 1: Core Command Implementation

### Task 1.1: Create `cmd/install_hook.go`

**File:** `/workspace/cmd/install_hook.go`
**Package:** `cmd`

#### Sub-tasks

1. **1.1.1** — Add package declaration and imports
   - `"errors"`, `"fmt"`, `"os"`, `"path/filepath"`
   - `"github.com/callmeradical/kaleidoscope/output"`

2. **1.1.2** — Define the hook script template as a Go string constant
   - Name it `hookScript`
   - Exact content as specified in tech-spec §2 (POSIX `sh`, resolves `ks` via
     `command -v`, auto-starts Chrome, checks `.ks-project.json`, runs
     `ks snapshot`, runs `ks diff`, always exits 0)

3. **1.1.3** — Implement `RunInstallHook(args []string)` function
   Follow this exact validation sequence:

   a. **Check `.ks-project.json`** — call `os.Stat(".ks-project.json")`. If
      file does not exist, call `output.Fail` with error
      `"no .ks-project.json found in current directory"` and hint
      `"Run from the project root, or create a .ks-project.json first"`,
      then `os.Exit(2)`.

   b. **Check `.git/` directory** — call `os.Stat(".git")`. If not found or
      not a directory, call `output.Fail` with error
      `"no .git/ directory found"` and hint
      `"Run from the root of a git repository"`, then `os.Exit(2)`.

   c. **Check for existing hook** — call `os.Stat(".git/hooks/pre-commit")`.
      If the file already exists, call `output.Fail` with error
      `"pre-commit hook already exists at .git/hooks/pre-commit"` and hint
      `"Remove the existing hook manually and re-run install-hook to replace it"`,
      then `os.Exit(2)`.

   d. **Ensure `.git/hooks/` directory exists** — call
      `os.MkdirAll(".git/hooks", 0755)`. On error, call `output.Fail` with
      `err.Error()` and empty hint, then `os.Exit(2)`.

   e. **Write the hook script** — call
      `os.WriteFile(".git/hooks/pre-commit", []byte(hookScript), 0644)`.
      On error, call `output.Fail` with `err.Error()` and empty hint,
      then `os.Exit(2)`.

   f. **Set executable permission** — call
      `os.Chmod(".git/hooks/pre-commit", 0755)` explicitly after the write
      (overrides any restrictive umask). On error, call `output.Fail` with
      `err.Error()` and empty hint, then `os.Exit(2)`.

   g. **Return success** — call `output.Success("install-hook", map[string]any{...})`
      with fields: `"hookPath": ".git/hooks/pre-commit"`, `"mode": "0755"`,
      `"advisory": true`.

4. **1.1.4** — Suppress unused import warnings
   Ensure `fmt` is used (e.g., in `fmt.Errorf` wrapping) or use `errors.New`
   directly; adjust imports to match actual usage.

---

## Phase 2: Wire Command into Main Entry Point

### Task 2.1: Update `main.go` switch statement

**File:** `/workspace/main.go`

#### Sub-tasks

1. **2.1.1** — Add `case "install-hook":` to the switch block (after
   `case "install-skills":` to keep the Skills section grouped):
   ```go
   case "install-hook":
       cmd.RunInstallHook(cmdArgs)
   ```

2. **2.1.2** — Add `install-hook` to the `usage` string under the `Skills:`
   section, after the existing `install-skills` entry:
   ```
     install-hook            Install git pre-commit hook for regression checks
   ```

---

## Phase 3: Register Command in Usage Registry

### Task 3.1: Add `install-hook` entry to `CommandUsage` map

**File:** `/workspace/cmd/usage.go`

#### Sub-tasks

1. **3.1.1** — Add a new key `"install-hook"` to the `CommandUsage` map with
   the following value string:
   ```
   ks install-hook

   Installs a git pre-commit hook that automatically runs ks snapshot and ks diff
   on every commit. The hook is advisory (always exits 0) and auto-starts Chrome
   if not running.

   Options:
     (none)

   Errors:
     - Exits 2 if .ks-project.json is not found
     - Exits 2 if not run from a git repository root
     - Exits 2 if .git/hooks/pre-commit already exists
   ```

---

## Phase 4: Tests

### Task 4.1: Create `cmd/install_hook_test.go`

**File:** `/workspace/cmd/install_hook_test.go`

#### Sub-tasks

1. **4.1.1** — Add package declaration (`package cmd`) and imports:
   `"os"`, `"os/exec"`, `"path/filepath"`, `"strings"`, `"testing"`

2. **4.1.2** — Write helper `setupTempDir(t *testing.T) string` that creates a
   temporary directory with `t.TempDir()`, changes working directory to it with
   `os.Chdir`, and registers `t.Cleanup` to restore the original directory.

3. **4.1.3** — Write `TestRunInstallHook_MissingProjectConfig`:
   - Setup: temp dir without `.ks-project.json`
   - Call `RunInstallHook([]string{})` via subprocess (use `exec.Command` with
     `os.Args[0]` and `-test.run=TestRunInstallHook_MissingProjectConfig` pattern,
     or use a captured-output approach)
   - Assert: exit code 2, output JSON contains
     `"no .ks-project.json found in current directory"`

4. **4.1.4** — Write `TestRunInstallHook_NotGitRepo`:
   - Setup: temp dir with `.ks-project.json` but no `.git/` directory
   - Assert: exit code 2, output JSON contains `"no .git/ directory found"`

5. **4.1.5** — Write `TestRunInstallHook_HookAlreadyExists`:
   - Setup: temp dir with `.ks-project.json`, `.git/hooks/` dir, and existing
     `.git/hooks/pre-commit` file (any content)
   - Assert: exit code 2, output JSON contains
     `"pre-commit hook already exists at .git/hooks/pre-commit"`
   - Assert: existing hook file content is unchanged (not overwritten)

6. **4.1.6** — Write `TestRunInstallHook_SuccessCleanInstall`:
   - Setup: temp dir with `.ks-project.json` and `.git/` directory (no hooks dir)
   - Call `RunInstallHook([]string{})` (or use subprocess approach)
   - Assert: exit code 0
   - Assert: `.git/hooks/pre-commit` file exists
   - Assert: file has mode `0755` (read with `os.Stat` and check
     `info.Mode().Perm() == 0755`)
   - Assert: file content contains `#!/bin/sh` and `ks snapshot` and `ks diff`
     and `exit 0`
   - Assert: output JSON contains `"ok":true` and `"hookPath"`

7. **4.1.7** — Write `TestRunInstallHook_HooksDirCreatedIfAbsent`:
   - Setup: temp dir with `.ks-project.json` and `.git/` dir but no `.git/hooks/`
   - Assert: after `RunInstallHook`, `.git/hooks/` was created automatically

---

## Phase 5: Verification

### Task 5.1: Run test suite

#### Sub-tasks

1. **5.1.1** — Run `go build ./...` to verify the package compiles without
   errors.

2. **5.1.2** — Run `go test ./...` to confirm all existing tests still pass and
   the new tests in `cmd/install_hook_test.go` pass.

3. **5.1.3** — Manual smoke test (if a local git repo environment is available):
   - Create a temp git repo with `git init` and a `.ks-project.json`
   - Run `ks install-hook` and verify JSON success output
   - Confirm `.git/hooks/pre-commit` exists with mode `0755`
   - Run `git commit --allow-empty -m "test"` and confirm hook executes without
     blocking the commit

---

## Dependency Notes

- `ks snapshot` and `ks diff` commands are invoked by the **generated shell script**
  at commit time, not by Go code — no Go-level dependency on those command
  implementations is introduced.
- The `install-hook` command itself only checks that `.ks-project.json` **exists**;
  it does not parse the file, so it has no runtime dependency on the project config
  schema from US-001.

---

## File Change Summary

| File | Action |
|---|---|
| `/workspace/cmd/install_hook.go` | **Create** — new command implementation |
| `/workspace/cmd/install_hook_test.go` | **Create** — unit tests |
| `/workspace/main.go` | **Edit** — add `case "install-hook"` + usage string entry |
| `/workspace/cmd/usage.go` | **Edit** — add `"install-hook"` to `CommandUsage` map |
