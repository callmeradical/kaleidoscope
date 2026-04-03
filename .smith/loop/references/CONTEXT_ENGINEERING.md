# Context Engineering Reference

Use this loop as a focused, single-story execution context.

## Core Principles
- One story per loop; do not drift scope.
- Persist learnings in files (progress/errors/guardrails), not chat memory.
- If repeated failures occur, add/refine signs in guardrails.
- Reset with a fresh loop when context is polluted or objectives change.

## Context Health Signals
- Healthy: clear story scope, progress advancing, validation converging.
- Warning: repeated similar errors, no story state change after iterations.
- Critical: circular failures; prefer focused autofix/troubleshooting loop.
