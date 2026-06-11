---
name: gowdk-docs
description: Write or update GOWDK documentation. Use for README, docs/product, docs/engineering, docs/language, docs/compiler, docs/reference, getting-started, examples prose, or fixing stale/duplicated documentation.
---

# GOWDK Docs

Concise, direct, functional. Examples before explanation.

## Baselines

Each doc lane has one owner — write in the right one and link from the others:

- `docs/product/` — vision, requirements, `roadmap.md` (THE status source of
  truth: capability matrix with Implemented/Partial/Planned), language-server.
- `docs/engineering/` — architecture, conventions, naming-conventions,
  code-quality, testing, ci, release, security + `decisions/` (9 ADRs,
  numbered; new ones from `.agents/templates/adr.md`).
- `docs/language/` — the language contract (`grammar.md` is canonical syntax;
  per-topic files: blocks, markup, guards, layouts, actions, api, forms, ...).
- `docs/compiler/` — pipeline, generated-output, manifest, build-report,
  project-structure contracts.
- `docs/reference/` — `cli.md` (all commands/flags), `config.md`,
  `diagnostic-codes.md` (must mirror `internal/diagnostics/registry.go`),
  routing, addons, deployment, hooks.
- Onboarding: `README.md`, `docs/getting-started.md` — both pin the current
  release (`v0.2.8` install snippets); version changes go through
  `gowdk-version-bump`, not ad-hoc edits.
- `CHANGELOG.md` — canonical change log (`## v0.x.y - date` with
  `### Changed` / `### Implemented` / `### Known Gaps`).

## Core Workflow

1. Identify the owning lane above and check whether the fact already lives
   somewhere else — link to the source of truth instead of duplicating.
2. State only what a reader needs: what works, what is partial, what is
   missing, what is out of scope, and which command or file to use.
3. Put practical examples before explanation; prefer a runnable command or a
   real `examples/` file over prose.
4. Verify every command, flag, path, and diagnostic code you write actually
   exists:

```bash
go run ./cmd/gowdk --help
go run ./cmd/gowdk explain <diagnostic-code>
ls <every path you reference>
```

## Guardrails

- Remove filler, marketing tone, repeated background, and framework
  comparisons unless they prevent a real usage mistake.
- Do not document aspirational behavior as current; mark it Planned with a
  roadmap reference.
- Docs reflecting a behavior change ship in the same PR as that change; if the
  behavior is not merged yet, do not document it.
- Do not imply production readiness anywhere — this is an experimental 0.x.

## Report

List the docs touched, the commands/paths verified, and any stale content
found but intentionally left for a follow-up.
