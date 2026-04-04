# Implementation Plan: Pre-Commit Hook Integration (US-007)

## Overview

Implements `ks install-hook` — a CLI command that writes an executable git pre-commit hook script to `.git/hooks/pre-commit`. The hook auto-runs `ks snapshot && ks diff` on every commit, outputs structured diff JSON to stdout, always exits 0 (advisory), auto-starts Chrome if needed, and fails gracefully when URLs are unreachable.

**Story:** US-007
**Depends on:** US-001, US-002, US-003, US-004 (for full end-to-end testing; the command itself is a pure file-write operation)

---

## Phase 1: Core Command Implementation

### Task 1.1 — Create `cmd/install_hook.go`

**File:** `cmd/install_hook.go` (new file)

#### Sub-task 1.1.1 — Define the hook script template constant

- Define a package-level `const hookScript` string containing the POSIX sh hook script.
- The script must:
  - Start with `#!/bin/sh` shebang and header comment block (installed by `ks install-hook`, advisory note).
  - Use `command -v ks` at runtime to resolve the `ks` binary path (no hardcoded paths).
  - Check Chrome status via `"$KS" status 2>/dev/null | grep -q '"running":true'`; if not running, attempt `"$KS" start --local >/dev/null 2>&1` with a graceful fallback (`exit 0`) on failure.
  - Run `"$KS" snapshot 2>/dev/null`; on failure print an advisory message to stderr and `exit 0`.
  - Run `"$KS" diff 2>/dev/null || true` (diff JSON goes to stdout).
  - End with an unconditional `exit 0`.
  - Use `set -e` for internal operations, but guard every external `ks` call with explicit fallback.
  - Route all progress/diagnostic messages to stderr; structured JSON to stdout.

#### Sub-task 1.1.2 — Implement the `--force` flag parser

- Scan `args []string` for the `--force` flag.
- Return a bool `force` indicating whether overwrite is permitted.
- No other flags are defined for this command.

#### Sub-task 1.1.3 — Implement `RunInstallHook(args []string)`

Step-by-step logic:

1. **Parse flags** — extract `--force` from args (Sub-task 1.1.2).
2. **Project config check** — `os.Stat(".ks-project.json")`.
   - If error (file absent): call `output.Fail("install-hook", errors.New("no .ks-project.json found — run 'ks project init' first"), "")` then `os.Exit(2)`.
3. **Git repo check** — `os.Stat(".git")`.
   - If error (directory absent): call `output.Fail("install-hook", errors.New("not a git repository — no .git/ directory found"), "")` then `os.Exit(2)`.
4. **Ensure hooks directory** — `os.MkdirAll(".git/hooks", 0755)` to handle repos where the hooks dir doesn't yet exist (bare clones, etc.).
5. **Existing hook check** — `os.Stat(".git/hooks/pre-commit")`.
   - If the file exists AND `force` is false: call `output.Fail("install-hook", errors.New("pre-commit hook already exists at .git/hooks/pre-commit — use --force to overwrite"), "Run 'ks install-hook --force' to replace it, or remove .git/hooks/pre-commit manually.")` then `os.Exit(1)`.
   - If the file exists AND `force` is true: proceed (overwrite).
   - If the file does not exist: proceed.
6. **Write hook** — `os.WriteFile(".git/hooks/pre-commit", []byte(hookScript), 0755)`.
   - On error: call `output.Fail("install-hook", fmt.Errorf("failed to write pre-commit hook: %w", err), "")` then `os.Exit(2)`.
7. **Success output** — `output.Success("install-hook", map[string]any{...})` with:
   - `"path"`: `".git/hooks/pre-commit"`
   - `"message"`: `"pre-commit hook installed successfully"`
   - `"note"`: `"Hook is advisory (exits 0). Edit .git/hooks/pre-commit to customize."`

#### Sub-task 1.1.4 — Add required imports

- `"errors"` (or `fmt.Errorf` for wrapping, `errors.New` for plain messages)
- `"fmt"`
- `"os"`
- `"github.com/callmeradical/kaleidoscope/output"`

---

## Phase 2: CLI Registration

### Task 2.1 — Add `case "install-hook"` to `main.go`

**File:** `main.go`

#### Sub-task 2.1.1 — Add switch case

In the `switch command { ... }` block (after `case "install-skills":` and before `default:`), add:

```go
case "install-hook":
    cmd.RunInstallHook(cmdArgs)
```

#### Sub-task 2.1.2 — Add "Git Integration" section to the `usage` string

In the `var usage` string, add a new `Git Integration:` section (after `Skills:`):

```
Git Integration:
  install-hook [--force]  Install pre-commit hook for automatic regression checks
```

---

## Phase 3: Usage Documentation

### Task 3.1 — Add `"install-hook"` entry to `CommandUsage` in `cmd/usage.go`

**File:** `cmd/usage.go`

#### Sub-task 3.1.1 — Add usage entry to `CommandUsage` map

Add the following key/value pair to the `CommandUsage` map (after the `"install-skills"` entry):

```
"install-hook": `ks install-hook [--force]

Write a git pre-commit hook that automatically runs snapshot and diff checks.

Options:
  --force    Overwrite an existing pre-commit hook

Output:
  { "ok": true, "result": { "path": ".git/hooks/pre-commit", "message": "..." } }

Behavior:
  - Fails if no .ks-project.json exists in the current directory
  - Warns and exits if a pre-commit hook already exists (use --force to replace)
  - The installed hook: auto-starts Chrome, runs 'ks snapshot', runs 'ks diff'
  - Hook always exits 0 (advisory/non-blocking)
  - Hook outputs diff JSON to stdout for agent consumption

Examples:
  ks install-hook           # Install hook (fails if one exists)
  ks install-hook --force   # Overwrite existing hook`,
```

---

## Phase 4: Tests

### Task 4.1 — Create `cmd/install_hook_test.go`

**File:** `cmd/install_hook_test.go` (new file)

#### Sub-task 4.1.1 — Test: no `.ks-project.json` → exits with error

- Create a temp directory with no `.ks-project.json`.
- Change working directory to temp dir.
- Invoke the command logic (capture output or use a subprocess).
- Assert `ok: false` and error message matches `"no .ks-project.json"`.
- Assert exit code 2.

#### Sub-task 4.1.2 — Test: not a git repo → exits with error

- Create a temp directory with a `.ks-project.json` but no `.git/`.
- Assert `ok: false` and error message matches `"not a git repository"`.
- Assert exit code 2.

#### Sub-task 4.1.3 — Test: successful install (no existing hook)

- Create a temp directory with `.ks-project.json` and `.git/hooks/` subdirectory.
- Call `RunInstallHook([]string{})`.
- Assert the file `.git/hooks/pre-commit` is created.
- Assert file permissions are `0755` (executable).
- Assert file content starts with `#!/bin/sh`.
- Assert output JSON has `ok: true` and `result.path == ".git/hooks/pre-commit"`.

#### Sub-task 4.1.4 — Test: existing hook without `--force` → warns and does not overwrite

- Set up as above; write a sentinel string to `.git/hooks/pre-commit`.
- Call `RunInstallHook([]string{})` (no `--force`).
- Assert output JSON has `ok: false`.
- Assert error message contains `"--force"`.
- Assert the existing hook file still contains the sentinel string (not overwritten).

#### Sub-task 4.1.5 — Test: existing hook with `--force` → overwrites

- Set up as above; write a sentinel string to `.git/hooks/pre-commit`.
- Call `RunInstallHook([]string{"--force"})`.
- Assert output JSON has `ok: true`.
- Assert the hook file content is the new hook script (sentinel is gone).

#### Sub-task 4.1.6 — Test: hook script is valid POSIX sh

- After a successful install, run `sh -n .git/hooks/pre-commit`.
- Assert exit code 0 (syntax check passes).

#### Sub-task 4.1.7 — Test: hook script exits 0 unconditionally

- Inspect the hook script constant directly for `exit 0` at the end.
- Or invoke `sh .git/hooks/pre-commit` in an environment where `ks` is not in PATH and assert exit code 0.

---

## Phase 5: Verification

### Task 5.1 — Build verification

- Run `go build ./...` and confirm it compiles cleanly with no errors.

### Task 5.2 — Test suite

- Run `go test ./...` and confirm all tests pass (including the new `cmd/install_hook_test.go`).

### Task 5.3 — Manual acceptance checklist (for reviewer)

Confirm all acceptance criteria from US-007:

| Check | Command / Observation |
|-------|-----------------------|
| Hook file is written | `ls -l .git/hooks/pre-commit` shows executable file |
| Hook is executable | `stat -c %a .git/hooks/pre-commit` returns `755` |
| No `.ks-project.json` returns error | `ks install-hook` in empty dir → `ok: false`, exit 2 |
| No `.git/` returns error | `ks install-hook` in non-git dir → `ok: false`, exit 2 |
| Existing hook warns without `--force` | second `ks install-hook` → `ok: false`, file unchanged |
| `--force` overwrites existing hook | `ks install-hook --force` → `ok: true`, file replaced |
| Hook exits 0 always | `sh .git/hooks/pre-commit` (no ks in PATH) → exit code 0 |
| Hook script is valid POSIX sh | `sh -n .git/hooks/pre-commit` → exit 0 |
| Help flag works | `ks install-hook --usage` → prints usage text |

---

## File Changelist Summary

| File | Action |
|------|--------|
| `cmd/install_hook.go` | **Create** — `RunInstallHook` implementation + hook script constant |
| `cmd/install_hook_test.go` | **Create** — unit/integration tests for all acceptance criteria |
| `cmd/usage.go` | **Edit** — add `"install-hook"` entry to `CommandUsage` map |
| `main.go` | **Edit** — add `case "install-hook"` to switch; add Git Integration section to usage string |

No new packages, external dependencies, or build system changes are required.
