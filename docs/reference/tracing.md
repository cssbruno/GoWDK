# Tracing

`runtime/trace` is the first GOWDK Trace runtime slice. It is dependency-free
and can be used from normal Go programs without generated app wiring.

Use [Observability](observability.md) for generated app instrumentation,
the local viewer, and optional OTLP export.

## Basic Usage

```go
package main

import (
	"context"
	"os"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

func main() {
	tracer := gowdktrace.NewTracer(
		gowdktrace.WithSink(gowdktrace.NewJSONLSink(os.Stdout)),
	)

	ctx, span := tracer.Start(context.Background(), "GET /patients",
		gowdktrace.WithSurface(gowdktrace.SurfaceBackend),
		gowdktrace.WithLane(gowdktrace.LaneRoute),
		gowdktrace.WithAttributes(map[string]any{
			gowdktrace.AttrHTTPRoute: "/patients",
		}),
	)
	defer span.End()

	_ = ctx
	span.Event("info", "loaded patients", nil)
	span.SetStatus(gowdktrace.StatusOK, "")
}
```

`Start` returns `nil` for the span when sampling is disabled. Span methods are
nil-safe, so `defer span.End()` remains valid.

## Trace Context

Use `Inject` and `Extract` with carriers such as `http.Header`:

```go
headers := http.Header{}
gowdktrace.Inject(ctx, headers)

ctx = gowdktrace.Extract(context.Background(), headers)
```

The wire format is W3C `traceparent`, with valid `tracestate` preserved when
present:

```text
00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
```

`Extract` rejects malformed `traceparent` values and ignores malformed
`tracestate` values while preserving the trace identity. Header limits are
fixed: `traceparent` is capped at 256 bytes and `tracestate` at 512 bytes. The
remote sampled flag is input context only; the local sampler still decides
whether a new span is recorded.

## Sinks

Current sinks:

- `NewConsoleSink(io.Writer)`: one readable line per completed span.
- `NewJSONLSink(io.Writer)`: one JSON span per line.
- `NewRingSink(limit)`: bounded in-memory recent spans, dropping oldest on
  overflow.
- `MultiSink(...)`: sends spans to multiple sinks in order.
- `ExporterSink(exporter)`: adapts an OTLP-like exporter interface.
- `NewCollector(limit, options...)`: sink plus local JSON/SSE HTTP handler and
  browser span ingest.

## Collector

```go
collector := gowdktrace.NewCollector(256)
tracer := gowdktrace.NewTracer(gowdktrace.WithSink(collector))

http.Handle("/_gowdk/traces", collector.Handler())
```

`GET /_gowdk/traces` returns:

```json
{
  "spans": [],
  "dropped": 0,
  "rejected": 0,
  "health": {
    "spans": 0,
    "dropped": 0,
    "rejected": 0,
    "subscribers": 0,
    "subscriberQueueDepth": 0,
    "subscriberQueueCapacity": 0,
    "sseLimit": 32,
    "ingestRateLimit": 120,
    "ingestRateWindowDuration": "1m0s"
  }
}
```

`GET /_gowdk/traces/events` or a request with `Accept: text/event-stream`
streams `event: gowdk-trace` messages for existing and future spans. The viewer
handler adds `GET /` for the self-contained UI and `POST /browser` for generated
browser spans.

POST ingest is treated as untrusted input:

- requests must use `Content-Type: application/json` or `application/*+json`;
- browser-originated requests must be same-origin when an `Origin` header is
  present;
- request bodies are capped at 1 MiB and batches at 128 spans;
- span names, attributes, events, strings, and encoded snapshot size are
  bounded before storage;
- POST ingest is rate-limited per remote address by default;
- SSE subscribers are capped by default and slow subscribers are dropped instead
  of blocking span recording.

Tune local collector limits when mounting it directly:

```go
collector := gowdktrace.NewCollector(
	256,
	gowdktrace.WithCollectorSSELimit(16),
	gowdktrace.WithCollectorIngestRate(60, time.Minute),
)
```

`collector.HealthSnapshot()` returns the same storage, dropped, rejected,
subscriber, and ingest-limit counters for app-owned health endpoints.

Generated apps keep browser ingest on the main app handler but serve the
readable viewer, JSON data, and SSE stream from
`runtime/app.LocalTraceViewerService` on a separate loopback listener. The
generated binary prints that local viewer URL at startup. If an application
mounts `Collector.Handler()` or `ViewerHandler()` itself, it can use
`runtime/app.LocalTraceAccess` as a local-only gate: it ignores
client-supplied `Host` and requires both the remote peer and the server listener
address recorded in `http.LocalAddrContextKey` to be loopback, with no forwarded
proxy headers. Internet-facing trace routes still need normal authentication,
authorization, TLS, reverse-proxy, and production rate-limit policy.

## Log Correlation And Health

Use the active trace context to attach stable `slog` attributes:

```go
logger.InfoContext(ctx, "loaded patient", gowdktrace.SlogArgs(ctx)...)
```

`SlogAttrs(ctx)` returns `trace_id` and `span_id` attributes. `SlogArgs(ctx)`
returns the same data as alternating key/value arguments for `slog.Logger`
methods. Both return nil when the context has no valid trace identity.

`tracer.HealthSnapshot()` reports the sampler, sampling ratio when known,
sampled spans, successful exports, export failures, and recent export latency.
Generated app health includes this snapshot when a tracer is attached.

## Sampling

- `AlwaysOn()`
- `AlwaysOff()`
- `RatioSampler(0.10)`

The disabled `AlwaysOff` path with no start options is allocation-free.

## Current Limits

- Collector storage is in-memory and process-local.
- Generated app instrumentation is opt-in through `addons/observability` and
  debug builds.
- Concrete OTLP export lives in the nested `runtime/trace/otel` module so the
  root module does not depend on OpenTelemetry.
- Durable storage, hosted analysis, metrics/log backends, production sampling
  policy, and production access policy stay app-owned.
