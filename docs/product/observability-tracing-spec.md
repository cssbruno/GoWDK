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

- Add OpenTelemetry SDK or OTLP dependencies to the root module.
- Persist traces durably.
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

### Hardening

- Console output is for development and local diagnostics. `ConsoleSink` escapes
  control characters in every field (span names, status messages, source paths,
  and event messages), so untrusted values cannot forge log lines or emit
  terminal control sequences. Use `JSONLSink` for production logging where
  downstream tooling parses structured records.
- Trace and span IDs come from an injectable `IDGenerator`. The default
  `CryptoIDGenerator` uses `crypto/rand` and never falls back to a predictable
  value: on entropy failure it records the loss through `EntropyFailureCount` and
  the handler set by `SetEntropyFailureHandler`, and the tracer drops the span
  rather than emit a guessable ID. Tests inject deterministic IDs through
  `WithIDGenerator`.
- `SourceRef.File` is normalized before a snapshot leaves the process. The
  default `SourcePolicy` (relative mode) reduces absolute filesystem paths to
  project-relative logical paths consistently across the viewer, JSON/SSE,
  console, and OTLP surfaces; `SourcePathAbsolute` is an opt-in debug mode for
  local development only. In production, only project-relative source paths
  leave the process.
- The `runtime/trace/otel` bridge distinguishes GOWDK-owned providers (`NewSink`,
  and `NewSinkWithProvider(nil)`) from app-owned providers
  (`NewSinkWithProvider(provider)`): `Shutdown` only shuts down a provider the
  sink owns, unless `WithProviderShutdown` transfers ownership.
  `WithNativeIdentity` asserts the provider uses `SnapshotIDGenerator`, so GOWDK
  IDs are not duplicated as attributes; otherwise they are preserved as
  `gowdk.trace_id`/`gowdk.span_id`. The bridge sets span kind by lane, preserves
  event level as `gowdk.event.level`, applies a stable instrumentation
  name/version and default `service.name` resource, converts only the closed
  scalar/array value model, and drops unsupported values (counted, and listed in
  `gowdk.dropped_attributes`) instead of stringifying them.

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

- Generated instrumentation is debug-gated through `addons/observability`;
  production sampling, access policy, and durable storage remain app-owned.
- Collector data is in-memory only and process-local.
- The SSE stream is a local inspection aid, not a durable delivery channel.
- Concrete OTLP transport lives in the nested optional `runtime/trace/otel`
  module.
