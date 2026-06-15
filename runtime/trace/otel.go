package trace

import "context"

const (
	AttrGOWDKSurface           = "gowdk.surface"
	AttrGOWDKLane              = "gowdk.lane"
	AttrGOWDKSourceFile        = "gowdk.source.file"
	AttrGOWDKSourceLine        = "gowdk.source.line"
	AttrGOWDKSourceCol         = "gowdk.source.column"
	AttrHTTPRoute              = "http.route"
	AttrHTTPRequestMethod      = "http.request.method"
	AttrHTTPResponseStatusCode = "http.response.status_code"
	AttrURLPath                = "url.path"
	AttrServerAddress          = "server.address"
)

// OTLPSpan is intentionally shaped like the OTLP span model while remaining a
// dependency-free value type.
type OTLPSpan struct {
	TraceID           TraceID     `json:"traceId"`
	SpanID            SpanID      `json:"spanId"`
	ParentSpanID      SpanID      `json:"parentSpanId,omitempty"`
	Name              string      `json:"name"`
	StartTimeUnixNano int64       `json:"startTimeUnixNano"`
	EndTimeUnixNano   int64       `json:"endTimeUnixNano"`
	Attributes        []Attribute `json:"attributes,omitempty"`
	Events            []Event     `json:"events,omitempty"`
	Status            Status      `json:"status,omitempty"`
}

// Exporter receives completed spans in the dependency-free OTLP-like shape.
type Exporter interface {
	ExportSpans(context.Context, []OTLPSpan) error
}

// ExporterSink adapts an Exporter into a Sink.
func ExporterSink(exporter Exporter) Sink {
	return sinkFunc(func(ctx context.Context, span Snapshot) error {
		if exporter == nil {
			return nil
		}
		return exporter.ExportSpans(ctx, []OTLPSpan{OTLPSpanFromSnapshot(span)})
	})
}

// OTLPSpanFromSnapshot converts a completed span to the OTLP-like model.
func OTLPSpanFromSnapshot(span Snapshot) OTLPSpan {
	return OTLPSpan{
		TraceID:           span.TraceID,
		SpanID:            span.SpanID,
		ParentSpanID:      span.ParentSpanID,
		Name:              span.Name,
		StartTimeUnixNano: span.StartTime.UnixNano(),
		EndTimeUnixNano:   span.EndTime.UnixNano(),
		Attributes:        append([]Attribute(nil), span.Attributes...),
		Events:            append([]Event(nil), span.Events...),
		Status:            span.Status,
	}
}
