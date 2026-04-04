You are an autonomous coding agent.
Use the PRD JSON at: /workspace/.smith/loop/input/prd.json
Use the implementation plan at: /workspace/.agents/tasks/implementation-plan.md
Current build iteration: 1 of 10

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
- Run ID: kal-93484-autofix-kal-0d599-github-callmer-us-003
- Iteration: 1
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
- title: Autofix: Audit and Element Diff Engine

Selected Story (do not change scope):
- id: US-003
- title: Audit and Element Diff Engine
- acceptance criteria:
  - ks diff` compares latest snapshot against baseline and outputs structured JSON
  - ks diff <snapshot-id>` compares a specific snapshot against baseline
  - Audit deltas report per-category count changes (contrast, spacing, typography, touch targets)
  - Audit deltas track per-issue new/resolved status by matching on selector
  - Element changes detect appeared, disappeared, moved (position delta beyond threshold), and resized elements via ax-tree comparison
  - Element matching uses semantic identity (role + name) not CSS selectors
  - Exit code 0 when no regressions detected, exit code 1 when regressions exist
  - ks diff` returns an error if no baseline exists
- complete only this story in this loop

Tests-first gate for this iteration:
- This is a tests-only iteration.
- Add or update failing tests for the selected story.
- Do not mark any story as done in this iteration.
- Set active story status to 'in_progress' when work has started.

Global Quality Gates (must pass before completion):
- go test ./...



Progress Entry Requirements:
- Append run details (run id, iteration, commands, pass/fail results, files changed) to the progress log.
- Record key learnings/patterns for future iterations.

Completion Signal:
Only print <promise>COMPLETE</promise> when this selected story is fully complete and verified.