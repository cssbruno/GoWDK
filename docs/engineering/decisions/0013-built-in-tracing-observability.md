# ADR 0013: Built-In Tracing Observability

Date: 2026-06-15

Status: Accepted

## Context

GOWDK Runtime needs trace IDs, spans, and local trace inspection across
generated routes, contracts, jobs, workers, islands, and future dev tooling.
The first phase must be usable from plain Go and must not force OpenTelemetry,
Sentry, or a hosted collector into the root module.

Repository constraints:

- Runtime core stays dependency-light and standard-library first.
- Optional integrations should live in nested modules or addons when they add
  third-party dependencies.
- Generated JavaScript and generated Go should consume stable runtime contracts,
  not own observability policy.
- Trace context should interoperate with standard HTTP infrastructure.

## Decision

GOWDK accepts a dependency-free `runtime/trace` package as the core tracing
boundary.

Rules:

- Trace identity uses W3C Trace Context `traceparent` IDs.
- The core package exposes `TraceID`, `SpanID`, `Tracer`, `Span`, `Sampler`,
  `Sink`, and context propagation helpers.
- Span metadata uses explicit GOWDK surfaces (`backend`, `frontend`, `worker`)
  and lanes (`route`, `guard`, `handler`, `ssr`, `action`, `api`, `fragment`,
  `contract`, `job`, `island`, `nav`, `user`).
- The root package includes only dependency-free sinks: console, JSON Lines,
  in-memory ring, multi-sink, exporter adapter, and an in-process collector
  handler for JSON/SSE local inspection.
- OpenTelemetry compatibility is represented by semantic-convention attribute
  keys and an OTLP-like exporter interface/value shape. Concrete OTLP
  transports must remain optional and outside the root dependency graph.
- Generated app auto-instrumentation is later work and must consume this
  runtime API rather than creating a parallel tracing model.

## Consequences

### Positive

- Plain Go applications can use the same trace API as future generated apps.
- The root module dependency graph remains unchanged.
- Trace propagation interoperates through standard `traceparent` headers.
- The collector gives local visibility without requiring a separate service.

### Negative

- The first slice does not send data to hosted observability systems.
- The in-memory collector is process-local and loses traces on restart.
- Generated app wiring still needs a later integration phase.

### Neutral

- Concrete OpenTelemetry export can be added later as a nested optional module.
- Trace names and attribute keys can be shared by compiler/runtime code without
  adopting an external SDK in core.

