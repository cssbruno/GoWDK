# Implementation Plan: Lifecycle Services

## Context

Spec: `docs/product/lifecycle-services-spec.md`

ADR: `docs/engineering/decisions/0015-generated-binary-lifecycle-services.md`

Issues: #481 for generic lifecycle/background-service hooks. #480 is kept out
of core; MCP can be an app-owned or external lifecycle service.

## Assumptions

- Lifecycle service refs are top-level config and apply to each generated app
  target.
- Provider refs are import/function descriptors rather than function-valued
  config, preserving the compiler/runtime boundary.
- Generated Go compilation verifies provider symbol existence and signature.

## Proposed Changes

- Add `gowdk.LifecycleConfig` and `gowdk.ServiceRef`; parse and validate them
  through literal and executable config loading.
- Add `runtime/app.Service`, `ServiceContext`, `Application`, `RunOptions`,
  `ServiceHooks`, and `Run`.
- Generate `gowdkapp.App()` plus a private identity-aware mux constructor.
- Generate service provider imports and `configuredServices()`.
- Change generated `cmd/server` to call `runtime/app.Run`.
- Add generated `ContractRegistry()` singleton for shared in-process contract
  access; keep `NewContractRegistry()` for isolated registries.

## Data And API Impact

- New public config field: `Config.Lifecycle`.
- New public runtime lifecycle API under `runtime/app`.
- Generated app packages add `App()` and `ContractRegistry()` when executable
  contracts exist.
- Existing `Handler()`, `ServeMux()`, and middleware registration remain
  source-compatible.

## Tests

- Unit: config parser/bridge, runtime lifecycle supervisor.
- Generator: `App()`, generated main, service imports/provider calls, module
  detection, shared contract registry.
- Smoke: generated app/package compile and build commands.

## Verification Commands

```sh
go test ./internal/project ./runtime/app ./internal/appgen
go test ./...
go build ./cmd/gowdk
scripts/test-go-modules.sh
```

## Rollback Plan

- Remove `Config.Lifecycle`, generated `App()` service wiring, and
  `runtime/app.Run`; restore generated main to direct `ListenAndServe`.
- Keep `Handler()` and `ServeMux()` compatibility throughout rollback.

## Risks

- Long-running services that ignore context can delay shutdown until timeout.
- Invalid service provider signatures surface at generated app Go build time.
- App-owned service packages can introduce new module requirements; generated
  app module wiring must include local app modules when refs point into them.
