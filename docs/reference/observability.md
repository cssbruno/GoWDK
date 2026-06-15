# Observability

`addons/observability` enables generated GOWDK Trace wiring for debug builds.
It registers `FeatureObservability`; `runtime/trace` remains the dependency-free
root runtime, and optional OTLP export lives in the nested
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
behind `runtime/app.LocalTraceAccess`, which only allows loopback clients unless
the app supplies a stricter `TraceAccess` function.

Current generated instrumentation:

- Backend request route spans extract incoming `traceparent`.
- Generated action, API, fragment, command, and query routes record handler
  spans with `.gwdk` source refs when debug metadata is enabled.
- Guards and contract command/query/job/event/worker operations record child
  spans when a tracer is present in context.
- `runtime/contracts.EventEnvelope` and file outbox records carry an optional
  `traceparent`; old records without it remain readable.
- Generated browser runtime spans partial submits and SPA navigation, injects
  `traceparent`, and posts frontend spans to the local collector.
- JS islands, WASM island loaders, and page-level client Go WASM loaders reuse
  `window.__gowdkTrace`.

For app-owned Go handlers, record a user event on the active span:

```go
app.Trace(ctx, "loaded patient", map[string]any{"patientID": id})
```

Export to OTLP from an app that opts into the nested module:

```go
sink, err := otel.NewSink(ctx, otel.WithEndpoint("localhost:4318"), otel.WithInsecure())
if err != nil {
	return err
}
defer sink.Shutdown(ctx)

tracer := trace.NewTracer(trace.WithSink(sink))
```

Do not treat the local collector or viewer as a production observability
backend. Production deployments should set sampling deliberately, keep viewer
access gated or disabled, and send spans to app-owned telemetry infrastructure.
