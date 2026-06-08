# Implementation Plan: Framework Context Bridge

## Context

Spec: `.llm/features/framework-context-bridge.md`

GOWDK generated apps expose `http.Handler`. Optional framework adapters wrap
that handler for teams using Gin, Echo, or Fiber.

## Assumptions

- Framework context access is an escape hatch for integration-specific concerns.
- Normal GOWDK handlers should keep using standard `context.Context`, cookies,
  headers, and `runtime/app` helpers.
- Fiber should keep using `github.com/gofiber/fiber/v2/middleware/adaptor`
  instead of a repository-owned response bridge.

## Proposed Changes

- Add an unexported context key in each adapter module.
- Wrap requests with the active framework context before calling the generated
  handler.
- Expose a package-local `Context(context.Context)` accessor.
- Update adapter tests and framework integration docs.

## Files Expected To Change

- `runtime/adapters/gin/gin.go`
- `runtime/adapters/gin/gin_test.go`
- `runtime/adapters/echo/echo.go`
- `runtime/adapters/echo/echo_test.go`
- `runtime/adapters/fiber/fiber.go`
- `runtime/adapters/fiber/fiber_test.go`
- `docs/reference/framework-integrations.md`

## Data And API Impact

- New public helper functions in optional nested adapter modules.
- No generated code changes.
- No root module dependency changes.

## Tests

- Unit: adapter tests assert context accessors return the current framework
  context in wrapped handlers.
- Integration: covered by `scripts/test-go-modules.sh`.

## Verification Commands

```sh
gofmt -w runtime/adapters/gin/gin.go runtime/adapters/gin/gin_test.go runtime/adapters/echo/echo.go runtime/adapters/echo/echo_test.go runtime/adapters/fiber/fiber.go runtime/adapters/fiber/fiber_test.go
go test ./...
scripts/test-go-modules.sh
```

## Rollback Plan

- Remove the adapter context accessors and request-context wrapping. Existing
  `Handler` and `Mount` behavior can return to direct `http.Handler` wrapping.

## Risks

- Framework contexts can encourage coupling. Docs should present the bridge as
  optional and keep GOWDK route and handler behavior framework-neutral.
