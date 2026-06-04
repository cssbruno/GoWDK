package gowdk

// Feature names the capabilities that addons make available to the compiler.
type Feature string

const (
	FeatureStatic  Feature = "static"
	FeatureActions Feature = "actions"
	FeaturePartial Feature = "partial"
	FeatureSSR     Feature = "ssr"
	FeatureAPI     Feature = "api"
	FeatureEmbed   Feature = "embed"
	FeatureCSS     Feature = "css"
)

// Addon is the minimal contract every optional GOWDK capability implements.
type Addon interface {
	Name() string
	Features() []Feature
}

type addon struct {
	name     string
	features []Feature
}

// NewAddon creates a simple addon declaration for capability registration.
func NewAddon(name string, features ...Feature) Addon {
	return addon{name: name, features: append([]Feature(nil), features...)}
}

func (a addon) Name() string {
	return a.name
}

func (a addon) Features() []Feature {
	return append([]Feature(nil), a.features...)
}

// FeatureSet is a lookup table of enabled addon capabilities.
type FeatureSet map[Feature]bool

// EnabledFeatures returns the set of capabilities enabled by a config.
func EnabledFeatures(config Config) FeatureSet {
	features := FeatureSet{}
	for _, addon := range config.Addons {
		for _, feature := range addon.Features() {
			features[feature] = true
		}
	}
	return features
}

// Has reports whether a feature is present in the set.
func (features FeatureSet) Has(feature Feature) bool {
	return features[feature]
}

// HasFeature reports whether a config enables a feature through an addon.
func (config Config) HasFeature(feature Feature) bool {
	return EnabledFeatures(config).Has(feature)
}
