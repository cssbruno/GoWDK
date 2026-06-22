package trace

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cssbruno/gowdk/runtime/security"
)

type spanContextKey struct{}

const defaultSinkTimeout = 5 * time.Second

// SinkLogger receives completed-span export failures. Set it to nil to silence
// sink failure logging. It defaults to the standard log package.
var SinkLogger func(message string) = func(message string) {
	log.Print(message)
}

// Span records one sampled unit of work. Methods are nil-safe so callers can
// defer span.End() even when sampling is disabled.
type Span struct {
	mu           sync.Mutex
	tracer       *Tracer
	traceID      TraceID
	spanID       SpanID
	parentSpanID SpanID
	name         string
	surface      Surface
	lane         Lane
	source       SourceRef
	attributes   []Attribute
	events       []Event
	status       Status
	start        time.Time
	end          time.Time
	ended        bool
}

// Start starts a span using the default tracer unless WithTracer is supplied.
func Start(ctx context.Context, name string, options ...StartOption) (context.Context, *Span) {
	if tracer, ok := TracerFromContext(ctx); ok {
		return tracer.Start(ctx, name, options...)
	}
	return defaultTracer.Start(ctx, name, options...)
}

// SpanFrom returns the active sampled span from ctx.
func SpanFrom(ctx context.Context) *Span {
	if ctx == nil {
		return nil
	}
	span, _ := ctx.Value(spanContextKey{}).(*Span)
	return span
}

// End completes the span and sends an immutable snapshot to the configured
// sink. Calling End more than once is safe.
func (span *Span) End() {
	if span == nil {
		return
	}
	span.EndTime(time.Now().UTC())
}

// EndTime completes the span at t. It is useful for deterministic tests.
func (span *Span) EndTime(t time.Time) {
	if span == nil {
		return
	}
	var snapshot Snapshot
	var sink Sink
	span.mu.Lock()
	if span.ended {
		span.mu.Unlock()
		return
	}
	span.ended = true
	span.end = t
	snapshot = span.snapshotLocked()
	if span.tracer != nil {
		sink = span.tracer.sink
	}
	span.mu.Unlock()
	if sink != nil {
		recordSpanAsync(sink, snapshot)
	}
}

func recordSpanAsync(sink Sink, snapshot Snapshot) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				logSinkFailure(fmt.Errorf("panic: %v", recovered))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), defaultSinkTimeout)
		defer cancel()
		if err := sink.RecordSpan(ctx, snapshot); err != nil {
			logSinkFailure(err)
		}
	}()
}

func logSinkFailure(err error) {
	if err == nil || SinkLogger == nil {
		return
	}
	SinkLogger("gowdk trace: sink failed: " + security.RedactSecrets(err.Error()))
}

// Event records a timestamped event on the span.
func (span *Span) Event(level string, message string, attrs map[string]any) {
	if span == nil || message == "" {
		return
	}
	span.mu.Lock()
	defer span.mu.Unlock()
	if span.ended {
		return
	}
	span.events = append(span.events, Event{
		Time:       time.Now().UTC(),
		Level:      level,
		Message:    message,
		Attributes: attributesFromMap(attrs),
	})
}

// Set records or replaces one span attribute.
func (span *Span) Set(key string, value any) {
	if span == nil || key == "" {
		return
	}
	span.mu.Lock()
	defer span.mu.Unlock()
	if span.ended {
		return
	}
	for index := range span.attributes {
		if span.attributes[index].Key == key {
			if normalized, ok := normalizeAttribute(Attribute{Key: key, Value: value}); ok {
				span.attributes[index] = normalized
			}
			return
		}
	}
	if normalized, ok := normalizeAttribute(Attribute{Key: key, Value: value}); ok {
		span.attributes = append(span.attributes, normalized)
	}
}

// SetStatus records the final span status.
func (span *Span) SetStatus(code StatusCode, message string) {
	if span == nil {
		return
	}
	span.mu.Lock()
	defer span.mu.Unlock()
	if span.ended {
		return
	}
	span.status = Status{Code: code, Message: message}
}

// TraceContext returns the span's W3C trace identity.
func (span *Span) TraceContext() TraceContext {
	if span == nil {
		return TraceContext{}
	}
	span.mu.Lock()
	defer span.mu.Unlock()
	return TraceContext{TraceID: span.traceID, SpanID: span.spanID, Sampled: true}
}

func (span *Span) tracerRef() *Tracer {
	if span == nil {
		return nil
	}
	span.mu.Lock()
	defer span.mu.Unlock()
	return span.tracer
}

// Snapshot returns an immutable copy of the span's current state.
func (span *Span) Snapshot() Snapshot {
	if span == nil {
		return Snapshot{}
	}
	span.mu.Lock()
	defer span.mu.Unlock()
	return span.snapshotLocked()
}

func (span *Span) snapshotLocked() Snapshot {
	end := span.end
	if end.IsZero() {
		end = time.Now().UTC()
	}
	duration := end.Sub(span.start)
	if duration < 0 {
		duration = 0
	}
	// Normalize the source path here, the single point every completed-span
	// snapshot is created, so the viewer, JSON/SSE, console, and OTLP surfaces
	// never observe an absolute local filesystem path by default.
	return cloneSnapshot(Snapshot{
		TraceID:      span.traceID,
		SpanID:       span.spanID,
		ParentSpanID: span.parentSpanID,
		Name:         span.name,
		Surface:      span.surface,
		Lane:         span.lane,
		Source:       span.source,
		Attributes:   span.attributes,
		Events:       span.events,
		Status:       span.status,
		StartTime:    span.start,
		EndTime:      end,
		DurationNS:   duration.Nanoseconds(),
	})
}

func attributesFromMap(attrs map[string]any) []Attribute {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]Attribute, 0, len(attrs))
	for key, value := range attrs {
		if key == "" {
			continue
		}
		out = append(out, Attribute{Key: key, Value: value})
	}
	return cloneAttributes(out)
}
