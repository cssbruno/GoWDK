package appgen

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
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
	Fragments    []FragmentEndpoint
	SSR          []SSRRoute
	AutoRoutes   bool
	ProxyBackend bool
	Config       gowdk.Config
	IR           *gwdkir.Program
}

// ActionEndpoint describes a generated action handler.
type ActionEndpoint struct {
	PageID           string
	ActionName       string
	Method           string
	Route            string
	Guards           []string
	InputName        string
	InputType        string
	InputFields      []string
	RequiredFields   []string
	RequiredMessages map[string]string
	ValidationRules  []ActionValidationRule
	ValidatesInput   bool
	Redirect         string
	Fragments        []ActionFragment
	ErrorPage        string
	Binding          source.BackendBinding
	BackendAlias     string
	Source           string
	SourceSpan       source.SourceSpan
}

// ActionValidationRule describes one generated server-side form constraint.
type ActionValidationRule struct {
	Field            string
	MinLength        int
	MinLengthMessage string
	MaxLength        int
	MaxLengthMessage string
	Pattern          string
	PatternMessage   string
}

// APIEndpoint describes a generated API handler.
type APIEndpoint struct {
	PageID       string
	APIName      string
	Method       string
	Route        string
	Guards       []string
	ErrorPage    string
	Binding      source.BackendBinding
	BackendAlias string
	Source       string
	SourceSpan   source.SourceSpan
}

// FragmentEndpoint describes a generated server fragment handler.
type FragmentEndpoint struct {
	PageID       string
	FragmentName string
	Method       string
	Route        string
	RouteParams  []source.RouteParam
	Target       string
	HTML         string
	Package      string
	Uses         map[string]string
	Guards       []string
	Binding      source.BackendBinding
	BackendAlias string
	Source       string
	SourceSpan   source.SourceSpan
}

// ActionFragment describes a generated partial response fragment.
type ActionFragment struct {
	Target string
	HTML   string
}

// SSRRoute describes a generated request-time page handler.
type SSRRoute struct {
	PageID           string
	Route            string
	Render           gowdk.RenderMode
	Cache            string
	ErrorPage        string
	DynamicParams    []string
	RouteParams      []source.RouteParam
	Guards           []string
	HasLoad          bool
	LoadBinding      source.BackendBinding
	LoadBackendAlias string
	Source           string
	SourceSpan       source.SourceSpan
	HTML             string
	Replacements     []SSRReplacement
	LoadReplacements []SSRLoadReplacement
}

type SSRReplacement = source.SSRReplacement

type SSRLoadReplacement = source.SSRLoadReplacement
