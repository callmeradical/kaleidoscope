---
name: feature
description: Structured feature delivery workflow for brownfield codebases. Scope, design, build in vertical slices, and validate against acceptance criteria using TDD.
---

# Feature Delivery Workflow

You are adding a new feature to an existing codebase. Your goal is to deliver the feature in testable vertical slices, proving each slice works before moving to the next.

**Always use the `$tdd` skill alongside this workflow.** Every slice is built with the red-green-refactor cycle.

## Phase 1: Scope

Clarify what you are building and where it fits in the existing system.

1. Read the issue description, acceptance criteria, and any linked documents (PRDs, designs, related issues).
2. Explore the codebase to understand:
   - Existing modules and patterns that the feature will extend or interact with.
   - The project's conventions (file layout, naming, error handling, testing patterns).
   - Existing test infrastructure (test helpers, fixtures, mocking patterns).
3. Extract from the issue:
   - **User goal:** What should the user be able to do when this is done?
   - **Acceptance criteria:** Concrete, testable conditions for completion. If the issue does not have explicit acceptance criteria, derive them and post them as a comment for confirmation.
   - **Non-goals:** What is explicitly out of scope.
   - **Dependencies:** What existing functionality does this build on?
4. Identify risks: breaking changes, migration needs, performance implications.

## Phase 2: Design

Plan the implementation as a series of vertical slices.

1. Break the feature into **vertical slices** -- each slice delivers a thin end-to-end piece of functionality that can be tested independently.
   - Good slice: "User can create a widget via the API and it persists."
   - Bad slice: "Add the database schema for widgets" (horizontal -- not testable end-to-end).
2. Order slices by dependency and value. The first slice should be the **tracer bullet** -- the simplest possible end-to-end path that proves the architecture works.
3. For each slice, identify:
   - The public interface being added or extended.
   - The test that will prove the slice works.
   - Any existing code that needs modification.

**Post the slice plan as a comment on the issue.** For features with more than 3 slices, wait for approval before proceeding. For features with 3 or fewer slices, proceed directly.

## Phase 3: Build

Implement each slice using the `$tdd` red-green-refactor cycle.

For each slice:

1. **RED:** Write a test that describes the desired behavior through the public interface. The test must fail.
2. **GREEN:** Write the minimum code to make the test pass. Follow existing codebase patterns and conventions.
3. **REFACTOR:** If the new code introduces duplication or the design can be improved, refactor while keeping tests green.
4. Run the full test suite to verify no regressions.
5. Commit the slice.

**Rules:**
- One slice at a time. Do not start the next slice until the current one is committed and green.
- Follow the existing codebase's patterns. If the project uses a specific ORM, router, or error handling convention, use it.
- If a slice reveals that the design needs to change, update the slice plan comment on the issue before continuing.
- Do not introduce speculative features or premature abstractions.

## Phase 4: Validation

Prove the feature meets all acceptance criteria.

1. Map each acceptance criterion to one or more passing tests.
2. Verify the feature works end-to-end by reviewing the test coverage across all slices.
3. Run the full test suite.
4. If the feature has UI components, verify they render correctly (use browser automation if available).
5. Review the total diff:
   - Are there any changes not covered by tests?
   - Are there any acceptance criteria without corresponding test evidence?
6. Commit the final state with a message summarizing the feature and referencing the issue.

## Scope Management

- **Small feature (1-3 slices, no new module boundaries):** Post the slice plan as a comment and proceed immediately.
- **Medium feature (4-6 slices, extends existing modules):** Post the slice plan and wait for approval before Phase 3.
- **Large feature (>6 slices, new modules, or significant interface changes):** Post a design document covering architecture, data model, and the slice plan. Wait for approval.

## Completion Criteria

- [ ] Every acceptance criterion has at least one passing test.
- [ ] The feature is built in vertical slices, each tested independently.
- [ ] The codebase conventions are followed (no alien patterns introduced).
- [ ] The full test suite passes with no regressions.
- [ ] The build completes successfully.
- [ ] The commit message references the issue and summarizes the delivered capability.
