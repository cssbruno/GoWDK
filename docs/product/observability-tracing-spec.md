# Feature Spec: GOWDK Trace

## Problem

GOWDK needs a tracing core that can be used by plain Go applications and later
by generated runtime paths without taking a dependency on OpenTelemetry,
Sentry, or a hosted observability backend.

## Goals

- Provide a dependency-free `runtime/trace` package.
- Use W3C Trace Context IDs and `traceparent` propagation.
- Support backend, frontend, and worker surfaces plus GOWDK lanes such as
  route, guard, handler, SSR, action, API, fragment, contract, job, island,
  navigation, and user spans.
- Provide context helpers: `Start`, `SpanFrom`, span events, attributes, and
  status.
- Provide pluggable sinks: console, JSON Lines, in-memory ring, multi-sink, and
  exporter adapter.
- Provide sampling: always on, always off, ratio, and test-counted sampling.
- Provide a self-contained collector handler that serves recent spans as JSON
  and streams spans over SSE.
- Keep the root module dependency graph unchanged.

## Non-Goals

- Auto-instrument generated handlers in this phase.
- Add OpenTelemetry SDK or OTLP dependencies to the root module.
- Persist traces durably.
- Provide a browser viewer or dev overlay in this phase.
- Define `.gwdk` tracing syntax.

## Users

- Go developers who want lightweight local trace output in plain Go programs.
- Future generated GOWDK app wiring that needs a stable runtime trace API.
- Tooling that wants recent in-process traces without a collector service.

## Requirements

### Functional

- `runtime/trace` exposes W3C-compatible `TraceID` and `SpanID` values.
- `trace.Start(ctx, name, opts...)` starts a sampled span and returns a context
  carrying that span.
- `trace.SpanFrom(ctx)` returns the active sampled span.
- Span methods are nil-safe: `End`, `Event`, `Set`, and `SetStatus` can be
  called even when sampling returned `nil`.
- `trace.Inject` and `trace.Extract` round-trip `traceparent` through carriers
  such as `http.Header`.
- `trace.RingSink` keeps the newest spans, drops oldest on overflow, and
  exposes a drop count.
- `trace.Collector` implements a sink and serves recent spans as JSON or SSE.
- The exporter adapter produces an OTLP-like span shape without importing
  OpenTelemetry packages.

### Non-Functional

- Runtime package imports must stay standard-library only.
- `AlwaysOff` with no start options must allocate zero objects on the hot path.
- Sinks must be safe for concurrent use.
- The ring sink must not block on external I/O.

## Acceptance Criteria

- [x] `go test ./runtime/trace`
- [x] `go test ./runtime/trace -run '^$' -bench BenchmarkStartAlwaysOff -benchmem`
- [x] `go test ./...`
- [x] No root `go.mod` dependency changes.

## Current Limits

- Generated app routes are not auto-instrumented yet.
- Collector data is in-memory only and process-local.
- The SSE stream is a local inspection aid, not a durable delivery channel.
- OTLP export is an interface and value shape only; concrete OTLP transport
  belongs in a later nested optional module.

