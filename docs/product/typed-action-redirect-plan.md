# Implementation Plan: Typed Action Redirect Slice

## Context

Relevant spec: `docs/product/typed-action-redirect-spec.md`.

Roadmap phase: typed actions and forms.

## Assumptions

- The first executable action can redirect without calling user logic.
- Form parsing can happen before redirect to establish the generated handler
  shape without exposing submitted values.
- Action pages in this slice use non-dynamic routes.

## Proposed Changes

- Extend `manifest.Action` with body, input variable/type, validation flag, and
  redirect target.
- Update `internal/parser` to capture and parse `act name {}` bodies.
- Update action docs and current limitations.
- Extend `internal/appgen` so generated apps can include POST redirect handlers
  for supported action metadata.
- Wire `cmd/gowdk build --app` to pass parsed manifest data to app generation.
- Add tests for parser diagnostics, generated action route redirects, and unsafe
  redirect validation.

## Files Expected To Change

- `internal/manifest/manifest.go`
- `internal/parser/page.go`
- `internal/parser/page_test.go`
- `internal/appgen/appgen.go`
- `internal/appgen/appgen_test.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `docs/language/actions.md`
- `docs/product/missing-implementation-checklist.md`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`
- `docs/reference/cli.md`
- `docs/compiler/generated-output.md`

## Data And API Impact

- Manifest `Action` gains new exported fields.
- `appgen.GenerateWithOptions` accepts action routes for generated app output.
- Existing `appgen.Generate` remains as a static-only compatibility wrapper.

## Tests

- Unit: parser accepts supported action body lines.
- Unit: parser rejects unknown action body lines and unsafe redirects.
- Unit: generated app source includes action routes.
- Integration: generated binary redirects on `POST /route`.

## Verification Commands

```sh
go test ./internal/parser ./internal/appgen ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove action body fields, parser support, generated action route output, and
  related docs/tests.
- Keep static app generation and static route serving unchanged.

## Risks

- The term "typed action" can imply complete user Go type binding; docs must
  state this is only metadata and redirect handler scaffolding.
- Redirect behavior without CSRF is not production complete; docs must keep CSRF
  marked as planned.
