# Dependency Policy

GOWDK should keep dependencies minimal while avoiding risky hand-rolled implementations for complex domains.

## Current Policy

- Do not add production dependencies without a clear reason documented in the change or an ADR.
- Prefer standard library packages for simple compiler, CLI, and runtime work.
- Prefer maintained libraries for complex domains such as authentication, authorization, cryptography, payments, parsing, and dates.
- Keep optional integrations behind addons or nested modules when possible.

## Documentation

Document major dependency decisions in `docs/engineering/decisions/`.

## Release Review Gates

Run these gates before release packaging.

```sh
go list -m all
go list -m -json all
scripts/test-go-modules.sh
scripts/vulncheck-go-modules.sh
```

Review the `go list` output for unexpected new modules and record any
production dependency decision in an ADR. Review module licenses from the
`go list -m -json all` output and each module's repository metadata before
publishing release notes. `govulncheck` must complete without reachable
vulnerability findings, or the release notes must document the finding,
exploitability, and mitigation.

CI and release packaging pin Go `1.26.4` because earlier Go 1.26 patch versions
have reachable standard-library vulnerabilities. Local release verification
should use the same or newer Go patch version before trusting `govulncheck`
output.

Add automated dependency and license checks to CI before claiming production
readiness.

## Current Dependency Classification

- Compiler core: standard library plus repository packages under `internal/`,
  and `golang.org/x/tools/go/packages` for Go package loading during endpoint
  binding inspection.
- Runtime core: standard library plus repository packages under `runtime/`.
- Optional HTTP adapters: `runtime/adapters/chi`, `runtime/adapters/echo`,
  `runtime/adapters/gin`, and `runtime/adapters/fiber`; each framework adapter
  is a nested Go module so framework dependencies do not enter the root module
  graph. Generated code remains `net/http` first by default.
- Optional broker/realtime adapters: Redis Streams, NATS, SSE, and WebSocket
  packages under `runtime/contracts`; concrete Redis Streams, NATS, and
  WebSocket adapters are nested Go modules. Dependency-free adapters such as
  file outbox, memory broker, and SSE stay in the root module. Applications opt
  in.

Nested optional modules are intentionally not listed in a checked-in root
`go.work`; ordinary root `go test ./...` and `go build ./cmd/gowdk` should stay
outside workspace mode so optional framework and broker dependencies do not
enter the root module graph. `scripts/go-modules.sh` discovers the root module
plus nested runtime modules for multi-module CI gates.
Nested optional modules that import the root GOWDK module should still require
the current released `github.com/cssbruno/gowdk` version and keep a local
`replace github.com/cssbruno/gowdk => ../../..` for repository tests outside
workspace mode. Update those required versions when cutting a release that
changes root runtime APIs used by nested modules.
- Optional CSS/tool adapters: `addons/tailwind`; it shells out to a user-owned
  Tailwind executable and does not download Tailwind during normal builds.
- Test/dev only: workflow Node checks and VS Code packaging scripts.
