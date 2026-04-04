---
name: task
description: Manage Smith task contracts during loop execution with consistent lifecycle updates. Use when a loop is bound to a task_contract_id and needs start/log/handoff status updates.
---

# Task Workflow

Use this skill when a loop has `task_contract_id` metadata and must keep task state synchronized.

## Required Sequence

1. Start a task session at loop bootstrap:

```bash
task usage --new-session
```

2. Select/queue work when needed:

```bash
task list --status draft
task next
task start        # auto-selects next eligible task when id is omitted
```

3. Move a specific bound task into active execution:

```bash
task start <task-id>
```

4. Record meaningful lifecycle checkpoints:

```bash
task log <task-id> "<progress update>"
```

5. Write completion handoff details:

```bash
task handoff <task-id> --done a,b --remaining c,d
```

6. Create/schedule tasks directly when needed:

```bash
task add --objective "Implement X" --project-id smith --provider-profile-id codex-default --validation "go test ./..."
```

## Notes

- Keep logs milestone-based; avoid command-by-command spam.
- Keep `done` and `remaining` short, concrete, and outcome-focused.
- Use `task next`/`task start` to pull queued work in priority order.
- Do not mutate unrelated tasks.
