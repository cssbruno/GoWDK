package trace

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"
)

var defaultTracer = NewTracer()

// Tracer owns sampling, identity generation, and completed-span delivery.
type Tracer struct {
	sink           Sink
	sampler        Sampler
	idGen          IDGenerator
	sampledSpans   atomic.Uint64
	exportedSpans  atomic.Uint64
	exportFailures atomic.Uint64
	lastExportNS   atomic.Int64
	maxExportNS    atomic.Int64
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

// Start starts a span. If sampling rejects the span the returned span is nil.
// A dynamic sampler's rejection still propagates the generated trace identity
// with Sampled:false on the returned context, so a descendant started from it
// inherits the drop (parent-based sampling keeps an unsampled trace whole)
// instead of re-rolling as a new root. A statically-off tracer (AlwaysOff)
// short-circuits earlier and returns the context unchanged without allocating.
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
	traceState := parent.TraceState
	if !traceID.Valid() {
		traceID = generator.NewTraceID()
		traceState = ""
	}
	spanID := generator.NewSpanID()
	if !traceID.Valid() || !spanID.Valid() {
		// Identity could not be generated (an entropy failure on the default
		// generator). Drop the span rather than emit a predictable identifier.
		// The loss is observable through EntropyFailureCount / the handler.
		return ctx, nil
	}
	samplingContext := SamplingContext{
		TraceID:       traceID,
		ParentSpanID:  parent.SpanID,
		HasParent:     hasParent,
		ParentSampled: hasParent && parent.Sampled,
		Name:          name,
		Surface:       cfg.surface,
		Lane:          cfg.lane,
		Attributes:    cloneAttributes(cfg.attributes),
	}
	if cfg.tracer.sampler != nil && !cfg.tracer.sampler.Sample(samplingContext) {
		// Sampling rejected this span. Propagate the generated identity with
		// Sampled:false so a descendant started from the returned context
		// inherits the drop rather than re-rolling as a new root. No tracer is
		// attached and no span is allocated, so the dropped span is never
		// recorded or exported.
		return ContextWithTraceContext(ctx, TraceContext{TraceID: traceID, SpanID: spanID, Sampled: false, TraceState: traceState}), nil
	}
	cfg.tracer.sampledSpans.Add(1)
	ctx = ContextWithTracer(ctx, cfg.tracer)
	span := &Span{
		tracer:     cfg.tracer,
		traceID:    traceID,
		spanID:     spanID,
		traceState: traceState,
		name:       name,
		surface:    cfg.surface,
		lane:       cfg.lane,
		source:     cfg.source,
		attributes: cloneAttributes(cfg.attributes),
		status:     Status{Code: StatusUnset},
		start:      cfg.start,
	}
	if hasParent {
		span.parentSpanID = parent.SpanID
	}
	ctx = context.WithValue(ctx, spanContextKey{}, span)
	ctx = ContextWithTraceContext(ctx, TraceContext{TraceID: traceID, SpanID: spanID, Sampled: true, TraceState: traceState})
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

// SamplingContext is passed to samplers before a span is allocated. HasParent
// and ParentSampled describe the propagated parent decision so a parent-based
// sampler can keep a trace whole: every span in a sampled trace is kept, and
// every span in an unsampled trace is dropped.
type SamplingContext struct {
	TraceID       TraceID
	ParentSpanID  SpanID
	HasParent     bool
	ParentSampled bool
	Name          string
	Surface       Surface
	Lane          Lane
	Attributes    []Attribute
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

func (sampler staticSampler) description() string {
	if sampler.value {
		return "always_on"
	}
	return "always_off"
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
		return ratioSampler{ratio: ratio, threshold: threshold}
	}
}

type ratioSampler struct {
	ratio     float64
	threshold uint64
}

func (sampler ratioSampler) Sample(ctx SamplingContext) bool {
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
	return value <= sampler.threshold
}

func (sampler ratioSampler) description() string {
	return "ratio"
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

func (sampler *CountedSampler) description() string {
	return "counted"
}

// TracerHealthSnapshot is a dependency-free point-in-time view of local tracer
// sampling and sink export health.
type TracerHealthSnapshot struct {
	Sampler             string `json:"sampler"`
	SamplingRatio       string `json:"samplingRatio,omitempty"`
	SampledSpans        uint64 `json:"sampledSpans"`
	ExportedSpans       uint64 `json:"exportedSpans"`
	ExportFailures      uint64 `json:"exportFailures"`
	LastExportLatencyNS int64  `json:"lastExportLatencyNs"`
	MaxExportLatencyNS  int64  `json:"maxExportLatencyNs"`
}

// HealthSnapshot returns local sampling and sink export health.
func (tracer *Tracer) HealthSnapshot() TracerHealthSnapshot {
	if tracer == nil {
		return TracerHealthSnapshot{}
	}
	return TracerHealthSnapshot{
		Sampler:             samplerDescription(tracer.sampler),
		SamplingRatio:       samplerRatio(tracer.sampler),
		SampledSpans:        tracer.sampledSpans.Load(),
		ExportedSpans:       tracer.exportedSpans.Load(),
		ExportFailures:      tracer.exportFailures.Load(),
		LastExportLatencyNS: tracer.lastExportNS.Load(),
		MaxExportLatencyNS:  tracer.maxExportNS.Load(),
	}
}

type describedSampler interface {
	description() string
}

func samplerDescription(sampler Sampler) string {
	if sampler == nil {
		return "always_on"
	}
	if described, ok := sampler.(describedSampler); ok {
		return described.description()
	}
	return "custom"
}

func samplerRatio(sampler Sampler) string {
	if ratio, ok := sampler.(ratioSampler); ok {
		return strconv.FormatFloat(ratio.ratio, 'f', -1, 64)
	}
	return ""
}

func (tracer *Tracer) recordExport(duration time.Duration, err error) {
	if tracer == nil {
		return
	}
	ns := duration.Nanoseconds()
	if ns < 0 {
		ns = 0
	}
	tracer.lastExportNS.Store(ns)
	updateMaxInt64(&tracer.maxExportNS, ns)
	if err != nil {
		tracer.exportFailures.Add(1)
		return
	}
	tracer.exportedSpans.Add(1)
}

func updateMaxInt64(target *atomic.Int64, candidate int64) {
	for {
		current := target.Load()
		if candidate <= current {
			return
		}
		if target.CompareAndSwap(current, candidate) {
			return
		}
	}
}
