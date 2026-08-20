package gowdk

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/cssbruno/gowdk/runtime/corsorigin"
	gowdki18n "github.com/cssbruno/gowdk/runtime/i18n"
	runtimeseo "github.com/cssbruno/gowdk/runtime/seo"
)

// Param returns a declared dynamic route param by name.
func (params BuildParams) Param(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || params.Route == nil {
		return "", false
	}
	value, ok := params.Route[name]
	return value, ok
}

// RouteParams returns a copy of the declared route params.
func (params BuildParams) RouteParams() map[string]string {
	if len(params.Route) == 0 {
		return nil
	}
	out := make(map[string]string, len(params.Route))
	for name, value := range params.Route {
		out[name] = value
	}
	return out
}

// LocaleCode returns the active build locale, when localized route generation
// is enabled.
func (params BuildParams) LocaleCode() string {
	return strings.TrimSpace(params.Locale)
}

// DefaultMode returns SPA when no explicit default render mode is set.
func (config RenderConfig) DefaultMode() RenderMode {
	if config.Default == "" {
		return SPA
	}
	return config.Default
}

// I18NConfig controls locale-aware route generation. When Locales is empty,
// GOWDK emits the existing single-locale routes.
type I18NConfig struct {
	Locales           []LocaleConfig
	DefaultLocale     string
	OmitDefaultPrefix bool
	// Errors localizes stable runtime error codes. Missing entries use each
	// error's safe default message.
	Errors gowdki18n.ErrorBundle
}

// LocaleConfig declares one locale available to build-time and request-time
// generated routes. PathPrefix is optional; when omitted, "/<Code>" is used.
type LocaleConfig struct {
	Code       string
	PathPrefix string
	Name       string
}

// LocalizedRoute describes one locale-expanded route.
type LocalizedRoute struct {
	Locale string
	Route  string
}

// Enabled reports whether localized route generation is configured.
func (config I18NConfig) Enabled() bool {
	return len(config.Locales) > 0
}

// DefaultLocaleCode returns the configured default locale or the first locale.
func (config I18NConfig) DefaultLocaleCode() string {
	if len(config.Locales) == 0 {
		return ""
	}
	if strings.TrimSpace(config.DefaultLocale) != "" {
		return strings.TrimSpace(config.DefaultLocale)
	}
	return strings.TrimSpace(config.Locales[0].Code)
}

// LocaleCodes returns configured locale codes in declaration order.
func (config I18NConfig) LocaleCodes() []string {
	if len(config.Locales) == 0 {
		return nil
	}
	codes := make([]string, 0, len(config.Locales))
	for _, locale := range config.Locales {
		code := strings.TrimSpace(locale.Code)
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

// LocalizedRoutes returns the concrete route variants for the configured
// locale policy. With no locale policy it returns the original route.
func (config I18NConfig) LocalizedRoutes(route string) []LocalizedRoute {
	if !config.Enabled() {
		return []LocalizedRoute{{Route: route}}
	}
	routes := make([]LocalizedRoute, 0, len(config.Locales))
	for _, locale := range config.Locales {
		code := strings.TrimSpace(locale.Code)
		if code == "" {
			continue
		}
		routes = append(routes, LocalizedRoute{
			Locale: code,
			Route:  config.LocalizeRoute(route, code),
		})
	}
	return routes
}

// LocalizeRoute applies the configured path-prefix policy for one locale.
func (config I18NConfig) LocalizeRoute(route string, locale string) string {
	prefix := config.PathPrefix(locale)
	if prefix == "" {
		return route
	}
	route = "/" + strings.TrimLeft(route, "/")
	if route == "/" {
		return prefix
	}
	return prefix + route
}

// PathPrefix returns the normalized route prefix for one configured locale.
func (config I18NConfig) PathPrefix(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" || !config.Enabled() {
		return ""
	}
	if config.OmitDefaultPrefix && strings.EqualFold(locale, config.DefaultLocaleCode()) {
		return ""
	}
	for _, candidate := range config.Locales {
		if !strings.EqualFold(strings.TrimSpace(candidate.Code), locale) {
			continue
		}
		return localePathPrefix(candidate)
	}
	return ""
}

// Validate checks the locale route policy.
func (config I18NConfig) Validate() error {
	if !config.Enabled() {
		return nil
	}
	seenCodes := map[string]bool{}
	seenPrefixes := map[string]string{}
	defaultLocale := strings.TrimSpace(config.DefaultLocale)
	if defaultLocale == "" {
		defaultLocale = strings.TrimSpace(config.Locales[0].Code)
	}
	for index, locale := range config.Locales {
		code := strings.TrimSpace(locale.Code)
		if code == "" {
			return fmt.Errorf("I18N.Locales[%d].Code is required", index)
		}
		if !validLocaleCode(code) {
			return fmt.Errorf("I18N.Locales[%d].Code %q is not a supported locale code", index, code)
		}
		codeKey := strings.ToLower(code)
		if seenCodes[codeKey] {
			return fmt.Errorf("I18N.Locales contains duplicate locale %q", code)
		}
		seenCodes[codeKey] = true
		prefix := localePathPrefix(locale)
		if err := validateLocalePathPrefix(prefix); err != nil {
			return fmt.Errorf("I18N.Locales[%d].PathPrefix: %w", index, err)
		}
		if config.OmitDefaultPrefix && strings.EqualFold(code, defaultLocale) {
			continue
		}
		if previous := seenPrefixes[prefix]; previous != "" {
			return fmt.Errorf("I18N.Locales prefix %q is used by both %q and %q", prefix, previous, code)
		}
		seenPrefixes[prefix] = code
	}
	if strings.TrimSpace(config.DefaultLocale) != "" && !seenCodes[strings.ToLower(defaultLocale)] {
		return fmt.Errorf("I18N.DefaultLocale %q is not declared in I18N.Locales", defaultLocale)
	}
	return nil
}

func localePathPrefix(locale LocaleConfig) string {
	prefix := strings.TrimSpace(locale.PathPrefix)
	if prefix == "" {
		prefix = "/" + strings.ToLower(strings.TrimSpace(locale.Code))
	}
	prefix = "/" + strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "/" {
		return ""
	}
	return prefix
}

func validateLocalePathPrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("must not resolve to the root path")
	}
	if strings.ContainsAny(prefix, "?#{}") {
		return fmt.Errorf("%q must not contain query, fragment, or route parameter syntax", prefix)
	}
	if strings.Contains(prefix, `\`) {
		return fmt.Errorf("%q must not contain backslashes", prefix)
	}
	for _, char := range prefix {
		if unicode.IsSpace(char) || char < 0x20 || char == 0x7f {
			return fmt.Errorf("%q must not contain whitespace or control characters", prefix)
		}
	}
	for _, segment := range strings.Split(strings.Trim(prefix, "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%q contains unsafe path segment %q", prefix, segment)
		}
	}
	return nil
}

func validLocaleCode(code string) bool {
	parts := strings.Split(code, "-")
	if len(parts) == 0 || len(parts[0]) < 2 || len(parts[0]) > 3 {
		return false
	}
	for _, r := range parts[0] {
		if !asciiLetter(r) {
			return false
		}
	}
	for _, part := range parts[1:] {
		if len(part) < 2 || len(part) > 8 {
			return false
		}
		for _, r := range part {
			if !asciiLetter(r) && !asciiDigit(r) {
				return false
			}
		}
	}
	return true
}

func asciiLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func asciiDigit(r rune) bool {
	return r >= '0' && r <= '9'
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

// LifecycleConfig declares process-level services that the generated binary
// starts alongside the generated web app.
type LifecycleConfig struct {
	Services []ServiceRef
}

// ServiceRef names a package-level provider imported by the generated app.
// Function must have signature:
//
//	func() ([]runtime/app.Service, error)
type ServiceRef struct {
	ImportPath string
	Function   string
}

// Validate checks the structural lifecycle contract. Provider symbol existence
// and signatures are verified by the generated app Go build.
func (config LifecycleConfig) Validate() error {
	for index, service := range config.Services {
		importPath := strings.TrimSpace(service.ImportPath)
		function := strings.TrimSpace(service.Function)
		switch {
		case importPath == "" && function == "":
			return fmt.Errorf("Lifecycle.Services[%d] must declare ImportPath and Function", index)
		case importPath == "":
			return fmt.Errorf("Lifecycle.Services[%d].ImportPath is required", index)
		case function == "":
			return fmt.Errorf("Lifecycle.Services[%d].Function is required", index)
		}
	}
	return nil
}

// EnabledForGeneratedAPIs reports whether generated API/contract routes should
// install CORS handling.
func (config CORSConfig) EnabledForGeneratedAPIs() bool {
	return config.Enabled
}

// Validate checks structural safety rules for the generated CORS policy.
func (config CORSConfig) Validate() error {
	if !config.Enabled {
		if len(config.AllowedOrigins) > 0 || len(config.AllowedMethods) > 0 || len(config.AllowedHeaders) > 0 || len(config.ExposedHeaders) > 0 || config.AllowCredentials || config.MaxAgeSeconds != 0 {
			return fmt.Errorf("Build.CORS policy fields require Enabled = true")
		}
		return nil
	}
	if config.MaxAgeSeconds < 0 {
		return fmt.Errorf("Build.CORS.MaxAgeSeconds must be non-negative")
	}
	if len(config.AllowedOrigins) == 0 {
		return fmt.Errorf("Build.CORS.AllowedOrigins must declare at least one origin when CORS is enabled")
	}
	for _, origin := range config.AllowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			return fmt.Errorf("Build.CORS.AllowedOrigins cannot contain an empty origin")
		}
		if origin == "*" && config.AllowCredentials {
			return fmt.Errorf("Build.CORS cannot combine wildcard origin %q with AllowCredentials", origin)
		}
		if origin != "*" {
			if err := validateCORSOrigin(origin); err != nil {
				return err
			}
		}
	}
	for _, method := range config.AllowedMethods {
		method = strings.TrimSpace(method)
		if method == "" {
			return fmt.Errorf("Build.CORS.AllowedMethods cannot contain an empty method")
		}
		if !isHTTPToken(method) {
			return fmt.Errorf("Build.CORS.AllowedMethods contains invalid method %q", method)
		}
	}
	for _, header := range append(append([]string{}, config.AllowedHeaders...), config.ExposedHeaders...) {
		header = strings.TrimSpace(header)
		if header == "" {
			return fmt.Errorf("Build.CORS headers cannot contain an empty header name")
		}
		if !isHTTPToken(header) {
			return fmt.Errorf("Build.CORS headers contains invalid header name %q", header)
		}
	}
	return nil
}

func validateCORSOrigin(origin string) error {
	_, err := corsorigin.Parse(origin)
	if err != nil {
		return fmt.Errorf("Build.CORS.AllowedOrigins contains invalid origin %q: %w", origin, err)
	}
	return nil
}

func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r > 127 || !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", r) {
			return false
		}
	}
	return true
}

const DefaultCSRFSecretEnv = "GOWDK_CSRF_SECRET"

// EnabledForGeneratedEndpoints reports whether generated state-changing
// endpoints should emit CSRF token injection and validation. CSRF is on by
// default; Disabled is the single explicit opt-out.
func (config CSRFConfig) EnabledForGeneratedEndpoints() bool {
	return !config.Disabled
}

// SecretEnvName returns the environment variable used by generated apps to
// read the CSRF signing secret.
func (config CSRFConfig) SecretEnvName() string {
	if strings.TrimSpace(config.SecretEnv) == "" {
		return DefaultCSRFSecretEnv
	}
	return strings.TrimSpace(config.SecretEnv)
}

// VerificationSecretEnvNames returns the runtime environment variables whose
// secrets validate existing CSRF tokens without signing new tokens.
func (config CSRFConfig) VerificationSecretEnvNames() []string {
	names := make([]string, len(config.VerificationSecretEnvs))
	for index, name := range config.VerificationSecretEnvs {
		names[index] = strings.TrimSpace(name)
	}
	return names
}

// SecretEnvNames returns the primary CSRF secret environment variable followed
// by every verification-only secret environment variable.
func (config CSRFConfig) SecretEnvNames() []string {
	return append([]string{config.SecretEnvName()}, config.VerificationSecretEnvNames()...)
}

// Validate checks the structural CSRF configuration without reading secret
// values from the runtime environment.
func (config CSRFConfig) Validate() error {
	if config.Disabled && (strings.TrimSpace(config.SecretEnv) != "" || len(config.VerificationSecretEnvs) > 0 || strings.TrimSpace(config.CookieName) != "" || strings.TrimSpace(config.FieldName) != "" || strings.TrimSpace(config.HeaderName) != "" || config.Insecure) {
		return fmt.Errorf("Build.CSRF.Disabled cannot be combined with CSRF secret, token-name, or Insecure fields")
	}
	seen := map[string]bool{config.SecretEnvName(): true}
	for index, name := range config.VerificationSecretEnvNames() {
		if name == "" {
			return fmt.Errorf("Build.CSRF.VerificationSecretEnvs[%d] must name an environment variable", index)
		}
		if seen[name] {
			return fmt.Errorf("Build.CSRF secret environment variable %q is declared more than once", name)
		}
		seen[name] = true
	}
	return nil
}

// DefaultRequestBodyLimitBytes is the default generated request body cap for
// action and API endpoints.
const DefaultRequestBodyLimitBytes int64 = 1 << 20

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
	// Hybrid allows a route to combine app output and request-time behavior.
	Hybrid RenderMode = "hybrid"
	// SSR renders full pages at request time through the SSR addon.
	SSR RenderMode = "ssr"
)

// ParseRenderMode validates a render mode from source.
func ParseRenderMode(value string) (RenderMode, error) {
	mode := RenderMode(value)
	switch mode {
	case SPA, Hybrid, SSR:
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
	return mode == SPA
}

// Feature names a compiler or generator capability selected from Config.Features
// or the deprecated Config.Addons compatibility lane.
// A feature flag enables GOWDK-owned behavior; it does not by itself mean the
// addon object runs request-time application code.
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

// FeatureConfig selects built-in compiler behavior through typed config.
type FeatureConfig struct {
	SPA           bool
	Actions       bool
	Partial       bool
	SSR           bool
	API           bool
	Embed         bool
	CSS           bool
	RateLimit     bool
	Contracts     bool
	Realtime      bool
	Auth          AuthFeatureConfig
	DB            bool
	SEO           SEOFeatureConfig
	Observability bool
}

// AuthFeatureConfig owns built-in auth feature selection and its typed session
// configuration.
type AuthFeatureConfig struct {
	Enabled bool
	Session AuthSessionOptions
}

// SEOFeatureConfig owns built-in SEO feature selection and options.
type SEOFeatureConfig struct {
	Enabled bool
	Options SEOOptions
}

func (config FeatureConfig) enabled() FeatureSet {
	features := FeatureSet{}
	for feature, enabled := range map[Feature]bool{
		FeatureSPA: config.SPA, FeatureActions: config.Actions,
		FeaturePartial: config.Partial, FeatureSSR: config.SSR,
		FeatureAPI: config.API, FeatureEmbed: config.Embed,
		FeatureCSS: config.CSS, FeatureRateLimit: config.RateLimit,
		FeatureContracts: config.Contracts, FeatureRealtime: config.Realtime,
		FeatureAuth: config.Auth.Enabled, FeatureDB: config.DB,
		FeatureSEO: config.SEO.Enabled, FeatureObservability: config.Observability,
	} {
		if enabled {
			features[feature] = true
		}
	}
	return features
}

// ExtensionProtocolVersion is the executable build-time extension contract
// supported by this GOWDK release.
const ExtensionProtocolVersion = 1

const (
	ExtensionCapabilityCSSProcessor    = "gowdk.css-processor"
	ExtensionCapabilityGoBlockConsumer = "gowdk.go-block-consumer"
	ExtensionCapabilitySEOProvider     = "gowdk.seo-provider"
	ExtensionCapabilityAuthSession     = "gowdk.auth-session-provider"
)

// ExtensionPhase names an explicit compiler phase an extension participates in.
type ExtensionPhase string

const (
	ExtensionPhaseValidate      ExtensionPhase = "validate"
	ExtensionPhasePlan          ExtensionPhase = "plan"
	ExtensionPhaseGeneratedGo   ExtensionPhase = "generated-go"
	ExtensionPhaseCSS           ExtensionPhase = "css"
	ExtensionPhaseBuildMetadata ExtensionPhase = "build-metadata"
)

// ExtensionCapabilityDescriptor declares one versioned host capability.
type ExtensionCapabilityDescriptor struct {
	Name     string
	Version  int
	Required bool
}

// ExtensionDescriptor identifies a behaviorful build-time extension.
type ExtensionDescriptor struct {
	Name            string
	ProtocolVersion int
	Phases          []ExtensionPhase
	Capabilities    []ExtensionCapabilityDescriptor
}

// Extension is distinct from built-in feature selection. Optional compiler
// capabilities are exposed through ExtensionCapabilityProvider.
type Extension interface {
	Name() string
	Descriptor() ExtensionDescriptor
}

// Addon is a config declaration for a named feature set. Some addons also
// implement build-time extension interfaces such as CSSProcessor, SEOProvider,
// or GoBlockConsumer; request-time services remain wired by generated app hooks
// or runtime packages.
type Addon interface {
	Name() string
	Features() []Feature
}

// AuthSessionOptions is the generated-app-safe subset of auth session
// configuration. Secret values stay out of compiler config; generated apps read
// them from SecretEnv at runtime.
type AuthSessionOptions struct {
	SecretEnv  string
	CookieName string
	TTL        time.Duration
	Insecure   bool
}

// AuthSessionProvider is implemented by auth addons that can supply generated
// app session setup. The compiler uses it to wire built-in auth guard backing
// code without requiring per-app hooks.
type AuthSessionProvider interface {
	AuthSessionOptions() AuthSessionOptions
}

// SEOURL describes one additional URL that an SEO addon can add to the
// generated sitemap. Loc may be absolute or root-relative.
type SEOURL = runtimeseo.URL

// SEODynamicSitemap configures an app-owned request-time sitemap provider for
// generated binaries. ImportPath and Function name a Go function with the
// signature:
//
//	func(context.Context) ([]seo.URL, error)
//
// The generated handler combines those URLs with build-time public URLs.
type SEODynamicSitemap struct {
	ImportPath   string
	Function     string
	MaxURLs      int
	CacheSeconds int
}

// SEOOptions configures build-time sitemap.xml and robots.txt emission.
type SEOOptions struct {
	BaseURL          string
	Disallow         []string
	ExtraURLs        []SEOURL
	ExtraURLProvider func() []SEOURL `json:"-"`
	DynamicSitemap   SEODynamicSitemap
}

// SEOProvider is implemented by addons that can supply build-time SEO output
// options to the compiler.
type SEOProvider interface {
	SEOOptions() SEOOptions
}

// GoBlockConsumer is an optional build-time addon extension point for targeted
// go blocks such as go addon.contracts {}.
type GoBlockConsumer interface {
	GoBlockTargets() []string
	ValidateGoBlock(target GoBlockTarget, context GoBlockContext) []GoBlockDiagnostic
	GeneratedGo(target GoBlockTarget, context GoBlockContext) ([]GoBlockFile, error)
}

// GoBlockConsumerContext is the cancellable form used by executable hosts.
type GoBlockConsumerContext interface {
	GoBlockConsumer
	ValidateGoBlockContext(context.Context, GoBlockTarget, GoBlockContext) []GoBlockDiagnostic
	GeneratedGoContext(context.Context, GoBlockTarget, GoBlockContext) ([]GoBlockFile, error)
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

type builtinAddon struct{ addon }

// NewAddon creates the legacy built-in feature marker. New configuration should
// prefer Config.Features; the constructor remains trusted for 0.x compatibility.
func NewAddon(name string, features ...Feature) Addon {
	return NewBuiltinAddon(name, features...)
}

// NewBuiltinAddon creates the deprecated constructor adapter used by bundled
// feature packages. The concrete marker is sealed so ordinary custom Addon
// implementations cannot claim compiler-owned features without implementing a
// matching executable capability.
func NewBuiltinAddon(name string, features ...Feature) Addon {
	return builtinAddon{addon{name: name, features: append([]Feature(nil), features...)}}
}

func (a addon) Name() string {
	return a.name
}

func (a addon) Features() []Feature {
	return append([]Feature(nil), a.features...)
}

// ValidateAddons checks addon identity and feature ownership before compiler
// planning so invalid declarations fail as config errors instead of panicking
// during feature lookup or addon execution.
func ValidateAddons(addons []Addon) error {
	names := map[string]int{}
	features := map[Feature]int{}
	for index, addon := range addons {
		if addonIsNil(addon) {
			return fmt.Errorf("addons[%d] is nil", index)
		}
		name := strings.TrimSpace(addon.Name())
		if name == "" {
			return fmt.Errorf("addons[%d].Name is required", index)
		}
		if previous, ok := names[name]; ok {
			return fmt.Errorf("addons[%d] %q duplicates addons[%d]", index, name, previous)
		}
		names[name] = index
		addonFeatures := addon.Features()
		if len(addonFeatures) == 0 {
			return fmt.Errorf("addons[%d] %q must declare at least one feature", index, name)
		}
		if err := validateAddonFeatureContracts(index, name, addon, addonFeatures); err != nil {
			return err
		}
		for featureIndex, feature := range addonFeatures {
			if strings.TrimSpace(string(feature)) == "" {
				return fmt.Errorf("addons[%d] %q declares empty feature at index %d", index, name, featureIndex)
			}
			if previous, ok := features[feature]; ok && !duplicateFeatureAllowed(feature) {
				return fmt.Errorf("addons[%d] %q duplicates feature %q already owned by addons[%d]", index, name, feature, previous)
			}
			if _, ok := features[feature]; !ok {
				features[feature] = index
			}
			if isCoreFeature(feature) && !isBuiltinAddon(addon) && !legacyExecutableFeature(addon, feature) {
				return fmt.Errorf("addons[%d] %q cannot claim built-in feature %q; use Config.Features or migrate executable behavior to Config.Extensions", index, name, feature)
			}
		}
	}
	return nil
}

func isBuiltinAddon(value Addon) bool {
	_, ok := value.(builtinAddon)
	return ok
}

func isCoreFeature(feature Feature) bool {
	switch feature {
	case FeatureSPA, FeatureActions, FeaturePartial, FeatureSSR, FeatureAPI,
		FeatureEmbed, FeatureCSS, FeatureRateLimit, FeatureContracts,
		FeatureRealtime, FeatureAuth, FeatureDB, FeatureSEO,
		FeatureObservability:
		return true
	default:
		return false
	}
}

func legacyExecutableFeature(addon Addon, feature Feature) bool {
	capabilities := ResolveAddonCapabilities(addon)
	switch feature {
	case FeatureCSS:
		return capabilities.CSSProcessor != nil
	case FeatureSEO:
		return capabilities.SEOProvider != nil
	case FeatureAuth:
		return capabilities.AuthSessionProvider != nil
	default:
		return false
	}
}

func validateAddonFeatureContracts(index int, name string, addon Addon, features []Feature) error {
	capabilities := ResolveAddonCapabilities(addon)
	for _, feature := range features {
		switch feature {
		case FeatureSEO:
			if capabilities.SEOProvider == nil {
				return fmt.Errorf("addons[%d] %q declares feature %q but does not provide gowdk.SEOProvider capability", index, name, feature)
			}
		case FeatureAuth:
			if capabilities.AuthSessionProvider == nil {
				return fmt.Errorf("addons[%d] %q declares feature %q but does not provide gowdk.AuthSessionProvider capability", index, name, feature)
			}
		}
	}
	return nil
}

func addonIsNil(addon Addon) bool {
	if addon == nil {
		return true
	}
	value := reflect.ValueOf(addon)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func duplicateFeatureAllowed(feature Feature) bool {
	return feature == FeatureSPA
}

// FeatureSet is a lookup table of enabled compiler/generator capabilities.
type FeatureSet map[Feature]bool

// EnabledFeatures returns built-in feature selection plus deprecated addon
// compatibility markers. Executable extensions cannot claim built-in features.
func EnabledFeatures(config Config) FeatureSet {
	features := config.Features.enabled()
	for _, addon := range config.Addons {
		if addonIsNil(addon) {
			continue
		}
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

// HasFeature reports whether typed or compatibility config selects a feature.
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
	ProjectRoot string
	ConfigDir   string
	SourceRoot  string
	WorkingDir  string
	Sources     []CSSSource
	OutputDir   string
	Build       BuildConfig
	CSS         CSSConfig
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

// CSSProcessorContext is the cancellable form used by executable hosts.
type CSSProcessorContext interface {
	CSSProcessor
	ProcessCSSContext(context.Context, CSSContext) (CSSResult, error)
}

// AddonCapabilities describes the optional compiler and generated-output
// capabilities exposed by an addon. Executable config bridges use this
// descriptor because Go interface method sets cannot be reconstructed
// dynamically from data.
type AddonCapabilities struct {
	CSSProcessor        CSSProcessor
	SEOProvider         SEOProvider
	AuthSessionProvider AuthSessionProvider
	GoBlockConsumer     GoBlockConsumer
}

// AddonCapabilityProvider exposes explicit optional addon capabilities.
// Ordinary in-process addons can keep implementing the optional interfaces
// directly; bridges and adapters should implement this descriptor instead.
type AddonCapabilityProvider interface {
	AddonCapabilities() AddonCapabilities
}

// ExtensionCapabilityProvider exposes the executable capabilities of an
// Extension independently from built-in feature selection.
type ExtensionCapabilityProvider interface {
	ExtensionCapabilities() AddonCapabilities
}

// ResolveExtensionCapabilities returns the explicit capabilities of an
// executable extension.
func ResolveExtensionCapabilities(extension Extension) AddonCapabilities {
	if extension == nil {
		return AddonCapabilities{}
	}
	provider, ok := extension.(ExtensionCapabilityProvider)
	if !ok {
		return AddonCapabilities{}
	}
	return provider.ExtensionCapabilities()
}

// ValidateExtensions checks protocol, identity, phases, and capability
// descriptors without executing extension operations.
func ValidateExtensions(extensions []Extension) error {
	names := map[string]int{}
	for index, extension := range extensions {
		if extension == nil {
			return fmt.Errorf("extensions[%d] is nil", index)
		}
		descriptor := extension.Descriptor()
		name := strings.TrimSpace(extension.Name())
		if name == "" || strings.TrimSpace(descriptor.Name) != name {
			return fmt.Errorf("extensions[%d] has inconsistent name %q", index, name)
		}
		if previous, ok := names[name]; ok {
			return fmt.Errorf("extensions[%d] %q duplicates extensions[%d]", index, name, previous)
		}
		names[name] = index
		if descriptor.ProtocolVersion != ExtensionProtocolVersion {
			return fmt.Errorf("extensions[%d] %q requires protocol %d; GOWDK supports %d", index, name, descriptor.ProtocolVersion, ExtensionProtocolVersion)
		}
		phases := map[ExtensionPhase]bool{}
		for phaseIndex, phase := range descriptor.Phases {
			switch phase {
			case ExtensionPhaseValidate, ExtensionPhasePlan, ExtensionPhaseGeneratedGo, ExtensionPhaseCSS, ExtensionPhaseBuildMetadata:
			default:
				return fmt.Errorf("extensions[%d] %q has unknown phase %q at index %d", index, name, phase, phaseIndex)
			}
			if phases[phase] {
				return fmt.Errorf("extensions[%d] %q repeats phase %q", index, name, phase)
			}
			phases[phase] = true
		}
		resolved := ResolveExtensionCapabilities(extension)
		capabilityNames := map[string]bool{}
		for capabilityIndex, capability := range descriptor.Capabilities {
			if strings.TrimSpace(capability.Name) == "" || capability.Version <= 0 {
				return fmt.Errorf("extensions[%d] %q has invalid capability at index %d", index, name, capabilityIndex)
			}
			if capabilityNames[capability.Name] {
				return fmt.Errorf("extensions[%d] %q repeats capability %q", index, name, capability.Name)
			}
			capabilityNames[capability.Name] = true
			if capability.Version != 1 && capability.Required {
				return fmt.Errorf("extensions[%d] %q requires unsupported capability %q version %d", index, name, capability.Name, capability.Version)
			}
			available := true
			switch capability.Name {
			case ExtensionCapabilityCSSProcessor:
				available = resolved.CSSProcessor != nil
			case ExtensionCapabilityGoBlockConsumer:
				available = resolved.GoBlockConsumer != nil
			case ExtensionCapabilitySEOProvider:
				available = resolved.SEOProvider != nil
			case ExtensionCapabilityAuthSession:
				available = resolved.AuthSessionProvider != nil
			default:
				available = false
			}
			if capability.Required && !available {
				return fmt.Errorf("extensions[%d] %q requires unavailable capability %q", index, name, capability.Name)
			}
		}
	}
	return nil
}

// ResolveAddonCapabilities returns explicit addon capabilities when available,
// otherwise it derives them from the optional interfaces implemented directly
// by the addon.
func ResolveAddonCapabilities(addon Addon) AddonCapabilities {
	if addonIsNil(addon) {
		return AddonCapabilities{}
	}
	if provider, ok := addon.(AddonCapabilityProvider); ok {
		return provider.AddonCapabilities()
	}
	var capabilities AddonCapabilities
	if processor, ok := addon.(CSSProcessor); ok {
		capabilities.CSSProcessor = processor
	}
	if provider, ok := addon.(SEOProvider); ok {
		capabilities.SEOProvider = provider
	}
	if provider, ok := addon.(AuthSessionProvider); ok {
		capabilities.AuthSessionProvider = provider
	}
	if consumer, ok := addon.(GoBlockConsumer); ok {
		capabilities.GoBlockConsumer = consumer
	}
	return capabilities
}
