# Implementation Plan: Action Request Body Limit

## Context

Relevant spec: `docs/product/action-body-limit-spec.md`.

Roadmap phase: typed actions and forms / generated server baseline.

## Assumptions

- A fixed 1 MiB cap is reasonable for the current non-upload form subset.
- Future upload support will introduce separate limits and storage rules.
- The generated app should remain dependency-free.

## Proposed Changes

- Add a generated `maxActionBodyBytes` constant.
- Wrap action request bodies with `http.MaxBytesReader` before `ParseForm`.
- Map MaxBytesReader parse failures to HTTP 413.
- Add generated source and binary tests.
- Update generated-output, operations, security, and checklist docs.

## Files Expected To Change

- `internal/appgen/appgen.go`
- `internal/appgen/appgen_test.go`
- `docs/compiler/generated-output.md`
- `docs/engineering/operations.md`
- `docs/engineering/security.md`
- `docs/product/missing-implementation-checklist.md`

## Data And API Impact

- No public Go API changes.
- Generated action binaries gain a fixed request body cap.

## Tests

- Unit: generated source contains `http.MaxBytesReader`.
- Integration: generated binary returns HTTP 413 for oversized action POSTs.
- Regression: valid action POSTs still redirect.

## Verification Commands

```sh
go test ./internal/appgen ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove body-limit generation, tests, and docs/checklist updates.

## Risks

- A fixed limit can be too small for future action payloads; this is acceptable
  until configurable body limits and upload support exist.
