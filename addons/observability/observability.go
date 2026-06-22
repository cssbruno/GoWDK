// Package observability declares the optional GOWDK Trace compiler/runtime
// capability and re-exports dependency-free runtime trace helpers.
package observability

import (
	"time"

	"github.com/cssbruno/gowdk"
	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

// ImportPath is the canonical Go import path for the observability addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/observability"

// Tracer records spans for generated app instrumentation.
type Tracer = gowdktrace.Tracer

// Collector stores recent spans and serves the self-contained viewer.
type Collector = gowdktrace.Collector

// Addon enables generated app trace wiring.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("observability", gowdk.FeatureObservability)
}

// CollectorOption configures a Collector.
type CollectorOption = gowdktrace.CollectorOption

// NewCollector creates a bounded in-memory trace collector.
func NewCollector(limit int, options ...CollectorOption) *Collector {
	return gowdktrace.NewCollector(limit, options...)
}

// WithCollectorSSELimit configures the collector's concurrent SSE stream cap.
func WithCollectorSSELimit(limit int) CollectorOption {
	return gowdktrace.WithCollectorSSELimit(limit)
}

// WithCollectorIngestRate configures the collector's per-client POST rate.
func WithCollectorIngestRate(limit int, window time.Duration) CollectorOption {
	return gowdktrace.WithCollectorIngestRate(limit, window)
}

// NewTracer creates a dependency-free tracer.
func NewTracer(options ...gowdktrace.TracerOption) *Tracer {
	return gowdktrace.NewTracer(options...)
}

// WithSink configures the completed span sink.
func WithSink(sink gowdktrace.Sink) gowdktrace.TracerOption {
	return gowdktrace.WithSink(sink)
}

// AlwaysOn samples every span.
func AlwaysOn() gowdktrace.Sampler {
	return gowdktrace.AlwaysOn()
}

// AlwaysOff samples no spans.
func AlwaysOff() gowdktrace.Sampler {
	return gowdktrace.AlwaysOff()
}

// RatioSampler samples a deterministic fraction of traces.
func RatioSampler(ratio float64) gowdktrace.Sampler {
	return gowdktrace.RatioSampler(ratio)
}

// SamplerRule overrides the sampling decision for matching spans.
type SamplerRule = gowdktrace.SamplerRule

// ParentBasedSampler keeps a trace whole: it honors a propagated parent
// decision and samples roots with root. It is the production-safe default for
// distributed tracing.
func ParentBasedSampler(root gowdktrace.Sampler) gowdktrace.Sampler {
	return gowdktrace.ParentBasedSampler(root)
}

// RuleSampler applies per-span override rules (silence health checks, force
// high-value endpoints on) before falling back to base.
func RuleSampler(base gowdktrace.Sampler, rules ...gowdktrace.SamplerRule) gowdktrace.Sampler {
	return gowdktrace.RuleSampler(base, rules...)
}

// DropSpansNamed builds a rule that always drops spans with the exact name.
func DropSpansNamed(name string) gowdktrace.SamplerRule {
	return gowdktrace.DropSpansNamed(name)
}

// KeepSpansNamed builds a rule that always keeps spans with the exact name.
func KeepSpansNamed(name string) gowdktrace.SamplerRule {
	return gowdktrace.KeepSpansNamed(name)
}
