package appgen

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// Result describes generated app artifacts.
type Result struct {
	AppDir      string
	MainPath    string
	PackagePath string
	// ModulePath is the generated nested go.mod path when app generation has to
	// fall back to module isolation. It is empty when the generated app lives
	// inside and builds from the application module.
	ModulePath string
	OutputDir  string
	Files      []string
	BinaryPath string
	Role       string
	Contracts  []string
	Jobs       []string
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
	Program      *compiler.ValidatedProgram
	// IR is the legacy raw-IR option path. Production auto-routing should pass
	// Program so generation receives a compiler-validated phase token.
	IR      *gwdkir.Program
	Sitemap buildgen.RuntimeSitemapPlan
}

// ApplicationPlan is the normalized generated-application plan consumed by
// appgen emission. Construct it through PlanApplication or
// PlanBackendApplication so route defaults, endpoint projections, SSR artifacts,
// sitemap data, and generator-local validation are finalized before writes.
type ApplicationPlan struct {
	options     Options
	outputDir   string
	backendOnly bool
	valid       bool
}

func (plan ApplicationPlan) optionsForEmit() (Options, error) {
	if !plan.valid {
		return Options{}, errInvalidApplicationPlan
	}
	return plan.options, nil
}

// OptionsFromValidatedProgram returns production generator options for
// compiler-validated IR-driven route generation.
func OptionsFromValidatedProgram(config gowdk.Config, program compiler.ValidatedProgram) Options {
	ir := program.Program()
	return Options{AutoRoutes: true, Config: config, Program: &program, IR: &ir}
}

// OptionsFromIR returns the production generator options for compiler IR-driven
// route generation.
func OptionsFromIR(config gowdk.Config, ir *gwdkir.Program) Options {
	return Options{AutoRoutes: true, Config: config, IR: ir}
}

// ActionEndpoint describes a generated action handler.
type ActionEndpoint struct {
	EndpointID       gwdkir.EndpointID
	PageID           string
	ActionName       string
	Method           string
	Route            string
	Guards           []string
	InputName        string
	InputType        string
	InputFields      []string
	UploadFields     []ActionUploadField
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

// ActionUploadField describes one generated multipart upload field policy.
type ActionUploadField struct {
	Field               string
	MaxFiles            int
	MaxBytes            int64
	AllowedContentTypes []string
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
	EndpointID   gwdkir.EndpointID
	PageID       string
	APIName      string
	Method       string
	Route        string
	Guards       []string
	ErrorPage    string
	CORS         gwdkir.EndpointCORS
	Binding      source.BackendBinding
	BackendAlias string
	Source       string
	SourceSpan   source.SourceSpan
}

// FragmentEndpoint describes a generated server fragment handler.
type FragmentEndpoint struct {
	EndpointID   gwdkir.EndpointID
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
	LayoutErrorPages []LayoutErrorPage
	Locale           string
	DynamicParams    []string
	RouteParams      []source.RouteParam
	Layouts          []string
	Guards           []string
	HasLoad          bool
	LoadBinding      source.BackendBinding
	LoadBackendAlias string
	Source           string
	SourceSpan       source.SourceSpan
	HTML             string
	Replacements     []SSRReplacement
	LoadReplacements []SSRLoadReplacement
	ListSpecs        []SSRListSpec
	CondSpecs        []SSRCondSpec
	QueryRegions     []SSRQueryRegion
}

type LayoutErrorPage struct {
	Layout    string
	ErrorPage string
}

type SSRReplacement = source.SSRReplacement

type SSRLoadReplacement = source.SSRLoadReplacement

type SSRListSpec = source.SSRListSpec

type SSRCondSpec = source.SSRCondSpec

type SSRListField = source.SSRListField

type SSRQueryRegion = source.SSRQueryRegion
