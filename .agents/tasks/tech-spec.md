# Technical Specification: Pre-Commit Hook Integration (US-007)

**Story**: US-007
**Feature**: Snapshot History and Regression Detection
**Status**: Spec Only — Do Not Implement

---

## 1. Architecture Overview

US-007 introduces a single new command `ks install-hook` that writes a git pre-commit shell script to `.git/hooks/pre-commit`. The hook orchestrates `ks snapshot` and `ks diff` (defined in sibling user stories) on every git commit, then emits structured JSON to stdout for agent consumption. The hook is always advisory (exit 0) so it never blocks commits.

```
Developer / AI Agent
        │
        ▼
  git commit
        │
        ▼
.git/hooks/pre-commit  (written by `ks install-hook`)
        │
        ├─► ks start     (if Chrome not running)
        ├─► ks snapshot  (capture current state)
        └─► ks diff      (compare vs baseline)
              │
              └─► stdout: JSON diff result
                  exit 0 always
```

**New files to create:**
- `cmd/installhook.go` — implementation of `ks install-hook`

**Existing files to modify:**
- `main.go` — register `install-hook` case in the switch statement
- `cmd/usage.go` — add usage entry for `install-hook`

**Dependencies on other stories (not implemented here):**
- `ks snapshot` command (US-001 through US-006) must exist for the hook to function
- `ks diff` command must exist for the hook to function
- `.ks-project.json` project config must be readable (used to validate project is configured)

---

## 2. Detailed Component Design

### 2.1 `ks install-hook` Command (`cmd/installhook.go`)

**Responsibilities:**
1. Validate that `.ks-project.json` exists in CWD; fail with structured error if not
2. Detect the git root directory by walking up from CWD looking for `.git/`
3. Check if `.git/hooks/pre-commit` already exists:
   - If it does and `--force` is not set: emit a warning JSON and return (no overwrite)
   - If it does and `--force` is set: overwrite
4. Write the hook script to `.git/hooks/pre-commit` with mode `0755`
5. Emit `output.Success("install-hook", ...)` with the hook path

**Flags:**
- `--force` — overwrite existing hook without warning

**Function signature:**
```go
// cmd/installhook.go
package cmd

func RunInstallHook(args []string) {
    force := hasFlag(args, "--force")
    // ... implementation
}
```

**Algorithm:**
```
1. stat(".ks-project.json")
   → if missing: output.Fail("install-hook", err, "Run `ks init` to create a project config") ; os.Exit(2)

2. gitRoot = findGitRoot(".")
   → if not found: output.Fail(..., "Not inside a git repository") ; os.Exit(2)

3. hookPath = filepath.Join(gitRoot, ".git", "hooks", "pre-commit")

4. if exists(hookPath) && !force:
       output.Fail("install-hook",
           errors.New("pre-commit hook already exists: " + hookPath),
           "Use --force to overwrite")
       os.Exit(2)

5. Write hookScript (see §2.2) to hookPath with 0755 permissions

6. output.Success("install-hook", map[string]any{
       "hookPath":  hookPath,
       "overwrite": force && existedBefore,
   })
```

**`findGitRoot` helper:**
```go
// Walks up from dir until it finds a .git directory or reaches filesystem root.
func findGitRoot(dir string) (string, error) {
    abs, err := filepath.Abs(dir)
    if err != nil { return "", err }
    for {
        if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
            return abs, nil
        }
        parent := filepath.Dir(abs)
        if parent == abs {
            return "", errors.New("not a git repository")
        }
        abs = parent
    }
}
```

### 2.2 Pre-Commit Hook Script

The hook is a POSIX shell script written verbatim to `.git/hooks/pre-commit`.

```sh
#!/bin/sh
# kaleidoscope pre-commit hook
# Installed by: ks install-hook
# This hook is advisory — it always exits 0 and never blocks commits.

KS=$(command -v ks 2>/dev/null)
if [ -z "$KS" ]; then
  echo '{"ok":false,"command":"pre-commit","error":"ks not found in PATH","hint":"Install kaleidoscope and ensure it is on PATH"}' >&2
  exit 0
fi

# Auto-start Chrome if not running
"$KS" status > /dev/null 2>&1
if [ $? -ne 0 ]; then
  echo '{"ok":true,"command":"pre-commit","info":"starting Chrome"}' >&2
  "$KS" start > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo '{"ok":false,"command":"pre-commit","error":"failed to start Chrome","hint":"Run: ks start"}' >&2
    exit 0
  fi
fi

# Take snapshot
SNAPSHOT_OUT=$("$KS" snapshot 2>&1)
SNAPSHOT_EXIT=$?
if [ $SNAPSHOT_EXIT -ne 0 ]; then
  printf '%s\n' "$SNAPSHOT_OUT"
  exit 0
fi

# Run diff against baseline
DIFF_OUT=$("$KS" diff 2>&1)
printf '%s\n' "$DIFF_OUT"

# Always advisory — never block the commit
exit 0
```

**Design decisions:**
- `command -v ks` used instead of `which` for POSIX portability
- All diagnostic output goes to stdout (not stderr) because agents read stdout
- `ks status` used to probe Chrome liveness before attempting `ks start`
- `ks snapshot` failures are surfaced via the JSON they already emit, then hook exits 0
- Each intermediate line is JSON-compatible for agent parsing

### 2.3 Hook Script as Go String Constant

The script is embedded as a Go string constant in `cmd/installhook.go` — **not** as an `embed.FS` file — to keep the binary self-contained and avoid asset pipeline complexity:

```go
const hookScript = `#!/bin/sh
# kaleidoscope pre-commit hook
...
`
```

### 2.4 Graceful Failure Modes

| Scenario | Hook Behavior |
|---|---|
| `ks` not in PATH | Emit JSON error to stdout, exit 0 |
| No `.ks-project.json` | `ks snapshot` returns structured error JSON, hook emits it, exits 0 |
| Chrome fails to start | Emit JSON error, exit 0 |
| URL unreachable | `ks snapshot` detects and returns error JSON; hook emits, exits 0 |
| `ks diff` fails | Emit `ks diff` error JSON, exit 0 |

All failure paths produce valid JSON on stdout so the agent can parse them without special-casing stderr.

---

## 3. API Definitions

### 3.1 `ks install-hook` Output

**Success (hook written for first time):**
```json
{
  "ok": true,
  "command": "install-hook",
  "result": {
    "hookPath": "/path/to/repo/.git/hooks/pre-commit",
    "overwrite": false
  }
}
```

**Success with `--force` overwrite:**
```json
{
  "ok": true,
  "command": "install-hook",
  "result": {
    "hookPath": "/path/to/repo/.git/hooks/pre-commit",
    "overwrite": true
  }
}
```

**Error — hook already exists (no --force):**
```json
{
  "ok": false,
  "command": "install-hook",
  "error": "pre-commit hook already exists: /path/to/repo/.git/hooks/pre-commit",
  "hint": "Use --force to overwrite"
}
```

**Error — no `.ks-project.json`:**
```json
{
  "ok": false,
  "command": "install-hook",
  "error": "no .ks-project.json found in current directory",
  "hint": "Run `ks init` to create a project config first"
}
```

**Error — not a git repository:**
```json
{
  "ok": false,
  "command": "install-hook",
  "error": "not a git repository",
  "hint": "Run `ks install-hook` from inside a git repository"
}
```

### 3.2 Pre-Commit Hook stdout JSON (for agent consumption)

The hook emits whatever `ks snapshot` and `ks diff` emit (both follow the `output.Result` contract):

```json
{"ok":true,"command":"snapshot","result":{"snapshotId":"abc123","url":"http://localhost:3000","timestamp":"2026-04-04T12:00:00Z"}}
{"ok":true,"command":"diff","result":{"regression":false,"newIssues":0,"resolvedIssues":2,"details":{}}}
```

If Chrome was auto-started, an informational line is emitted first:
```json
{"ok":true,"command":"pre-commit","info":"starting Chrome"}
```

---

## 4. Data Model Changes

US-007 introduces **no new persistent data structures**. It only writes a file to `.git/hooks/pre-commit`. No changes to `.kaleidoscope/`, no new JSON schemas, no database modifications.

### 4.1 `.ks-project.json` (read-only in this story)

`install-hook` reads `.ks-project.json` only to validate its existence. The schema is owned by the project-config story (not US-007). For the purpose of this story, the check is:

```go
if _, err := os.Stat(".ks-project.json"); os.IsNotExist(err) {
    output.Fail("install-hook", errors.New("no .ks-project.json found in current directory"), "Run `ks init` to create a project config first")
    os.Exit(2)
}
```

No unmarshalling of `.ks-project.json` is needed in this story.

---

## 5. Main.go and usage.go Changes

### 5.1 `main.go` switch case

Add after the `install-skills` case:

```go
case "install-hook":
    cmd.RunInstallHook(cmdArgs)
```

And update the `usage` string to include:

```
Hooks:
  install-hook [--force]  Install git pre-commit hook for regression detection
```

### 5.2 `cmd/usage.go`

Add to the `CommandUsage` map:

```go
"install-hook": {
    Summary: "Install a git pre-commit hook that runs snapshot and diff on every commit",
    Usage:   "ks install-hook [--force]",
    Flags: []FlagDoc{
        {Flag: "--force", Description: "Overwrite existing hook without warning"},
    },
    Examples: []string{
        "ks install-hook",
        "ks install-hook --force",
    },
    Output: `{"ok":true,"command":"install-hook","result":{"hookPath":".git/hooks/pre-commit","overwrite":false}}`,
},
```

---

## 6. Security Considerations

### 6.1 Hook Script Injection

The hook script is a static string constant with no dynamic interpolation of user-supplied data. There is no risk of injection through `ks install-hook` arguments.

### 6.2 File Permissions

The hook is written with mode `0755` (owner: rwx, group: r-x, other: r-x), matching standard git hook conventions. No more permissive than needed.

### 6.3 Git Root Traversal

`findGitRoot` walks up the directory tree looking for `.git`. It correctly terminates at the filesystem root (`parent == abs`), preventing infinite loops. It does not follow symlinks when checking for `.git`.

### 6.4 Overwrite Protection

The default behavior refuses to overwrite an existing hook. This prevents silent destruction of a user's existing pre-commit workflow. The `--force` flag is required for overwrite, making the destructive action explicit and intentional.

### 6.5 Advisory-Only Exit Code

The hook always exits 0. This is a deliberate design decision: the agent decides whether a regression is a blocker, not the hook. This prevents the hook from becoming a denial-of-service against the developer's workflow.

### 6.6 PATH Safety in Hook Script

The hook uses `command -v ks` to locate the binary rather than assuming a fixed path. This is safe across different shell environments (system shells, CI, developer workstations). If `ks` is not found, the hook fails gracefully with a JSON message rather than executing an unknown binary.

---

## 7. Testing Considerations

Per the quality gate `go test ./...`, the following test cases should be covered in `cmd/installhook_test.go`:

| Test Case | Expected Behavior |
|---|---|
| No `.ks-project.json` | Returns error JSON, exit code 2 |
| Not inside a git repo | Returns error JSON, exit code 2 |
| First install (no existing hook) | Writes hook, returns success JSON |
| Hook already exists, no `--force` | Returns error JSON (no overwrite) |
| Hook already exists, `--force` | Overwrites hook, returns success with `overwrite: true` |
| Written hook is executable (`0755`) | `os.Stat` mode check |
| Written hook contains expected shebang and `ks snapshot` invocation | String contains check |

---

## 8. File Change Summary

| File | Action | Description |
|---|---|---|
| `cmd/installhook.go` | **Create** | `RunInstallHook`, `findGitRoot`, `hookScript` constant |
| `main.go` | **Modify** | Add `case "install-hook": cmd.RunInstallHook(cmdArgs)` and update `usage` string |
| `cmd/usage.go` | **Modify** | Add `"install-hook"` entry to `CommandUsage` map |
| `cmd/installhook_test.go` | **Create** | Unit tests for all acceptance criteria |
