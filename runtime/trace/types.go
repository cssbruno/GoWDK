// Package trace provides a dependency-free tracing core for GOWDK Runtime and
// plain Go applications.
package trace

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TraceID is a W3C Trace Context trace-id: 16 bytes encoded as 32 lowercase
// hexadecimal characters.
type TraceID string

// SpanID is a W3C Trace Context parent-id/span-id: 8 bytes encoded as 16
// lowercase hexadecimal characters.
type SpanID string

// Surface identifies where a span was produced.
type Surface string

const (
	SurfaceBackend  Surface = "backend"
	SurfaceFrontend Surface = "frontend"
	SurfaceWorker   Surface = "worker"
)

// Lane identifies the GOWDK execution lane represented by a span.
type Lane string

const (
	LaneRoute    Lane = "route"
	LaneGuard    Lane = "guard"
	LaneHandler  Lane = "handler"
	LaneSSR      Lane = "ssr"
	LaneAction   Lane = "action"
	LaneAPI      Lane = "api"
	LaneFragment Lane = "fragment"
	LaneContract Lane = "contract"
	LaneJob      Lane = "job"
	LaneIsland   Lane = "island"
	LaneNav      Lane = "nav"
	LaneUser     Lane = "user"
)

// StatusCode follows the OpenTelemetry span status shape without importing the
// OpenTelemetry SDK.
type StatusCode string

const (
	StatusUnset StatusCode = "unset"
	StatusOK    StatusCode = "ok"
	StatusError StatusCode = "error"
)

// Status records the final span status.
type Status struct {
	Code    StatusCode `json:"code,omitempty"`
	Message string     `json:"message,omitempty"`
}

// SourceRef points a span back to GOWDK source or generated runtime metadata.
type SourceRef struct {
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	OwnerKind string `json:"ownerKind,omitempty"`
	OwnerID   string `json:"ownerId,omitempty"`
}

// Attribute is an OpenTelemetry-compatible key/value attribute. Supported
// values are string, bool, integer, float64, and homogeneous slices of those
// scalar forms. Trace boundaries copy supported values and drop unsupported
// pointer/map/object values so snapshots remain serialization-safe.
type Attribute struct {
	Key   string `json:"key"`
	Value any    `json:"value,omitempty"`
}

// Event records a timestamped span event.
type Event struct {
	Time       time.Time   `json:"time"`
	Level      string      `json:"level,omitempty"`
	Message    string      `json:"message"`
	Attributes []Attribute `json:"attributes,omitempty"`
}

// Snapshot is the immutable JSON/export shape of a completed span.
type Snapshot struct {
	TraceID      TraceID     `json:"traceId"`
	SpanID       SpanID      `json:"spanId"`
	ParentSpanID SpanID      `json:"parentSpanId,omitempty"`
	Name         string      `json:"name"`
	Surface      Surface     `json:"surface,omitempty"`
	Lane         Lane        `json:"lane,omitempty"`
	Source       SourceRef   `json:"source,omitempty"`
	Attributes   []Attribute `json:"attributes,omitempty"`
	Events       []Event     `json:"events,omitempty"`
	Status       Status      `json:"status,omitempty"`
	StartTime    time.Time   `json:"startTime"`
	EndTime      time.Time   `json:"endTime"`
	DurationNS   int64       `json:"durationNs"`
}

// TraceContext is the trace identity stored in context.Context and encoded in
// W3C traceparent/tracestate headers.
type TraceContext struct {
	TraceID    TraceID
	SpanID     SpanID
	Sampled    bool
	Remote     bool
	TraceState string
}

const (
	// MaxTraceparentHeaderBytes is the maximum traceparent header size accepted
	// by Extract and ParseTraceparent.
	MaxTraceparentHeaderBytes = 256
	// MaxTracestateHeaderBytes is the W3C tracestate list-member byte budget.
	MaxTracestateHeaderBytes = 512
)

const maxTracestateMembers = 32

type (
	contextKey       struct{}
	tracerContextKey struct{}
)

// NewTraceID returns a valid W3C trace ID from the default ID generator
// (CryptoIDGenerator). It returns "" only when the system CSPRNG cannot
// provide entropy; see EntropyFailureCount. It never falls back to a
// predictable value.
func NewTraceID() TraceID {
	return defaultIDGenerator.NewTraceID()
}

// NewSpanID returns a valid W3C span ID from the default ID generator
// (CryptoIDGenerator). It returns "" only when the system CSPRNG cannot
// provide entropy; see EntropyFailureCount. It never falls back to a
// predictable value.
func NewSpanID() SpanID {
	return defaultIDGenerator.NewSpanID()
}

// Valid reports whether id is a valid non-zero W3C trace ID.
func (id TraceID) Valid() bool {
	return validHexID(string(id), 32)
}

// Valid reports whether id is a valid non-zero W3C span ID.
func (id SpanID) Valid() bool {
	return validHexID(string(id), 16)
}

func validHexID(value string, length int) bool {
	if len(value) != length {
		return false
	}
	var nonZero bool
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
		if r != '0' {
			nonZero = true
		}
	}
	return nonZero
}

// TraceContextFromContext returns trace identity from ctx when present.
func TraceContextFromContext(ctx context.Context) (TraceContext, bool) {
	if ctx == nil {
		return TraceContext{}, false
	}
	if span := SpanFrom(ctx); span != nil {
		return span.TraceContext(), true
	}
	traceContext, ok := ctx.Value(contextKey{}).(TraceContext)
	return traceContext, ok && traceContext.TraceID.Valid() && traceContext.SpanID.Valid()
}

// ContextWithTraceContext returns a context carrying trace identity.
func ContextWithTraceContext(ctx context.Context, traceContext TraceContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if !traceContext.TraceID.Valid() || !traceContext.SpanID.Valid() {
		return ctx
	}
	if traceContext.TraceState != "" {
		traceState, err := parseTracestate(traceContext.TraceState)
		if err != nil {
			traceContext.TraceState = ""
		} else {
			traceContext.TraceState = traceState
		}
	}
	return context.WithValue(ctx, contextKey{}, traceContext)
}

// ContextWithTracer returns a context carrying tracer for child spans started
// by helper packages such as runtime/app and runtime/contracts.
func ContextWithTracer(ctx context.Context, tracer *Tracer) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if tracer == nil {
		return ctx
	}
	return context.WithValue(ctx, tracerContextKey{}, tracer)
}

// TracerFromContext returns the tracer stored in ctx, or the tracer that owns
// the active sampled span.
func TracerFromContext(ctx context.Context) (*Tracer, bool) {
	if ctx == nil {
		return nil, false
	}
	if span := SpanFrom(ctx); span != nil {
		if tracer := span.tracerRef(); tracer != nil {
			return tracer, true
		}
	}
	tracer, ok := ctx.Value(tracerContextKey{}).(*Tracer)
	return tracer, ok && tracer != nil
}

// ParseTraceparent parses a W3C traceparent header.
func ParseTraceparent(value string) (TraceContext, error) {
	return ParseTraceContext(value, "")
}

// ParseTraceContext parses W3C traceparent and tracestate headers. Invalid
// traceparent input rejects the context. Invalid tracestate input is dropped so
// a usable trace identity is still preserved without propagating ambiguous
// vendor state.
func ParseTraceContext(traceparent string, tracestate string) (TraceContext, error) {
	traceContext, err := parseTraceparent(traceparent)
	if err != nil {
		return TraceContext{}, err
	}
	if traceState, err := parseTracestate(tracestate); err == nil {
		traceContext.TraceState = traceState
	}
	return traceContext, nil
}

func parseTraceparent(value string) (TraceContext, error) {
	if len(value) > MaxTraceparentHeaderBytes {
		return TraceContext{}, errors.New("traceparent exceeds byte limit")
	}
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 4 {
		return TraceContext{}, errors.New("traceparent must have four dash-separated fields")
	}
	if !validLowerHex(parts[0], 2) {
		return TraceContext{}, errors.New("traceparent version is invalid")
	}
	if parts[0] != "00" {
		return TraceContext{}, fmt.Errorf("unsupported traceparent version %q", parts[0])
	}
	traceID := TraceID(parts[1])
	spanID := SpanID(parts[2])
	if !traceID.Valid() {
		return TraceContext{}, errors.New("traceparent trace id is invalid")
	}
	if !spanID.Valid() {
		return TraceContext{}, errors.New("traceparent span id is invalid")
	}
	if !validLowerHex(parts[3], 2) {
		return TraceContext{}, errors.New("traceparent flags are invalid")
	}
	flags, err := strconv.ParseUint(parts[3], 16, 8)
	if err != nil {
		return TraceContext{}, errors.New("traceparent flags are invalid")
	}
	return TraceContext{TraceID: traceID, SpanID: spanID, Sampled: flags&1 == 1, Remote: true}, nil
}

func parseTracestate(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) > MaxTracestateHeaderBytes {
		return "", errors.New("tracestate exceeds byte limit")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	members := strings.Split(value, ",")
	if len(members) > maxTracestateMembers {
		return "", errors.New("tracestate has too many members")
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(members))
	for _, raw := range members {
		member := strings.TrimSpace(raw)
		if member == "" {
			return "", errors.New("tracestate member is empty")
		}
		key, memberValue, ok := strings.Cut(member, "=")
		if !ok || key == "" {
			return "", errors.New("tracestate member is invalid")
		}
		if key != strings.TrimSpace(key) || memberValue != strings.TrimSpace(memberValue) {
			return "", errors.New("tracestate member whitespace is invalid")
		}
		if !validTracestateKey(key) || !validTracestateValue(memberValue) {
			return "", errors.New("tracestate member is invalid")
		}
		if _, ok := seen[key]; ok {
			return "", errors.New("tracestate member is duplicated")
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key+"="+memberValue)
	}
	return strings.Join(normalized, ","), nil
}

func validTracestateKey(key string) bool {
	if len(key) > 256 {
		return false
	}
	left, right, hasTenant := strings.Cut(key, "@")
	if hasTenant {
		return validTracestateKeyPart(left, false, 241) && validTracestateKeyPart(right, true, 14)
	}
	return validTracestateKeyPart(left, true, 256)
}

func validTracestateKeyPart(value string, requireLetterStart bool, maxLength int) bool {
	if value == "" || len(value) > maxLength {
		return false
	}
	for index, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && (!requireLetterStart || index > 0):
		case index > 0 && (r == '_' || r == '-' || r == '*' || r == '/'):
		default:
			return false
		}
	}
	return true
}

func validTracestateValue(value string) bool {
	if len(value) > 256 {
		return false
	}
	if strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r > 0x7e || r == ',' || r == '=' {
			return false
		}
	}
	return true
}

func validLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// Traceparent encodes traceContext as a W3C traceparent header.
func Traceparent(traceContext TraceContext) string {
	if !traceContext.TraceID.Valid() || !traceContext.SpanID.Valid() {
		return ""
	}
	flags := "00"
	if traceContext.Sampled {
		flags = "01"
	}
	return "00-" + string(traceContext.TraceID) + "-" + string(traceContext.SpanID) + "-" + flags
}

// Carrier is the minimal interface implemented by http.Header and compatible
// request metadata maps.
type Carrier interface {
	Get(string) string
	Set(string, string)
}

// Extract reads W3C traceparent/tracestate headers from carrier into ctx.
func Extract(ctx context.Context, carrier Carrier) context.Context {
	if carrier == nil {
		return ctx
	}
	traceContext, err := ParseTraceContext(carrier.Get("traceparent"), carrier.Get("tracestate"))
	if err != nil {
		return ctx
	}
	return ContextWithTraceContext(ctx, traceContext)
}

// Inject writes the active span or trace context from ctx into carrier.
func Inject(ctx context.Context, carrier Carrier) {
	if carrier == nil {
		return
	}
	traceContext, ok := TraceContextFromContext(ctx)
	if !ok {
		return
	}
	if value := Traceparent(traceContext); value != "" {
		carrier.Set("traceparent", value)
	}
	if traceState, err := parseTracestate(traceContext.TraceState); err == nil && traceState != "" {
		carrier.Set("tracestate", traceState)
	}
}
