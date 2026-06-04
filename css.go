package gowdk

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
