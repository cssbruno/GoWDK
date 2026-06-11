# ADR 0001: LLM-Ready Project Structure

Date: 2026-06-04

Status: Accepted (amended 2026-06-11: `.llm/workflows/` was replaced by skills
in `.agents/skills/`, and `.llm/templates/` moved to `.agents/templates/`;
`AGENTS.md` is now the instruction file for any coding agent, not only Codex)

## Context

The repository started empty and needed a structure that helps LLM-assisted coding sessions and humans build a larger project without losing product intent, architecture decisions, or verification habits.

## Decision

Use `AGENTS.md` as the source of truth for Codex instructions. Keep reusable, tool-neutral LLM workflows in `.llm/workflows/`, templates in `.llm/templates/`, product docs in `docs/product/`, and engineering docs in `docs/engineering/`.

**Amendment (2026-06-11):** `AGENTS.md` is the always-on instruction file for any coding agent, not only Codex. The `.llm/` directory was retired: recurring task workflows became skills under `.agents/skills/`, and reusable templates moved to `.agents/templates/`. Product and engineering doc locations are unchanged.

The product-specific implementation is documented separately in ADR 0002.

## Consequences

### Positive

- Future LLM-assisted coding sessions have consistent startup context.
- Planning, implementation, review, and documentation have defined homes.
- Product and engineering decisions have stable locations as GOWDK grows.

### Negative

- The repository contains process documentation that must stay in sync with implementation reality.
- Tool-neutral workflows must avoid depending on any single coding assistant's private behavior.

### Neutral

- Agent instructions stay centralized in `AGENTS.md`, while reusable skills and templates remain tool-neutral (under `.agents/` since the 2026-06-11 amendment).

## Alternatives Considered

- Add adapters for other coding assistants. Rejected for now because reusable templates are tool-neutral and do not require separate adapter files.
- Store all guidance in `AGENTS.md`. Rejected because long reusable workflows and templates should not consume Codex project-doc budget on every task.

## Follow-Up

- Keep `README.md`, `AGENTS.md`, and `docs/engineering/architecture.md` aligned when commands, package boundaries, or render rules change.
- Add nested `AGENTS.md` files only when a future subtree needs more specific agent instructions.
