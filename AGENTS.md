# AGENTS.md

Scope: entire repository.

## Repository Role

This repository is GOWDK, a portable Go web compiler. The product direction is compile-first: static/action-first output is core, and request-time full-page SSR is an optional addon.

Use this file as the source of truth for Codex in this repository. Shared planning workflows and output templates live in `.llm/` so they can be reused by any capable coding LLM.

## Startup Context

Before making non-trivial changes, read these files in order:

1. `README.md`
2. `docs/product/vision.md`
3. `docs/product/requirements.md`
4. `docs/product/roadmap.md`
5. `docs/engineering/architecture.md`
6. The relevant workflow under `.llm/workflows/`

If a file still contains placeholders, treat that as unknown project context. Make the smallest reasonable assumption, state it, and update the relevant document when the decision becomes real.

## Working Agreement

- Prefer a working vertical slice over broad scaffolding that is not exercised.
- Keep changes scoped to the requested behavior and update docs/tests when behavior or commands change.
- Preserve user changes. Never revert unrelated edits unless explicitly asked.
- Use the repository's existing stack, style, and helpers once they exist.
- Do not add production dependencies without a clear reason documented in the change or an ADR.
- Do not commit secrets, credentials, generated private keys, local env files, or vendor-specific tokens.
- When a command fails, capture the command, failure, and next step in your response or the relevant docs.
- Before handing off, run the most relevant available verification command. If no verification exists yet, say so plainly.

## Planning Standard

For new features or large changes:

1. Write or update a feature spec using `.llm/templates/feature-spec.md`.
2. Write a short implementation plan using `.llm/templates/implementation-plan.md`.
3. Identify risks, tests, migration needs, and rollback strategy before editing core code.
4. Implement in small, reviewable increments.
5. Update the spec when implementation reality changes.

For architectural decisions that are hard to reverse, add an ADR under `docs/engineering/decisions/` using `.llm/templates/adr.md`.

## Engineering Principles

- Design around clear domain boundaries. Avoid catch-all `utils`, `common`, or `shared` modules unless the reuse is real and stable.
- Keep data flow easy to trace. Prefer direct code over factories, registries, and indirection until complexity proves it is needed.
- Separate product requirements, domain logic, integration boundaries, and infrastructure concerns.
- Make failure modes explicit: validation, auth, retries, timeouts, persistence errors, and partial failures should be visible in code.
- Favor boring, observable, testable systems over clever abstractions.
- Keep public APIs and persisted data contracts documented.

## GOWDK Product Rules

- Core GOWDK renders at build time by default.
- SSR is an addon, not the framework identity.
- Static and action pages can use backend routes without full-page SSR.
- `paths {}` runs at build time and declares dynamic static routes.
- `build {}` runs at build time.
- `load {}` runs at request time and requires request-time rendering.
- `act {}` runs POST/action requests.
- `api {}` runs API requests.
- `view {}` renders markup.
- Dynamic static/action routes require `paths {}` unless switched to SSR.
- Partial updates use server fragments, not full-page SSR.
- Single-binary deploy must work with or without SSR.

## Quality Gates

Keep these commands current:

- Run all tests: `go test ./...`
- Build CLI: `go build ./cmd/gowdk`
- Format changed Go files: `gofmt -w <files>`

If a future package adds a more specific validation command, document it in `README.md` and run it for relevant changes.

## Codex Rules

- Treat `AGENTS.md` as the always-on project instruction file.
- Keep this file under the default Codex project-doc size limit by moving long process details into `.llm/workflows/` and `.llm/templates/`.
- If more specific rules are needed for a future subdirectory, add a nested `AGENTS.md` close to that code and keep it short.
- When Codex discovers missing project facts, update the relevant docs instead of relying on chat history.

## Documentation Rules

- Product intent lives in `docs/product/`.
- Engineering decisions and system design live in `docs/engineering/`.
- Reusable LLM task workflows live in `.llm/workflows/`.
- Reusable LLM output templates live in `.llm/templates/`.
- Update documentation in the same change that makes it stale.

## Implementation Direction

The next implementation steps should follow `docs/product/roadmap.md`:

1. Portable file discovery and manifest generation.
2. Component compiler.
3. Static/prerender output.
4. CSS/plugin extension points.
5. One-binary static server.
6. Typed actions, partial/server fragments, and then SSR addon.
