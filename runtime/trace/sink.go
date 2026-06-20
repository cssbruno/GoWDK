package trace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// Sink receives completed span snapshots.
type Sink interface {
	RecordSpan(context.Context, Snapshot) error
}

type sinkFunc func(context.Context, Snapshot) error

func (fn sinkFunc) RecordSpan(ctx context.Context, span Snapshot) error {
	return fn(ctx, span)
}

// MultiSink sends completed spans to each sink in order.
func MultiSink(sinks ...Sink) Sink {
	copied := append([]Sink(nil), sinks...)
	return sinkFunc(func(ctx context.Context, span Snapshot) error {
		var joined error
		for _, sink := range copied {
			if sink == nil {
				continue
			}
			if err := sink.RecordSpan(ctx, span); err != nil {
				joined = errors.Join(joined, err)
			}
		}
		return joined
	})
}

// ConsoleSink writes one readable line per completed span.
type ConsoleSink struct {
	mu     sync.Mutex
	writer io.Writer
}

// NewConsoleSink creates a console sink.
func NewConsoleSink(writer io.Writer) *ConsoleSink {
	return &ConsoleSink{writer: writer}
}

// RecordSpan implements Sink.
func (sink *ConsoleSink) RecordSpan(ctx context.Context, span Snapshot) error {
	if sink == nil || sink.writer == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	duration := time.Duration(span.DurationNS)
	sink.mu.Lock()
	defer sink.mu.Unlock()
	_, err := fmt.Fprintf(sink.writer, "%s trace=%s span=%s parent=%s surface=%s lane=%s duration=%s status=%s\n", span.Name, span.TraceID, span.SpanID, span.ParentSpanID, span.Surface, span.Lane, duration, span.Status.Code)
	return err
}

// JSONLSink writes one JSON object per completed span.
type JSONLSink struct {
	mu     sync.Mutex
	writer io.Writer
}

// NewJSONLSink creates a JSON Lines sink.
func NewJSONLSink(writer io.Writer) *JSONLSink {
	return &JSONLSink{writer: writer}
}

// RecordSpan implements Sink.
func (sink *JSONLSink) RecordSpan(ctx context.Context, span Snapshot) error {
	if sink == nil || sink.writer == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	payload, err := json.Marshal(span)
	if err != nil {
		return err
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if _, err := sink.writer.Write(payload); err != nil {
		return err
	}
	_, err = sink.writer.Write([]byte("\n"))
	return err
}
