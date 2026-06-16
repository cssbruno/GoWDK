package lang

// StabilityTier is the per-construct stability classification, mirroring the
// Stability field the diagnostics registry records for diagnostic codes.
type StabilityTier string

const (
	// TierStable: accepted today and not expected to change shape within 0.x
	// without a deprecation step.
	TierStable StabilityTier = "stable"
	// TierPartial: accepted for a narrower slice than the final contract.
	TierPartial StabilityTier = "partial"
	// TierPlanned: not accepted yet; using it is rejected with DiagnosticCode.
	TierPlanned StabilityTier = "planned"
	// TierDeprecated: previously accepted spelling, now rejected with
	// DiagnosticCode.
	TierDeprecated StabilityTier = "deprecated"
)

// ConstructKind groups language constructs by surface.
type ConstructKind string

const (
	ConstructBlock     ConstructKind = "block"
	ConstructKeyword   ConstructKind = "metadata-keyword"
	ConstructDirective ConstructKind = "g-directive"
	ConstructEndpoint  ConstructKind = "endpoint"
)

// ConstructStability is the stability record for one language construct. For
// planned and deprecated constructs DiagnosticCode names the diagnostic emitted
// when the construct is used.
type ConstructStability struct {
	Name           string
	Kind           ConstructKind
	Tier           StabilityTier
	DiagnosticCode string
}

// ConstructStabilities returns the code-level source of truth for per-construct
// stability tiers. The published table in docs/language/stability.md is verified
// against it by TestStabilityTableMatchesRegistry. Metadata keywords are derived
// from MetadataKeywords so the two cannot drift.
func ConstructStabilities() []ConstructStability {
	constructs := []ConstructStability{
		// Top-level blocks.
		{Name: "package", Kind: ConstructBlock, Tier: TierStable},
		{Name: "import", Kind: ConstructBlock, Tier: TierStable},
		{Name: "use", Kind: ConstructBlock, Tier: TierStable},
		{Name: "paths {}", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "build {}", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "load {}", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "view {}", Kind: ConstructBlock, Tier: TierStable},
		{Name: "style {}", Kind: ConstructBlock, Tier: TierStable},
		{Name: "client {}", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "go {}", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "store", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "props", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "state", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "emits", Kind: ConstructBlock, Tier: TierPartial},
		{Name: "unsupported top-level block", Kind: ConstructBlock, Tier: TierPlanned, DiagnosticCode: "unsupported_top_level_block"},

		// Supported g: directives. Flow control is stable; the rest are partial.
		{Name: "g:if", Kind: ConstructDirective, Tier: TierStable},
		{Name: "g:else-if", Kind: ConstructDirective, Tier: TierStable},
		{Name: "g:else", Kind: ConstructDirective, Tier: TierStable},
		{Name: "g:for", Kind: ConstructDirective, Tier: TierStable},
		{Name: "g:each", Kind: ConstructDirective, Tier: TierStable},
		{Name: "g:when", Kind: ConstructDirective, Tier: TierStable},
		{Name: "g:key", Kind: ConstructDirective, Tier: TierStable},
		{Name: "g:unsafe-html", Kind: ConstructDirective, Tier: TierStable},
		{Name: "g:bind:value", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:bind:checked", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:post", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:target", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:swap", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:island", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:command", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:query", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:subscribe", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:event", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:ref", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:slot", Kind: ConstructDirective, Tier: TierPartial},

		// Supported g: directive families (validated separately from the
		// exact-name set, so view.SupportedDirectiveNames() excludes them).
		{Name: "g:on:*", Kind: ConstructDirective, Tier: TierPartial},
		{Name: "g:message:*", Kind: ConstructDirective, Tier: TierPartial},

		// Planned g: directives, rejected on use. They currently surface as the
		// generic parse_error rather than a typed code (see
		// docs/language/conformance.md), so DiagnosticCode is left unset until
		// markup rejections carry their own code.
		{Name: "g:transition", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:animate", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:window", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:document", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:body", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:head", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:await", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:async", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:use", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:action", Kind: ConstructDirective, Tier: TierPlanned},
		{Name: "g:attach", Kind: ConstructDirective, Tier: TierPlanned},

		// Endpoint declaration forms.
		{Name: "act", Kind: ConstructEndpoint, Tier: TierStable},
		{Name: "api", Kind: ConstructEndpoint, Tier: TierStable},
		{Name: "fragment", Kind: ConstructEndpoint, Tier: TierPartial},
		{Name: "act block form", Kind: ConstructEndpoint, Tier: TierDeprecated, DiagnosticCode: "old_action_block_syntax"},
		{Name: "api block form", Kind: ConstructEndpoint, Tier: TierDeprecated, DiagnosticCode: "old_api_block_syntax"},
	}

	for _, keyword := range MetadataKeywords {
		constructs = append(constructs, ConstructStability{
			Name: keyword,
			Kind: ConstructKeyword,
			Tier: TierStable,
		})
	}

	return constructs
}
