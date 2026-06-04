package gowdk

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

// ModuleConfig names a source group inside a GOWDK app. Current build discovery
// uses module sources; future code generation can use Type as user-defined
// metadata to decide which artifacts to emit.
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
	Assets      AssetMode
	Stylesheets []Stylesheet
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
