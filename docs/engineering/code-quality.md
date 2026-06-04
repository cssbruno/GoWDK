# Code Quality

## Purpose

This document defines the engineering quality rules for GOWDK code. Naming and
product-name rules live in `docs/engineering/naming-conventions.md`.

## Package Boundaries

- Keep root `package gowdk` as a public API surface only. Do not put compiler,
  runtime, addon, CLI, or generated-app implementation in the repository root.
- Keep compiler internals under `internal/`.
- Keep generated app runtime contracts under `runtime/`.
- Keep optional capabilities under `addons/`.
- Keep validation, parsing, manifest construction, static generation, runtime
  contracts, and optional addons in separate packages.
- Do not let `runtime/render` depend on `addons/ssr`; SSR depends on render
  core, not the other way around.

## Implementation Rules

- Prefer clear names over comments that restate the code.
- Keep modules focused on one responsibility.
- Keep public contracts documented.
- Avoid speculative abstraction.
- Prefer direct code over factories, registries, or package-level indirection
  until the extra layer is required by real behavior.
- Do not create catch-all `utils`, `common`, or `shared` packages. Put helpers
  near their domain first; extract only when reuse is stable.
- Return errors with enough context to diagnose the failing file, route, page,
  module, or output path.
- Add succinct comments only where they explain non-obvious behavior or preserve
  a contract that future maintainers might accidentally break.
- Use `gofmt` for all Go changes.

## Tests

- Keep tests close to the package they validate.
- Tests for the root public API live outside the root package so `gowdk.go`
  remains the only root Go file.
- Add or update tests when behavior, public contracts, generated output, parser
  rules, diagnostics, or route behavior changes.
- Prefer focused tests that prove the changed behavior over broad fixture churn.

## Dependencies

- Add production dependencies only when they remove meaningful implementation or
  maintenance risk.
- Document major dependency choices in an ADR.
- Follow `docs/engineering/dependency-policy.md` for dependency review details.
