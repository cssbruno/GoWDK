package gowdk

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	runtimeseo "github.com/cssbruno/gowdk/runtime/seo"
)

// Config describes how a GOWDK application should be discovered, compiled,
// and packaged.
type Config struct {
	AppName   string
	Source    SourceConfig
	Modules   []ModuleConfig
	Render    RenderConfig
	I18N      I18NConfig
	Env       EnvConfig
	Lifecycle LifecycleConfig
	Build     BuildConfig
	CSS       CSSConfig
	Addons    []Addon
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

// BuildParams carries compile-time route values into Go build helpers.
type BuildParams struct {
	Route  map[string]string `json:"route,omitempty"`
	Locale string            `json:"locale,omitempty"`
}

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

// BuildConfig controls output artifacts and frontend asset packaging.
type BuildConfig struct {
	Output              string
	Mode                BuildMode
	Assets              AssetMode
	ObfuscateAssets     bool
	Head                HeadConfig
	CSRF                CSRFConfig
	CORS                CORSConfig
	SecurityHeaders     SecurityHeadersConfig
	BodyLimits          BodyLimitsConfig
	AllowMissingBackend bool
	Stylesheets         []Stylesheet
	Scripts             []Script
	Worker              ContractWorkerConfig
	Cron                ContractCronConfig
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

// CORSConfig controls generated CORS headers and preflight handling for API
// and web contract endpoints. It is disabled by default, so generated
// endpoints remain same-origin unless a policy is declared.
type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
}

// EnabledForGeneratedAPIs reports whether generated API/contract routes should
// install CORS handling.
func (config CORSConfig) EnabledForGeneratedAPIs() bool {
	return config.Enabled
}

// Validate checks structural safety rules for the generated CORS policy.
func (config CORSConfig) Validate() error {
	if !config.Enabled {
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
	if strings.ContainsAny(origin, "\r\n") {
		return fmt.Errorf("Build.CORS.AllowedOrigins contains invalid origin %q", origin)
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("Build.CORS.AllowedOrigins contains invalid origin %q: %w", origin, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("Build.CORS.AllowedOrigins origin %q must use http or https", origin)
	}
	if parsed.User != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("Build.CORS.AllowedOrigins origin %q must not include userinfo, query, or fragment", origin)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("Build.CORS.AllowedOrigins origin %q must not include a path", origin)
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
// BackendBinary, and optional deployment recipes.
type BuildTargetConfig struct {
	Name          string
	Modules       []string
	Output        string
	App           string
	Binary        string
	WASM          string
	BackendApp    string
	BackendBinary string
	WorkerApp     string
	WorkerBinary  string
	Worker        ContractWorkerConfig
	CronApp       string
	CronBinary    string
	Cron          ContractCronConfig
	DeployRecipes []string
}

// ContractWorkerConfig controls generated standalone contract worker targets.
// EventSource is required and must name a function returning
// (contracts.EventSource, error). SeenStore and Backoff are optional provider
// hooks returning (contracts.SeenStore, error) and
// (contracts.EventWorkerBackoff, error).
type ContractWorkerConfig struct {
	EventSource ServiceRef
	SeenStore   ServiceRef
	Backoff     ServiceRef
}

// ContractCronConfig controls generated standalone scheduled job targets.
type ContractCronConfig struct {
	Jobs []ContractCronJobConfig
}

// ContractCronJobConfig declares one generated cron role job. Type accepts the
// scanned job type name, package-qualified name, or full import-path-qualified
// name. Schedule currently supports @once and @every <duration>.
type ContractCronJobConfig struct {
	Type            string
	Schedule        string
	OverlapPolicy   string
	MissedRunPolicy string
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
