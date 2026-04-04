# Implementation Plan: Pre-Commit Hook Integration (US-007)

## Overview

Implement `ks install-hook` — a CLI command that writes a git pre-commit hook to `.git/hooks/pre-commit`. The hook runs `ks snapshot && ks diff`, outputs structured JSON to stdout, always exits 0 (advisory), auto-starts Chrome if needed, and fails gracefully on unreachable URLs.

**Files to create/modify:**
| File | Action |
|------|--------|
| `cmd/install_hook.go` | Create — `RunInstallHook` implementation |
| `cmd/util.go` | Edit — add `findGitRoot()` helper |
| `main.go` | Edit — add `install-hook` case + usage entry |

---

## Phase 1: Utility Helper

### Task 1.1 — Add `findGitRoot()` to `cmd/util.go`

**Sub-tasks:**
- 1.1.1 Open `cmd/util.go` and append the `findGitRoot()` function.
- 1.1.2 Add required imports (`os`, `fmt`, `path/filepath`) scoped to the function (verify the package already uses them, or add to an import block).
- 1.1.3 Implement the upward-directory-walk loop:
  - Start at `os.Getwd()`.
  - Each iteration: `os.Stat(filepath.Join(dir, ".git"))` — if found, return `dir, nil`.
  - Detect filesystem root: if `filepath.Dir(dir) == dir`, return `"", fmt.Errorf("not inside a git repository")`.
  - Advance: `dir = filepath.Dir(dir)`.
- 1.1.4 Verify the function signature: `func findGitRoot() (string, error)`.

---

## Phase 2: Core Command Implementation

### Task 2.1 — Create `cmd/install_hook.go`

**Sub-tasks:**

#### 2.1.1 — File scaffolding
- Create `cmd/install_hook.go` with `package cmd`.
- Add imports: `fmt`, `os`, `path/filepath`, `github.com/callmeradical/kaleidoscope/output`.

#### 2.1.2 — Embed hook script constant
- Define a package-level `const hookScript = \`...\`` containing the complete shell script verbatim (no `embed.FS`):
  ```sh
  #!/bin/sh
  # Kaleidoscope pre-commit hook
  # Installed by: ks install-hook
  # Advisory only — always exits 0

  KS=$(command -v ks 2>/dev/null)
  if [ -z "$KS" ]; then
    echo '{"ok":false,"command":"pre-commit","error":"ks not found in PATH","hint":"install kaleidoscope and ensure it is in PATH"}' >&2
    exit 0
  fi

  # Auto-start Chrome if not running
  if ! "$KS" status >/dev/null 2>&1; then
    "$KS" start --local >/dev/null 2>&1 || true
  fi

  # Run snapshot; on failure, emit warning and skip diff
  SNAP=$("$KS" snapshot 2>&1)
  SNAP_OK=$(echo "$SNAP" | grep -c '"ok":true' || true)

  if [ "$SNAP_OK" -eq 0 ]; then
    echo "$SNAP"
    echo '{"ok":false,"command":"pre-commit","error":"snapshot failed — skipping diff","hint":"check that project URLs are reachable"}' >&2
    exit 0
  fi

  # Run diff and emit to stdout for agent consumption
  "$KS" diff

  exit 0
  ```

#### 2.1.3 — Implement `RunInstallHook(args []string)`

Step-by-step logic inside the function:

**Step A — Parse flags**
- Check `hasFlag(args, "--force")` → store as `force bool`.

**Step B — Validate `.ks-project.json` exists in CWD**
- Call `os.Getwd()` to get CWD.
- Call `os.Stat(filepath.Join(cwd, ".ks-project.json"))`.
- If error (file missing): call `output.Fail("install-hook", "no .ks-project.json found in current directory", "run 'ks project init' to create a project config")` and `os.Exit(2)`.

**Step C — Locate git root**
- Call `findGitRoot()`.
- If error: call `output.Fail("install-hook", err.Error(), "")` and `os.Exit(2)`.

**Step D — Compute hook path**
- `hookPath := filepath.Join(gitRoot, ".git", "hooks", "pre-commit")`

**Step E — Check for existing hook**
- Call `os.Stat(hookPath)`.
- If file exists AND `force` is false:
  - Call `output.Fail("install-hook", fmt.Sprintf("pre-commit hook already exists at %s", hookPath), "re-run with --force to overwrite")` and `os.Exit(2)`.
- If file exists AND `force` is true:
  - Set `overwritten = true` (used in success result).

**Step F — Ensure hooks directory exists**
- `os.MkdirAll(filepath.Join(gitRoot, ".git", "hooks"), 0755)` — tolerates already-existing dir.

**Step G — Write hook script**
- `os.WriteFile(hookPath, []byte(hookScript), 0755)`.
- If error: call `output.Fail("install-hook", fmt.Sprintf("failed to write hook: %v", err), "")` and `os.Exit(2)`.

**Step H — Set executable bit explicitly**
- `os.Chmod(hookPath, 0755)`.
- If error: call `output.Fail("install-hook", fmt.Sprintf("failed to set executable bit: %v", err), "")` and `os.Exit(2)`.

**Step I — Output success**
- Call `output.Success("install-hook", map[string]interface{}{"path": hookPath, "overwritten": overwritten})`.

---

## Phase 3: CLI Registration

### Task 3.1 — Update `main.go` switch statement

**Sub-tasks:**
- 3.1.1 Add a new case to the `switch command` block (after the `install-skills` case, before `default`):
  ```go
  case "install-hook":
      cmd.RunInstallHook(cmdArgs)
  ```

### Task 3.2 — Update usage string in `main.go`

**Sub-tasks:**
- 3.2.1 Locate the `Skills:` section in the `usage` variable.
- 3.2.2 Add a new `Workflow:` section directly after `Skills:` (or append before `Options:`):
  ```
  Workflow:
    install-hook [--force]  Install git pre-commit hook for regression checks
  ```

---

## Phase 4: Output Contract Verification

### Task 4.1 — Verify `output` package interface

**Sub-tasks:**
- 4.1.1 Read `output/format.go` to confirm the signatures of `output.Success` and `output.Fail`.
- 4.1.2 Confirm that `output.Fail` accepts `(command, error, hint string)` parameters (matching the JSON shape `{"ok":false,"command":"...","error":"...","hint":"..."}`).
- 4.1.3 Confirm that `output.Success` accepts `(command string, result interface{})` and marshals to `{"ok":true,"command":"...","result":{...}}`.
- 4.1.4 Adjust call sites in `cmd/install_hook.go` if actual signatures differ from the spec assumption.

---

## Phase 5: Quality Gate

### Task 5.1 — Run tests

**Sub-tasks:**
- 5.1.1 Run `go build ./...` to ensure the project compiles with no errors.
- 5.1.2 Run `go test ./...` to execute all tests and confirm no regressions.
- 5.1.3 Manually verify the command routing by checking that `main.go` switch now includes `"install-hook"`.

---

## Acceptance Criteria Checklist

| Criterion | Covered By |
|-----------|-----------|
| `ks install-hook` writes executable script to `.git/hooks/pre-commit` | Phase 2, Steps F–H |
| Hook runs `ks snapshot` then `ks diff` | Phase 2, Task 2.1.2 (hook script) |
| Hook outputs structured diff JSON to stdout | Phase 2, Task 2.1.2 (`ks diff` stdout) |
| Hook always exits 0 (advisory) | Phase 2, Task 2.1.2 (`exit 0` everywhere) |
| Hook auto-starts Chrome via `ks start` | Phase 2, Task 2.1.2 (status check + start) |
| Fails gracefully if URLs unreachable | Phase 2, Task 2.1.2 (snapshot failure branch) |
| Warns if hook exists, no silent overwrite | Phase 2, Step E |
| Error if no `.ks-project.json` | Phase 2, Step B |
| `--force` flag overwrites existing hook | Phase 2, Steps A + E |
| `ks install-hook` visible in `ks --help` | Phase 3, Task 3.2 |
| `go test ./...` passes | Phase 5 |
