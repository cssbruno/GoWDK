package lang

import (
	"encoding/json"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// SiteMap is an editor-facing route and file map.
type SiteMap struct {
	Pages     []SiteMapPage     `json:"pages"`
	Routes    []SiteMapRoute    `json:"routes,omitempty"`
	Endpoints []SiteMapEndpoint `json:"endpoints,omitempty"`
}

// SiteMapPage describes one movable page file and its route identity.
type SiteMapPage struct {
	ID            string           `json:"id"`
	Route         string           `json:"route"`
	Source        string           `json:"source"`
	Render        gowdk.RenderMode `json:"render"`
	Layouts       []string         `json:"layouts,omitempty"`
	Guard         []string         `json:"guard,omitempty"`
	DynamicParams []string         `json:"dynamicParams,omitempty"`
	Blocks        SiteMapBlocks    `json:"blocks"`
}

// SiteMapBlocks records which top-level source blocks are present.
type SiteMapBlocks struct {
	Paths     bool     `json:"paths"`
	Build     bool     `json:"build"`
	Load      bool     `json:"load"`
	View      bool     `json:"view"`
	Actions   []string `json:"actions,omitempty"`
	APIs      []string `json:"apis,omitempty"`
	Fragments []string `json:"fragments,omitempty"`
}

// SiteMapRoute describes one generated route graph entry.
type SiteMapRoute struct {
	Kind          compiler.RouteKind `json:"kind"`
	Method        string             `json:"method"`
	Route         string             `json:"route"`
	PageID        string             `json:"pageId"`
	Package       string             `json:"package,omitempty"`
	Render        gowdk.RenderMode   `json:"render,omitempty"`
	Cache         string             `json:"cache,omitempty"`
	DynamicParams []string           `json:"dynamicParams,omitempty"`
	RouteParams   []routeParamJSON   `json:"routeParams,omitempty"`
	Layouts       []string           `json:"layouts,omitempty"`
	Guards        []string           `json:"guards,omitempty"`
	Source        string             `json:"source,omitempty"`
	SourceSpan    *sourceSpanJSON    `json:"sourceSpan,omitempty"`
	Handler       string             `json:"handler,omitempty"`
}

// SiteMapEndpoint describes one generated endpoint graph entry.
type SiteMapEndpoint struct {
	Kind           compiler.EndpointKind       `json:"kind"`
	EndpointSource string                      `json:"endpointSource,omitempty"`
	Source         string                      `json:"source,omitempty"`
	SourceSpan     *sourceSpanJSON             `json:"sourceSpan,omitempty"`
	Method         string                      `json:"method"`
	Route          string                      `json:"route"`
	PageID         string                      `json:"pageId"`
	Symbol         string                      `json:"symbol,omitempty"`
	Package        string                      `json:"package,omitempty"`
	PackagePath    string                      `json:"packagePath,omitempty"`
	PackageName    string                      `json:"packageName,omitempty"`
	Cache          string                      `json:"cache,omitempty"`
	DynamicParams  []string                    `json:"dynamicParams,omitempty"`
	RouteParams    []routeParamJSON            `json:"routeParams,omitempty"`
	Guards         []string                    `json:"guards,omitempty"`
	CSRF           bool                        `json:"csrf,omitempty"`
	Handler        string                      `json:"handler,omitempty"`
	BindingStatus  source.BackendBindingStatus `json:"bindingStatus,omitempty"`
	BindingMessage string                      `json:"bindingMessage,omitempty"`
	Signature      source.BackendSignatureKind `json:"signature,omitempty"`
	InputType      string                      `json:"inputType,omitempty"`
	Contract       *siteMapContractJSON        `json:"contract,omitempty"`
}

type siteMapContractJSON struct {
	Name        string                       `json:"name"`
	Kind        gwdkir.ContractKind          `json:"kind"`
	Status      gwdkir.ContractBindingStatus `json:"status"`
	Message     string                       `json:"message,omitempty"`
	ImportAlias string                       `json:"importAlias,omitempty"`
	ImportPath  string                       `json:"importPath,omitempty"`
	Type        string                       `json:"type,omitempty"`
	Result      string                       `json:"result,omitempty"`
	Roles       []string                     `json:"roles,omitempty"`
	Handler     string                       `json:"handler,omitempty"`
	Register    string                       `json:"register,omitempty"`
}

type sourceSpanJSON struct {
	Start sourcePositionJSON `json:"start"`
	End   sourcePositionJSON `json:"end"`
}

type sourcePositionJSON struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// BuildSiteMapFromIR converts stable compiler IR into the editor-facing site
// map.
func BuildSiteMapFromIR(config gowdk.Config, ir gwdkir.Program) SiteMap {
	pages := siteMapPages(config, ir)
	metadata := compiler.BuildRouteMetadataFromIR(config, ir)
	return siteMapFromMetadata(pages, metadata)
}

func siteMapPages(config gowdk.Config, ir gwdkir.Program) []SiteMapPage {
	pages := make([]SiteMapPage, 0, len(ir.Pages))
	for _, page := range ir.Pages {
		pages = append(pages, SiteMapPage{
			ID:            page.ID,
			Route:         page.Route,
			Source:        page.Source,
			Render:        page.RenderMode(config.Render.DefaultMode()),
			Layouts:       page.Layouts,
			Guard:         page.Guards,
			DynamicParams: page.DynamicParams(),
			Blocks: SiteMapBlocks{
				Paths:     page.Blocks.Paths,
				Build:     page.Blocks.Build,
				Load:      page.Blocks.Server,
				View:      page.Blocks.View,
				Actions:   actionNames(page.Blocks.Actions),
				APIs:      apiNames(page.Blocks.APIs),
				Fragments: fragmentEndpointNames(page.Blocks.Fragments),
			},
		})
	}
	return pages
}

func siteMapFromMetadata(pages []SiteMapPage, metadata compiler.RouteMetadata) SiteMap {
	return SiteMap{
		Pages:     pages,
		Routes:    siteMapRoutes(metadata.Routes),
		Endpoints: siteMapEndpoints(metadata.Endpoints),
	}
}

// SiteMapJSON returns the JSON site map for parsed and validated files.
func SiteMapJSON(config gowdk.Config, paths []string) ([]byte, Diagnostics) {
	return SiteMapJSONWithOptions(config, paths, CheckOptions{})
}

// SiteMapJSONWithOptions returns the JSON site map with explicit project
// context.
func SiteMapJSONWithOptions(config gowdk.Config, paths []string, options CheckOptions) ([]byte, Diagnostics) {
	result, diagnostics := CheckFilesWithOptions(config, paths, options)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	payload, err := json.MarshalIndent(BuildSiteMapFromIR(config, result.IR), "", "  ")
	if err != nil {
		return nil, Diagnostics{{Severity: "error", Message: err.Error()}}
	}
	return append(payload, '\n'), diagnostics
}

func actionNames(actions []gwdkir.Action) []string {
	if len(actions) == 0 {
		return nil
	}
	names := make([]string, 0, len(actions))
	for _, action := range actions {
		names = append(names, action.Name)
	}
	return names
}

func apiNames(apis []gwdkir.API) []string {
	if len(apis) == 0 {
		return nil
	}
	names := make([]string, 0, len(apis))
	for _, api := range apis {
		names = append(names, api.Name)
	}
	return names
}

func siteMapRoutes(routes []compiler.RouteBinding) []SiteMapRoute {
	if len(routes) == 0 {
		return nil
	}
	out := make([]SiteMapRoute, 0, len(routes))
	for _, route := range routes {
		out = append(out, SiteMapRoute{
			Kind:          route.Kind,
			Method:        route.Method,
			Route:         route.Route,
			PageID:        route.PageID,
			Package:       route.Package,
			Render:        route.Render,
			Cache:         route.Cache,
			DynamicParams: append([]string(nil), route.DynamicParams...),
			RouteParams:   routeParamsJSON(route.RouteParams),
			Layouts:       append([]string(nil), route.Layouts...),
			Guards:        append([]string(nil), route.Guards...),
			Source:        route.Source,
			SourceSpan:    siteMapSourceSpanJSON(route.SourceSpan),
			Handler:       route.Handler,
		})
	}
	return out
}

func siteMapEndpoints(endpoints []compiler.EndpointBinding) []SiteMapEndpoint {
	if len(endpoints) == 0 {
		return nil
	}
	out := make([]SiteMapEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		item := SiteMapEndpoint{
			Kind:           endpoint.Kind,
			EndpointSource: endpoint.EndpointSource,
			Source:         endpoint.Source,
			SourceSpan:     siteMapSourceSpanJSON(endpoint.SourceSpan),
			Method:         endpoint.Method,
			Route:          endpoint.Route,
			PageID:         endpoint.PageID,
			Symbol:         endpoint.Symbol,
			Package:        endpoint.Package,
			PackagePath:    endpoint.PackagePath,
			PackageName:    endpoint.PackageName,
			Cache:          endpoint.Cache,
			DynamicParams:  append([]string(nil), endpoint.DynamicParams...),
			RouteParams:    routeParamsJSON(endpoint.RouteParams),
			Guards:         append([]string(nil), endpoint.Guards...),
			CSRF:           endpoint.CSRF,
			Handler:        endpoint.Handler,
			BindingStatus:  endpoint.BindingStatus,
			BindingMessage: endpoint.BindingMessage,
			Signature:      endpoint.BindingSignature,
			InputType:      endpoint.BindingInputType,
		}
		if endpoint.Contract.Name != "" {
			item.Contract = &siteMapContractJSON{
				Name:        endpoint.Contract.Name,
				Kind:        endpoint.Contract.Kind,
				Status:      endpoint.Contract.Status,
				Message:     endpoint.Contract.Message,
				ImportAlias: endpoint.Contract.ImportAlias,
				ImportPath:  endpoint.Contract.ImportPath,
				Type:        endpoint.Contract.Type,
				Result:      endpoint.Contract.Result,
				Roles:       append([]string(nil), endpoint.Contract.Roles...),
				Handler:     endpoint.Contract.Handler,
				Register:    endpoint.Contract.Register,
			}
		}
		out = append(out, item)
	}
	return out
}

func siteMapSourceSpanJSON(span source.SourceSpan) *sourceSpanJSON {
	if span.Start.Line <= 0 || span.Start.Column <= 0 || span.End.Line <= 0 || span.End.Column <= 0 {
		return nil
	}
	return &sourceSpanJSON{
		Start: sourcePositionJSON{Line: span.Start.Line, Column: span.Start.Column},
		End:   sourcePositionJSON{Line: span.End.Line, Column: span.End.Column},
	}
}
