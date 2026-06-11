# Implementation Plan: View URL And HTML Safety

## Context

Relevant issue: https://github.com/cssbruno/GoWDK/issues/61

## Assumptions

- `view {}` remains a GOWDK-owned safe subset.
- User JavaScript belongs in configured/scoped script assets or
  compiler-owned island behavior, not raw HTML event handlers.
- `data-*` custom attributes are ordinary escaped attributes and should remain
  allowed.

## Proposed Changes

- Add shared view safety helpers for URL-bearing attributes, inline handler
  attributes, `srcdoc`, and blocked raw HTML elements.
- Run literal safety checks during HTML element parsing.
- Run resolved URL checks during render after interpolation.
- Keep route-param taint checks for URL, style, `srcdoc`, and event-handler
  attributes.
- Preserve the stronger dev-loop input display path fix for symlinked working
  directories.
- Add focused tests and update markup docs.

## Files Expected To Change

- `internal/view/parser.go`
- `internal/view/element.go`
- `internal/view/interpolate.go`
- `internal/view/safety.go`
- `internal/view/view_test.go`
- `internal/buildgen/*_test.go`
- `cmd/gowdk/dev_loop.go`
- `cmd/gowdk/main_test.go`
- `docs/language/markup.md`

## Data And API Impact

- No public Go API changes.
- Invalid view markup that previously rendered now fails compilation or
  rendering.

## Tests

- Unit: `go test ./internal/view`
- Integration: `go test ./internal/buildgen`
- CLI regression: `go test ./cmd/gowdk`
- End-to-end: not required for this slice.
- Manual: inspect diagnostics from failed view rendering if needed.

## Verification Commands

```sh
go test ./internal/view
go test ./internal/buildgen
go test ./cmd/gowdk
go test ./...
```

## Rollback Plan

- Revert the view safety helper, call sites, docs, and tests.

## Risks

- Existing experimental `.gwdk` files with raw inline handlers or unsafe URL
  schemes will start failing; this is intentional hardening.
