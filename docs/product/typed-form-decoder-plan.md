# Implementation Plan: Typed Form Decoder Slice

## Context

Relevant spec: `docs/product/typed-form-decoder-spec.md`.

Roadmap phase: typed actions and forms.

## Assumptions

- User Go type resolution is not available yet, so generated input types hold
  decoded `formValues` rather than field-specific struct members.
- Same-page `g:post` forms are the source of truth for first-slice expected
  field names.
- Unknown submitted fields should fail closed to avoid mass assignment.
- Missing fields remain valid until validation contracts are implemented.

## Proposed Changes

- Extend `runtime/form` with schema-based allowlist decoding helpers.
- Add view-level action form field discovery for direct HTML controls in
  `g:post` forms.
- Extend `appgen.ActionRoute` with inferred input fields.
- Generate named input types and per-action decoder functions in generated app
  source.
- Update action docs, generated-output docs, requirements, architecture, and
  the missing implementation checklist.

## Files Expected To Change

- `runtime/form/form.go`
- `runtime/form/form_test.go`
- `internal/view/view.go`
- `internal/view/view_test.go`
- `internal/appgen/appgen.go`
- `internal/appgen/appgen_test.go`
- `cmd/gowdk/main_test.go`
- `README.md`
- `docs/language/actions.md`
- `docs/compiler/generated-output.md`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`
- `docs/product/missing-implementation-checklist.md`

## Data And API Impact

- `appgen.ActionRoute` gains `InputFields []string`.
- `runtime/form` gains exported decoder helper types/functions.
- Generated app source remains dependency-free.

## Tests

- Unit: runtime form decoder helper behavior.
- Unit: view action-form field inference.
- Unit: generated source includes decoder types/functions.
- Integration: generated binary redirects for expected fields and returns 400
  for unexpected fields.
- End-to-end: CLI `build --app --bin` action smoke still redirects.

## Verification Commands

```sh
go test ./runtime/form ./internal/view ./internal/appgen ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove runtime decoder helpers, action field inference, generated decoder
  output, and docs/checklist updates.
- Keep first action redirect and `g:post` lowering unchanged.

## Risks

- The generated type name can imply full user type resolution; docs must state
  this slice only creates a named input wrapper with decoded values.
- Component-hidden fields are not inferred yet, so docs and tests must keep that
  limitation visible.
