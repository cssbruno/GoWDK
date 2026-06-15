package gowdk

import (
	"fmt"
	"strings"
)

// Config describes how a GOWDK application should be discovered, compiled,
// and packaged.
type Config struct {
	AppName string
	Source  SourceConfig
	Modules []ModuleConfig
	Render  RenderConfig
	Env     EnvConfig
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

// RenderConfig controls default render behavior. SPA is the default when
// omitted.
type RenderConfig struct {
	Default RenderMode
}

// DefaultMode returns SPA when no explicit default render mode is set.
func (config RenderConfig) DefaultMode() RenderMode {
	if config.Default == "" {
		return SPA
	}
	return config.Default
}

// EnvConfig declares the runtime environment contract for generated apps.
// It names expected variables and secrets, but never stores secret values.
type EnvConfig struct {
	Vars    []EnvVar
	Secrets []SecretEnv
}

// EnvVar declares a normal non-secret environment variable. Defaults must only
// be used for safe non-secret local or runtime values.
type EnvVar struct {
	Name     string
	Required bool
	Default  string
}

// SecretEnv declares a secret environment variable. Secret values intentionally
// have no config field and must come from the runtime environment.
type SecretEnv struct {
	Name     string
	Required bool
	// MinBytes rejects a present-but-too-short secret at build time and at
	// generated-app startup. Zero means no minimum. This lets the env contract
	// fail fast on a weak signing key instead of deferring the failure to the
	// first request that constructs the signer.
	MinBytes int
}

// EnvValidationError describes one invalid or missing env contract entry.
type EnvValidationError struct {
	Code    string
	Name    string
	Message string
}

func (err EnvValidationError) Error() string {
	return err.Message
}

// EnvValidationErrors is a list of env contract validation failures.
type EnvValidationErrors []EnvValidationError

func (errs EnvValidationErrors) Error() string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}

// Validate checks the env contract. If lookup is nil, only structural rules are
// checked. If lookup is provided, required missing names are reported too.
func (config EnvConfig) Validate(lookup func(string) (string, bool)) error {
	var diagnostics EnvValidationErrors
	seen := map[string]string{}
	for _, variable := range config.Vars {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			diagnostics = append(diagnostics, EnvValidationError{Code: "env_name_required", Message: "environment variable name is required"})
			continue
		}
		diagnostics = append(diagnostics, validateEnvDuplicate(seen, name, "Vars")...)
		if secretLikeEnvName(name) {
			diagnostics = append(diagnostics, EnvValidationError{
				Code:    "secret_env_in_vars",
				Name:    name,
				Message: fmt.Sprintf("%s looks like a secret and must be declared in Env.Secrets", name),
			})
		}
		if lookup != nil && variable.Required && variable.Default == "" {
			if value, ok := lookup(name); !ok || strings.TrimSpace(value) == "" {
				diagnostics = append(diagnostics, EnvValidationError{Code: "missing_required_env", Name: name, Message: fmt.Sprintf("%s is required but is not set", name)})
			}
		}
	}
	for _, secret := range config.Secrets {
		name := strings.TrimSpace(secret.Name)
		if name == "" {
			diagnostics = append(diagnostics, EnvValidationError{Code: "secret_env_name_required", Message: "secret environment variable name is required"})
			continue
		}
		diagnostics = append(diagnostics, validateEnvDuplicate(seen, name, "Secrets")...)
		if lookup != nil {
			value, ok := lookup(name)
			trimmed := strings.TrimSpace(value)
			switch {
			case secret.Required && (!ok || trimmed == ""):
				diagnostics = append(diagnostics, EnvValidationError{Code: "missing_required_secret", Name: name, Message: fmt.Sprintf("%s is required but is not set", name)})
			case secret.MinBytes > 0 && trimmed != "" && len(trimmed) < secret.MinBytes:
				diagnostics = append(diagnostics, EnvValidationError{Code: "short_secret", Name: name, Message: fmt.Sprintf("%s must be at least %d bytes", name, secret.MinBytes)})
			}
		}
	}
	if len(diagnostics) > 0 {
		return diagnostics
	}
	return nil
}

func validateEnvDuplicate(seen map[string]string, name string, section string) EnvValidationErrors {
	if previous := seen[name]; previous != "" {
		return EnvValidationErrors{{
			Code:    "duplicate_env_name",
			Name:    name,
			Message: fmt.Sprintf("%s is declared more than once in Env.%s and Env.%s", name, previous, section),
		}}
	}
	seen[name] = section
	return nil
}

func secretLikeEnvName(name string) bool {
	for _, suffix := range []string{"_SECRET", "_TOKEN", "_PASSWORD", "_KEY"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// BuildConfig controls output artifacts and frontend asset packaging.
type BuildConfig struct {
	Output              string
	Mode                BuildMode
	Assets              AssetMode
	ObfuscateAssets     bool
	Head                HeadConfig
	CSRF                CSRFConfig
	SecurityHeaders     SecurityHeadersConfig
	BodyLimits          BodyLimitsConfig
	AllowMissingBackend bool
	Stylesheets         []Stylesheet
	Scripts             []Script
	Targets             []BuildTargetConfig
}

// HeadConfig controls app-level document head tags emitted around page
// metadata.
type HeadConfig struct {
	SiteName    string
	Favicon     string
	Image       string
	TwitterCard string
}

// SecurityHeadersConfig declares generated runtime response headers. Audit
// policy can require these headers statically, and generated audit tests can
// verify that the handler emits them.
type SecurityHeadersConfig struct {
	Enabled bool
	Headers map[string]string
}

const DefaultCSRFSecretEnv = "GOWDK_CSRF_SECRET"

// CSRFConfig controls generated CSRF token wiring for browser-reachable
// state-changing endpoints.
type CSRFConfig struct {
	Enabled    bool
	Disabled   bool
	SecretEnv  string
	CookieName string
	FieldName  string
	HeaderName string
	Insecure   bool
}

// EnabledForGeneratedEndpoints reports whether generated state-changing
// endpoints should emit CSRF token injection and validation. CSRF is on by
// default; Disabled is the explicit opt-out. Enabled is retained for older
// configs that already set it.
func (config CSRFConfig) EnabledForGeneratedEndpoints() bool {
	return !config.Disabled
}

// SecretEnvName returns the environment variable used by generated apps to
// read the CSRF signing secret.
func (config CSRFConfig) SecretEnvName() string {
	if config.SecretEnv == "" {
		return DefaultCSRFSecretEnv
	}
	return config.SecretEnv
}

// DefaultRequestBodyLimitBytes is the default generated request body cap for
// action and API endpoints.
const DefaultRequestBodyLimitBytes int64 = 1 << 20

// BodyLimitsConfig controls generated request body caps. Omitted or non-positive
// values use the default 1 MiB cap.
type BodyLimitsConfig struct {
	ActionBytes int64
	APIBytes    int64
}

// ActionLimitBytes returns the configured action body cap or the default cap.
func (config BodyLimitsConfig) ActionLimitBytes() int64 {
	if config.ActionBytes > 0 {
		return config.ActionBytes
	}
	return DefaultRequestBodyLimitBytes
}

// APILimitBytes returns the configured API body cap or the default cap.
func (config BodyLimitsConfig) APILimitBytes() int64 {
	if config.APIBytes > 0 {
		return config.APIBytes
	}
	return DefaultRequestBodyLimitBytes
}

// BuildTargetConfig declares one configured build target. Modules selects the
// configured source modules compiled into Output, App, Binary, WASM, BackendApp,
// and BackendBinary.
type BuildTargetConfig struct {
	Name          string
	Modules       []string
	Output        string
	App           string
	Binary        string
	WASM          string
	BackendApp    string
	BackendBinary string
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

// ObfuscatesAssets reports whether compiler-owned generated browser assets
// should be transformed for production output.
func (config BuildConfig) ObfuscatesAssets() bool {
	return config.ObfuscateAssets
}

// RenderMode describes where full-page HTML is produced.
type RenderMode string

const (
	// SPA emits a non-SSR app shell and client-side route experience.
	SPA RenderMode = "spa"
	// Action emits a non-SSR app shell while allowing backend actions.
	Action RenderMode = "action"
	// Hybrid allows a route to combine app output and request-time behavior.
	Hybrid RenderMode = "hybrid"
	// SSR renders full pages at request time through the SSR addon.
	SSR RenderMode = "ssr"
)

// ParseRenderMode validates a render mode from source.
func ParseRenderMode(value string) (RenderMode, error) {
	mode := RenderMode(value)
	switch mode {
	case SPA, Action, Hybrid, SSR:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown render mode %q", value)
	}
}

// RequiresSSR reports whether this mode always needs the SSR addon. Hybrid
// pages need SSR only when they declare explicit request-time capabilities.
func (mode RenderMode) RequiresSSR() bool {
	return mode == SSR
}

// IsBuildTime reports whether this mode is always build-time. Hybrid defaults
// to build-time unless explicit request-time capabilities are declared.
func (mode RenderMode) IsBuildTime() bool {
	return mode == SPA || mode == Action
}

// Feature names the capabilities that addons make available to the compiler.
type Feature string

const (
	FeatureSPA           Feature = "spa"
	FeatureActions       Feature = "actions"
	FeaturePartial       Feature = "partial"
	FeatureSSR           Feature = "ssr"
	FeatureAPI           Feature = "api"
	FeatureEmbed         Feature = "embed"
	FeatureCSS           Feature = "css"
	FeatureRateLimit     Feature = "ratelimit"
	FeatureContracts     Feature = "contracts"
	FeatureRealtime      Feature = "realtime"
	FeatureAuth          Feature = "auth"
	FeatureDB            Feature = "db"
	FeatureSEO           Feature = "seo"
	FeatureObservability Feature = "observability"
)

// Addon is the minimal contract every optional GOWDK capability implements.
type Addon interface {
	Name() string
	Features() []Feature
}

// SEOURL describes one additional URL that an SEO addon can add to the
// generated sitemap. Loc may be absolute or root-relative.
type SEOURL struct {
	Loc        string
	LastMod    string
	ChangeFreq string
	Priority   string
}

// SEOOptions configures build-time sitemap.xml and robots.txt emission.
type SEOOptions struct {
	BaseURL          string
	Disallow         []string
	ExtraURLs        []SEOURL
	ExtraURLProvider func() []SEOURL `json:"-"`
}

// SEOProvider is implemented by addons that can supply build-time SEO output
// options to the compiler.
type SEOProvider interface {
	SEOOptions() SEOOptions
}

// GoBlockConsumer is an optional addon extension point for targeted go blocks
// such as go addon.contracts {}.
type GoBlockConsumer interface {
	GoBlockTargets() []string
	ValidateGoBlock(target GoBlockTarget, context GoBlockContext) []GoBlockDiagnostic
	GeneratedGo(target GoBlockTarget, context GoBlockContext) ([]GoBlockFile, error)
}

// GoBlockTarget describes one parsed go block passed to an addon.
type GoBlockTarget struct {
	Target       string
	OwnerKind    string
	OwnerID      string
	OwnerPackage string
	SourcePath   string
	Body         string
	Span         SourceSpan
}

// GoBlockContext describes the compiler lane that owns a go block target.
type GoBlockContext struct {
	Render RenderMode
}

// GoBlockDiagnostic is an addon-produced diagnostic for a go block target.
type GoBlockDiagnostic struct {
	Code    string
	Message string
	Span    SourceSpan
}

// GoBlockFile is a generated file emitted by an addon go block consumer. Path is
// relative to the generated app directory.
type GoBlockFile struct {
	Path    string
	Source  string
	Package string
}

// SourcePosition is a 1-based source location exposed to addon go block
// consumers.
type SourcePosition struct {
	Line   int
	Column int
}

// SourceSpan is a 1-based source range exposed to addon go block consumers.
type SourceSpan struct {
	Start SourcePosition
	End   SourcePosition
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

// Script describes one script tag emitted into generated HTML.
// Type is optional; use "module" for ES module bundles.
type Script struct {
	Src  string
	Type string
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
	Assets          []CSSAsset
	Stylesheets     []Stylesheet
	PageStylesheets map[string][]Stylesheet
}

// CSSProcessor is implemented by addons that emit CSS at build time.
type CSSProcessor interface {
	Addon
	ProcessCSS(CSSContext) (CSSResult, error)
}
