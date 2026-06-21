// Package otel bridges GOWDK's dependency-free runtime trace snapshots to the
// OpenTelemetry SDK and OTLP exporters.
package otel

import (
	"context"
	"crypto/rand"
	"sync/atomic"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	// instrumentationName and instrumentationVersion identify this bridge as
	// the OpenTelemetry instrumentation scope. The version is the bridge's
	// stable GOWDK-to-OpenTelemetry semantic-convention version, not the GOWDK
	// release version, so downstream consumers can pin to a known mapping.
	instrumentationName    = "github.com/cssbruno/gowdk/runtime/trace"
	instrumentationVersion = "0.1.0"

	// defaultServiceName is the service.name applied to the default resource of
	// a GOWDK-owned provider when the application does not supply its own.
	defaultServiceName = "gowdk"

	// AttrGOWDKEventLevel carries a GOWDK span event's level (for example
	// "info", "warn", "error") on the emitted OTel event, which has no native
	// level field.
	AttrGOWDKEventLevel = "gowdk.event.level"
	// AttrGOWDKDroppedAttributes lists the keys of attributes that could not be
	// represented in OpenTelemetry's closed value model and were dropped. It is
	// the documented loss marker for unsupported attribute values.
	AttrGOWDKDroppedAttributes = "gowdk.dropped_attributes"

	attrGOWDKTraceID         = "gowdk.trace_id"
	attrGOWDKSpanID          = "gowdk.span_id"
	attrGOWDKParentSpanID    = "gowdk.parent_span_id"
	attrGOWDKSourceOwnerKind = "gowdk.source.owner_kind"
	attrGOWDKSourceOwnerID   = "gowdk.source.owner_id"
)

// Option configures the OTLP HTTP bridge.
type Option func(*config)

type config struct {
	endpoint string
	insecure bool
	headers  map[string]string
}

// WithEndpoint sets the OTLP HTTP endpoint, for example "localhost:4318".
func WithEndpoint(endpoint string) Option {
	return func(config *config) {
		config.endpoint = endpoint
	}
}

// WithInsecure disables TLS for local collectors.
func WithInsecure() Option {
	return func(config *config) {
		config.insecure = true
	}
}

// WithHeaders adds static OTLP HTTP headers.
func WithHeaders(headers map[string]string) Option {
	return func(config *config) {
		if len(headers) == 0 {
			return
		}
		config.headers = map[string]string{}
		for key, value := range headers {
			config.headers[key] = value
		}
	}
}

// unsupportedAttributes counts attribute values that could not be represented
// in OpenTelemetry's closed value model and were dropped.
var unsupportedAttributes atomic.Uint64

// UnsupportedAttributeCount reports how many attribute values have been dropped
// because their Go type is outside the supported scalar/array model. Dropped
// keys are also recorded on the span or event via AttrGOWDKDroppedAttributes.
func UnsupportedAttributeCount() uint64 {
	return unsupportedAttributes.Load()
}

// Sink records GOWDK spans through an OpenTelemetry TracerProvider.
type Sink struct {
	provider *sdktrace.TracerProvider
	tracer   oteltrace.Tracer
	// ownsProvider is true when this sink created the provider (NewSink) or was
	// explicitly granted ownership (WithProviderShutdown). Only an owned
	// provider is shut down by Sink.Shutdown.
	ownsProvider bool
	// preservesIdentity is true when the provider is guaranteed to emit native
	// OTel trace/span IDs equal to the GOWDK snapshot IDs (it uses
	// SnapshotIDGenerator). When true the GOWDK IDs are not duplicated as
	// ordinary attributes.
	preservesIdentity bool
}

type snapshotIDContextKey struct{}

type snapshotIDs struct {
	traceID oteltrace.TraceID
	spanID  oteltrace.SpanID
}

// ProviderOption configures how a Sink relates to an app-supplied provider.
type ProviderOption func(*Sink)

// WithProviderShutdown transfers provider lifecycle ownership to the sink, so
// Sink.Shutdown flushes and shuts the provider down. Without it a provider
// passed to NewSinkWithProvider is treated as app-owned and left running by
// Sink.Shutdown.
func WithProviderShutdown() ProviderOption {
	return func(sink *Sink) { sink.ownsProvider = true }
}

// WithNativeIdentity asserts that the supplied provider was configured with
// SnapshotIDGenerator, so emitted OTel trace/span IDs equal the GOWDK snapshot
// IDs. When set, the bridge relies on native identity and does not duplicate
// the GOWDK IDs as ordinary attributes. Set it only when the provider really
// uses SnapshotIDGenerator; otherwise leave it unset and the GOWDK IDs are
// preserved as gowdk.trace_id / gowdk.span_id attributes.
func WithNativeIdentity() ProviderOption {
	return func(sink *Sink) { sink.preservesIdentity = true }
}

// NewSink creates an OTLP HTTP-backed sink. GOWDK owns the resulting provider:
// it is configured with SnapshotIDGenerator (so native OTel identity equals the
// GOWDK snapshot identity) and a default service resource, and Sink.Shutdown
// shuts it down.
func NewSink(ctx context.Context, options ...Option) (*Sink, error) {
	var cfg config
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	exporterOptions := []otlptracehttp.Option{}
	if cfg.endpoint != "" {
		exporterOptions = append(exporterOptions, otlptracehttp.WithEndpoint(cfg.endpoint))
	}
	if cfg.insecure {
		exporterOptions = append(exporterOptions, otlptracehttp.WithInsecure())
	}
	if len(cfg.headers) > 0 {
		exporterOptions = append(exporterOptions, otlptracehttp.WithHeaders(cfg.headers))
	}
	exporter, err := otlptracehttp.New(ctx, exporterOptions...)
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithIDGenerator(SnapshotIDGenerator{}),
		sdktrace.WithResource(defaultResource()),
	)
	sink := newSink(provider)
	sink.ownsProvider = true
	sink.preservesIdentity = true
	return sink, nil
}

// NewSinkWithProvider creates a sink backed by an existing, app-owned provider.
// It is useful when applications already own the OpenTelemetry lifecycle and
// resources.
//
// By default the provider is treated as borrowed: Sink.Shutdown does not shut
// it down, and GOWDK trace/span IDs are preserved as gowdk.trace_id /
// gowdk.span_id attributes because native OTel identity is not guaranteed. Pass
// WithProviderShutdown to transfer lifecycle ownership, and WithNativeIdentity
// when the provider is configured with SnapshotIDGenerator. A nil provider is a
// shorthand for a GOWDK-owned, identity-preserving provider.
func NewSinkWithProvider(provider *sdktrace.TracerProvider, options ...ProviderOption) *Sink {
	if provider == nil {
		provider = sdktrace.NewTracerProvider(
			sdktrace.WithIDGenerator(SnapshotIDGenerator{}),
			sdktrace.WithResource(defaultResource()),
		)
		sink := newSink(provider)
		sink.ownsProvider = true
		sink.preservesIdentity = true
		return sink
	}
	sink := newSink(provider)
	for _, option := range options {
		if option != nil {
			option(sink)
		}
	}
	return sink
}

func newSink(provider *sdktrace.TracerProvider) *Sink {
	return &Sink{
		provider: provider,
		tracer:   provider.Tracer(instrumentationName, oteltrace.WithInstrumentationVersion(instrumentationVersion)),
	}
}

// defaultResource returns the default GOWDK service resource. It merges the
// SDK's default resource (process and SDK metadata) with a stable GOWDK
// service.name so app-owned providers that omit a resource still emit
// identifiable spans.
func defaultResource() *resource.Resource {
	merged, err := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName(defaultServiceName)))
	if err != nil {
		return resource.Default()
	}
	return merged
}

// Shutdown flushes and closes the underlying TracerProvider, but only when the
// sink owns it. A borrowed (app-owned) provider is left running so the
// application controls its lifecycle.
func (sink *Sink) Shutdown(ctx context.Context) error {
	if sink == nil || sink.provider == nil || !sink.ownsProvider {
		return nil
	}
	return sink.provider.Shutdown(ctx)
}

// RecordSpan implements gowdktrace.Sink.
func (sink *Sink) RecordSpan(ctx context.Context, span gowdktrace.Snapshot) error {
	if sink == nil || sink.tracer == nil {
		return nil
	}
	ids, err := snapshotIDsFromSpan(span)
	if err != nil {
		return err
	}
	parent, err := parentContext(ctx, span)
	if err != nil {
		return err
	}
	parent = context.WithValue(parent, snapshotIDContextKey{}, ids)

	spanAttrs, dropped := convertAttributes(span.Attributes)
	spanAttrs = append(spanAttrs, semanticAttributes(span, sink.preservesIdentity)...)
	if len(dropped) > 0 {
		spanAttrs = append(spanAttrs, attribute.StringSlice(AttrGOWDKDroppedAttributes, dropped))
	}

	_, otelSpan := sink.tracer.Start(parent, span.Name,
		oteltrace.WithTimestamp(span.StartTime),
		oteltrace.WithSpanKind(spanKind(span)),
		oteltrace.WithAttributes(spanAttrs...),
	)
	for _, event := range span.Events {
		eventAttrs, droppedEvent := convertAttributes(event.Attributes)
		if event.Level != "" {
			eventAttrs = append(eventAttrs, attribute.String(AttrGOWDKEventLevel, event.Level))
		}
		if len(droppedEvent) > 0 {
			eventAttrs = append(eventAttrs, attribute.StringSlice(AttrGOWDKDroppedAttributes, droppedEvent))
		}
		otelSpan.AddEvent(event.Message,
			oteltrace.WithTimestamp(event.Time),
			oteltrace.WithAttributes(eventAttrs...),
		)
	}
	switch span.Status.Code {
	case gowdktrace.StatusError:
		otelSpan.SetStatus(codes.Error, span.Status.Message)
	case gowdktrace.StatusOK:
		otelSpan.SetStatus(codes.Ok, span.Status.Message)
	}
	otelSpan.End(oteltrace.WithTimestamp(span.EndTime))
	return nil
}

// spanKind maps a GOWDK lane to an OpenTelemetry span kind. Request-handling
// lanes are servers, outbound contract calls are clients, and everything else
// is internal.
func spanKind(span gowdktrace.Snapshot) oteltrace.SpanKind {
	switch span.Lane {
	case gowdktrace.LaneRoute, gowdktrace.LaneHandler, gowdktrace.LaneSSR,
		gowdktrace.LaneAction, gowdktrace.LaneAPI:
		return oteltrace.SpanKindServer
	case gowdktrace.LaneContract:
		return oteltrace.SpanKindClient
	default:
		return oteltrace.SpanKindInternal
	}
}

// semanticAttributes builds the gowdk.* attributes describing surface, lane,
// and source. The GOWDK trace/span IDs are added only when native OTel identity
// is not guaranteed (preservesIdentity is false); otherwise they would
// duplicate the span's own IDs. The source file is normalized through the
// active trace source policy so absolute filesystem paths are not exported.
func semanticAttributes(span gowdktrace.Snapshot, preservesIdentity bool) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 8)
	if !preservesIdentity {
		attrs = append(attrs,
			attribute.String(attrGOWDKTraceID, string(span.TraceID)),
			attribute.String(attrGOWDKSpanID, string(span.SpanID)),
			attribute.String(attrGOWDKParentSpanID, string(span.ParentSpanID)),
		)
	}
	attrs = append(attrs,
		attribute.String(gowdktrace.AttrGOWDKSurface, string(span.Surface)),
		attribute.String(gowdktrace.AttrGOWDKLane, string(span.Lane)),
	)
	if file := gowdktrace.NormalizeSourceFile(span.Source.File, gowdktrace.CurrentSourcePolicy()); file != "" {
		attrs = append(attrs, attribute.String(gowdktrace.AttrGOWDKSourceFile, file))
	}
	if span.Source.Line > 0 {
		attrs = append(attrs, attribute.Int(gowdktrace.AttrGOWDKSourceLine, span.Source.Line))
	}
	if span.Source.Column > 0 {
		attrs = append(attrs, attribute.Int(gowdktrace.AttrGOWDKSourceCol, span.Source.Column))
	}
	if span.Source.OwnerKind != "" {
		attrs = append(attrs, attribute.String(attrGOWDKSourceOwnerKind, span.Source.OwnerKind))
	}
	if span.Source.OwnerID != "" {
		attrs = append(attrs, attribute.String(attrGOWDKSourceOwnerID, span.Source.OwnerID))
	}
	return attrs
}

func snapshotIDsFromSpan(span gowdktrace.Snapshot) (snapshotIDs, error) {
	traceID, err := oteltrace.TraceIDFromHex(string(span.TraceID))
	if err != nil {
		return snapshotIDs{}, err
	}
	spanID, err := oteltrace.SpanIDFromHex(string(span.SpanID))
	if err != nil {
		return snapshotIDs{}, err
	}
	return snapshotIDs{traceID: traceID, spanID: spanID}, nil
}

func parentContext(ctx context.Context, span gowdktrace.Snapshot) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !span.TraceID.Valid() || !span.ParentSpanID.Valid() {
		return ctx, nil
	}
	traceID, err := oteltrace.TraceIDFromHex(string(span.TraceID))
	if err != nil {
		return ctx, err
	}
	spanID, err := oteltrace.SpanIDFromHex(string(span.ParentSpanID))
	if err != nil {
		return ctx, err
	}
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     true,
	})
	return oteltrace.ContextWithRemoteSpanContext(ctx, spanContext), nil
}

// SnapshotIDGenerator preserves GOWDK trace and span IDs when OpenTelemetry SDK
// spans are created from GOWDK trace snapshots.
type SnapshotIDGenerator struct{}

func (SnapshotIDGenerator) NewIDs(ctx context.Context) (oteltrace.TraceID, oteltrace.SpanID) {
	if ids, ok := ctx.Value(snapshotIDContextKey{}).(snapshotIDs); ok {
		return ids.traceID, ids.spanID
	}
	return randomTraceID(), randomSpanID()
}

func (SnapshotIDGenerator) NewSpanID(ctx context.Context, traceID oteltrace.TraceID) oteltrace.SpanID {
	if ids, ok := ctx.Value(snapshotIDContextKey{}).(snapshotIDs); ok && ids.traceID == traceID {
		return ids.spanID
	}
	return randomSpanID()
}

func randomTraceID() oteltrace.TraceID {
	for {
		var id oteltrace.TraceID
		if _, err := rand.Read(id[:]); err == nil && id.IsValid() {
			return id
		}
	}
}

func randomSpanID() oteltrace.SpanID {
	for {
		var id oteltrace.SpanID
		if _, err := rand.Read(id[:]); err == nil && id.IsValid() {
			return id
		}
	}
}

// convertAttributes maps GOWDK attributes to OTel key/values using the closed
// scalar/array value model. Values outside the model are dropped (not
// stringified): they increment the unsupported-attribute counter and their keys
// are returned so the caller can record them via AttrGOWDKDroppedAttributes.
func convertAttributes(attrs []gowdktrace.Attribute) (out []attribute.KeyValue, dropped []string) {
	out = make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		keyValue, ok := convertAttribute(attr)
		if !ok {
			unsupportedAttributes.Add(1)
			dropped = append(dropped, attr.Key)
			continue
		}
		out = append(out, keyValue)
	}
	return out, dropped
}

func convertAttribute(attr gowdktrace.Attribute) (attribute.KeyValue, bool) {
	key := attribute.Key(attr.Key)
	switch value := attr.Value.(type) {
	case string:
		return key.String(value), true
	case bool:
		return key.Bool(value), true
	case int:
		return key.Int(value), true
	case int64:
		return key.Int64(value), true
	case float64:
		return key.Float64(value), true
	case []string:
		return key.StringSlice(value), true
	case []bool:
		return key.BoolSlice(value), true
	case []int:
		return key.IntSlice(value), true
	case []int64:
		return key.Int64Slice(value), true
	case []float64:
		return key.Float64Slice(value), true
	default:
		return attribute.KeyValue{}, false
	}
}
