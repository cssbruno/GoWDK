# Implementation Plan: Multipart Action Forms

## Context

Relevant spec: [Multipart Action Forms](../product/multipart-action-forms.md)
and GitHub issue #505.

## Assumptions

- Direct file controls use view-owned policy attributes:
  `g:max-file-size`, `g:max-files`, and MIME `accept`.
- The existing `Build.BodyLimits.ActionBytes` setting remains the outer request
  cap.
- File persistence stays outside generated code.

## Proposed Changes

- Extend `runtime/form` with multipart `Data`, `File`, `FilePolicy`, and decode
  helpers.
- Let action form schema extraction accept multipart forms and record file
  policy metadata.
- Teach Go input metadata about `form.File` and `[]form.File`.
- Generate multipart parsing and typed upload decoding only for actions whose
  schema includes files.
- Add runtime app helpers for low-level multipart handlers.
- Update docs, tests, and one native example.

## Files Expected To Change

- `runtime/form/*`
- `runtime/app/backend.go`
- `runtime/actions/form_decode.go`
- `internal/viewparse/directives.go`
- `internal/viewrender/*`
- `internal/source/*`
- `internal/compiler/*`
- `internal/appgen/*`
- `docs/language/*`, `docs/compiler/generated-output.md`,
  `docs/product/requirements.md`
- `examples/endpoints/*`

## Data And API Impact

- New public runtime types: `form.File`, `form.FilePolicy`, `form.Data`.
- New accepted action input field types: `form.File` and `[]form.File`.
- New view directives: `g:max-file-size` and `g:max-files`.
- No storage API or generated persistence contract is added.

## Tests

- Unit: runtime multipart policy, view schema policy, compiler input type
  discovery.
- Integration: appgen generated source for multipart parsing and file decoders.
- End-to-end: generated binary action upload success and rejected upload cases.
- Manual: run endpoints example if time permits.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./runtime/form ./runtime/app ./runtime/actions ./internal/viewrender ./internal/source ./internal/compiler ./internal/appgen
go test ./...
go build ./cmd/gowdk
scripts/test-go-modules.sh
```

## Rollback Plan

- Revert runtime upload types, file-control schema extraction, compiler type
  additions, generated multipart parsing, docs, and example changes together.
- Existing URL-encoded action forms continue to use the old `form.Values` path.

## Risks

- Multipart parsing order must preserve CSRF validation for hidden form tokens.
- Typed file support must not change contract/query schemas that reuse scalar
  input metadata.
- Request and per-file limits must reject before user handlers see invalid
  uploads.
