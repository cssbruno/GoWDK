# Implementation Plan: Action Form Directive

## Context

Relevant spec: `docs/product/action-form-directive-spec.md`.

Roadmap phase: typed actions now, partial/server fragments later.

## Assumptions

- `g:post` can lower to normal HTML form attributes for the first action slice.
- Only actions with generated redirect metadata are available to `g:post` today.

## Proposed Changes

- Extend the static view parser to parse directive attributes with `{name}`.
- Extend the view renderer with page-scoped action routes.
- Lower `g:post={name}` to `method="post"` and `action="<route>"`.
- Pass page action route context from `internal/staticgen`.
- Update the buildable static action example and docs.

## Files Expected To Change

- `internal/view/view.go`
- `internal/view/view_test.go`
- `internal/staticgen/staticgen.go`
- `internal/staticgen/staticgen_test.go`
- `examples/basic/signup.page.gwdk`
- `docs/language/actions.md`
- `docs/language/markup.md`
- `docs/product/missing-implementation-checklist.md`

## Data And API Impact

- Adds `view.Options` and `view.RenderWithOptions`.
- Existing renderer entrypoints remain compatible.

## Tests

- Unit: directive parse/render success.
- Unit: directive validation failures.
- Integration: static build emits lowered form.
- Integration: static build fails before output for unknown action.

## Verification Commands

```sh
go test ./internal/view ./internal/staticgen ./cmd/gowdk
go test ./...
```

## Rollback Plan

- Remove `view.Options`, directive parsing/lowering, tests, docs, and the
  `g:post` usage from the signup example.

## Risks

- This is not yet a partial-update directive; docs must keep `g:target` and
  `g:swap` planned.
