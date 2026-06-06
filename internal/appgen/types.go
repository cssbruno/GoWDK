package appgen

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// Result describes generated app artifacts.
type Result struct {
	AppDir      string
	MainPath    string
	PackagePath string
	ModulePath  string
	OutputDir   string
	Files       []string
	BinaryPath  string
}

// Options configures generated app output.
type Options struct {
	Actions      []ActionEndpoint
	APIs         []APIEndpoint
	SSR          []SSRRoute
	AutoRoutes   bool
	ProxyBackend bool
	Config       gowdk.Config
	IR           *gwdkir.Program
}

// ActionEndpoint describes a generated action handler.
type ActionEndpoint struct {
	PageID         string
	ActionName     string
	Method         string
	Route          string
	Guards         []string
	InputName      string
	InputType      string
	InputFields    []string
	RequiredFields []string
	ValidatesInput bool
	Redirect       string
	Fragments      []ActionFragment
	Binding        manifest.BackendBinding
	BackendAlias   string
}

// APIEndpoint describes a generated API handler.
type APIEndpoint struct {
	PageID       string
	APIName      string
	Method       string
	Route        string
	Guards       []string
	Binding      manifest.BackendBinding
	BackendAlias string
}

// ActionFragment describes a generated partial response fragment.
type ActionFragment struct {
	Target string
	HTML   string
}

// SSRRoute describes a generated request-time page handler.
type SSRRoute struct {
	PageID        string
	Route         string
	Render        gowdk.RenderMode
	DynamicParams []string
	Guards        []string
	HasLoad       bool
	HTML          string
	Replacements  []SSRReplacement
}

// SSRReplacement maps a generated placeholder back to a request route param.
type SSRReplacement struct {
	Param       string
	Placeholder string
}
