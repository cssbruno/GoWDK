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

The wire format is W3C `traceparent`:

```text
00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
```

## Sinks

Current sinks:

- `NewConsoleSink(io.Writer)`: one readable line per completed span.
- `NewJSONLSink(io.Writer)`: one JSON span per line.
- `NewRingSink(limit)`: bounded in-memory recent spans, dropping oldest on
  overflow.
- `MultiSink(...)`: sends spans to multiple sinks in order.
- `ExporterSink(exporter)`: adapts an OTLP-like exporter interface.
- `NewCollector(limit)`: sink plus local JSON/SSE HTTP handler and browser span
  ingest.

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
  "dropped": 0
}
```

`GET /_gowdk/traces/events` or a request with `Accept: text/event-stream`
streams `event: gowdk-trace` messages for existing and future spans. The viewer
handler adds `GET /` for the self-contained UI and `POST /browser` for generated
browser spans.

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
