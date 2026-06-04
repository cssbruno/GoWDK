# Refactor Workflow

Use this workflow when simplifying or reorganizing code without intended behavior changes.

## Guardrails

- Establish current behavior first through tests, snapshots, docs, or focused code reading.
- Keep public interfaces stable unless the task explicitly includes changing them.
- Prefer deleting duplication and indirection over adding new abstraction.
- Move in small steps that can each be reviewed.

## Verification

- Run the same tests before and after when possible.
- Add characterization tests before touching fragile behavior.
- Document any behavior that was ambiguous and how it was preserved.
