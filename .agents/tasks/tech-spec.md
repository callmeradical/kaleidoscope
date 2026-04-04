# Tech Spec: Pre-Commit Hook Integration (US-007)

## Overview

This spec covers `ks install-hook` — a command that writes a git pre-commit hook script to `.git/hooks/pre-commit`. The hook runs `ks snapshot && ks diff` on every commit, outputs structured JSON to stdout, always exits 0 (advisory/non-blocking), auto-starts Chrome if needed, and fails gracefully on unreachable URLs.

This spec assumes `ks snapshot` and `ks diff` exist (from US-001 through US-006). It does not re-specify those commands.

---

## Architecture Overview

```
main.go
  └─ case "install-hook" → cmd.RunInstallHook(args)

cmd/install_hook.go          ← new file
  ├── validates .ks-project.json exists in CWD
  ├── checks for existing .git/hooks/pre-commit
  ├── writes hook script (embedded template)
  └── sets executable bit (0755)

Hook script (.git/hooks/pre-commit)
  ├── ks status → if not running, ks start
  ├── ks snapshot
  ├── ks diff → captured to stdout
  └── exit 0 always
```

No new packages are introduced. The command lives entirely in `cmd/install_hook.go`.

---

## Component Design

### `cmd/install_hook.go`

**Entry point:** `RunInstallHook(args []string)`

**Logic:**

1. **Validate project config** — check that `.ks-project.json` exists in the current working directory. If not, call `output.Fail` and `os.Exit(2)`.

2. **Locate git root** — look for `.git/` starting at CWD, walking up. If not found, call `output.Fail` and `os.Exit(2)`.

3. **Check for existing hook** — if `.git/hooks/pre-commit` exists:
   - If `--force` flag is NOT set: call `output.Fail` with a hint to re-run with `--force` to overwrite, then `os.Exit(2)`.
   - If `--force` is set: proceed (overwrite).

4. **Write hook script** — write the embedded hook script (see below) to `.git/hooks/pre-commit`.

5. **Set executable bit** — `os.Chmod(".git/hooks/pre-commit", 0755)`.

6. **Output success** — `output.Success("install-hook", ...)`.

**Flags:**
- `--force` — overwrite an existing hook without error (still logs a warning in the JSON output).

**Output on success:**
```json
{
  "ok": true,
  "command": "install-hook",
  "result": {
    "path": ".git/hooks/pre-commit",
    "overwritten": false
  }
}
```

**Output on error (hook exists, no --force):**
```json
{
  "ok": false,
  "command": "install-hook",
  "error": "pre-commit hook already exists at .git/hooks/pre-commit",
  "hint": "re-run with --force to overwrite"
}
```

**Output on error (no .ks-project.json):**
```json
{
  "ok": false,
  "command": "install-hook",
  "error": "no .ks-project.json found in current directory",
  "hint": "run 'ks project init' to create a project config"
}
```

---

### Hook Script (embedded string constant)

The hook script is a plain shell script written as a Go string constant (no `embed.FS` needed — it's small enough to inline). It is written verbatim to disk.

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

Key behaviors encoded in the script:
- `ks` not in PATH → emit JSON error to stderr, exit 0.
- Chrome not running → `ks start --local` silently; continue even if start fails.
- `ks snapshot` fails (unreachable URLs) → emit snapshot output + advisory JSON, exit 0.
- `ks diff` output goes to stdout for agent/CI consumption.

---

### `main.go` changes

Add one case to the switch statement:

```go
case "install-hook":
    cmd.RunInstallHook(cmdArgs)
```

Add to usage string under **Skills** or new **Workflow** section:

```
Workflow:
  install-hook [--force]  Install git pre-commit hook for regression checks
```

---

### `cmd/util.go` — helper (if not already present)

Add `findGitRoot() (string, error)` to walk from CWD upward until `.git/` is found or filesystem root is reached.

```go
func findGitRoot() (string, error) {
    dir, err := os.Getwd()
    if err != nil {
        return "", err
    }
    for {
        if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
            return dir, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            return "", fmt.Errorf("not inside a git repository")
        }
        dir = parent
    }
}
```

---

## Data Model Changes

No new persistent data structures. The hook is a plain shell script on disk.

**Files written:**
| Path | Committed? | Description |
|------|-----------|-------------|
| `.git/hooks/pre-commit` | No (`.git/` is untracked) | The hook shell script |

**Files read (pre-flight check):**
| Path | Purpose |
|------|---------|
| `.ks-project.json` | Confirms project is initialized before writing hook |

`.ks-project.json` is defined by earlier user stories (US-001). This command only checks for its existence (using `os.Stat`); it does not parse it.

---

## API Definitions

No HTTP/RPC APIs. The command interface follows the existing CLI pattern:

```
ks install-hook [--force]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` | bool | false | Overwrite existing hook without error |

Exit codes:
- `0` — success
- `2` — error (JSON error written to stdout, consistent with other commands)

---

## Security Considerations

1. **No arbitrary code injection** — the hook script is a fixed constant embedded in the binary, not user-supplied. Arguments are not interpolated into the script body.

2. **No silent overwrites** — existing hooks are never overwritten without `--force`. This prevents accidental destruction of user-authored hooks.

3. **Hook exits 0** — the hook is advisory and never blocks commits. This is a deliberate design choice: the agent (not the hook) decides whether to abort.

4. **Executable bit** — the script is written with mode `0755`. This is standard for git hooks and does not elevate privileges beyond the current user.

5. **`ks start` is scoped to `--local`** — Chrome auto-start uses project-local state (`--local`), avoiding interference with global state.

6. **No secrets in hook** — the script contains no credentials, tokens, or environment variable captures. All sensitive config is read by `ks` itself at runtime.

---

## File Summary

| File | Action | Description |
|------|--------|-------------|
| `cmd/install_hook.go` | Create | `RunInstallHook` implementation |
| `cmd/util.go` | Edit | Add `findGitRoot()` helper |
| `main.go` | Edit | Add `install-hook` case + usage entry |

---

## Acceptance Criteria Mapping

| Criterion | Implementation |
|-----------|---------------|
| Writes executable script to `.git/hooks/pre-commit` | `os.WriteFile` + `os.Chmod(0755)` in `RunInstallHook` |
| Hook runs `ks snapshot` then `ks diff` | Hook script body |
| Outputs structured diff JSON to stdout | `ks diff` stdout in hook script |
| Always exits 0 | `exit 0` at end of hook; all error paths also `exit 0` |
| Auto-starts Chrome via `ks start` | `ks status` check + `ks start --local` in hook |
| Fails gracefully if URLs unreachable | Snapshot failure branch in hook emits advisory JSON, continues |
| Warns if hook already exists, no silent overwrite | Pre-flight `os.Stat` check; error unless `--force` |
| Error if no `.ks-project.json` | Pre-flight `os.Stat` check in `RunInstallHook` |
