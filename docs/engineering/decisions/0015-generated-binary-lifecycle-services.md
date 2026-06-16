# ADR 0015: Generated Binary Lifecycle Services

Date: 2026-06-16

Status: Accepted

## Context

Generated apps expose request-level `net/http` entry points, but the generated
binary has been web-only. Apps that need background work or an additional
in-process server have had to replace the generated main or use hidden
goroutines in generated app code.

The lifecycle extension must preserve the compiler/runtime boundary from ADR
0006 and the generated Go boundary from ADR 0005. It must also avoid turning a
specific protocol adapter, such as MCP, into framework core.

## Decision

GOWDK will provide a generic generated-binary lifecycle:

- `gowdk.Config.Lifecycle.Services` declares import/function service provider
  refs.
- Generated app packages expose `App()` for process startup while keeping
  `Handler()` and `ServeMux()` request-only.
- `runtime/app.Run` owns signal handling, service supervision, cancellation,
  and graceful HTTP shutdown.
- Lifecycle services use `runtime/app.Service`, `ServiceContext`, and
  `ServiceHooks`.
- Contract-aware generated apps expose a shared `ContractRegistry()` and pass
  it through `ServiceContext.Values` under a string key. `runtime/app` does not
  import `runtime/contracts`.

MCP is not a built-in GOWDK addon or runtime package. An MCP bridge can be
written as app code or an external package that returns lifecycle services.

## Consequences

### Positive

- Generated binaries can run workers, metrics endpoints, protocol bridges, and
  other app-owned servers without replacing generated main.
- Startup errors and shutdown behavior become visible and testable.
- The framework owns only lifecycle supervision, not app protocol semantics.

### Negative

- Invalid provider symbols fail during generated app Go compilation, not during
  config AST parsing.
- Services that ignore context can still delay process exit until timeout.

### Neutral

- `Handler()` and `ServeMux()` remain the integration surface for external
  routers and custom mains.
- Target-specific lifecycle configuration remains future work.

## Alternatives Considered

- Function-valued config. Rejected because generated binaries cannot carry
  build-time function values across the compiler/runtime boundary.
- Built-in MCP addon. Rejected because MCP is a protocol adapter and should not
  become framework implementation.
- `init()` goroutines. Rejected because they hide errors and have no shutdown
  contract.

## Follow-Up

- Add examples for generic app-owned services.
- Re-scope issue #480 as external/app-owned MCP adapter documentation if kept
  open.
