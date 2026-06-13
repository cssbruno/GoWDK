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
	Paths   bool     `json:"paths"`
	Build   bool     `json:"build"`
	Load    bool     `json:"load"`
	View    bool     `json:"view"`
	Actions []string `json:"actions,omitempty"`
	APIs    []string `json:"apis,omitempty"`
}

// SiteMapRoute describes one generated route graph entry.
type SiteMapRoute struct {
	Kind    compiler.RouteKind `json:"kind"`
	Method  string             `json:"method"`
	Route   string             `json:"route"`
	PageID  string             `json:"pageId"`
	Handler string             `json:"handler,omitempty"`
}

// SiteMapEndpoint describes one generated action/API endpoint graph entry.
type SiteMapEndpoint struct {
	Kind          compiler.EndpointKind       `json:"kind"`
	Method        string                      `json:"method"`
	Route         string                      `json:"route"`
	PageID        string                      `json:"pageId"`
	Symbol        string                      `json:"symbol,omitempty"`
	Package       string                      `json:"package,omitempty"`
	DynamicParams []string                    `json:"dynamicParams,omitempty"`
	RouteParams   []routeParamJSON            `json:"routeParams,omitempty"`
	BindingStatus source.BackendBindingStatus `json:"bindingStatus,omitempty"`
	Signature     source.BackendSignatureKind `json:"signature,omitempty"`
	InputType     string                      `json:"inputType,omitempty"`
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
				Paths:   page.Blocks.Paths,
				Build:   page.Blocks.Build,
				Load:    page.Blocks.Load,
				View:    page.Blocks.View,
				Actions: actionNames(page.Blocks.Actions),
				APIs:    apiNames(page.Blocks.APIs),
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
			Kind:    route.Kind,
			Method:  route.Method,
			Route:   route.Route,
			PageID:  route.PageID,
			Handler: route.Handler,
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
		out = append(out, SiteMapEndpoint{
			Kind:          endpoint.Kind,
			Method:        endpoint.Method,
			Route:         endpoint.Route,
			PageID:        endpoint.PageID,
			Symbol:        endpoint.Symbol,
			Package:       endpoint.Package,
			DynamicParams: append([]string(nil), endpoint.DynamicParams...),
			RouteParams:   routeParamsJSON(endpoint.RouteParams),
			BindingStatus: endpoint.BindingStatus,
			Signature:     endpoint.BindingSignature,
			InputType:     endpoint.BindingInputType,
		})
	}
	return out
}
