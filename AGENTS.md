# AGENTS.md

Scope: entire repository.

## Repository Role

This repository is GOWDK, a portable Go web compiler. WDK has no canonical
expansion; no one knows what it stands for, and the practical product shorthand
is that GOWDK ships apps. The product direction is GOWDK Compiler plus GOWDK
Runtime: full pages default to build-time output, backend endpoints are core
request-time behavior, and `server {}` / `go server {}` select the integrated
non-default request-time page lane.

Use this file as the always-on instruction file for any coding agent working in
this repository (Codex, Claude Code, hosted agents, IDE assistants).
Task-specific skills and reusable output templates live in `.agents/`.

## Startup Context

Before making non-trivial changes, read these files in order:

1. `README.md`
2. `docs/README.md`
3. `docs/product/vision.md`
4. `docs/product/requirements.md`
5. `docs/product/roadmap.md`
6. `docs/engineering/architecture.md`
7. The relevant skill under `.agents/skills/`

If a file still contains placeholders, treat that as unknown project context.
Make the smallest reasonable assumption, state it, and update the relevant
document when the decision becomes real.

## Working Agreement

- Prefer a working vertical slice over broad scaffolding that is not exercised.
- Keep changes scoped to the requested behavior and update docs/tests when
  behavior or commands change.
- Preserve user changes. Never revert unrelated edits unless explicitly asked.
- Use the repository's existing stack, style, and helpers once they exist.
- Do not add production dependencies without a clear reason documented in the
  change or an ADR.
- Do not commit secrets, credentials, generated private keys, local env files, or
  vendor-specific tokens.
- When a command fails, capture the command, failure, and next step in your
  response or the relevant docs.
- Before handing off, run the most relevant available verification command. If
  no verification exists yet, say so plainly.

## Skills

Recurring task runbooks live in `.agents/skills/<name>/SKILL.md`. Each one
carries the concrete baselines (files, contracts, gates) for its lane—start from
the matching skill instead of improvising or adding one-off plan files:

- `gowdk-feature`: new feature or capability, spec to verified vertical slice.
- `gowdk-bug`: reproduce, diagnose, and fix a defect or regression.
- `gowdk-refactor`: behavior-preserving simplification or reorganization.
- `gowdk-review`: review a diff, PR, or subsystem against repo contracts.
- `gowdk-language-change`: public `.gwdk` syntax or semantics changes.
- `gowdk-compiler-internal`: AST/IR/analyzer/diagnostics internals.
- `gowdk-generated-output`: build output, generated Go, runtime contracts.
- `gowdk-docs`: documentation across all doc lanes.
- `gowdk-pr-body`: pull request titles and bodies.
- `gowdk-version-bump`: release version bumps across all surfaces.

## Planning Standard

For new features or large changes:

1. Write or update a feature spec using `.agents/templates/feature-spec.md`.
2. Write a short implementation plan using
   `.agents/templates/implementation-plan.md`.
3. Identify risks, tests, migration needs, and rollback strategy before editing
   core code.
4. Implement in small, reviewable increments.
5. Update the spec when implementation reality changes.

For architectural decisions that are hard to reverse, add an ADR under
`docs/engineering/decisions/` using `.agents/templates/adr.md`.

## Engineering Principles

- Design around clear domain boundaries. Avoid catch-all `utils`, `common`, or
  `shared` modules unless the reuse is real and stable.
- Keep data flow easy to trace. Prefer direct code over factories, registries,
  and indirection until complexity proves it is needed.
- Separate product requirements, domain logic, integration boundaries, and
  infrastructure concerns.
- Make failure modes explicit: validation, auth, retries, timeouts, persistence
  errors, and partial failures should be visible in code.
- Favor boring, observable, testable systems over clever abstractions.
- Keep public APIs and persisted data contracts documented.

## GOWDK Product Rules

- Full pages default to build-time SPA output.
- SSR is an integrated non-default request-time page-rendering lane selected
  with `server {}` or `go server {}`.
- SPA and action pages can use backend endpoints without full-page SSR.
- `paths {}` runs at build time and declares dynamic SPA routes.
- `build {}` runs at build time.
- `server {}` runs at request time, selects request-time rendering, and requires
  the SSR addon.
- `act Name POST "/path"` declares POST/action endpoints.
- `api Name METHOD "/path"` declares API endpoints.
- `view {}` renders markup.
- Dynamic SPA routes require `paths {}` unless switched to request-time SSR;
  action endpoints inherit generated concrete page paths.
- Partial updates use server fragments, not full-page SSR.
- Single-binary deploy must work with or without SSR.

## Quality Gates

Keep these commands current:

- Run all tests: `scripts/test-go-modules.sh`
- Run root module tests: `go test ./...`
- Build CLI: `go build ./cmd/gowdk`
- Format changed Go files: `gofmt -w <files>`
- Check documentation links: `scripts/check-docs-links.sh`
- Check documentation style: `scripts/check-docs-style.sh`
- Check removed syntax: `scripts/check-removed-syntax.sh`
- Check evergreen versions: `scripts/check-doc-versions.sh`

If a future package adds a more specific validation command, document it in
`README.md` and run it for relevant changes.

## Agent Rules

- Treat `AGENTS.md` as the always-on project instruction file for every coding
  agent.
- Keep this file small by moving long process details into `.agents/skills/` and
  `.agents/templates/`.
- If more specific rules are needed for a future subdirectory, add a nested
  `AGENTS.md` close to that code and keep it short.
- When an agent discovers missing project facts, update the relevant docs or the
  matching skill's baselines instead of relying on chat history.

## Documentation Rules

- Start at `docs/README.md` and write in the owning documentation lane.
- Product direction lives in `docs/product/vision.md`.
- Capability status lives in `docs/product/requirements.md`.
- Dependency-aware sequencing lives in `docs/product/roadmap.md`.
- Accepted `.gwdk` syntax is pinned by
  `docs/language/conformance.md`; language topic pages explain it.
- Commands, config, runtime contracts, metadata, and integrations live in
  `docs/reference/`.
- Compiler handoffs and generated-output contracts live in `docs/compiler/`.
- Engineering decisions, system design, security, operations, and implementation
  plans live in `docs/engineering/`.
- Reusable agent task skills live in `.agents/skills/`.
- Reusable agent output templates live in `.agents/templates/`.
- Update documentation in the same change that makes it stale.
- Documentation should be concise, direct, and functional. Prefer short
  sections, clear bullets, and concrete commands over narrative explanation.
- Put practical examples before explanation. Remove filler, marketing tone,
  repeated background, and framework comparisons unless they prevent a real
  implementation or usage mistake.
- Link to the owning source instead of duplicating long status descriptions.
- Do not use an issue, implementation plan, ADR, version number, or document
  count as the only statement of current status.

## Implementation Direction

Use `docs/product/requirements.md` for current capability status and
`docs/product/roadmap.md` for dependency-aware sequencing. Do not copy the
roadmap step list into agent instructions; completed slices and priorities
change more frequently than this file.
