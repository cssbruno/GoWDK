// Package otel bridges GOWDK's dependency-free runtime trace snapshots to the
// OpenTelemetry SDK and OTLP exporters.
package otel

import (
	"context"
	"fmt"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
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

// Sink records GOWDK spans through an OpenTelemetry TracerProvider.
type Sink struct {
	provider *sdktrace.TracerProvider
	tracer   oteltrace.Tracer
}

// NewSink creates an OTLP HTTP-backed sink.
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
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	return NewSinkWithProvider(provider), nil
}

// NewSinkWithProvider creates a sink backed by an existing provider. It is
// useful when applications already own OpenTelemetry lifecycle and resources.
func NewSinkWithProvider(provider *sdktrace.TracerProvider) *Sink {
	if provider == nil {
		provider = sdktrace.NewTracerProvider()
	}
	return &Sink{provider: provider, tracer: provider.Tracer("github.com/cssbruno/gowdk/runtime/trace")}
}

// Shutdown flushes and closes the underlying TracerProvider.
func (sink *Sink) Shutdown(ctx context.Context) error {
	if sink == nil || sink.provider == nil {
		return nil
	}
	return sink.provider.Shutdown(ctx)
}

// RecordSpan implements gowdktrace.Sink.
func (sink *Sink) RecordSpan(ctx context.Context, span gowdktrace.Snapshot) error {
	if sink == nil || sink.tracer == nil {
		return nil
	}
	parent, err := parentContext(ctx, span)
	if err != nil {
		return err
	}
	attrs := spanAttributes(span)
	_, otelSpan := sink.tracer.Start(parent, span.Name,
		oteltrace.WithTimestamp(span.StartTime),
		oteltrace.WithAttributes(attrs...),
	)
	for _, event := range span.Events {
		otelSpan.AddEvent(event.Message,
			oteltrace.WithTimestamp(event.Time),
			oteltrace.WithAttributes(attributes(event.Attributes)...),
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

func spanAttributes(span gowdktrace.Snapshot) []attribute.KeyValue {
	attrs := attributes(span.Attributes)
	attrs = append(attrs,
		attribute.String("gowdk.trace_id", string(span.TraceID)),
		attribute.String("gowdk.span_id", string(span.SpanID)),
		attribute.String("gowdk.parent_span_id", string(span.ParentSpanID)),
		attribute.String("gowdk.surface", string(span.Surface)),
		attribute.String("gowdk.lane", string(span.Lane)),
		attribute.String("gowdk.source.file", span.Source.File),
		attribute.Int("gowdk.source.line", span.Source.Line),
		attribute.Int("gowdk.source.column", span.Source.Column),
		attribute.String("gowdk.source.owner_kind", span.Source.OwnerKind),
		attribute.String("gowdk.source.owner_id", span.Source.OwnerID),
	)
	return attrs
}

func attributes(attrs []gowdktrace.Attribute) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		key := attribute.Key(attr.Key)
		switch value := attr.Value.(type) {
		case string:
			out = append(out, key.String(value))
		case bool:
			out = append(out, key.Bool(value))
		case int:
			out = append(out, key.Int(value))
		case int64:
			out = append(out, key.Int64(value))
		case float64:
			out = append(out, key.Float64(value))
		default:
			out = append(out, key.String(fmt.Sprint(value)))
		}
	}
	return out
}
