---
name: smith-prd
description: "Author a smith-compatible PRD as markdown through codebase exploration and structured interview. Output passes smith's ingress validation. Triggers on: smith prd, write a smith prd, create prd for smith, plan a smith feature."
---

# Smith PRD Author

Create a **markdown PRD** that passes smith's ingress validation (`ParsePRDMarkdown` + `ValidateReport`). The PRD is a collaborative, human-readable document. Smith handles markdown-to-JSON conversion at ingress.

---

## The Job

1. Receive a feature description from the user
2. Explore the codebase to understand current state, relevant patterns, and integration points
3. Interview the user with structured questions about scope, stories, acceptance criteria, and dependencies
4. Generate a markdown PRD using smith's canonical heading conventions
5. Self-validate against all ingress rules below before presenting
6. Save to `.agents/tasks/prd-<slug>.md`

**Important:** Do NOT implement anything. Only generate the PRD markdown.

---

## Step 1: Understand the Feature

Ask the user for a description of the problem and potential solutions. Then explore the codebase to:
- Verify assertions about current behavior
- Identify files and patterns that will be affected
- Find existing utilities or abstractions that should be reused
- Understand related ADRs and guardrails that apply

---

## Step 2: Interview

Ask structured clarifying questions in batches of 3-5. Focus on:

- **Problem/Goal:** What problem does this solve? Why now?
- **Scope:** What should it do? What should it NOT do?
- **Stories:** What are the discrete units of work? What are the dependencies between them?
- **Acceptance Criteria:** How do we know each story is done? What are the failure/error cases?
- **Surface Areas:** Which surfaces does each story touch (CLI, API, UI)? Stories touching 3+ surfaces must be split or have explicit dependencies.
- **Constraints:** Backward compatibility, performance, security considerations?
- **Related ADRs:** Does this work implement or depend on an existing ADR? Would it warrant a new one?

Always ask:
- **Is this a new capability or a modification of existing behavior?**
- **What are the negative/error cases?** (smith requires at least one per story)

---

## Step 3: Generate the PRD

Output a markdown file following smith's canonical heading conventions. The parser (`ParsePRDMarkdown`) recognizes these exact heading names.

### Template

```markdown
# <Project Name>

## Overview

<Problem statement and solution summary. This becomes the PRD overview.>

## Goals

- <Goal 1>
- <Goal 2>

## Non-Goals

- <Explicitly out of scope>

## Success Metrics

- <How success is measured>

## Open Questions

- <Remaining unknowns, if any>

## Rules

- <Business rules, constraints, or guardrails that apply>

## Quality Gates

- mise run test:unit
- mise run lint
- mise run build

## Stories

### US-001: <Story Title>

#### Description

As a <actor>, I want <feature> so that <benefit>.

<Additional context or detail as needed.>

#### Status

open

#### Acceptance Criteria

- <Specific, measurable criterion>
- <Another criterion>
- <At least one negative/error case per story>

#### Depends On

<Comma-separated list of story IDs, e.g. US-001, US-002. Omit section or leave empty if none.>

### US-002: <Next Story Title>

...
```

### Ingress Paths

After saving, tell the user:

```
PRD saved to <path>.

Submit to smith via:
  smith prd submit --file <path>

Or paste as a GitHub issue body and submit via:
  smith prd submit --issue owner/repo#<number>
```

---

## Smith Ingress Validation Rules

The PRD **must** satisfy all of these rules to pass `smith prd submit`. Self-check the output against every rule before presenting.

### Document-Level (Errors — block ingress)

| Field | Rule |
|-------|------|
| Project | Non-empty. Parsed from the `# <heading>` (h1) |
| Overview | Non-empty. Parsed from `## Overview` (or Summary, Description, Product Overview) |
| Quality Gates | At least 1 non-empty command under `## Quality Gates` |
| Stories | At least 1 story required |

### Story IDs (Errors — block ingress)

| Rule | Detail |
|------|--------|
| Format | Must be `US-001`, `US-002`, etc. (uppercase `US-` + 3-digit zero-padded number) |
| Sequential | IDs must match array position: first story = US-001, second = US-002, etc. |
| Unique | No duplicate story IDs |

### Story Fields (Errors — block ingress)

| Field | Rule |
|-------|------|
| Title | Non-empty. Parsed from `### US-001: <Title>` |
| Description | Non-empty. Parsed from `#### Description` section |
| Status | Must be `open`, `in_progress`, or `done`. Default `open` for new PRDs. Max 1 story can be `in_progress` |
| Acceptance Criteria | At least 1 criterion under `#### Acceptance Criteria` |

### Story Dependencies (Errors — block ingress)

| Rule | Detail |
|------|--------|
| Known refs | `dependsOn` IDs must reference existing story IDs in the same PRD |
| No forward refs | Can only depend on earlier stories (lower index). US-003 can depend on US-001 but not US-004 |

### Surface Bundling (Error — blocks ingress)

Stories that touch 3 or more surfaces (CLI, API, UI) without declaring dependencies on stories that cover individual surfaces must be split. The surfaces are detected by keywords:
- **CLI**: cli, command, terminal, flag
- **API**: api, http, endpoint, handler, server
- **UI**: ui, ux, frontend, front-end, page, component, screen

### Acceptance Criteria Quality (Warnings — allow ingress but flagged)

| Rule | Detail |
|------|--------|
| Negative case | At least one criterion per story must cover a failure/error scenario. Use words like: invalid, missing, reject, error, fail, without, cannot, prevent, malformed, denied, empty, unknown, unauthorized |
| No weak language | Avoid vague terms: "works as expected", "handles edge cases", "appropriate", "proper", "robust", "seamless", "user-friendly", "nice to have", "improve", "enhance", "clean up", "better". Criteria shorter than 4 words are also flagged unless they match the negative-case pattern |
| Max criteria | Keep to 15 or fewer criteria per story |
| Max size | Title + description + all criteria should stay under 3000 characters per story |

---

## Story Sizing

Each story must be completable in a single smith loop iteration. If a story feels too large (many surfaces, complex logic, long criteria list), split it into smaller stories with dependencies.

---

## Naming

If the user provides a directory path instead of a filename, choose:
`prd-<short-slug>.md` where `<short-slug>` is 1-3 meaningful words.
Examples: `prd-pr-review-webhook.md`, `prd-task-kanban.md`, `prd-doppler-sync.md`

---

## Quality Gates

Always include these smith defaults unless the user specifies otherwise:

```
- mise run test:unit
- mise run lint
- mise run build
```

---

## Checklist Before Saving

Before writing the file, verify:

- [ ] H1 heading is the project name (non-empty)
- [ ] `## Overview` section is present and non-empty
- [ ] `## Quality Gates` has at least 1 command
- [ ] At least 1 story exists
- [ ] Story IDs are `US-001`, `US-002`, ... sequential, no gaps, no duplicates
- [ ] Every story has a non-empty title, description, and at least 1 acceptance criterion
- [ ] Every story has at least one negative/error case in acceptance criteria
- [ ] No weak language in acceptance criteria
- [ ] No story touches 3+ surfaces without dependencies
- [ ] Dependencies only reference earlier stories
- [ ] All stories have `open` status (for new PRDs)
- [ ] Each story is small enough for one loop iteration
