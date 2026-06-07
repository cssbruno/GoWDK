# AGENTS.md

Scope: entire repository.

## Repository Role

This repository is GOWDK, a portable Go web compiler. WDK has no canonical
expansion; no one knows what it stands for, and the practical product shorthand
is that GOWDK ships apps. The product direction is a Go-first component/page
compiler plus app/runtime kit: full pages default to build-time output, backend
endpoints are core request-time behavior, and `@render ssr` is an integrated
non-default request-time page lane.

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

- Full pages default to build-time SPA output.
- SSR is an integrated non-default request-time page-rendering lane selected
  with `@render ssr`.
- SPA and action pages can use backend endpoints without full-page SSR.
- `paths {}` runs at build time and declares dynamic SPA routes.
- `build {}` runs at build time.
- `load {}` runs at request time and requires request-time rendering.
- `act Name POST "/path"` declares POST/action endpoints.
- `api Name METHOD "/path"` declares API endpoints.
- `view {}` renders markup.
- Dynamic SPA routes require `paths {}` unless switched to SSR; action endpoints
  inherit generated concrete page paths.
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
- Documentation should be concise, direct, and functional. Prefer short
  sections, clear bullets, and concrete commands over narrative explanation.
- State only what a reader needs to know: what works, what is partial, what is
  missing, what is intentionally out of scope, and what command or file to use.
- Put practical examples before explanation. Remove filler, marketing tone,
  repeated background, and framework comparisons unless they prevent a real
  implementation or usage mistake.
- Do not duplicate long status descriptions across docs. Link to the source of
  truth when a short reference is enough.

## Implementation Direction

The next implementation steps should follow `docs/product/roadmap.md`:

1. GOWDK AST and analyzer.
2. Stable internal IR for templates, client behavior, routes, assets,
   endpoints, SSR pages, and generated output.
3. Unified endpoint metadata and generated adapter IR.
4. CSRF-wired actions, fragments, guards, and production-safe backend docs.
5. Request-time page rendering with `load {}`, guards, typed route params, and
   error handling.
6. Hybrid/cache policy, static-first SPA navigation, richer components, islands,
   tooling, and documentation sync.
