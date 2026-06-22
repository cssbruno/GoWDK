package trace_test

import (
	"context"
	"testing"

	"github.com/cssbruno/gowdk/runtime/trace"
)

// staticIDGenerator returns fixed IDs so tests can assert deterministic
// identity without relying on entropy failure.
type staticIDGenerator struct {
	traceID trace.TraceID
	spanID  trace.SpanID
}

func (generator staticIDGenerator) NewTraceID() trace.TraceID { return generator.traceID }
func (generator staticIDGenerator) NewSpanID() trace.SpanID   { return generator.spanID }

func TestWithIDGeneratorInjectsDeterministicIDs(t *testing.T) {
	wantTrace := trace.TraceID("0123456789abcdef0123456789abcdef")
	wantSpan := trace.SpanID("0123456789abcdef")
	tracer := trace.NewTracer(trace.WithIDGenerator(staticIDGenerator{traceID: wantTrace, spanID: wantSpan}))

	_, span := tracer.Start(context.Background(), "unit")
	if span == nil {
		t.Fatal("expected sampled span")
	}
	got := span.TraceContext()
	if got.TraceID != wantTrace || got.SpanID != wantSpan {
		t.Fatalf("injected IDs not used: %#v", got)
	}
}

func TestInvalidGeneratedIDDropsSpanWithoutPredictableFallback(t *testing.T) {
	// A generator that cannot produce identity returns empty IDs. The tracer
	// must drop the span rather than emit a predictable identifier.
	tracer := trace.NewTracer(trace.WithIDGenerator(staticIDGenerator{}))
	ctx := context.Background()
	next, span := tracer.Start(ctx, "unit")
	if span != nil {
		t.Fatal("expected no span when identity cannot be generated")
	}
	if next != ctx {
		t.Fatal("expected unchanged context when identity cannot be generated")
	}
}

func TestDefaultGeneratorProducesUniqueValidIDs(t *testing.T) {
	seen := make(map[trace.TraceID]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := trace.NewTraceID()
		if !id.Valid() {
			t.Fatalf("default generator produced invalid trace id %q", id)
		}
		if seen[id] {
			t.Fatalf("default generator produced duplicate trace id %q", id)
		}
		seen[id] = true
	}
}

func TestRatioSamplerWithDefaultGeneratorApproximatesRatio(t *testing.T) {
	// The default crypto generator yields uniformly distributed sampling keys,
	// so deterministic ratio sampling keeps its statistical assumptions. The
	// tolerance is many standard deviations wide, so this is not flaky.
	const ratio = 0.5
	const samples = 4000
	sampler := trace.RatioSampler(ratio)

	sampled := 0
	for i := 0; i < samples; i++ {
		if sampler.Sample(trace.SamplingContext{TraceID: trace.NewTraceID()}) {
			sampled++
		}
	}
	fraction := float64(sampled) / float64(samples)
	if fraction < ratio-0.1 || fraction > ratio+0.1 {
		t.Fatalf("ratio sampling fraction %.3f deviates from %.2f beyond tolerance", fraction, ratio)
	}
}
