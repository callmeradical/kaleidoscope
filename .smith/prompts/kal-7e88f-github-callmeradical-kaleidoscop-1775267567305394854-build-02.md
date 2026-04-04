You are an autonomous coding agent.
Use the PRD JSON at: /workspace/.smith/loop/input/prd.json
Use the implementation plan at: /workspace/.agents/tasks/implementation-plan.md
Current build iteration: 2 of 10

Paths:
- PRD: /workspace/.smith/loop/input/prd.json
- AGENTS (optional): /workspace/AGENTS.md
- Progress Log: /workspace/.smith/loop/progress.md
- Guardrails: /workspace/.smith/loop/guardrails.md
- Guardrails Reference: /workspace/.smith/loop/references/GUARDRAILS.md
- Context Reference: /workspace/.smith/loop/references/CONTEXT_ENGINEERING.md
- Errors Log: /workspace/.smith/loop/errors.log
- Activity Log: /workspace/.smith/loop/activity.log
- Activity Logger: /workspace/.smith/loop/log-activity.sh
- No-commit: false
- Repo Root: /workspace
- Run ID: kal-7e88f-github-callmeradical-kaleidoscop-us-002
- Iteration: 2
- Run Log: /workspace/.smith/loop/run.log
- Run Summary: /workspace/.smith/loop/run-summary.md

Rules (Non-Negotiable):
- Implement only the work required for the selected story.
- Do not change unrelated code or switch to other stories.
- Confirm existing behavior in code before assuming missing functionality.
- If No-commit is true, do not commit or push.
- Use the $tdd skill workflow while developing.


Your Task (Do this in order):
1. Read the guardrails and context reference before any code edits.
2. Read recent errors and prior progress to avoid repeating failures.
3. Read the PRD and implementation plan before making edits.
4. If AGENTS exists, follow its build/test instructions.
5. Implement only selected story scope.
6. Run verification and quality-gate commands.
7. Log major actions with the activity logger command.
8. Update progress log with implementation and verification outcomes.
9. If frontend/UI changed, validate in a browser before completion.

Loop Behavior Notes:
- Update story status in the PRD JSON according to progress.
- Do not switch scope to another story in this run.


Do NOT implement anything not defined in the PRD.
If you encounter a blocker, record it in the PRD and stop.

Issue Context:
- title: Snapshot Capture and History

Selected Story (do not change scope):
- id: US-002
- title: Snapshot Capture and History
- acceptance criteria:
  - ks snapshot` creates a correctly structured directory under `.kaleidoscope/snapshots/` with one subdirectory per URL path
  - Each URL subdirectory contains 4 breakpoint PNGs, `audit.json`, and `ax-tree.json
  - Root `snapshot.json` manifest includes timestamp, commit hash (when in a git repo), and project config at capture time
  - Snapshot ID format is `<timestamp>-<short-commit-hash>` or timestamp-only outside git
  - First snapshot with no existing baseline auto-creates `.kaleidoscope/baselines.json
  - ks history` lists snapshots with timestamp, commit hash, and summary stats
  - ks snapshot` fails gracefully with a clear error if project URLs are unreachable
- complete only this story in this loop

Completion gate for this iteration:
- Implement production code needed to satisfy tests.
- Mark a story as 'done' only when verification commands pass.

Global Quality Gates (must pass before completion):
- go test ./...



Progress Entry Requirements:
- Append run details (run id, iteration, commands, pass/fail results, files changed) to the progress log.
- Record key learnings/patterns for future iterations.

Completion Signal:
Only print <promise>COMPLETE</promise> when this selected story is fully complete and verified.