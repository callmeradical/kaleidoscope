---
name: user-story
description: Structured user story delivery workflow for brownfield codebases. Frame the story, derive testable acceptance criteria, deliver in vertical slices, and validate using TDD.
---

# User Story Workflow

You are implementing a user story in an existing codebase. A user story describes a capability from the user's perspective. Your job is to deliver that capability with acceptance criteria proven by tests.

**Always use the `$tdd` skill alongside this workflow.** Every acceptance criterion becomes a test before it becomes code.

## Phase 1: Story Framing

Ensure the story is clear enough to implement and test.

1. Read the issue and extract:
   - **Actor:** Who benefits from this capability?
   - **Need:** What does the actor need to do?
   - **Outcome:** What is the desired result?
   - Format: "As a [actor], I want [need], so that [outcome]."
2. Explore the codebase to understand:
   - What existing functionality supports this actor and workflow.
   - Which modules and interfaces will be touched.
   - The project's conventions and patterns.
3. Identify **constraints and assumptions** (e.g., authentication required, rate limits, data format expectations).
4. Identify **unhappy paths** -- what happens when things go wrong (invalid input, missing data, permission denied).

## Phase 2: Acceptance Criteria

Derive precise, testable acceptance criteria.

1. If the issue has explicit acceptance criteria, verify they are:
   - **Specific:** No ambiguity about what "done" means.
   - **Testable:** Each criterion can be verified with a deterministic test.
   - **Complete:** Happy paths, edge cases, and error cases are covered.
2. If the issue lacks clear criteria, derive them and post as a comment:
   - Use Given/When/Then format where possible.
   - Include at least one negative case (what should NOT happen).
   - Include boundary conditions.
3. Map each criterion to the public interface it will be tested through.

**For stories with more than 5 acceptance criteria, post the criteria and wait for approval before proceeding.**

## Phase 3: Vertical Slice Delivery

Implement the story one acceptance criterion at a time.

1. Order the acceptance criteria by dependency. Start with the simplest happy-path criterion as the **tracer bullet**.
2. For each criterion, use the `$tdd` red-green-refactor cycle:
   - **RED:** Write a test that captures the acceptance criterion through a public interface. The test must fail.
   - **GREEN:** Write the minimum code to make the test pass. Follow existing codebase patterns.
   - **REFACTOR:** Improve structure while keeping tests green.
3. Run the full test suite after each criterion to verify no regressions.
4. Commit after each criterion passes.

**Rules:**
- One criterion at a time. Do not parallelize.
- Tests should read like specifications. Someone reading the test should understand the story without reading the issue.
- Follow existing codebase conventions. Match the project's error handling, logging, and naming patterns.
- If implementing a criterion reveals missing criteria, add them and note the addition in the issue.

## Phase 4: Story Validation

Prove the story is complete.

1. Map every acceptance criterion to its passing test(s). Create a checklist:
   ```
   [x] Given X, when Y, then Z -- test_file:line
   [x] Given A, when B, then C -- test_file:line
   ```
2. Verify unhappy paths are covered (error conditions, invalid input, edge cases).
3. Run the full test suite.
4. Review the diff for any untested code paths.
5. Commit with a message summarizing the delivered capability and referencing the issue.

## Scope Management

- **Small story (1-3 acceptance criteria):** Proceed directly through all phases.
- **Medium story (4-7 acceptance criteria):** Post criteria in Phase 2 and proceed after confirmation.
- **Large story (>7 acceptance criteria, or cross-module changes):** Post criteria and a slice plan. Wait for approval.

## Completion Criteria

- [ ] Every acceptance criterion has at least one passing test.
- [ ] Tests read as specifications of the story's behavior.
- [ ] Happy paths, edge cases, and error cases are tested.
- [ ] The codebase conventions are followed.
- [ ] The full test suite passes with no regressions.
- [ ] The commit message references the issue and summarizes the story.
