---
name: bug-fix
description: Structured bug fix workflow for brownfield codebases. Reproduce, isolate root cause, patch with minimal blast radius, and lock in regression coverage using TDD.
---

# Bug Fix Workflow

You are resolving a bug in an existing codebase. Your goal is to produce the smallest safe fix with regression coverage that proves the bug is resolved and will not recur.

**Always use the `$tdd` skill alongside this workflow.** Every phase below integrates with the red-green-refactor cycle.

## Phase 1: Reproduce

Before touching any production code, prove the bug exists with a deterministic failing check.

1. Read the issue description, reproduction steps, and any linked error logs or screenshots.
2. Explore the codebase to understand the affected module, its public interfaces, and existing test coverage.
3. Write a **single failing test** that captures the broken behavior through a public interface. This is your RED test.
   - The test must fail for the reason described in the issue, not for an unrelated reason.
   - The test must use the public API of the module -- do not reach into internals.
   - If the bug is environment-specific (e.g., race condition, platform-dependent), document the reproduction conditions in the test.
4. Run the test suite to confirm:
   - Your new test fails (RED).
   - All existing tests still pass (no collateral damage from test setup).

**Do not proceed to Phase 2 until you have a reliably failing test.**

## Phase 2: Root Cause

Identify why the behavior is wrong. The failing test from Phase 1 is your anchor.

1. Trace the execution path from the test's entry point to the failure.
2. Identify the fault boundary -- the smallest unit of code responsible for the incorrect behavior.
3. Check for related code paths that may have the same underlying issue (sibling bugs).
4. Document your root cause hypothesis as a comment in the test file or in your progress log:
   - What is the fault?
   - Why does it produce the observed behavior?
   - What is the expected correct behavior?

## Phase 3: Patch

Implement the smallest change that resolves the root cause.

1. Fix the fault identified in Phase 2. Prefer the minimal change -- do not refactor adjacent code in this phase.
2. Run your failing test -- it should now pass (GREEN).
3. Run the full test suite to verify no regressions.
4. If the fix touches a hot path or shared utility, consider whether the change needs additional coverage for edge cases. If so, add one test at a time (RED -> GREEN) per the `$tdd` workflow.

**Rules:**
- Do not change behavior beyond what the issue describes.
- Do not introduce new features or refactors in the fix commit.
- If the fix requires a larger structural change, stop and post an implementation plan comment on the issue describing the approach. Wait for approval before proceeding.

## Phase 4: Regression Coverage

Lock in the fix so it cannot silently regress.

1. Review your test(s) from Phase 1. Do they adequately cover:
   - The exact reproduction case from the issue?
   - At least one boundary condition near the fault?
   - The "happy path" that should remain unaffected?
2. Add any missing coverage, one test at a time (RED -> GREEN).
3. Run the full test suite one final time.
4. Commit with a message that references the issue and describes the root cause.

## Scope Management

- **Small fix (1-3 files changed, clear root cause):** Proceed directly through all phases without posting a plan.
- **Medium fix (4-10 files, or root cause is ambiguous):** After Phase 2, post a brief root cause analysis and proposed fix as a comment on the issue. Proceed after confirmation.
- **Large fix (>10 files, or requires interface changes):** After Phase 2, post a detailed implementation plan. Wait for an approval comment before proceeding to Phase 3.

## Completion Criteria

- [ ] A test exists that would have caught this bug before the fix.
- [ ] The fix is minimal and does not change unrelated behavior.
- [ ] All existing tests pass.
- [ ] The commit message references the issue and summarizes the root cause.
