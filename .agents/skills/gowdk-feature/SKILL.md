---
name: gowdk-feature
description: Plan and build a new GOWDK feature or substantial capability, from spec to verified vertical slice. Use for new user-facing behavior, new compiler/runtime capabilities, or large changes that need a feature spec and implementation plan.
---

# GOWDK Feature

Prefer one working vertical slice over broad scaffolding.

## Baselines

- Status source of truth: `docs/product/roadmap.md` (capability matrix with
  Implemented/Partial/Planned per surface). A feature is not "done" until its
  roadmap row is accurate.
- Standing architectural decisions you must not silently violate
  (`docs/engineering/decisions/`): 0002 compile-first render model (build-time
  default, request-time opt-in), 0003 JS default / WASM islands opt-in,
  0005 generated-Go emission boundary, 0006 compiler/runtime boundary,
  0007 static-first SPA navigation, 0008 bounded client language,
  0009 optional inline Go authoring.
- A user-facing feature usually cuts through the full pipeline: grammar
  (`docs/language/grammar.md`, `internal/parser`) → IR
  (`gwdkir.Program`, lowering in `internal/gwdkanalysis`) → validation +
  diagnostics registry (`internal/compiler`,
  `internal/diagnostics/registry.go`) → generators
  (`internal/buildgen`/`internal/appgen`) → runtime (`runtime/`, `addons/`) →
  editor (`internal/lsp`, `editors/vscode`) → docs + `examples/`.
- Specs and plans are kept artifacts: create them from
  `.agents/templates/feature-spec.md` and
  `.agents/templates/implementation-plan.md` under `docs/`.
- Dependency policy: `docs/engineering/dependency-policy.md` — no new
  production dependency without a documented reason or an ADR.

## Core Workflow

1. Read `docs/product/vision.md`, `docs/product/requirements.md`,
   `docs/product/roadmap.md`, and `docs/engineering/architecture.md`. Check
   the ADR list above for prior decisions touching the feature.
2. Identify the user, goal, constraints, and non-goals.
3. Write or update a feature spec with acceptance criteria that can be verified
   by a concrete command or test.
4. Write a short implementation plan: risks, tests, data/API impact, migration,
   rollback. List which pipeline stages from the baseline the slice touches.
5. Implement the smallest coherent slice that reaches a real boundary — a
   buildable `examples/` app or a passing CLI smoke beats stubs across layers.
6. Add an example under `examples/` when the feature has user-facing syntax;
   CI runs `gowdk check/build` over examples, so the example is also a gate.
7. Update the spec, roadmap row, and docs when implementation reality changes.
8. Verify and record commands and outcomes:

```bash
go test ./...
go build ./cmd/gowdk
scripts/test-go-modules.sh
go run ./cmd/gowdk build --out /tmp/gowdk-feature <your example>.gwdk
```

## Lane Handoffs

- Syntax/semantics work inside the feature: `gowdk-language-change`.
- Generated artifacts or runtime contracts: `gowdk-generated-output`.
- Hard-to-reverse architectural decisions: add an ADR from
  `.agents/templates/adr.md` under `docs/engineering/decisions/` (next number
  after the highest existing one).

## Report

State what slice works end to end, what is intentionally deferred, which
roadmap row changed, and which commands prove the acceptance criteria.
