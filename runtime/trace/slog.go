package trace

import (
	"context"
	"log/slog"
)

const (
	SlogTraceIDKey = "trace_id"
	SlogSpanIDKey  = "span_id"
)

// SlogAttrs returns stable slog attributes for the active trace context. It
// returns nil when ctx has no valid trace identity.
func SlogAttrs(ctx context.Context) []slog.Attr {
	traceContext, ok := TraceContextFromContext(ctx)
	if !ok {
		return nil
	}
	return []slog.Attr{
		slog.String(SlogTraceIDKey, string(traceContext.TraceID)),
		slog.String(SlogSpanIDKey, string(traceContext.SpanID)),
	}
}

// SlogArgs returns trace/span attributes as alternating key/value args for
// slog.Logger methods.
func SlogArgs(ctx context.Context) []any {
	attrs := SlogAttrs(ctx)
	if len(attrs) == 0 {
		return nil
	}
	args := make([]any, 0, len(attrs)*2)
	for _, attr := range attrs {
		args = append(args, attr.Key, attr.Value.String())
	}
	return args
}
