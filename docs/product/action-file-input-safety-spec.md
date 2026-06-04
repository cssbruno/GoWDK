# Feature Spec: Action File Input Safety

## Problem

Generated action forms currently infer direct static controls, but file uploads
have no security contract yet. File uploads need explicit body limits, storage
rules, content handling, validation, and logging guidance before they can be
accepted by generated action handlers.

## Goals

- Reject direct `<input type="file">` controls inside same-page `g:post` forms.
- Reject direct generated action forms with static
  `enctype="multipart/form-data"`.
- Reject dynamic `type` or `enctype` values in generated action form schema
  inference when they could hide unsupported upload behavior.
- Keep regular text-like controls working unchanged.

## Non-Goals

- Implement file uploads.
- Parse component-hidden file inputs.
- Add multipart body limits or storage rules.
- Add upload validation or content scanning.

## Users And Permissions

- Primary users: Go developers building static-first action forms.
- Roles or permissions: local compile and generated app build access.
- Data visibility rules: upload-related diagnostics must not include submitted
  file contents or local file paths.

## User Flow

1. User adds a direct file input to a generated action form.
2. `gowdk build --app` fails before generated handler output.
3. User removes the file input or waits for the future upload contract.

## Requirements

### Functional

- Direct `input type="file"` controls in `g:post` forms fail schema inference.
- Static multipart `enctype` on a `g:post` form fails schema inference.
- Dynamic input `type` and form `enctype` values fail schema inference.
- Non-file direct controls remain accepted.

### Non-Functional

- Performance: checks happen during the existing schema walk.
- Reliability: failures occur before generated app output.
- Accessibility: no generated markup changes.
- Security/privacy: diagnostics name unsupported form features, not submitted
  values.
- Observability: compiler errors point to generated action form constraints.

## Acceptance Criteria

- [x] View tests reject direct file inputs and multipart generated action forms.
- [x] Appgen tests prove file input rejection is reported with page context.
- [x] Docs/checklist describe current upload behavior and limitations.

## Edge Cases

- `type="FILE"` is rejected case-insensitively.
- `type="{kind}"` is rejected because the compiler cannot prove it is not a
  file input.
- File inputs hidden inside component calls remain outside this first-slice
  inference and must be handled by future component-aware schema analysis.

## Dependencies

- Internal: `internal/view`, `internal/appgen`.
- External: none.

## Open Questions

- What upload size limits, temporary storage, and scanning hooks should generated
  actions provide?
