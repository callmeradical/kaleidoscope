---
name: refactor
description: Structured refactoring workflow for brownfield codebases. Establish behavioral baselines, restructure safely, and verify parity using TDD. No behavior changes allowed.
---

# Refactor Workflow

You are restructuring existing code to improve quality without changing observable behavior. Refactoring means the system does exactly what it did before, but the code is cleaner, more testable, or better organized.

**Always use the `$tdd` skill alongside this workflow.** The TDD cycle is your proof that behavior is preserved. If a refactor breaks a test and behavior has not changed, the test was testing implementation -- fix the test. If behavior has changed, your refactor introduced a bug -- revert.

## Phase 1: Baseline

Before changing any production code, establish what the system does today.

1. Read the issue description to understand the refactoring goal (e.g., reduce duplication, deepen a module, extract a concern, simplify an interface).
2. Explore the codebase to understand:
   - The module(s) being refactored and their public interfaces.
   - Existing test coverage for those modules.
   - Callers and consumers of the public interfaces.
3. Run the existing test suite. All tests must pass. This is your behavioral baseline.
4. Identify **behavioral invariants** -- the observable behaviors that must remain unchanged:
   - What do the public APIs accept and return?
   - What side effects occur (file writes, network calls, state changes)?
   - What error conditions are handled and how?
5. If existing test coverage is thin, write **characterization tests** that capture current behavior before refactoring. Use the `$tdd` skill: one test at a time, each describing an observable behavior through a public interface.
   - Characterization tests are not aspirational -- they describe what the code does now, even if that behavior is imperfect.
   - Mark any tests that capture known-bad behavior with a comment explaining the issue.

**Do not proceed to Phase 2 until all behavioral invariants have test coverage.**

## Phase 2: Restructure

Make structural changes in small, verifiable steps.

1. Plan the refactoring sequence. Prefer a series of small, independently verifiable transformations over one large rewrite.
2. For each transformation:
   - Make the structural change.
   - Run the test suite immediately.
   - If all tests pass: the transformation preserved behavior. Commit.
   - If a test fails: determine whether the test was testing behavior (refactor introduced a bug -- revert) or implementation (test needs updating -- update the test, then verify the behavior is still correct).
3. After each transformation, check whether the public interface changed:
   - If yes: update all callers. Run the test suite.
   - If no: no caller changes needed.

**Rules:**
- Never change behavior and structure in the same commit.
- If you discover a bug during refactoring, do not fix it in the refactor. File it separately or note it in the issue.
- If the refactoring reveals that the original design had unintended behavior relied upon by consumers, stop and document it in the issue.
- Prefer deepening modules (moving complexity behind simple interfaces) over flattening them.

## Phase 3: Compatibility

Verify that the refactored code is a drop-in replacement.

1. Run the full test suite.
2. Verify all public API signatures are unchanged (or intentionally simplified).
3. If the refactoring changed module boundaries (e.g., split a package, moved a file):
   - Verify all imports compile.
   - Verify no circular dependencies were introduced.
4. If the refactoring introduced new abstractions (interfaces, types):
   - Verify they have test coverage.
   - Verify existing tests exercise the new abstractions through the same public interface.

## Phase 4: Regression Validation

Final proof that nothing broke.

1. Run the full test suite.
2. Run the build to verify everything compiles.
3. Compare test count before and after -- tests should not have been deleted (unless they were testing removed implementation details).
4. Review the diff: every change should be structural (moves, renames, extractions, simplifications). No behavior changes should be visible in the diff.
5. Commit with a message that describes what was restructured and why, explicitly noting that behavior is preserved.

## Scope Management

- **Small refactor (1-3 files, no interface changes):** Proceed directly. Characterization tests may not be needed if existing coverage is good.
- **Medium refactor (4-10 files, or interface simplification):** After Phase 1, post the behavioral invariants and planned transformation sequence as a comment on the issue. Proceed after confirmation.
- **Large refactor (>10 files, module boundary changes, or new abstractions):** After Phase 1, post a detailed plan including the characterization test coverage additions. Wait for approval.

## Completion Criteria

- [ ] All behavioral invariants are preserved (tests prove this).
- [ ] No behavior changes in the diff -- only structural improvements.
- [ ] The test suite passes with no new failures.
- [ ] The build completes successfully.
- [ ] Public API contracts are unchanged (or intentionally simplified with all callers updated).
- [ ] The commit message describes what was restructured and confirms behavior preservation.
