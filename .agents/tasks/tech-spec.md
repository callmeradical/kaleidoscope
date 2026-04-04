# Tech Spec: Pre-Commit Hook Integration (US-007)

## Overview

Implements `ks install-hook`, a new CLI command that writes a git pre-commit hook script to `.git/hooks/pre-commit`. The hook auto-runs `ks snapshot && ks diff` on every commit, outputs structured diff JSON to stdout, always exits 0 (advisory), auto-starts Chrome if needed, and fails gracefully when URLs are unreachable.

**Story:** US-007
**Depends on:** US-001 (project config / `.ks-project.json`), US-002 (snapshot command), US-003 (diff command), US-004 (baseline manager)

---

## Architecture Overview

```
main.go
  └─ case "install-hook" → cmd.RunInstallHook(args)

cmd/install_hook.go          # New file: install-hook command handler
  ├─ validates .ks-project.json exists in CWD
  ├─ checks for existing .git/hooks/pre-commit
  ├─ writes hook script (shell template, embedded or inline)
  └─ sets executable permissions (0755)

.git/hooks/pre-commit        # Written artifact (not in source)
  ├─ auto-starts Chrome via `ks start` if not running
  ├─ runs `ks snapshot`
  ├─ runs `ks diff`
  ├─ prints diff JSON to stdout
  └─ always exits 0
```

The command is a pure file-write operation — no Chrome dependency, no network calls.

---

## Detailed Component Design

### 1. `cmd/install_hook.go`

**Function:** `RunInstallHook(args []string)`

**Logic flow:**

1. **Project config check** — stat `.ks-project.json` in CWD. If absent, call `output.Fail` with a clear message and `os.Exit(2)`.
2. **Git repo check** — stat `.git/` in CWD. If absent, fail with "not a git repository".
3. **Existing hook check** — stat `.git/hooks/pre-commit`.
   - If it exists: print a warning via `output.Fail`-style result (but `ok: false`) and exit without overwriting. Do NOT silently overwrite.
   - The user must manually remove the existing hook or pass `--force` (see Flags).
4. **Hook script generation** — construct the shell script string (see Hook Script section below).
5. **Write hook** — `os.WriteFile(".git/hooks/pre-commit", script, 0755)`.
6. **Success output** — `output.Success("install-hook", ...)` with hook path and summary.

**Flags:**

| Flag | Behavior |
|------|----------|
| `--force` | Overwrite an existing pre-commit hook without warning |

**Error cases and messages:**

| Condition | Error message |
|-----------|---------------|
| No `.ks-project.json` | `"no .ks-project.json found — run 'ks project init' first"` |
| Not a git repo | `"not a git repository — no .git/ directory found"` |
| Hook already exists (no `--force`) | `"pre-commit hook already exists at .git/hooks/pre-commit — use --force to overwrite"` |
| Write failure | `"failed to write pre-commit hook: <os error>"` |

---

### 2. Hook Script Template

The hook script is an inline Go string constant (no embed needed — it is short and static):

```bash
#!/bin/sh
# kaleidoscope pre-commit hook
# Installed by: ks install-hook
# Advisory only — always exits 0. Edit this file to customize behavior.

set -e

KS=$(command -v ks 2>/dev/null)
if [ -z "$KS" ]; then
  echo "ks: kaleidoscope not found in PATH — skipping visual regression check" >&2
  exit 0
fi

# Auto-start Chrome if not running
if ! "$KS" status 2>/dev/null | grep -q '"running":true'; then
  echo "ks: starting Chrome..." >&2
  "$KS" start --local >/dev/null 2>&1 || {
    echo "ks: failed to start Chrome — skipping visual regression check" >&2
    exit 0
  }
fi

# Take snapshot
echo "ks: taking snapshot..." >&2
if ! "$KS" snapshot 2>/dev/null; then
  echo "ks: snapshot failed (URLs may be unreachable) — skipping diff" >&2
  exit 0
fi

# Run diff and emit JSON to stdout
echo "ks: running diff..." >&2
"$KS" diff 2>/dev/null || true

# Advisory: always exit 0
exit 0
```

**Key design decisions in the script:**
- `set -e` applies to internal operations, but every external `ks` call is guarded with `|| true` or explicit fallback so the hook never blocks a commit.
- Chrome start uses `--local` to keep state in `.kaleidoscope/` (project-scoped).
- Status check uses JSON grep to avoid parsing; if `ks status` fails entirely, Chrome start is attempted anyway.
- Diff JSON goes to stdout (for agent consumption); progress messages go to stderr.
- `ks` binary path is resolved at runtime via `command -v` so the hook works regardless of install location.

---

### 3. `main.go` Changes

Add a new case to the switch statement:

```go
case "install-hook":
    cmd.RunInstallHook(cmdArgs)
```

Add usage string to `cmd/usage.go` under `CommandUsage`:

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
  ks install-hook --force   # Overwrite existing hook`
```

Add to the `main.go` usage string (Skills section or new Git Integration section):

```
Git Integration:
  install-hook [--force]  Install pre-commit hook for automatic regression checks
```

---

## API Definitions

This feature is CLI-only. No HTTP API or inter-process protocol.

### `ks install-hook` Success Output

```json
{
  "ok": true,
  "command": "install-hook",
  "result": {
    "path": ".git/hooks/pre-commit",
    "message": "pre-commit hook installed successfully",
    "note": "Hook is advisory (exits 0). Edit .git/hooks/pre-commit to customize."
  }
}
```

### `ks install-hook` Failure Output (existing hook, no --force)

```json
{
  "ok": false,
  "command": "install-hook",
  "error": "pre-commit hook already exists at .git/hooks/pre-commit — use --force to overwrite",
  "hint": "Run 'ks install-hook --force' to replace it, or remove .git/hooks/pre-commit manually."
}
```

### Hook stdout (on commit, diff JSON)

The hook emits whatever `ks diff` outputs — a standard `output.Result` JSON payload:

```json
{
  "ok": true,
  "command": "diff",
  "result": {
    "baseline": "abc123",
    "current": "def456",
    "regressions": [...],
    "resolved": [...],
    "summary": { "newIssues": 2, "resolvedIssues": 1 }
  }
}
```

---

## Data Model Changes

No new persistent data structures are introduced by this command.

**File written at runtime (not tracked in source):**

| Path | Type | Owner |
|------|------|-------|
| `.git/hooks/pre-commit` | Shell script, mode 0755 | Written by `ks install-hook`, gitignored by git convention |

**Files read at runtime (must exist, owned by other stories):**

| Path | Purpose |
|------|---------|
| `.ks-project.json` | Presence check — confirms project is initialized |
| `.git/` | Presence check — confirms CWD is a git repository |

---

## Security Considerations

1. **No arbitrary code execution from config** — the hook script template is a hardcoded Go string constant. It does not interpolate any user-provided values from `.ks-project.json` or command args into the script body, preventing script injection.

2. **No silent overwrite** — existing hooks are preserved by default. The `--force` flag must be explicit. This prevents accidentally destroying custom hooks (e.g., linters, formatters) that a project already relies on.

3. **Hook exits 0 unconditionally** — the hook is advisory and never blocks commits. This is a deliberate design choice: regression detection is informational for the agent, not a gate. The agent decides whether to act on the diff output.

4. **`ks` binary path resolved at runtime** — `command -v ks` is used rather than hardcoding a path. This avoids path-traversal issues and ensures the hook works across different install locations.

5. **Stderr vs stdout separation** — progress/diagnostic messages go to stderr; structured JSON goes to stdout. This allows agents consuming stdout to parse clean JSON without filtering noise.

6. **File permissions** — hook is written with mode `0755` (executable by owner, readable/executable by group and others), matching standard git hook conventions.

7. **No secrets in hook** — the script contains no credentials, tokens, or environment-specific values. It is safe to commit if accidentally included (though `.git/` is always gitignored).

---

## Dependencies on Other Stories

| Story | What US-007 needs |
|-------|-------------------|
| US-001 (Project Config) | `.ks-project.json` must exist for `install-hook` to proceed; `RunInstallHook` reads it only for presence check |
| US-002 (Snapshot Command) | Hook invokes `ks snapshot`; `ks snapshot` must be a registered command in `main.go` |
| US-003 (Diff Command) | Hook invokes `ks diff`; `ks diff` must be a registered command in `main.go` |
| US-004 (Baseline Manager) | `ks diff` compares against a baseline managed by this story |

US-007 implementation can proceed independently (the hook script is static text), but end-to-end acceptance testing requires US-001 through US-004 to be implemented.

---

## File Changelist

| File | Change |
|------|--------|
| `cmd/install_hook.go` | New file — `RunInstallHook` implementation |
| `cmd/usage.go` | Add `"install-hook"` entry to `CommandUsage` map |
| `main.go` | Add `case "install-hook"` to switch; add to `usage` string |

No new packages or external dependencies are required.

---

## Quality Gates

- `go test ./...` must pass
- `go build ./...` must compile cleanly
- `ks install-hook` in a directory without `.ks-project.json` must output `ok: false` and exit 2
- `ks install-hook` in a non-git directory must output `ok: false` and exit 2
- `ks install-hook` must write an executable file at `.git/hooks/pre-commit`
- `ks install-hook` (second run, no `--force`) must warn and not overwrite
- `ks install-hook --force` must overwrite an existing hook
- Written hook must be valid POSIX sh (passes `sh -n`)
