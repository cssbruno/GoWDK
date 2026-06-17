package syntax

// MetadataKeywords is the canonical, ordered set of top-level metadata keywords
// the current parser recognizes. It is the single source of truth for metadata
// classification (the lexer and the formatter both consume it) and is
// cross-checked against the published stability table in
// docs/language/stability.md by TestStabilityTableCoversConstructs.
var MetadataKeywords = []string{
	"page",
	"route",
	"title",
	"description",
	"canonical",
	"image",
	"robots",
	"noindex",
	"preload",
	"prefetch",
	"layout",
	"cache",
	"revalidate",
	"error",
	"guard",
	"css",
	"component",
	"wasm",
	"asset",
}

var metadataKeywordSet = func() map[string]bool {
	set := make(map[string]bool, len(MetadataKeywords))
	for _, keyword := range MetadataKeywords {
		set[keyword] = true
	}
	return set
}()

// IsMetadataKeyword reports whether value is a top-level metadata keyword.
func IsMetadataKeyword(value string) bool {
	return metadataKeywordSet[value]
}
