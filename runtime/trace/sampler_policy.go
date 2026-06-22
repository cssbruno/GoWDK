package trace

import "strings"

// ParentBasedSampler returns a sampler that keeps a trace whole: when a span has
// a propagated parent, it honors the parent's sampling decision; for a root span
// (no parent) it delegates to root. This is the production-safe default for
// distributed tracing — a downstream service does not re-roll the dice and split
// one logical trace into sampled and unsampled fragments. Pair it with
// RatioSampler as the root to sample a deterministic fraction of whole traces:
//
//	trace.ParentBasedSampler(trace.RatioSampler(0.1))
func ParentBasedSampler(root Sampler) Sampler {
	if root == nil {
		root = AlwaysOn()
	}
	return samplerFunc(func(ctx SamplingContext) bool {
		if ctx.HasParent {
			return ctx.ParentSampled
		}
		return root.Sample(ctx)
	})
}

// SamplerRule overrides the sampling decision for spans whose SamplingContext
// matches. The first matching rule in a RuleSampler wins.
type SamplerRule struct {
	Match func(SamplingContext) bool
	// Keep is the decision applied when Match returns true.
	Keep bool
}

// RuleSampler applies the first matching rule's decision and falls back to base
// when no rule matches. It is the GOWDK-owned override hook for app and
// generated route/endpoint policy: silence health checks and noisy endpoints
// (Keep:false), force high-value endpoints on (Keep:true), and sample everything
// else with base.
//
//	trace.RuleSampler(
//	    trace.ParentBasedSampler(trace.RatioSampler(0.1)),
//	    trace.DropSpansNamed("GET /_gowdk/health"),
//	    trace.KeepSpansNamed("POST /checkout"),
//	)
func RuleSampler(base Sampler, rules ...SamplerRule) Sampler {
	if base == nil {
		base = AlwaysOn()
	}
	return samplerFunc(func(ctx SamplingContext) bool {
		for _, rule := range rules {
			if rule.Match != nil && rule.Match(ctx) {
				return rule.Keep
			}
		}
		return base.Sample(ctx)
	})
}

// MatchSpanName matches spans whose name equals name.
func MatchSpanName(name string) func(SamplingContext) bool {
	return func(ctx SamplingContext) bool { return ctx.Name == name }
}

// MatchSpanNamePrefix matches spans whose name starts with prefix.
func MatchSpanNamePrefix(prefix string) func(SamplingContext) bool {
	return func(ctx SamplingContext) bool { return strings.HasPrefix(ctx.Name, prefix) }
}

// MatchLane matches spans on the given GOWDK lane.
func MatchLane(lane Lane) func(SamplingContext) bool {
	return func(ctx SamplingContext) bool { return ctx.Lane == lane }
}

// MatchSurface matches spans on the given GOWDK surface.
func MatchSurface(surface Surface) func(SamplingContext) bool {
	return func(ctx SamplingContext) bool { return ctx.Surface == surface }
}

// DropSpansNamed builds a rule that always drops spans with the exact name, for
// health checks and other always-off endpoints.
func DropSpansNamed(name string) SamplerRule {
	return SamplerRule{Match: MatchSpanName(name), Keep: false}
}

// KeepSpansNamed builds a rule that always keeps spans with the exact name, for
// high-value endpoints that must be traced regardless of the base ratio.
func KeepSpansNamed(name string) SamplerRule {
	return SamplerRule{Match: MatchSpanName(name), Keep: true}
}
