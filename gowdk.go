package gowdk

import "fmt"

// Config describes how a GOWDK application should be discovered, compiled,
// and packaged.
type Config struct {
	AppName string
	Source  SourceConfig
	Modules []ModuleConfig
	Render  RenderConfig
	Build   BuildConfig
	CSS     CSSConfig
	Addons  []Addon
}

// SourceConfig selects portable .gwdk files for discovery.
type SourceConfig struct {
	Include []string
	Exclude []string
}

// ModuleConfig names a source group inside a GOWDK app. Build discovery uses
// selected module sources to decide what gets compiled into output, generated
// apps, and generated binaries. Type is user-defined metadata.
type ModuleConfig struct {
	Name   string
	Type   string
	Source SourceConfig
}

// RenderConfig controls default render behavior. Static is the default when
// omitted.
type RenderConfig struct {
	Default RenderMode
}

// DefaultMode returns Static when no explicit default render mode is set.
func (config RenderConfig) DefaultMode() RenderMode {
	if config.Default == "" {
		return Static
	}
	return config.Default
}

// BuildConfig controls output artifacts and frontend asset packaging.
type BuildConfig struct {
	Output      string
	Mode        BuildMode
	Assets      AssetMode
	Stylesheets []Stylesheet
	Targets     []BuildTargetConfig
}

// BuildTargetConfig declares one static build target. Modules selects the
// configured source modules compiled into Output, App, Binary, and WASM.
type BuildTargetConfig struct {
	Name    string
	Modules []string
	Output  string
	App     string
	Binary  string
	WASM    string
}

// CSSConfig controls discovered CSS inputs and page CSS output.
type CSSConfig struct {
	Include []string
	Exclude []string
	Default []string
	Output  CSSOutputConfig
}

// CSSOutputConfig controls generated page stylesheet locations.
type CSSOutputConfig struct {
	Dir        string
	HrefPrefix string
}

// AssetMode controls how frontend artifacts are shipped.
type AssetMode string

const (
	AssetExternal AssetMode = "external"
	Embed         AssetMode = "embed"
)

// BuildMode controls whether generated frontend artifacts include development
// metadata such as source maps. Development is the default when omitted.
type BuildMode string

const (
	Development BuildMode = "development"
	Production  BuildMode = "production"
)

// DebugAssets reports whether generated frontend artifacts should include
// debugging metadata.
func (config BuildConfig) DebugAssets() bool {
	return config.Mode != Production
}

// RenderMode describes where full-page HTML is produced.
type RenderMode string

const (
	// Static renders full pages at build time.
	Static RenderMode = "static"
	// Action renders the page statically while allowing backend actions.
	Action RenderMode = "action"
	// Hybrid allows a route to combine static output and request-time behavior.
	Hybrid RenderMode = "hybrid"
	// SSR renders full pages at request time through the SSR addon.
	SSR RenderMode = "ssr"
)

// ParseRenderMode validates a render mode from source.
func ParseRenderMode(value string) (RenderMode, error) {
	mode := RenderMode(value)
	switch mode {
	case Static, Action, Hybrid, SSR:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown render mode %q", value)
	}
}

// RequiresSSR reports whether this mode needs the SSR addon.
func (mode RenderMode) RequiresSSR() bool {
	return mode == SSR || mode == Hybrid
}

// IsBuildTime reports whether route params must be known at build time.
func (mode RenderMode) IsBuildTime() bool {
	return mode == Static || mode == Action
}

// Feature names the capabilities that addons make available to the compiler.
type Feature string

const (
	FeatureStatic    Feature = "static"
	FeatureActions   Feature = "actions"
	FeaturePartial   Feature = "partial"
	FeatureSSR       Feature = "ssr"
	FeatureAPI       Feature = "api"
	FeatureEmbed     Feature = "embed"
	FeatureCSS       Feature = "css"
	FeatureRateLimit Feature = "ratelimit"
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

// Stylesheet describes one stylesheet link emitted into generated HTML.
type Stylesheet struct {
	Href string
}

// CSSSource describes one discovered source file for compile-time CSS plugins.
type CSSSource struct {
	Path       string
	Kind       string
	Name       string
	CSSClasses []string
}

// CSSContext is passed to compile-time CSS processors.
type CSSContext struct {
	Sources   []CSSSource
	OutputDir string
	Build     BuildConfig
	CSS       CSSConfig
}

// CSSAsset is a CSS file emitted by a compile-time CSS processor.
type CSSAsset struct {
	Path     string
	Contents []byte
}

// CSSResult is returned by compile-time CSS processors.
type CSSResult struct {
	Assets      []CSSAsset
	Stylesheets []Stylesheet
}

// CSSProcessor is implemented by addons that emit CSS at build time.
type CSSProcessor interface {
	Addon
	ProcessCSS(CSSContext) (CSSResult, error)
}
