# Implementation Plan: GOWDK Trace P1

Spec: `docs/product/observability-tracing-spec.md`

Issue: #375

## Scope

Implement the core dependency-free `runtime/trace` package that can be imported
by plain Go programs before any compiler or generated-app integration lands.

## Assumptions

- The root module must not add production dependencies.
- Generated route/contract/job auto-instrumentation is later work.
- Local collection is process-local and best-effort.
- OpenTelemetry compatibility means stable names, attributes, and an OTLP-like
  export shape without importing the OTel SDK.

## Proposed Changes

- Add `runtime/trace` with:
  - W3C `TraceID`, `SpanID`, `traceparent` parse/inject/extract helpers.
  - `Tracer`, `Start`, `SpanFrom`, span attributes/events/status, and nil-safe
    span methods.
  - `Sampler` implementations for always-on, always-off, ratio, and counted
    test sampling.
  - `Sink`, `ConsoleSink`, `JSONLSink`, `RingSink`, `MultiSink`, and
    `ExporterSink`.
  - `Collector` implementing `Sink` plus JSON and SSE `http.Handler` output.
  - GOWDK surface/lane enums and source references.
- Add product, reference, architecture, roadmap, and README documentation.
- Add an ADR for the dependency-free tracing boundary.

## Files Expected To Change

- `runtime/trace/`
- `docs/product/observability-tracing-spec.md`
- `docs/engineering/observability-tracing-implementation-plan.md`
- `docs/engineering/decisions/0013-built-in-tracing-observability.md`
- `docs/reference/tracing.md`
- `README.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/engineering/architecture.md`

## Data And API Impact

- Adds a public experimental `runtime/trace` package.
- No existing public API changes.
- No generated output changes.
- No root module dependency changes.

## Tests

- Unit tests use external package `trace_test` to prove plain-Go importability.
- W3C traceparent parse/inject/extract round-trip.
- Ring overflow and drop-count behavior.
- JSON Lines and console sinks.
- Collector JSON and SSE output.
- Exporter adapter OTLP-like shape.
- Sampled-out `AlwaysOff` hot path benchmark with `0 B/op`.

## Verification Commands

```sh
go test ./runtime/trace
go test ./runtime/trace -run '^$' -bench BenchmarkStartAlwaysOff -benchmem
go test ./...
go build ./cmd/gowdk
scripts/test-go-modules.sh
git diff --check
```

## Rollback Plan

- Remove `runtime/trace` and tracing docs.
- No migration is required because the package is additive and generated apps do
  not depend on it in P1.

## Risks

- Public API naming may need adjustment once generated app instrumentation
  consumes the package.
- In-memory collector data is not durable and should not be documented as
  production trace storage.
- Concrete OTLP export must remain out of the root module unless it is placed
  in a nested optional module.

