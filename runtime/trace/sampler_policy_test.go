package trace_test

import (
	"testing"

	"github.com/cssbruno/gowdk/runtime/trace"
)

func TestParentBasedSamplerKeepsTraceWhole(t *testing.T) {
	sampler := trace.ParentBasedSampler(trace.AlwaysOff())

	if sampler.Sample(trace.SamplingContext{}) {
		t.Fatal("a root span must follow the root sampler (off)")
	}
	if !sampler.Sample(trace.SamplingContext{HasParent: true, ParentSampled: true}) {
		t.Fatal("a child of a sampled parent must be kept")
	}
	if sampler.Sample(trace.SamplingContext{HasParent: true, ParentSampled: false}) {
		t.Fatal("a child of an unsampled parent must be dropped")
	}

	rootOn := trace.ParentBasedSampler(trace.AlwaysOn())
	if !rootOn.Sample(trace.SamplingContext{}) {
		t.Fatal("a root span must follow the root sampler (on)")
	}
}

func TestParentBasedSamplerNilRootDefaultsOn(t *testing.T) {
	if !trace.ParentBasedSampler(nil).Sample(trace.SamplingContext{}) {
		t.Fatal("nil root sampler must default to always-on for roots")
	}
}

func TestRuleSamplerOverridesBase(t *testing.T) {
	sampler := trace.RuleSampler(
		trace.AlwaysOn(),
		trace.DropSpansNamed("GET /_gowdk/health"),
		trace.KeepSpansNamed("POST /checkout"),
	)

	if sampler.Sample(trace.SamplingContext{Name: "GET /_gowdk/health"}) {
		t.Fatal("health checks must be dropped by an override rule")
	}
	if !sampler.Sample(trace.SamplingContext{Name: "POST /checkout"}) {
		t.Fatal("high-value endpoints must be kept by an override rule")
	}
	if !sampler.Sample(trace.SamplingContext{Name: "GET /other"}) {
		t.Fatal("unmatched spans must fall back to the base sampler (on)")
	}

	offBase := trace.RuleSampler(trace.AlwaysOff(), trace.KeepSpansNamed("POST /checkout"))
	if offBase.Sample(trace.SamplingContext{Name: "GET /other"}) {
		t.Fatal("unmatched spans must fall back to the base sampler (off)")
	}
	if !offBase.Sample(trace.SamplingContext{Name: "POST /checkout"}) {
		t.Fatal("a keep rule must win over an off base")
	}
}

func TestRuleSamplerMatchersByLaneAndPrefix(t *testing.T) {
	sampler := trace.RuleSampler(
		trace.AlwaysOn(),
		trace.SamplerRule{Match: trace.MatchSpanNamePrefix("GET /_gowdk/"), Keep: false},
		trace.SamplerRule{Match: trace.MatchLane(trace.LaneContract), Keep: false},
	)
	if sampler.Sample(trace.SamplingContext{Name: "GET /_gowdk/traces"}) {
		t.Fatal("prefix-matched internal endpoints must be dropped")
	}
	if sampler.Sample(trace.SamplingContext{Name: "call", Lane: trace.LaneContract}) {
		t.Fatal("lane-matched spans must be dropped")
	}
	if !sampler.Sample(trace.SamplingContext{Name: "GET /home", Lane: trace.LaneRoute}) {
		t.Fatal("unmatched spans must follow the base sampler")
	}
}
