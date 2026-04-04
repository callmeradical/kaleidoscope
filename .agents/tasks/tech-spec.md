# Tech Spec: Pre-Commit Hook Integration (US-007)

## Overview

Implement `ks install-hook` — a command that writes a git pre-commit hook script
to `.git/hooks/pre-commit`. The hook automatically runs `ks snapshot && ks diff`
on every `git commit`, outputs structured diff JSON to stdout, and always exits 0
(advisory/non-blocking). Chrome is auto-started if not already running. The hook
fails gracefully when project URLs are unreachable.

---

## Architecture Overview

```
ks install-hook
     │
     ├── Validates: .ks-project.json exists in CWD
     ├── Validates: .git/ exists (is a git repo)
     ├── Warns:     .git/hooks/pre-commit already exists (prompt, no overwrite)
     └── Writes:    .git/hooks/pre-commit (executable shell script)

.git/hooks/pre-commit (shell script, generated)
     │
     ├── Checks Chrome status (ks status)
     ├── Auto-starts Chrome if not running (ks start --local)
     ├── Checks URL reachability (ks open <url> from .ks-project.json)
     ├── Runs: ks snapshot
     ├── Runs: ks diff
     ├── Outputs diff JSON to stdout
     └── Always: exit 0
```

**Dependencies on other user stories:**
- `ks snapshot` (US-001/US-002): Must exist. Takes a snapshot of all URLs in
  `.ks-project.json` and stores it in `.kaleidoscope/snapshots/`.
- `ks diff` (US-003/US-004/US-005): Must exist. Compares latest snapshot to
  baseline; outputs structured JSON diff to stdout.
- `.ks-project.json` (US-001): Project config file with URLs list.

This spec covers only the `install-hook` command and the hook script template.
It does not specify snapshot or diff internals.

---

## Component Design

### 1. `cmd/install_hook.go` — New Command

**File:** `/workspace/cmd/install_hook.go`
**Package:** `cmd`
**Entry point:** `RunInstallHook(args []string)`

#### Responsibilities

1. Validate `.ks-project.json` exists in CWD — return `output.Fail` if absent.
2. Validate `.git/` directory exists in CWD — return `output.Fail` if not a git repo.
3. Check if `.git/hooks/pre-commit` already exists:
   - If yes: print a warning via `output.Fail` and exit without overwriting.
   - The user must manually remove the existing hook before re-running.
4. Ensure `.git/hooks/` directory exists (create if missing).
5. Write the pre-commit hook script (see §2) to `.git/hooks/pre-commit`.
6. Set the hook file mode to `0755` (executable).
7. Return `output.Success` with install metadata.

#### Function Signature

```go
func RunInstallHook(args []string) {
    // 1. Check .ks-project.json
    // 2. Check .git/
    // 3. Check existing hook
    // 4. Write hook script
    // 5. output.Success or output.Fail
}
```

#### Success Output

```json
{
  "ok": true,
  "command": "install-hook",
  "result": {
    "hookPath": ".git/hooks/pre-commit",
    "mode": "0755",
    "advisory": true
  }
}
```

#### Failure Cases

| Condition | Error message | Hint |
|---|---|---|
| `.ks-project.json` not found | `"no .ks-project.json found in current directory"` | `"Run from the project root, or create a .ks-project.json first"` |
| Not a git repository | `"no .git/ directory found"` | `"Run from the root of a git repository"` |
| Hook already exists | `"pre-commit hook already exists at .git/hooks/pre-commit"` | `"Remove the existing hook manually and re-run install-hook to replace it"` |
| File write error | `err.Error()` | `""` |

---

### 2. Hook Script Template

The generated shell script at `.git/hooks/pre-commit`:

```sh
#!/bin/sh
# Kaleidoscope pre-commit hook — advisory visual regression check
# Installed by: ks install-hook
# This hook always exits 0 (non-blocking). Regression data is advisory only.

KS=$(command -v ks 2>/dev/null)
if [ -z "$KS" ]; then
  echo '{"ok":false,"command":"pre-commit","error":"ks not found in PATH","hint":"Install kaleidoscope: https://github.com/callmeradical/kaleidoscope"}' >&2
  exit 0
fi

# Auto-start Chrome if not running
STATUS=$("$KS" status 2>/dev/null)
if ! echo "$STATUS" | grep -q '"running":true'; then
  "$KS" start --local > /dev/null 2>&1
  sleep 1
fi

# Check project config exists
if [ ! -f ".ks-project.json" ]; then
  echo '{"ok":false,"command":"pre-commit","error":"no .ks-project.json found","hint":"Run ks install-hook from the project root"}' >&2
  exit 0
fi

# Run snapshot — capture current state
SNAPSHOT=$("$KS" snapshot 2>&1)
SNAP_OK=$(echo "$SNAPSHOT" | grep -c '"ok":true' || true)

if [ "$SNAP_OK" -eq 0 ]; then
  echo '{"ok":false,"command":"pre-commit","error":"snapshot failed","detail":'"$(echo "$SNAPSHOT" | tail -1)"'}' >&2
  exit 0
fi

# Run diff — compare snapshot to baseline
DIFF=$("$KS" diff 2>&1)
echo "$DIFF"

exit 0
```

**Design decisions:**
- The script is POSIX `sh` (not bash) for maximum portability.
- `ks` binary is resolved via `command -v` at runtime so it works with any PATH.
- All failure paths produce a JSON line to stderr to maintain structured output convention.
- Chrome auto-start uses `--local` flag so state is stored in project-local `.kaleidoscope/`.
- `sleep 1` after start gives Chrome time to initialize before the snapshot.
- The diff JSON from `ks diff` is forwarded to stdout verbatim for agent consumption.
- Exits 0 in all cases (per acceptance criteria: advisory/non-blocking).

---

### 3. Wiring in `main.go`

Add one case to the switch statement in `/workspace/main.go`:

```go
case "install-hook":
    cmd.RunInstallHook(cmdArgs)
```

Add one line to the usage string under "Skills:":

```
  install-hook            Install git pre-commit hook for regression checks
```

---

## Data Model Changes

No new data structures are introduced by this story.

The command reads `.ks-project.json` only to verify it exists; it does not parse
its contents. The hook script reads `.ks-project.json` indirectly by invoking
`ks snapshot` which will handle its own validation.

---

## API Definitions

No HTTP APIs. This is a CLI-only feature.

### Command: `ks install-hook`

```
Usage: ks install-hook

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

## File System Layout

```
<project-root>/
├── .ks-project.json          # Required, read by ks snapshot/diff
├── .git/
│   └── hooks/
│       └── pre-commit        # Written by ks install-hook (mode 0755)
└── .kaleidoscope/            # gitignored, runtime state
    ├── state.json
    └── snapshots/
```

---

## Security Considerations

1. **Shell injection risk in generated script:** The hook script template is a
   static string constant in Go source code. It contains no dynamic user input
   interpolated into shell code, eliminating injection vectors.

2. **File mode 0755:** The hook must be executable. The Go `os.WriteFile` call
   uses mode `0755`. On systems with restrictive umask, `os.Chmod` should be
   called explicitly after write to guarantee executability regardless of umask.

3. **No silent overwrite:** The command refuses to overwrite an existing hook.
   This prevents unintentional destruction of custom user hooks without consent.
   Users must manually inspect and remove the existing hook — an intentional
   friction point.

4. **Hook exits 0 always:** The hook never blocks a commit. This is intentional
   per the acceptance criteria. The agent (not the hook) decides what to do with
   regression output. Blocking commits would be an unauthorized side effect.

5. **`ks` binary resolution at runtime:** Using `command -v ks` in the hook
   means the hook works correctly even if `ks` is installed after the hook is
   written. It also avoids hardcoding an absolute path that may differ between
   developer machines.

6. **Chrome auto-start scope:** `--local` flag scopes Chrome state to the
   project directory, avoiding interference with other projects or global state.

---

## Implementation Checklist

- [ ] Create `/workspace/cmd/install_hook.go` with `RunInstallHook`
- [ ] Add `case "install-hook": cmd.RunInstallHook(cmdArgs)` to `main.go` switch
- [ ] Add `install-hook` to usage string in `main.go`
- [ ] Verify `os.Chmod(".git/hooks/pre-commit", 0755)` is called after write
- [ ] Write unit tests (if test infrastructure allows) covering:
  - Missing `.ks-project.json` → fail
  - Not a git repo → fail
  - Hook already exists → fail with warning
  - Clean install → success, file written, mode 0755
- [ ] Run `go test ./...` to verify no regressions
- [ ] Manual smoke test: run `ks install-hook` in a test git repo and confirm hook executes on `git commit`
