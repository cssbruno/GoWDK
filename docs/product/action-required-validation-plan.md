# Implementation Plan: Action Required Validation Slice

## Context

Relevant spec: `docs/product/action-required-validation-spec.md`.

Roadmap phase: typed actions and forms.

## Assumptions

- `valid(input)?` can execute a small generated validation rule set before real
  user validation functions exist.
- HTML `required` is the safest first rule because authors already understand
  it and it does not require type resolution.
- Generated app output must remain dependency-free, so generated source mirrors
  the simple validation result shape rather than importing runtime packages.

## Proposed Changes

- Extend view form-field inference to record whether a direct control is
  required.
- Extend `appgen.ActionRoute` with required input fields.
- Generate validation checks for required fields when `ValidatesInput` is true.
- Return HTTP 422 on validation failures.
- Update action docs, generated-output docs, requirements, architecture, and the
  missing implementation checklist.

## Files Expected To Change

- `internal/view/view.go`
- `internal/view/view_test.go`
- `internal/appgen/appgen.go`
- `internal/appgen/appgen_test.go`
- `cmd/gowdk/main_test.go`
- `docs/language/actions.md`
- `docs/compiler/generated-output.md`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`
- `docs/product/missing-implementation-checklist.md`

## Data And API Impact

- `appgen.ActionRoute` gains `RequiredFields []string`.
- Existing generated action routes remain source-compatible for callers that do
  not use validation metadata.

## Tests

- Unit: view required-field inference.
- Unit: generated app source includes required validation calls.
- Integration: generated binary returns 422 for missing required fields.
- End-to-end: CLI `build --app --bin` action smoke still redirects for valid
  submissions.

## Verification Commands

```sh
go test ./internal/view ./internal/appgen ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove required-field metadata, generated validation checks, tests, and docs.
- Keep first-slice form decoder wrappers unchanged.

## Risks

- Authors may expect full Go validation behavior from `valid(input)?`; docs must
  clearly state this slice only enforces static required fields.
