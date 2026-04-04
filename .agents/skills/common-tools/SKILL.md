---
name: common-tools
description: One-line usage reference for the core CLI tools preinstalled in the Smith replica runtime.
---

# Tool Usage

- `rg`: Search text with regex fast, e.g. `rg "TODO|FIXME" .`.
- `fd`: Find files/directories quickly, e.g. `fd "\\.go$"`.
- `fzf`: Fuzzy-filter long lists interactively, e.g. `fd | fzf`.
- `ast-grep`: Search by syntax/AST patterns, e.g. `ast-grep --pattern 'func $NAME($$$)'`.
- `jq`: Filter/transform JSON output, e.g. `smith ... --output json | jq '.items[]?.name'`.
- `bat`: View files with syntax + line numbers, e.g. `bat -n cmd/smith/main.go`.
- `eza`: List files with richer output than `ls`, e.g. `eza -la --git`.
- `ks`: Run browser automation and UI inspection, e.g. `ks open http://localhost`.
- `gh`: Use GitHub from CLI, e.g. `gh pr status`.
- `git`: Source control operations, e.g. `git status -sb`.
- `smith`: Smith CLI entrypoint, e.g. `smith --help`.
- `task`: Task-focused Smith CLI, e.g. `task --help`.
- `codex`: Run Codex CLI workflows, e.g. `codex --help`.
- `mise`: Install/manage language/tool versions, e.g. `mise install node@22`.
- `smith-install-tools`: Install extra apt/mise tools in-container, e.g. `smith-install-tools --tool python@3.12`.
- `go`: Build/test Go code, e.g. `go test ./...`.
- `python3`: Run Python scripts, e.g. `python3 script.py`.
- `pip`: Install Python packages, e.g. `pip install pyyaml`.
- `make`: Run repo automation targets, e.g. `make test`.
- `curl`: Call HTTP endpoints directly, e.g. `curl -fsSL http://localhost/healthz`.
