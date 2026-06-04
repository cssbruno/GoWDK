# Feature Workflow

Use this workflow for new user-facing behavior or substantial internal capabilities.

## 1. Understand

- Read `AGENTS.md`, `docs/product/vision.md`, `docs/product/requirements.md`, and `docs/engineering/architecture.md`.
- Identify the user, goal, constraints, and non-goals.
- Check existing decisions in `docs/engineering/decisions/`.

## 2. Specify

- Create or update a feature spec using `.llm/templates/feature-spec.md`.
- Write acceptance criteria that can be manually or automatically verified.
- Identify data, API, UI, security, migration, and operations impacts.

## 3. Plan

- Create a short implementation plan using `.llm/templates/implementation-plan.md`.
- Prefer one vertical slice that reaches a real user or integration boundary.
- List verification commands. If none exist, define what must be added.

## 4. Implement

- Make the smallest coherent change.
- Keep behavior close to tests.
- Update docs when setup, commands, APIs, or architecture change.

## 5. Verify

- Run relevant tests, lint, type checks, and builds.
- Add missing tests for meaningful behavior.
- Record commands and outcomes in the final response or PR description.
