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
- Run ID: kal-83e54-autofix-kal-276cc-github-callmer-us-005
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
- title: Autofix: Baseline Manager

Selected Story (do not change scope):
- id: US-005
- title: Baseline Manager
- acceptance criteria:
  - ks accept` promotes the latest snapshot to baseline for all project URLs
  - ks accept <snapshot-id>` promotes a specific snapshot
  - ks accept --url /dashboard` updates baseline for only that URL path, leaving others unchanged
  - .kaleidoscope/baselines.json` is correctly updated on disk after accept
  - ks accept` returns an error if no snapshots exist
  - Accepting a snapshot that is already the baseline is a no-op (idempotent)
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