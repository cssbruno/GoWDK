# Observability

`addons/observability` enables generated GOWDK Trace wiring for debug builds.
The addon name stays `observability`, but the current implemented surface is
GOWDK Trace, local inspection primitives, dependency-free health snapshots,
low-cardinality generated route metrics, and trace/span correlation helpers for
standard-library `slog`. It is not a hosted observability platform.

It registers `FeatureObservability`; `runtime/trace` remains the
dependency-free root runtime, and optional OTLP export lives in the nested
`runtime/trace/otel` module.

Enable it:

```sh
gowdk add observability
gowdk build --debug --app /tmp/gowdk-app pages/home.page.gwdk
```

Generated development builds mount the local trace viewer at:

```text
/_gowdk/traces
```

The viewer is off unless the addon is enabled and `Build.DebugAssets()` is true
(`Build.Mode != gowdk.Production`). Outside dev, generated apps mount the viewer
behind `runtime/app.LocalTraceAccess`, which only allows direct localhost or
loopback requests and rejects forwarded reverse-proxy requests unless the app
supplies an explicit `TraceAccess` function.

Current generated instrumentation:

- Backend request route spans extract incoming `traceparent` and valid
  `tracestate`.
- Generated SSR route and `server {}` spans record route IDs, render lane,
  source refs, response status, and load errors without storing raw request
  bodies or headers.
- Generated action, API, fragment, command, and query routes record handler
  spans with `.gwdk` source refs when debug metadata is enabled.
- Guards and contract command/query/job/event/worker operations record child
  spans when a tracer is present in context.
- `runtime/contracts.EventEnvelope` and file outbox records carry an optional
  `traceparent`; old records without it remain readable.
- Generated browser runtime spans partial submits and SPA navigation, injects
  trace context, and posts frontend spans to the local collector.
- JS islands, WASM island loaders, and page-level client Go WASM loaders reuse
  `window.__gowdkTrace`.
- `runtime/trace.SlogAttrs` and `SlogArgs` expose active `trace_id` and
  `span_id` values for app-owned structured logs.
- `runtime/app.Metrics` records request count, active request count, latency,
  errors, and generated backend route metrics keyed by route templates and
  endpoint IDs.
- Generated app health includes tracer export health when a tracer is attached,
  and the local collector JSON includes collector queue/reject health.

The generated local collector keeps a bounded in-memory ring of 1024 completed
spans. It requires JSON POST ingest, rejects cross-origin browser ingest, caps
request/body/span/event/attribute/string sizes, limits POST ingest rate and SSE
subscriber count, and exposes `dropped` plus `rejected` health counters in the
JSON/viewer surface. Generated code records stable route/endpoint IDs and source
metadata, and uses runtime redaction helpers for query strings, error messages,
and app-owned trace events.

## Mode Matrix

| Mode | Generated spans | Local collector/viewer | Intended access |
| --- | --- | --- | --- |
| `gowdk dev` / local debug | Enabled when the addon is present and debug assets are on. | Mounted at `/_gowdk/traces`. | Localhost only by default. |
| Preview/debug app builds | Enabled when `Build.DebugAssets()` is true. | Mounted only when the addon is present. | Keep behind `LocalTraceAccess` or an app-owned gate. |
| Production builds | Disabled by default because debug assets are off. | Not mounted by generated code. | Use app-owned telemetry export and access policy. |
| Direct `runtime/trace` use | App-controlled. | App-controlled. | The app must wrap public routes with auth, TLS, proxy, and rate-limit policy. |

For app-owned Go handlers, record a user event on the active span:

```go
app.Trace(ctx, "loaded patient", map[string]any{"patientID": id})
```

## Production-safe sampling

`runtime/trace` ships dependency-free sampling primitives; the app owns the
policy. For a deterministic fraction of *whole* traces, wrap a `RatioSampler` in
a `ParentBasedSampler` so a downstream service honors the upstream decision
instead of splitting a trace:

```go
sampler := trace.ParentBasedSampler(trace.RatioSampler(0.1))
tracer := trace.NewTracer(trace.WithSampler(sampler))
```

`RuleSampler` is the route/endpoint override hook: silence health checks and
noisy endpoints, force high-value endpoints on, and sample the rest with a base
sampler. The first matching rule wins.

```go
sampler := trace.RuleSampler(
	trace.ParentBasedSampler(trace.RatioSampler(0.1)),
	trace.DropSpansNamed("GET /_gowdk/health"),
	trace.KeepSpansNamed("POST /checkout"),
	trace.SamplerRule{Match: trace.MatchSpanNamePrefix("GET /_gowdk/"), Keep: false},
)
```

`AlwaysOn`, `AlwaysOff`, `RatioSampler`, `ParentBasedSampler`, `RuleSampler`, and
the matchers are also re-exported from `addons/observability`.

## OTLP export

Export to OTLP from an app that opts into the nested `runtime/trace/otel`
module. The bridge exposes the production knobs a real OTLP deployment needs:

```go
sink, err := otel.NewSink(ctx,
	otel.WithEndpoint("collector:4318"),
	otel.WithGzip(),                       // request compression
	otel.WithTLSClientConfig(tlsConfig),   // custom CA / client cert / server name
	otel.WithHeaders(map[string]string{"authorization": token}),
	otel.WithServiceName("checkout"),
	otel.WithServiceVersion("1.4.2"),
	otel.WithEnvironment("production"),
	otel.WithResourceAttributes(map[string]string{"team": "payments"}),
	otel.WithMaxQueueSize(4096),           // bounded queue: overflow is dropped, not unbounded memory
	otel.WithMaxExportBatchSize(512),
	otel.WithBatchTimeout(5*time.Second),
	otel.WithRetry(otel.RetryConfig{Enabled: true, InitialInterval: time.Second, MaxInterval: 10 * time.Second, MaxElapsedTime: time.Minute}),
)
if err != nil {
	return err
}
defer sink.Shutdown(ctx)

tracer := trace.NewTracer(trace.WithSink(sink))
```

- For local collectors use `otel.WithInsecure()` instead of TLS.
- `sink.ForceFlush(ctx)` drains buffered spans at a checkpoint (signal, pre-deploy)
  without shutting the provider down; `sink.Shutdown(ctx)` flushes and stops a
  GOWDK-owned provider.
- Drop/failure counters: the in-process ring exposes `Collector.Dropped()`, the
  OTLP path exposes `otel.ExporterFailureCount()` (export batches that failed
  after retries) and `otel.UnsupportedAttributeCount()` (attribute values outside
  the OTel value model).
- The bounded queue defines overflow behavior: when full, the batch processor
  drops new spans rather than growing memory without bound.

### GOWDK-owned vs app-owned

GOWDK owns stable primitives: the samplers, the snapshot model, the OTLP HTTP
bridge, and a GOWDK-configured provider via `otel.NewSink`. When an app needs
full control of the OpenTelemetry lifecycle, resources, or a different exporter,
it supplies its own provider with `otel.NewSinkWithProvider(provider)` (pass
`otel.WithProviderShutdown()` to hand lifecycle to the sink, and
`otel.WithNativeIdentity()` when the provider uses `otel.SnapshotIDGenerator`).
Sampling policy and telemetry-backend selection are always app-owned; GOWDK never
picks a sampling ratio or an exporter endpoint for you.

Do not treat the local collector or viewer as a production observability
backend. Production deployments should set sampling deliberately, keep viewer
access gated or disabled, and send spans, logs, and metrics to app-owned
telemetry infrastructure. Durable storage, hosted analysis, production metrics
export, richer log-pipeline integration, alerting, retention, and production
access policy remain future observability work.
