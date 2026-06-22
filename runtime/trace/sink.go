package trace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
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
//
// ConsoleSink is intended for development and local diagnostics. Span names,
// status messages, source paths, and event messages can derive from request,
// route, or browser-supplied values, so every field is escaped (see
// escapeControl) before it is written: control characters cannot forge an
// additional log line or emit raw terminal control sequences. Use JSONLSink for
// production logging where downstream tooling parses structured records.
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
	line := formatConsoleSpan(span)
	sink.mu.Lock()
	defer sink.mu.Unlock()
	_, err := io.WriteString(sink.writer, line)
	return err
}

// formatConsoleSpan renders one escaped, single-line representation of span.
// Every string field is passed through escapeControl so user- or
// browser-controlled values cannot break out of the line.
func formatConsoleSpan(span Snapshot) string {
	duration := time.Duration(span.DurationNS)
	var builder strings.Builder
	fmt.Fprintf(&builder, "%s trace=%s span=%s parent=%s surface=%s lane=%s duration=%s status=%s",
		escapeControl(span.Name),
		escapeControl(string(span.TraceID)),
		escapeControl(string(span.SpanID)),
		escapeControl(string(span.ParentSpanID)),
		escapeControl(string(span.Surface)),
		escapeControl(string(span.Lane)),
		duration,
		escapeControl(string(span.Status.Code)),
	)
	if message := span.Status.Message; message != "" {
		builder.WriteString(" message=")
		builder.WriteString(escapeControl(message))
	}
	if file := span.Source.File; file != "" {
		fmt.Fprintf(&builder, " source=%s:%d", escapeControl(file), span.Source.Line)
	}
	if len(span.Events) > 0 {
		builder.WriteString(" events=")
		for index, event := range span.Events {
			if index > 0 {
				builder.WriteByte('|')
			}
			builder.WriteString(escapeControl(event.Message))
		}
	}
	builder.WriteByte('\n')
	return builder.String()
}

// escapeControl renders value with control characters replaced by printable
// escape sequences so a span field cannot forge a new console log line or emit
// raw terminal control sequences. Newline, carriage return, and tab become \n,
// \r, \t; the backslash is doubled so escapes stay unambiguous; every other C0
// or C1 control byte and DEL becomes \xNN. Printable text, including spaces and
// non-control Unicode, is preserved so normal span names remain readable.
func escapeControl(value string) string {
	if value == "" {
		return value
	}
	if strings.IndexFunc(value, needsControlEscape) < 0 {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value) + 8)
	for _, r := range value {
		switch r {
		case '\\':
			builder.WriteString(`\\`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			if isControlRune(r) {
				fmt.Fprintf(&builder, `\x%02x`, r)
			} else {
				builder.WriteRune(r)
			}
		}
	}
	return builder.String()
}

func needsControlEscape(r rune) bool {
	return r == '\\' || isControlRune(r)
}

// isControlRune reports whether r is a C0 control (including DEL) or a C1
// control character, all of which can alter terminal state or log structure.
func isControlRune(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f)
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
