// Package otel bridges GOWDK's dependency-free runtime trace snapshots to the
// OpenTelemetry SDK and OTLP exporters.
package otel

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"sort"
	"sync/atomic"
	"time"

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

// RetryConfig bounds the exporter's transport-level retry/backoff for transient
// OTLP failures. It mirrors the OpenTelemetry exporter retry contract so an app
// that owns its own provider can reason about the same knobs.
type RetryConfig struct {
	Enabled         bool
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxElapsedTime  time.Duration
}

type config struct {
	endpoint    string
	insecure    bool
	gzip        bool
	headers     map[string]string
	tlsConfig   *tls.Config
	timeout     time.Duration
	retry       *RetryConfig
	serviceName string
	serviceVer  string
	environment string
	resourceKVs map[string]string

	maxQueueSize       int
	maxExportBatchSize int
	batchTimeout       time.Duration
	exportTimeout      time.Duration
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

// WithTLSClientConfig sets the TLS client configuration (custom CA bundle,
// client certificate, or server name) for OTLP over HTTPS, beyond the
// insecure/default-TLS choice.
func WithTLSClientConfig(tlsConfig *tls.Config) Option {
	return func(config *config) {
		config.tlsConfig = tlsConfig
	}
}

// WithGzip enables gzip request compression for OTLP HTTP.
func WithGzip() Option {
	return func(config *config) {
		config.gzip = true
	}
}

// WithTimeout sets the per-export request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(config *config) {
		if timeout > 0 {
			config.timeout = timeout
		}
	}
}

// WithRetry sets bounded transport retry/backoff for transient OTLP failures.
func WithRetry(retry RetryConfig) Option {
	return func(config *config) {
		value := retry
		config.retry = &value
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

// WithServiceName sets the service.name resource attribute. When unset the
// default GOWDK service name is used.
func WithServiceName(name string) Option {
	return func(config *config) {
		config.serviceName = name
	}
}

// WithServiceVersion sets the service.version resource attribute.
func WithServiceVersion(version string) Option {
	return func(config *config) {
		config.serviceVer = version
	}
}

// WithEnvironment sets the deployment.environment resource attribute (for
// example "production" or "staging").
func WithEnvironment(environment string) Option {
	return func(config *config) {
		config.environment = environment
	}
}

// WithResourceAttributes adds arbitrary resource attributes to every exported
// span's resource, for common OTLP deployment metadata.
func WithResourceAttributes(attrs map[string]string) Option {
	return func(config *config) {
		if len(attrs) == 0 {
			return
		}
		if config.resourceKVs == nil {
			config.resourceKVs = map[string]string{}
		}
		for key, value := range attrs {
			config.resourceKVs[key] = value
		}
	}
}

// WithMaxQueueSize bounds the in-memory span queue. When the queue is full,
// further spans are dropped by the batch processor rather than growing memory
// without bound. Pair with ForceFlush/Shutdown to drain on graceful exit.
func WithMaxQueueSize(size int) Option {
	return func(config *config) {
		if size > 0 {
			config.maxQueueSize = size
		}
	}
}

// WithMaxExportBatchSize bounds the number of spans sent per export request.
func WithMaxExportBatchSize(size int) Option {
	return func(config *config) {
		if size > 0 {
			config.maxExportBatchSize = size
		}
	}
}

// WithBatchTimeout sets the maximum delay before a non-full batch is exported.
func WithBatchTimeout(timeout time.Duration) Option {
	return func(config *config) {
		if timeout > 0 {
			config.batchTimeout = timeout
		}
	}
}

// WithExportTimeout sets the deadline for exporting one batch.
func WithExportTimeout(timeout time.Duration) Option {
	return func(config *config) {
		if timeout > 0 {
			config.exportTimeout = timeout
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

// exporterFailures counts OTLP export attempts that failed after the exporter's
// own retry/backoff was exhausted, for a GOWDK-owned provider.
var exporterFailures atomic.Uint64

// ExporterFailureCount reports how many OTLP export batches failed to send from
// a GOWDK-owned provider (NewSink). It surfaces export loss that would otherwise
// be visible only through the global OpenTelemetry error handler.
func ExporterFailureCount() uint64 {
	return exporterFailures.Load()
}

// countingExporter wraps an OTLP span exporter and counts batches that fail to
// export after retries, so export loss is observable via ExporterFailureCount.
type countingExporter struct {
	inner sdktrace.SpanExporter
}

func (exporter countingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if err := exporter.inner.ExportSpans(ctx, spans); err != nil {
		exporterFailures.Add(1)
		return err
	}
	return nil
}

func (exporter countingExporter) Shutdown(ctx context.Context) error {
	return exporter.inner.Shutdown(ctx)
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
	exporter, err := otlptracehttp.New(ctx, exporterOptionsFor(cfg)...)
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(countingExporter{inner: exporter}, batchOptionsFor(cfg)...),
		sdktrace.WithIDGenerator(SnapshotIDGenerator{}),
		sdktrace.WithResource(buildResource(cfg)),
	)
	sink := newSink(provider)
	sink.ownsProvider = true
	sink.preservesIdentity = true
	return sink, nil
}

// exporterOptionsFor builds the OTLP HTTP exporter options from the bridge
// config: endpoint, TLS (insecure or a custom client config), headers,
// compression, per-request timeout, and bounded retry/backoff.
func exporterOptionsFor(cfg config) []otlptracehttp.Option {
	options := []otlptracehttp.Option{}
	if cfg.endpoint != "" {
		options = append(options, otlptracehttp.WithEndpoint(cfg.endpoint))
	}
	if cfg.insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	if cfg.tlsConfig != nil {
		options = append(options, otlptracehttp.WithTLSClientConfig(cfg.tlsConfig))
	}
	if len(cfg.headers) > 0 {
		options = append(options, otlptracehttp.WithHeaders(cfg.headers))
	}
	if cfg.gzip {
		options = append(options, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
	}
	if cfg.timeout > 0 {
		options = append(options, otlptracehttp.WithTimeout(cfg.timeout))
	}
	if cfg.retry != nil {
		options = append(options, otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled:         cfg.retry.Enabled,
			InitialInterval: cfg.retry.InitialInterval,
			MaxInterval:     cfg.retry.MaxInterval,
			MaxElapsedTime:  cfg.retry.MaxElapsedTime,
		}))
	}
	return options
}

// batchOptionsFor builds the bounded batch-processor options: queue cap, batch
// size, batch delay, and per-export deadline.
func batchOptionsFor(cfg config) []sdktrace.BatchSpanProcessorOption {
	options := []sdktrace.BatchSpanProcessorOption{}
	if cfg.maxQueueSize > 0 {
		options = append(options, sdktrace.WithMaxQueueSize(cfg.maxQueueSize))
	}
	if cfg.maxExportBatchSize > 0 {
		options = append(options, sdktrace.WithMaxExportBatchSize(cfg.maxExportBatchSize))
	}
	if cfg.batchTimeout > 0 {
		options = append(options, sdktrace.WithBatchTimeout(cfg.batchTimeout))
	}
	if cfg.exportTimeout > 0 {
		options = append(options, sdktrace.WithExportTimeout(cfg.exportTimeout))
	}
	return options
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

// buildResource assembles the GOWDK-owned provider's resource from the config:
// service name (defaulting to the GOWDK service name), optional service version
// and deployment environment, and any custom resource attributes. Keys are
// applied in a stable order so the resource is deterministic.
func buildResource(cfg config) *resource.Resource {
	name := cfg.serviceName
	if name == "" {
		name = defaultServiceName
	}
	attrs := []attribute.KeyValue{semconv.ServiceName(name)}
	if cfg.serviceVer != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.serviceVer))
	}
	if cfg.environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment", cfg.environment))
	}
	keys := make([]string, 0, len(cfg.resourceKVs))
	for key := range cfg.resourceKVs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = append(attrs, attribute.String(key, cfg.resourceKVs[key]))
	}
	merged, err := resource.Merge(resource.Default(), resource.NewSchemaless(attrs...))
	if err != nil {
		return resource.NewSchemaless(attrs...)
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

// ForceFlush drains any spans buffered in the batch processor without shutting
// the provider down, so callers can flush at a checkpoint (before a deploy, on a
// signal, after a critical request) and keep tracing. Unlike Shutdown it is safe
// on a borrowed provider: flushing pending spans does not stop it.
func (sink *Sink) ForceFlush(ctx context.Context) error {
	if sink == nil || sink.provider == nil {
		return nil
	}
	return sink.provider.ForceFlush(ctx)
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
		gowdktrace.LaneAction, gowdktrace.LaneAPI, gowdktrace.LaneFragment:
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
