# Agent Instructions

# Documentation

Documentation for the project is found in the docs folder. This is considered
the source of truth for what the project is supposed to do. Some features may
not be implemented yet. Care should be taken to implement features with the
overall plan in mind.

### Local binary paths

- `bd`: `/Users/quincy/.local/bin/bd`

### Issue tracker

Beads issue tracker is used via `bd` CLI. See `docs/agents/issue-tracker.md`.

When committing, include `.beads/issues.jsonl` and `.beads/interactions.jsonl` if they are modified. These are passive exports that should stay in sync with the commit they were generated from.

### Triage labels

Standard canonical roles are used. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout with root `CONTEXT.md`. See `docs/agents/domain.md`.

### Test-Driven Development (TDD)

All code modifications must use the `tdd` skill (Test-Driven Development with the red-green-refactor loop) to verify
implementation correctness and ensure core logic is thoroughly tested.


