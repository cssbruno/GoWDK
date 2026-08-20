package gowdk

// Config is the root application configuration. Each field delegates to a
// focused config type; ValidateStructural rejects invalid cross-field states.
type Config struct {
	AppName    string
	Source     SourceConfig
	Modules    []ModuleConfig
	Render     RenderConfig
	I18N       I18NConfig
	Env        EnvConfig
	Lifecycle  LifecycleConfig
	Interop    InteropConfig
	Build      BuildConfig
	CSS        CSSConfig
	Features   FeatureConfig
	Extensions []Extension
	// Addons is the deprecated 0.x compatibility lane. New config should use
	// Features for built-ins and Extensions for build-time behavior.
	Addons []Addon
}

// SourceConfig selects portable .gwdk files for discovery.
type SourceConfig struct {
	Include []string
	Exclude []string
}

// ModuleConfig names a source group inside a GOWDK app.
type ModuleConfig struct {
	Name   string
	Type   string
	Source SourceConfig
}

// RenderConfig controls default render behavior. SPA is the default.
type RenderConfig struct {
	Default RenderMode
}

// BuildParams carries compile-time route values into Go build helpers.
type BuildParams struct {
	Route  map[string]string `json:"route,omitempty"`
	Locale string            `json:"locale,omitempty"`
}
