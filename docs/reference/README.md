# Reference

Reference docs describe current commands, configuration, runtime contracts,
generated metadata, and integration boundaries. Start with the
[Documentation Hub](../README.md); for task-oriented examples, use the
[Native Cookbook](../cookbook/README.md).

## CLI And Project Shape

- [CLI](cli.md): current `gowdk` command and flag surface.
- [Config](config.md): current Go config types, addons, modules, and build target
  fields.
- [Dev Server](dev.md): rebuild, live reload, generated-app restart, overlay, and
  HMR behavior.
- [Deployment](deployment.md): output shapes, generated binaries, Docker, and
  optional operations recipes.
- [Testing](testing.md): scaffolded smoke tests, generated-app tests, browser
  smoke, accessibility, and performance checks.

## Language And Generated Output Contracts

- [Routing](routing.md): route declarations, dynamic route behavior, and output
  paths.
- [Validation](validation.md): request and form validation boundaries.
- [Manifest](manifest.md): current manifest JSON.
- [Go Interop](go-interop.md): Go binding, build-data, typed parameter, and
  stub-generation behavior.
- [Hooks](hooks.md): lifecycle services, middleware, guard, and rate-limit
  ordering.
- [Errors](errors.md): error pages, panic boundaries, and cache policy.
- [Diagnostics](diagnostics.md): diagnostic output formats.
- [Diagnostic Codes](diagnostic-codes.md): diagnostic registry, stability, and
  `gowdk explain`.

## Runtime And Addons

- [Addons](addons.md): addon feature registration and discovery.
- [Contracts](contracts.md): runtime contract registry and generated
  command/query adapters.
- [Realtime](realtime.md): presentation-event fanout, SSE default, and WebSocket
  opt-in behavior.
- [Tracing](tracing.md): dependency-free trace IDs, spans, sinks, sampling,
  propagation, and local collection.
- [Observability](observability.md): generated tracing addon and debug-gated
  instrumentation.
- [Database](db.md): database addon conventions and app-owned boundaries.
- [Framework Integrations](framework-integrations.md): optional adapter
  integration notes.

## Frontend And Metadata

- [CSS](css.md): CSS inputs, processors, scoped output, and extension points.
- [Images](images.md): image optimization patterns and current non-goals.
- [SEO](seo.md): sitemap, robots, metadata, and structured-data behavior.
- [PWA And Offline](pwa-offline.md): user-owned service worker and manifest
  guidance.

## Sources Of Truth

- Accepted `.gwdk` syntax and semantics live under
  [Language](../language/README.md).
- Compiler pipeline and generated-output details live under
  [Compiler](../compiler/README.md).
- Product capability status lives in
  [Requirements](../product/requirements.md).
- Product direction and feature specifications live under
  [Product](../product/README.md).
- Architecture, production hardening, and operations guidance live under
  [Engineering](../engineering/README.md).
- Runnable examples and recipes live in the
  [Cookbook](../cookbook/README.md) and [Examples Index](../../examples/README.md).
