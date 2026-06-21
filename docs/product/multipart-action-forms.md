# Feature Spec: Multipart Action Forms

## Problem

Generated action forms reject file inputs and `multipart/form-data`, so upload
workflows must leave the action pipeline and reimplement decoding, request
limits, CSRF, validation fragments, and response handling in user-owned
handlers.

## Goals

- Let generated `g:post` action forms submit bounded multipart data.
- Require explicit per-file size, count, and content-type policy on direct file
  controls.
- Decode uploads into typed Go action input fields without unbounded buffering.
- Preserve generated CSRF, guard, unknown-field rejection, and validation
  fragment behavior.

## Non-Goals

- GOWDK does not store uploaded files.
- GOWDK does not inspect file contents, virus scan, resize, transcode, or infer
  trusted MIME types from bytes.
- Generated actions do not infer file controls hidden inside components.

## Users And Permissions

- Primary users: application authors building no-JavaScript-safe upload forms.
- Roles or permissions: existing page guards and handler authorization remain
  authoritative.
- Data visibility rules: submitted field values and file contents are not
  logged by generated code.

## User Flow

1. The page declares a `g:post` action form with
   `enctype="multipart/form-data"`.
2. Every direct `input type="file"` declares `g:max-file-size`, `g:max-files`,
   and MIME `accept`.
3. The Go action handler accepts a typed input struct with `form.File` or
   `[]form.File` fields and streams file content to user-owned storage.

## Requirements

### Functional

- File policy attributes are literal, positive integers for size/count and
  literal MIME entries for `accept`.
- `accept` supports exact MIME types and `type/*` wildcards for server-side
  allow-list checks.
- Generated adapters parse multipart requests under `Build.BodyLimits.ActionBytes`.
- Runtime decoding rejects unexpected non-runtime form values and file fields.
- Runtime decoding rejects files that exceed count, size, or content-type
  policy before user handlers run.
- Typed action inputs can use `form.File` for one uploaded file and
  `[]form.File` for repeated uploads.

### Non-Functional

- Performance: generated code uses `http.MaxBytesReader` plus Go multipart
  parsing; file content is exposed through `Open()` and is not copied into
  generated structs.
- Reliability: decoding errors return the existing invalid-form response shape.
- Accessibility: upload forms remain normal browser forms.
- Security/privacy: storage is user-owned; GOWDK enforces declared limits and
  avoids value/content logging.
- Observability: generated endpoint metadata and no-store response behavior are
  unchanged.

## Acceptance Criteria

- [x] Multipart `g:post` forms compile when file controls carry explicit policy.
- [x] File controls without size, count, or content-type policy fail with clear
      diagnostics.
- [x] Generated bound action decoders populate `form.File` and `[]form.File`.
- [x] Oversized, over-count, wrong-type, and unexpected file submissions return
      HTTP 400 or 413 before the user handler runs.
- [x] CSRF validation and validation-fragment behavior remain available.
- [x] Runtime helpers and docs show that storage/persistence is user-owned.

## Edge Cases

- A multipart form without file controls is allowed and decoded as normal
  values.
- Empty optional file controls decode as missing files.
- A scalar `form.File` field rejects repeated submitted files.
- A `multiple` file control must still declare the explicit maximum count.

## Dependencies

- Internal: view form schema extraction, action adapter generation,
  `runtime/form`, compiler Go input metadata.
- External: Go standard library `mime/multipart` and `net/http`.

## Open Questions

- None for this first slice.
