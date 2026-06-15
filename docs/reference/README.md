# Reference

Reference docs describe current command, language-adjacent, runtime, and data
contracts. For task-oriented examples, start with the
[native cookbook](../cookbook/README.md). For full lessons, use the
[native learning path](../learning/native.md).

## CLI And Project Shape

- [CLI](cli.md): current `gowdk` command surface.
- [Config](config.md): current Go config types and build target fields.
- [Dev server](dev.md): `gowdk dev` rebuild, live reload, generated-app restart,
  overlay, and HMR behavior.
- [Deployment](deployment.md): output shapes, generated binaries, Docker, and
  optional operations recipes.
- [Testing](testing.md): scaffolded smoke tests, browser smoke, accessibility,
  and performance checks.

## Language And Generated Output Contracts

- [Routing](routing.md): route declarations, dynamic route behavior, and output
  paths.
- [Manifest](manifest.md): current manifest JSON.
- [Go interop](go-interop.md): Go binding, build-data, typed param, and
  stub-generation behavior.
- [Hooks](hooks.md): middleware, guard, and rate-limit ordering.
- [Errors](errors.md): error pages, panic boundaries, and cache policy.
- [Diagnostics](diagnostics.md): diagnostic output formats.
- [Diagnostic codes](diagnostic-codes.md): diagnostic registry, stability, and
  `gowdk explain`.

## Runtime And Addons

- [Addons](addons.md): current addon feature registration and discovery.
- [Contracts](contracts.md): runtime contract registry and generated
  command/query adapters.
- [Realtime](realtime.md): presentation-event fanout setup, SSE default, and
  WebSocket opt-in behavior.
- [Tracing](tracing.md): dependency-free runtime trace IDs, spans, sinks,
  sampling, propagation, and local collection.
- [Observability](observability.md): generated tracing addon and debug-gated
  instrumentation.
- [Database](db.md): current database addon conventions and boundaries.
- [Framework integrations](framework-integrations.md): adapter integration notes.

## Frontend And Metadata

- [CSS](css.md): current CSS extension point.
- [Images](images.md): image optimization patterns and current non-goals.
- [SEO](seo.md): optional sitemap.xml and robots.txt emission.
- [PWA/offline](pwa-offline.md): optional user-owned service worker and
  manifest guidance.

## Source Of Truth

- Language syntax and semantics live under [language](../language/README.md).
- Compiler pipeline and generated-output details live under
  [compiler](../compiler/README.md).
- Product status lives in [requirements](../product/requirements.md).
- Production hardening guidance lives in
  [engineering/security](../engineering/security.md) and
  [engineering/operations](../engineering/operations.md).
