# Guardrails

## Core Signs

### Sign: Read Before Writing
- **Trigger**: Before modifying any file
- **Instruction**: Read and understand existing behavior first
- **Added after**: Baseline workflow requirement

### Sign: Scope Discipline
- **Trigger**: During implementation of a selected story
- **Instruction**: Implement only selected story scope; do not drift
- **Added after**: Repeated multi-story drift during loops

### Sign: Test Before Completion
- **Trigger**: Before marking story complete
- **Instruction**: Execute all quality-gate commands and verify pass
- **Added after**: Validation regressions in autonomous runs
