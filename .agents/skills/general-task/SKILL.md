---
name: general-task
description: Structured workflow for general development tasks in brownfield codebases. Analyze, implement, and verify with TDD safety net.
---

# General Task Workflow

You are executing a development task in an existing codebase. Tasks are well-defined units of work that do not fit neatly into bug-fix, feature, or refactor categories -- examples include adding a new API endpoint, writing a migration script, creating a utility function, or integrating with an external service.

**Always use the `$tdd` skill alongside this workflow.** Even straightforward tasks benefit from test-first discipline in brownfield codebases.

## Phase 1: Analyze

Understand the task and its impact on the existing system.

1. Read the issue description to understand:
   - **What** needs to be done (the deliverable).
   - **Why** it needs to be done (the motivation).
   - **Where** it fits in the existing codebase.
2. Explore the codebase to understand:
   - Which existing modules, patterns, and conventions are relevant.
   - What existing test infrastructure is available.
   - What the task depends on and what depends on the task's output.
3. Identify the **acceptance criteria** -- if the issue does not state explicit criteria, derive them:
   - What is the observable outcome of the completed task?
   - How can completion be verified?
4. Identify risks: breaking changes, compatibility concerns, performance implications.

## Phase 2: Plan

Break the task into verifiable steps.

1. Decompose the task into a sequence of concrete implementation steps.
2. For each step, identify:
   - What will change (files, interfaces, configurations).
   - What test will verify the change.
3. Order steps by dependency. Prefer steps that can be tested independently.

**For tasks with more than 5 steps, post the plan as a comment on the issue. Wait for approval before proceeding.**

## Phase 3: Implement

Execute the plan using the `$tdd` red-green-refactor cycle.

For each step:

1. **RED:** Write a test that describes the expected behavior of this step through a public interface. The test must fail.
2. **GREEN:** Write the minimum code to make the test pass. Follow existing codebase patterns and conventions.
3. **REFACTOR:** Improve structure while keeping tests green.
4. Run the full test suite to verify no regressions.
5. Commit the step.

**Rules:**
- One step at a time.
- Follow existing codebase conventions. If the project uses specific patterns for error handling, logging, or configuration, use them.
- Do not introduce scope creep. If you discover adjacent improvements, note them in the issue but do not implement them.
- If a step proves more complex than expected, update the plan comment on the issue.

## Phase 4: Verify

Prove the task is complete.

1. Map each acceptance criterion or step to its passing test.
2. Run the full test suite.
3. Run any build commands to verify compilation/bundling.
4. Review the diff:
   - Is every change covered by a test?
   - Are there any unintended side effects?
5. Commit the final state with a message describing the completed task and referencing the issue.

## Scope Management

- **Small task (1-3 files, clear outcome):** Proceed directly through all phases.
- **Medium task (4-10 files, or involves new interfaces):** Post the plan after Phase 2. Proceed after confirmation.
- **Large task (>10 files, cross-module, or involves external integrations):** Post a detailed plan. Wait for approval.

## Completion Criteria

- [ ] The deliverable described in the issue is implemented.
- [ ] Each acceptance criterion has test evidence.
- [ ] The codebase conventions are followed.
- [ ] The full test suite passes with no regressions.
- [ ] The build completes successfully.
- [ ] The commit message references the issue and describes the completed work.
