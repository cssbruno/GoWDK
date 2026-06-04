# Implementation Plan: Action File Input Safety

## Context

Relevant spec: `docs/product/action-file-input-safety-spec.md`.

Roadmap phase: typed actions and forms.

## Assumptions

- File uploads should fail closed until upload-specific security rules exist.
- Direct static controls in `g:post` forms are the only scope for this slice.
- Component-hidden controls require future component-aware form schema analysis.

## Proposed Changes

- Extend action form schema inference to reject static file inputs.
- Reject dynamic input `type` values and dynamic/static multipart form
  `enctype` values in `g:post` forms.
- Add view and appgen tests for unsupported upload diagnostics.
- Update action docs, generated-output docs, security docs, and the missing
  implementation checklist.

## Files Expected To Change

- `internal/view/view.go`
- `internal/view/view_test.go`
- `internal/appgen/appgen_test.go`
- `docs/language/actions.md`
- `docs/compiler/generated-output.md`
- `docs/engineering/security.md`
- `docs/product/missing-implementation-checklist.md`

## Data And API Impact

- No public API changes.
- `view.ActionFormSchema` returns new validation errors for unsafe upload
  controls.

## Tests

- Unit: view rejects `input type="file"` in a `g:post` form.
- Unit: view rejects multipart `enctype` in a `g:post` form.
- Unit: appgen reports page context for file input rejection.

## Verification Commands

```sh
go test ./internal/view ./internal/appgen
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove upload-specific schema validation, tests, and docs/checklist updates.

## Risks

- Dynamic input types that are harmless today will be rejected in generated
  action forms. This is intentional until upload behavior is explicit.
