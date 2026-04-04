---
name: chore
description: Structured workflow for maintenance and housekeeping tasks in brownfield codebases. Dependency updates, configuration changes, CI adjustments, and other non-behavioral changes with regression safety via TDD.
---

# Chore / Maintenance Workflow

You are performing a maintenance task in an existing codebase. Chores do not change user-facing behavior -- they keep the project healthy (dependency updates, configuration changes, CI/CD adjustments, documentation cleanup, tooling upgrades).

**Always use the `$tdd` skill alongside this workflow.** Even though chores are "non-behavioral," they can silently break things. The TDD safety net catches regressions.

## Phase 1: Inventory

Understand what needs to change and what could break.

1. Read the issue description to understand the maintenance goal.
2. Explore the codebase to identify:
   - Which files and configurations are affected.
   - Which existing tests exercise the affected areas.
   - What downstream systems depend on the things being changed (build scripts, CI pipelines, deployment configs).
3. List the concrete changes needed. Be explicit -- "update package X from v1.2 to v1.3" not "update dependencies."
4. Identify risk areas: changes that could cause build failures, test breakage, or deployment issues.

## Phase 2: Baseline

Establish that the current state is healthy before making changes.

1. Run the existing test suite. All tests must pass before you begin.
2. If the chore involves dependency changes, verify the current lockfile is consistent.
3. If the chore involves configuration changes, document the current configuration values you are changing.

**Do not proceed if the baseline is broken.** Report the pre-existing failure in the issue and wait for guidance.

## Phase 3: Execute

Make the changes incrementally, verifying after each step.

1. Make one logical change at a time (e.g., update one dependency, change one config value).
2. After each change, run the test suite.
   - If tests fail, determine whether the failure is expected (e.g., a breaking API change in an updated dependency) or unexpected.
   - For expected failures: fix the consuming code, add a test for the new behavior if needed (RED -> GREEN).
   - For unexpected failures: revert and investigate.
3. If the chore involves build or CI changes, verify the build completes successfully.

**Rules:**
- One logical change per commit when possible.
- Do not bundle behavior changes with maintenance changes.
- If an updated dependency requires code changes, those code changes are part of this chore -- but test them properly.

## Phase 4: Validate

Prove the maintenance work is complete and nothing is broken.

1. Run the full test suite.
2. Run any build commands to verify the project compiles/bundles correctly.
3. If the chore involved configuration changes, verify the new values are active and correct.
4. If the chore involved dependency updates, verify the lockfile is committed and consistent.
5. Commit with a clear message describing what was maintained and why.

## Scope Management

- **Simple chore (1-3 files, no risk of behavioral change):** Proceed directly through all phases.
- **Complex chore (multiple dependencies, tooling migration, or CI overhaul):** After Phase 1, post the inventory and risk assessment as a comment on the issue. Wait for approval before executing.

## Completion Criteria

- [ ] All changes are non-behavioral (no user-facing feature changes).
- [ ] The test suite passes with no new failures.
- [ ] The build completes successfully.
- [ ] Configuration changes are documented in the commit message.
- [ ] Dependency lockfiles are consistent and committed.
