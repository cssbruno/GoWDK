# Feature Spec: Lifecycle Services

## Problem

Generated binaries currently start only the generated web handler. Apps that
need an in-process worker, metrics endpoint, protocol bridge, or extra server
must replace the generated entrypoint or rely on `init()` goroutines copied
into generated output, which hides startup errors and has no shutdown contract.

## Goals

- Let `gowdk.config.go` declare app-owned service providers for generated
  binaries.
- Keep generated `Handler()` and `ServeMux()` as request-only `net/http`
  integration points.
- Add a supervised generated-binary lifecycle with signal handling, service
  startup errors, context cancellation, and graceful HTTP shutdown.
- Expose generated contract registries to lifecycle services without making
  `runtime/app` import `runtime/contracts`.

## Non-Goals

- No built-in MCP addon, MCP runtime package, or MCP protocol implementation.
- No framework-specific service abstraction.
- No target-specific service selection in v1.
- No generated worker/cron binary shape beyond the existing generated web
  binary.

## Requirements

- `gowdk.Config.Lifecycle.Services` names import/function pairs. Each provider
  has signature `func() ([]runtime/app.Service, error)`.
- Generated app packages expose `App() (*runtime/app.Application, error)`.
- Generated `cmd/server` uses `runtime/app.Run` and keeps `GOWDK_ADDR` plus the
  existing server timeout defaults.
- `runtime/app.Run` mounts services before starting the HTTP server, runs
  services concurrently, cancels on service/server error or SIGINT/SIGTERM,
  and shuts down with a configurable timeout.
- Generated contract routes and lifecycle services share `ContractRegistry()`;
  `NewContractRegistry()` remains a fresh-registry API.

## Acceptance Criteria

- Config loading accepts literal and executable lifecycle service refs.
- A generated app with service refs imports provider packages and appends all
  returned services.
- A generated binary can mount app-owned routes/services on the same mux.
- Runtime tests cover mount ordering, cancellation, service errors, no-op
  services, server errors, and shutdown timeout.
- Docs state that MCP adapters are app-owned or external lifecycle services,
  not GOWDK core.

## Dependencies

- Internal: `gowdk.Config`, `internal/project`, `internal/appgen`,
  `runtime/app`, generated contract registry helpers.
- External: none.
