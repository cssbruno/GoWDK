package contracts

import (
	"context"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

func traceparentFromContext(ctx context.Context) string {
	traceContext, ok := gowdktrace.TraceContextFromContext(ctx)
	if !ok {
		return ""
	}
	return gowdktrace.Traceparent(traceContext)
}

func traceOnlyContext(ctx context.Context) context.Context {
	traceCtx := context.Background()
	if tracer, ok := gowdktrace.TracerFromContext(ctx); ok {
		traceCtx = gowdktrace.ContextWithTracer(traceCtx, tracer)
	}
	if traceContext, ok := gowdktrace.TraceContextFromContext(ctx); ok {
		traceCtx = gowdktrace.ContextWithTraceContext(traceCtx, traceContext)
	}
	return traceCtx
}

func contextWithEventTraceparent(ctx context.Context, event EventEnvelope) context.Context {
	if event.TraceParent == "" {
		return ctx
	}
	return gowdktrace.Extract(ctx, traceparentCarrier(event.TraceParent))
}

type traceparentCarrier string

func (carrier traceparentCarrier) Get(key string) string {
	if key == "traceparent" {
		return string(carrier)
	}
	return ""
}

func (carrier traceparentCarrier) Set(string, string) {}

func startContractSpan(ctx context.Context, name string, lane gowdktrace.Lane, attrs map[string]any) (context.Context, *gowdktrace.Span) {
	if _, ok := gowdktrace.TracerFromContext(ctx); !ok {
		return ctx, nil
	}
	return gowdktrace.Start(ctx, name,
		gowdktrace.WithSurface(gowdktrace.SurfaceBackend),
		gowdktrace.WithLane(lane),
		gowdktrace.WithAttributes(attrs),
	)
}

func finishContractSpan(span *gowdktrace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.SetStatus(gowdktrace.StatusError, err.Error())
	} else {
		span.SetStatus(gowdktrace.StatusOK, "")
	}
	span.End()
}
