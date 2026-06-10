# Implementation Plan: Guard Default-Deny (warn + 403)

## Context

Decide what happens when a page declares **no** `@guard`. Today that is a hard
compile error (`missing_page_guard`,
[validate_page.go:223](../../internal/compiler/validate_page.go#L223)).

Target model — **secure-by-default**:

- **No `@guard` → the page is denied (403) at request time**, never public.
- **Compile time emits a warning, not an error**: "this page has no @guard; it
  will return 403 until you declare one." The build still succeeds.
- **To serve a page publicly you must explicitly write `@guard public`.**
- Protective guards (`auth.required`, `role:`, `permission:`) behave as today.

The point: silence can never produce a public page. You cannot *accidentally*
ship public — absence means locked. `public` is only ever reached by typing it.

This supersedes `.llm/plans/optional-page-guards.md` (delete that) and reverses
the "hard error" choice from `.llm/plans/thin-native-rbac.md` line 10.

## Why this shape

- A forgotten guard today is a build failure (annoying) — and the prior idea of
  making it *public* by default was unsafe. Default-deny is the safe middle:
  the build proceeds, the page is locked, and a warning tells you to decide.
- Denial needs no rendering, so it works even for static/build-time pages: the
  GET route returns 403 before serving HTML. This sidesteps the
  `guard_requires_request_render` constraint (that rule is about *rendering*
  protected content per-principal; a flat 403 renders nothing).

## Two-part change

### Part A — Warning severity in the compiler diagnostics layer (prerequisite)

`internal/compiler.ValidationError` has no severity; everything it emits is
fatal. The `internal/lang.Diagnostic` layer already has
`Severity string` + `HasErrors()` + an LSP mapping
([diagnostics.go:12](../../internal/lsp/diagnostics.go#L12)) and a warning
precedent ([accessibility.go:52](../../internal/lang/accessibility.go#L52)).

- Add `Severity` to `compiler.ValidationError`
  ([validate.go:11](../../internal/compiler/validate.go#L11)) — a small enum
  (`SeverityError` default, `SeverityWarning`). Default zero-value stays
  `error` so every existing diagnostic is unchanged.
- Teach the build gate to **fail only on errors**. `ValidateProgram` /
  `ValidatePage` consumers in [build.go:102](../../cmd/gowdk/build.go#L102) (and
  `dev_loop.go`, `doctor.go`, `contracts.go`, `route_report.go`) must split
  warnings from errors and only abort on errors.
- Carry severity across the compiler→lang bridge so editors render warnings as
  warnings (yellow), not errors. Map `compiler.SeverityWarning` →
  `lang` severity `"warning"`.

### Part B — Page guard semantics

In `validatePageGuards` ([validate_page.go:219](../../internal/compiler/validate_page.go#L219)):

- Guardless page → emit `missing_page_guard` as a **warning** (not error), with
  reworded message: "%s declares no @guard; its route will return 403 at request
  time. Add @guard public to serve it, or a protective guard such as
  @guard auth.required."
- `@guard public` alone → served (unchanged).
- `public` + protective → `public_guard_exclusive` (unchanged, error).
- Protective guard on build-time route → `guard_requires_request_render`
  (unchanged, error).

In codegen (`internal/appgen/source_guards.go` + the page GET route emitter):

- A page with **zero guards** must emit a GET route that **returns 403**
  (deny) instead of serving. This is the runtime fail-safe.
- A page with `@guard public` serves normally (today's no-guard behavior).
- Confirm the route handler can short-circuit to 403 for static/SPA pages
  before serving cached HTML.

## Files Expected To Change

- `internal/compiler/validate.go` — add `Severity` to `ValidationError`.
- `internal/compiler/validate_page.go` — `missing_page_guard` → warning + new
  message + still suppress when not source-backed.
- `cmd/gowdk/build.go`, `dev_loop.go`, `doctor.go`, `contracts.go`,
  `route_report.go` — gate build/exit on errors only; print warnings.
- compiler→`lang` diagnostic bridge (in `internal/lang/tools.go` /
  wherever compiler diagnostics are surfaced) — propagate severity.
- `internal/lsp/diagnostics.go` — already maps `"warning"`; just ensure the
  guard warning reaches it with the right severity.
- `internal/appgen/source_guards.go` + page route emitter — 403 default for
  guardless pages; `@guard public` serves.
- `internal/diagnostics/registry.go` — `missing_page_guard` is
  `StabilityStable`; keep the code but note it is now a **warning**, and that
  the runtime default is 403. (Severity change to a stable code should be called
  out in CHANGELOG.)
- `internal/diagnostics/explain.go` — rewrite `missing_page_guard` details:
  warning, 403 default, opt-in to public.
- `cmd/gowdk/init.go` — the scaffold's `@guard public`
  ([init.go:74](../../cmd/gowdk/init.go#L74),
  [init.go:152](../../cmd/gowdk/init.go#L152)) stays: the home page is
  genuinely, intentionally public, and under this model that line is now an
  honest explicit choice rather than a silent default.
- Docs sweep: `docs/language/{semantics,syntax,diagnostics}.md`,
  `docs/reference/{diagnostic-codes,routing}.md`,
  `docs/engineering/security.md`, `README.md`, `CHANGELOG.md`.

## Tests

- Unit (validate): guardless page → exactly one diagnostic, severity warning,
  code `missing_page_guard`, build not aborted. `@guard public` → no
  diagnostics. `public` + protective → `public_guard_exclusive` (error).
- Unit (severity plumbing): build proceeds with warnings present; fails with
  errors present; mixed → fails, both reported.
- Codegen/generated-app: guardless page GET → 403; `@guard public` GET → 200.
  Build-time/static guardless page also 403s at the route.
- Update existing `validate_test.go` (the missing-guard test at ~line 650 now
  asserts a **warning**, not just presence) and any golden diagnostics fixtures.

## Verification Commands

```sh
go test ./internal/compiler ./internal/diagnostics ./internal/lang ./internal/appgen ./cmd/gowdk
go test ./...
```

## Open Questions

1. **Severity representation in `compiler.ValidationError`** — typed enum vs a
   `set of warning-level codes` lookup. Recommend a `Severity` field (explicit,
   per-diagnostic) since other codes will want warnings later.
2. **Static/CDN export with no Go server** — a guardless page 403s via the
   generated server, but a pure static export has no server to enforce it. Same
   caveat the protected-guard rule already carries. Recommend: document it, and
   keep the compile-time warning loud so the author resolves it before export.
3. **Should the 403 default also be a build *error* under a strict config
   flag?** Optional `Guards.RequireExplicit` to upgrade the warning → error for
   teams that want a hard gate. Default off. Can be a follow-up.
