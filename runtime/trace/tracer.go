package trace

import (
	"context"
	"sync/atomic"
	"time"
)

var defaultTracer = NewTracer()

// Tracer owns sampling, identity generation, and completed-span delivery.
type Tracer struct {
	sink    Sink
	sampler Sampler
	idGen   IDGenerator
}

// TracerOption configures a Tracer.
type TracerOption func(*Tracer)

// WithSink sets the completed-span sink.
func WithSink(sink Sink) TracerOption {
	return func(tracer *Tracer) {
		tracer.sink = sink
	}
}

// WithSampler sets the sampler. Nil means AlwaysOn.
func WithSampler(sampler Sampler) TracerOption {
	return func(tracer *Tracer) {
		tracer.sampler = sampler
	}
}

// WithIDGenerator sets the trace/span ID generator. Nil keeps the default
// CryptoIDGenerator. Tests use this to inject deterministic identifiers without
// relying on entropy failure.
func WithIDGenerator(generator IDGenerator) TracerOption {
	return func(tracer *Tracer) {
		if generator != nil {
			tracer.idGen = generator
		}
	}
}

// NewTracer creates a Tracer.
func NewTracer(options ...TracerOption) *Tracer {
	tracer := &Tracer{sampler: AlwaysOn(), idGen: defaultIDGenerator}
	for _, option := range options {
		option(tracer)
	}
	if tracer.sampler == nil {
		tracer.sampler = AlwaysOn()
	}
	if tracer.idGen == nil {
		tracer.idGen = defaultIDGenerator
	}
	return tracer
}

// Start starts a span. If sampling rejects the span, the returned context is
// unchanged and the returned span is nil.
func (tracer *Tracer) Start(ctx context.Context, name string, options ...StartOption) (context.Context, *Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	if tracer == nil {
		tracer = defaultTracer
	}
	if len(options) == 0 {
		if static, ok := tracer.sampler.(staticSampler); ok && !static.value {
			return ctx, nil
		}
	}
	cfg := startConfig{tracer: tracer, start: time.Now().UTC()}
	for _, option := range options {
		option(&cfg)
	}
	if cfg.tracer == nil {
		cfg.tracer = tracer
	}
	if static, ok := cfg.tracer.sampler.(staticSampler); ok && !static.value {
		return ctx, nil
	}
	parent, hasParent := TraceContextFromContext(ctx)
	generator := cfg.tracer.idGen
	if generator == nil {
		generator = defaultIDGenerator
	}
	traceID := parent.TraceID
	if !traceID.Valid() {
		traceID = generator.NewTraceID()
	}
	spanID := generator.NewSpanID()
	if !traceID.Valid() || !spanID.Valid() {
		// Identity could not be generated (an entropy failure on the default
		// generator). Drop the span rather than emit a predictable identifier.
		// The loss is observable through EntropyFailureCount / the handler.
		return ctx, nil
	}
	samplingContext := SamplingContext{
		TraceID:      traceID,
		ParentSpanID: parent.SpanID,
		Name:         name,
		Surface:      cfg.surface,
		Lane:         cfg.lane,
		Attributes:   append([]Attribute(nil), cfg.attributes...),
	}
	if cfg.tracer.sampler != nil && !cfg.tracer.sampler.Sample(samplingContext) {
		return ctx, nil
	}
	ctx = ContextWithTracer(ctx, cfg.tracer)
	span := &Span{
		tracer:     cfg.tracer,
		traceID:    traceID,
		spanID:     spanID,
		name:       name,
		surface:    cfg.surface,
		lane:       cfg.lane,
		source:     cfg.source,
		attributes: append([]Attribute(nil), cfg.attributes...),
		status:     Status{Code: StatusUnset},
		start:      cfg.start,
	}
	if hasParent {
		span.parentSpanID = parent.SpanID
	}
	ctx = context.WithValue(ctx, spanContextKey{}, span)
	ctx = ContextWithTraceContext(ctx, TraceContext{TraceID: traceID, SpanID: spanID, Sampled: true})
	return ctx, span
}

type startConfig struct {
	tracer     *Tracer
	surface    Surface
	lane       Lane
	source     SourceRef
	attributes []Attribute
	start      time.Time
}

// StartOption configures one span.
type StartOption func(*startConfig)

// WithTracer starts a span with tracer instead of the default tracer.
func WithTracer(tracer *Tracer) StartOption {
	return func(cfg *startConfig) {
		if tracer != nil {
			cfg.tracer = tracer
		}
	}
}

// WithSurface records a span surface.
func WithSurface(surface Surface) StartOption {
	return func(cfg *startConfig) {
		cfg.surface = surface
	}
}

// WithLane records a span lane.
func WithLane(lane Lane) StartOption {
	return func(cfg *startConfig) {
		cfg.lane = lane
	}
}

// WithSource records a source reference.
func WithSource(source SourceRef) StartOption {
	return func(cfg *startConfig) {
		cfg.source = source
	}
}

// WithAttributes records initial span attributes.
func WithAttributes(attrs map[string]any) StartOption {
	return func(cfg *startConfig) {
		cfg.attributes = append(cfg.attributes, attributesFromMap(attrs)...)
	}
}

// WithStartTime sets a deterministic start time.
func WithStartTime(start time.Time) StartOption {
	return func(cfg *startConfig) {
		if !start.IsZero() {
			cfg.start = start
		}
	}
}

// SamplingContext is passed to samplers before a span is allocated.
type SamplingContext struct {
	TraceID      TraceID
	ParentSpanID SpanID
	Name         string
	Surface      Surface
	Lane         Lane
	Attributes   []Attribute
}

// Sampler decides whether a span should be recorded.
type Sampler interface {
	Sample(SamplingContext) bool
}

type samplerFunc func(SamplingContext) bool

func (fn samplerFunc) Sample(ctx SamplingContext) bool {
	return fn(ctx)
}

type staticSampler struct {
	value bool
}

func (sampler staticSampler) Sample(SamplingContext) bool {
	return sampler.value
}

// AlwaysOn samples every span.
func AlwaysOn() Sampler {
	return staticSampler{value: true}
}

// AlwaysOff samples no spans.
func AlwaysOff() Sampler {
	return staticSampler{value: false}
}

// RatioSampler returns a deterministic trace-id based sampler. Ratios <= 0 are
// always off; ratios >= 1 are always on.
func RatioSampler(ratio float64) Sampler {
	switch {
	case ratio <= 0:
		return AlwaysOff()
	case ratio >= 1:
		return AlwaysOn()
	default:
		threshold := uint64(ratio * float64(^uint64(0)))
		return samplerFunc(func(ctx SamplingContext) bool {
			if !ctx.TraceID.Valid() {
				return false
			}
			bytes := []byte(ctx.TraceID)
			var value uint64
			for i := 0; i < 16; i++ {
				value <<= 4
				switch c := bytes[i]; {
				case c >= '0' && c <= '9':
					value |= uint64(c - '0')
				case c >= 'a' && c <= 'f':
					value |= uint64(c-'a') + 10
				}
			}
			return value <= threshold
		})
	}
}

// CountedSampler is a deterministic test helper that samples every n-th span.
type CountedSampler struct {
	N     uint64
	count atomic.Uint64
}

// Sample implements Sampler.
func (sampler *CountedSampler) Sample(SamplingContext) bool {
	if sampler == nil || sampler.N == 0 {
		return false
	}
	return sampler.count.Add(1)%sampler.N == 0
}
