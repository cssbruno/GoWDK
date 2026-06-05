# Documentation Checklist

This checklist tracks the documentation baseline for the current compiler
reality. Keep it synchronized with `README.md`, `docs/product/requirements.md`,
and `docs/engineering/architecture.md`.

- [x] Rewrite getting started around the current reality: clone, build, run.
  See `docs/getting-started.md`.
- [x] Write real CLI docs with commands that work today.
  See `docs/reference/cli.md`.
- [x] Write config docs with real examples.
  See `docs/reference/config.md`.
- [x] Write architecture docs that explain the compiler pipeline.
  See `docs/engineering/architecture.md` and `docs/compiler/pipeline.md`.
- [x] Write client language docs with supported and unsupported syntax.
  See `docs/language/syntax.md`, `docs/language/markup.md`, and
  `docs/compiler/browser-compiler.md`.
- [x] Write component docs with state, props, slots, stores, and client
  behavior. See `docs/language/components.md`.
- [x] Write routing docs.
  See `docs/reference/routing.md`.
- [x] Write actions/forms docs.
  See `docs/language/actions.md`.
- [x] Write API docs.
  See `docs/language/api.md`.
- [x] Write deployment docs.
  See `docs/reference/deployment.md`.
- [x] Write browser compiler docs.
  See `docs/compiler/browser-compiler.md`.
- [x] Add examples that match the actual compiler, not planned features.
  See `examples/README.md`.
- [x] Clearly separate implemented features from planned features.
  See `README.md`, `docs/product/requirements.md`, language docs, and reference
  docs.
- [x] Keep README, requirements, architecture, and this checklist in sync.

## Maintenance Rule

When compiler behavior, CLI flags, generated output, or supported language
syntax changes, update the matching reference page in the same change. If a
planned feature becomes real, move it from "planned" or "not implemented" text
into the implemented/current behavior section and update
`docs/product/requirements.md`.
