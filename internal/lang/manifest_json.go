package lang

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewanalysis"
)

// ManifestSchemaVersion is the current gowdk manifest JSON schema version.
const ManifestSchemaVersion = 1

// The manifest JSON report keeps the historical field names of the public
// manifest model but is derived from the compiler IR; the manifest model
// itself no longer exists.

type manifestJSON struct {
	Version         int                      `json:"version"`
	Pages           map[string]pageJSON      `json:"pages"`
	Components      map[string]componentJSON `json:"components,omitempty"`
	Layouts         map[string]layoutJSON    `json:"layouts,omitempty"`
	BackendBindings []backendBindingJSON     `json:"backendBindings,omitempty"`
}

type pageJSON struct {
	Source          string                 `json:"source,omitempty"`
	Kind            string                 `json:"kind"`
	Package         string                 `json:"package,omitempty"`
	Route           string                 `json:"route"`
	Render          gowdk.RenderMode       `json:"render"`
	Cache           string                 `json:"cache,omitempty"`
	Metadata        *metadataJSON          `json:"metadata,omitempty"`
	Imports         []importJSON           `json:"imports,omitempty"`
	Uses            []useJSON              `json:"uses,omitempty"`
	Layouts         []string               `json:"layouts,omitempty"`
	DynamicParams   []string               `json:"dynamicParams,omitempty"`
	RouteParams     []routeParamJSON       `json:"routeParams,omitempty"`
	Paths           bool                   `json:"paths,omitempty"`
	Guard           []string               `json:"guard,omitempty"`
	CSS             []string               `json:"css,omitempty"`
	JS              []string               `json:"js,omitempty"`
	InlineJS        []string               `json:"inlineJS,omitempty"`
	Blocks          blocksJSON             `json:"blocks"`
	Actions         []actionJSON           `json:"actions,omitempty"`
	APIs            []apiJSON              `json:"apis,omitempty"`
	Fragments       []fragmentEndpointJSON `json:"fragments,omitempty"`
	Components      []string               `json:"components,omitempty"`
	Assets          []string               `json:"assets,omitempty"`
	CSSClasses      []string               `json:"cssClasses,omitempty"`
	StyleAttributes []string               `json:"styleAttributes,omitempty"`
	Artifacts       []artifactJSON         `json:"artifacts,omitempty"`
}

type componentJSON struct {
	Source    string       `json:"source,omitempty"`
	Kind      string       `json:"kind"`
	Package   string       `json:"package,omitempty"`
	Imports   []importJSON `json:"imports,omitempty"`
	Uses      []useJSON    `json:"uses,omitempty"`
	CSS       []string     `json:"css,omitempty"`
	JS        []string     `json:"js,omitempty"`
	InlineJS  []string     `json:"inlineJS,omitempty"`
	Assets    []string     `json:"assets,omitempty"`
	Props     []propJSON   `json:"props,omitempty"`
	PropsType *goTypeJSON  `json:"propsType,omitempty"`
	State     *stateJSON   `json:"state,omitempty"`
	Exports   []propJSON   `json:"exports,omitempty"`
	Emits     []emitJSON   `json:"emits,omitempty"`
}

type layoutJSON struct {
	Source  string    `json:"source,omitempty"`
	Kind    string    `json:"kind"`
	Package string    `json:"package,omitempty"`
	Uses    []useJSON `json:"uses,omitempty"`
}

type metadataJSON struct {
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	Canonical   string             `json:"canonical,omitempty"`
	Image       string             `json:"image,omitempty"`
	Robots      string             `json:"robots,omitempty"`
	NoIndex     bool               `json:"noindex,omitempty"`
	Preload     []headResourceJSON `json:"preload,omitempty"`
	Prefetch    []headResourceJSON `json:"prefetch,omitempty"`
}

type headResourceJSON struct {
	Href string `json:"href"`
	As   string `json:"as,omitempty"`
}

type routeParamJSON struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type propJSON struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Default    string `json:"default,omitempty"`
	DefaultSet bool   `json:"defaultSet,omitempty"`
}

type emitJSON struct {
	Name   string          `json:"name"`
	Params []emitParamJSON `json:"params,omitempty"`
}

type emitParamJSON struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type importJSON struct {
	Alias string `json:"alias,omitempty"`
	Path  string `json:"path"`
}

type useJSON struct {
	Alias   string `json:"alias"`
	Package string `json:"package"`
}

type goTypeJSON struct {
	Alias string `json:"alias"`
	Name  string `json:"name"`
}

type goFuncJSON struct {
	Alias string `json:"alias"`
	Name  string `json:"name"`
}

type stateJSON struct {
	Type goTypeJSON `json:"type"`
	Init goFuncJSON `json:"init"`
}

type blocksJSON struct {
	Paths     bool     `json:"paths"`
	Build     bool     `json:"build"`
	Load      bool     `json:"load"`
	View      bool     `json:"view"`
	Actions   []string `json:"actions,omitempty"`
	APIs      []string `json:"apis,omitempty"`
	Fragments []string `json:"fragments,omitempty"`
}

type actionJSON struct {
	Name           string         `json:"name"`
	Method         string         `json:"method,omitempty"`
	Route          string         `json:"route,omitempty"`
	InputName      string         `json:"inputName,omitempty"`
	InputType      string         `json:"inputType,omitempty"`
	ValidatesInput bool           `json:"validatesInput,omitempty"`
	Redirect       string         `json:"redirect,omitempty"`
	Fragments      []fragmentJSON `json:"fragments,omitempty"`
}

type fragmentJSON struct {
	Target string `json:"target"`
}

type fragmentEndpointJSON struct {
	Name   string `json:"name"`
	Method string `json:"method,omitempty"`
	Route  string `json:"route"`
	Target string `json:"target"`
}

type apiJSON struct {
	Name   string `json:"name,omitempty"`
	Method string `json:"method,omitempty"`
	Route  string `json:"route,omitempty"`
}

type artifactJSON struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type backendBindingJSON struct {
	Kind          string                      `json:"kind"`
	PageID        string                      `json:"pageId"`
	Source        string                      `json:"source,omitempty"`
	BlockName     string                      `json:"blockName"`
	Method        string                      `json:"method,omitempty"`
	Route         string                      `json:"route"`
	ImportPath    string                      `json:"importPath,omitempty"`
	PackageName   string                      `json:"packageName,omitempty"`
	FunctionName  string                      `json:"functionName"`
	Signature     source.BackendSignatureKind `json:"signature,omitempty"`
	InputType     string                      `json:"inputType,omitempty"`
	InputPointer  bool                        `json:"inputPointer,omitempty"`
	InputFields   []backendInputJSON          `json:"inputFields,omitempty"`
	ResultType    string                      `json:"resultType,omitempty"`
	ResultPointer bool                        `json:"resultPointer,omitempty"`
	ResultFields  []backendResultJSON         `json:"resultFields,omitempty"`
	Status        source.BackendBindingStatus `json:"status"`
	Message       string                      `json:"message,omitempty"`
}

type backendInputJSON struct {
	FieldName string `json:"fieldName"`
	FormName  string `json:"formName"`
	Type      string `json:"type"`
}

type backendResultJSON struct {
	Path     string `json:"path"`
	Selector string `json:"selector"`
	Type     string `json:"type"`
}

// marshalManifestJSON emits the manifest JSON report from the validated IR
// program and its backend handler binding records.
func marshalManifestJSON(result CheckResult, defaultMode gowdk.RenderMode) ([]byte, error) {
	pages := map[string]pageJSON{}
	for _, page := range applyDefaultRenderMode(result.IR.Pages, defaultMode) {
		dependencies := pageDependencies(page)
		pages[page.ID] = pageJSON{
			Source:          page.Source,
			Kind:            "page",
			Package:         page.Package,
			Route:           page.Route,
			Render:          page.RenderMode(gowdk.SPA),
			Cache:           page.Cache,
			Metadata:        metadataJSONFor(page.Metadata),
			Imports:         importsJSON(page.Imports),
			Uses:            usesJSON(page.Uses),
			Layouts:         page.Layouts,
			DynamicParams:   page.DynamicParams(),
			RouteParams:     routeParamsJSON(page.TypedRouteParams()),
			Paths:           page.Blocks.Paths,
			Guard:           page.Guards,
			CSS:             page.CSS,
			JS:              page.JS,
			InlineJS:        inlineScriptNames(page.InlineJS),
			Blocks:          blocksJSONFor(page),
			Actions:         actionsJSON(page.Blocks.Actions),
			APIs:            apisJSON(page.Blocks.APIs),
			Fragments:       fragmentEndpointsJSON(page.Blocks.Fragments),
			Components:      pageComponents(page),
			Assets:          dependencies.Assets,
			CSSClasses:      dependencies.CSSClasses,
			StyleAttributes: dependencies.StyleAttributes,
			Artifacts:       artifactsJSON(page),
		}
	}
	return json.MarshalIndent(manifestJSON{
		Version:         ManifestSchemaVersion,
		Pages:           pages,
		Components:      componentsJSON(result.IR.Components),
		Layouts:         layoutsJSON(result.IR.Layouts),
		BackendBindings: backendBindingsJSON(result.Bindings),
	}, "", "  ")
}

func metadataJSONFor(metadata gwdkir.PageMetadata) *metadataJSON {
	if metadata.Title == "" && metadata.Description == "" && metadata.Canonical == "" && metadata.Image == "" &&
		metadata.Robots == "" && !metadata.NoIndex && len(metadata.Preload) == 0 && len(metadata.Prefetch) == 0 {
		return nil
	}
	return &metadataJSON{
		Title:       metadata.Title,
		Description: metadata.Description,
		Canonical:   metadata.Canonical,
		Image:       metadata.Image,
		Robots:      metadata.Robots,
		NoIndex:     metadata.NoIndex,
		Preload:     headResourcesJSON(metadata.Preload),
		Prefetch:    headResourcesJSON(metadata.Prefetch),
	}
}

func headResourcesJSON(resources []gwdkir.HeadResource) []headResourceJSON {
	if len(resources) == 0 {
		return nil
	}
	out := make([]headResourceJSON, 0, len(resources))
	for _, resource := range resources {
		out = append(out, headResourceJSON{Href: resource.Href, As: resource.As})
	}
	return out
}

func routeParamsJSON(params []source.RouteParam) []routeParamJSON {
	if len(params) == 0 {
		return nil
	}
	out := make([]routeParamJSON, 0, len(params))
	for _, param := range params {
		paramType := param.Type
		if paramType == "" {
			paramType = "string"
		}
		out = append(out, routeParamJSON{Name: param.Name, Type: paramType})
	}
	return out
}

func backendBindingsJSON(bindings []source.BackendBinding) []backendBindingJSON {
	if len(bindings) == 0 {
		return nil
	}
	out := make([]backendBindingJSON, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, backendBindingJSON{
			Kind:          binding.Kind,
			PageID:        binding.PageID,
			Source:        binding.Source,
			BlockName:     binding.BlockName,
			Method:        binding.Method,
			Route:         binding.Route,
			ImportPath:    binding.ImportPath,
			PackageName:   binding.PackageName,
			FunctionName:  binding.FunctionName,
			Signature:     binding.Signature,
			InputType:     binding.InputType,
			InputPointer:  binding.InputPointer,
			InputFields:   backendInputFieldsJSON(binding.InputFields),
			ResultType:    binding.ResultType,
			ResultPointer: binding.ResultPointer,
			ResultFields:  backendResultFieldsJSON(binding.ResultFields),
			Status:        binding.Status,
			Message:       binding.Message,
		})
	}
	return out
}

func backendResultFieldsJSON(fields []source.BackendResultField) []backendResultJSON {
	if len(fields) == 0 {
		return nil
	}
	out := make([]backendResultJSON, 0, len(fields))
	for _, field := range fields {
		out = append(out, backendResultJSON{
			Path:     field.Path,
			Selector: field.Selector,
			Type:     field.Type,
		})
	}
	return out
}

func backendInputFieldsJSON(fields []source.BackendInputField) []backendInputJSON {
	if len(fields) == 0 {
		return nil
	}
	out := make([]backendInputJSON, 0, len(fields))
	for _, field := range fields {
		out = append(out, backendInputJSON{
			FieldName: field.FieldName,
			FormName:  field.FormName,
			Type:      field.Type,
		})
	}
	return out
}

func importsJSON(imports []gwdkir.Import) []importJSON {
	if len(imports) == 0 {
		return nil
	}
	out := make([]importJSON, 0, len(imports))
	for _, item := range imports {
		out = append(out, importJSON{Alias: item.Alias, Path: item.Path})
	}
	return out
}

func usesJSON(uses []gwdkir.Use) []useJSON {
	if len(uses) == 0 {
		return nil
	}
	out := make([]useJSON, 0, len(uses))
	for _, item := range uses {
		out = append(out, useJSON{Alias: item.Alias, Package: item.Package})
	}
	return out
}

func blocksJSONFor(page gwdkir.Page) blocksJSON {
	return blocksJSON{
		Paths:     page.Blocks.Paths,
		Build:     page.Blocks.Build,
		Load:      page.Blocks.Server,
		View:      page.Blocks.View,
		Actions:   actionNames(page.Blocks.Actions),
		APIs:      apiNames(page.Blocks.APIs),
		Fragments: fragmentEndpointNames(page.Blocks.Fragments),
	}
}

func actionsJSON(actions []gwdkir.Action) []actionJSON {
	if len(actions) == 0 {
		return nil
	}
	out := make([]actionJSON, 0, len(actions))
	for _, action := range actions {
		out = append(out, actionJSON{
			Name:           action.Name,
			Method:         action.Method,
			Route:          action.Route,
			InputName:      action.InputName,
			InputType:      action.InputType,
			ValidatesInput: action.ValidatesInput,
			Redirect:       action.Redirect,
			Fragments:      fragmentsJSON(action.Fragments),
		})
	}
	return out
}

func fragmentsJSON(fragments []gwdkir.Fragment) []fragmentJSON {
	if len(fragments) == 0 {
		return nil
	}
	out := make([]fragmentJSON, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, fragmentJSON{Target: fragment.Target})
	}
	return out
}

func apisJSON(apis []gwdkir.API) []apiJSON {
	if len(apis) == 0 {
		return nil
	}
	out := make([]apiJSON, 0, len(apis))
	for _, api := range apis {
		out = append(out, apiJSON{Name: api.Name, Method: api.Method, Route: api.Route})
	}
	return out
}

func fragmentEndpointNames(fragments []gwdkir.FragmentEndpoint) []string {
	if len(fragments) == 0 {
		return nil
	}
	names := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		names = append(names, fragment.Name)
	}
	return names
}

func fragmentEndpointsJSON(fragments []gwdkir.FragmentEndpoint) []fragmentEndpointJSON {
	if len(fragments) == 0 {
		return nil
	}
	out := make([]fragmentEndpointJSON, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, fragmentEndpointJSON{
			Name:   fragment.Name,
			Method: fragment.Method,
			Route:  fragment.Route,
			Target: fragment.Target,
		})
	}
	return out
}

func artifactsJSON(page gwdkir.Page) []artifactJSON {
	switch page.RenderMode(gowdk.SPA) {
	case gowdk.SPA, gowdk.Action:
	default:
		return nil
	}
	outputPath := htmlArtifactPath(page.Route)
	if outputPath == "" {
		return nil
	}
	return []artifactJSON{{Kind: "html", Path: outputPath}}
}

func htmlArtifactPath(route string) string {
	if !strings.HasPrefix(route, "/") || strings.ContainsAny(route, "?#") {
		return ""
	}
	trimmed := strings.Trim(route, "/")
	if trimmed == "" {
		return "index.html"
	}
	return path.Join(path.Clean("/" + trimmed)[1:], "index.html")
}

func pageComponents(page gwdkir.Page) []string {
	if !page.Blocks.View || page.Blocks.ViewBody == "" {
		return nil
	}
	if len(page.Blocks.ViewNodes) > 0 {
		return viewanalysis.ComponentReferencesFromNodes(page.Blocks.ViewNodes)
	}
	components, err := viewanalysis.ComponentReferences(page.Blocks.ViewBody)
	if err != nil {
		return nil
	}
	return components
}

func pageDependencies(page gwdkir.Page) viewanalysis.Dependencies {
	if !page.Blocks.View || page.Blocks.ViewBody == "" {
		return viewanalysis.Dependencies{}
	}
	if len(page.Blocks.ViewNodes) > 0 {
		return viewanalysis.ViewDependenciesFromNodes(page.Blocks.ViewNodes)
	}
	dependencies, err := viewanalysis.ViewDependencies(page.Blocks.ViewBody)
	if err != nil {
		return viewanalysis.Dependencies{}
	}
	return dependencies
}

func componentsJSON(components []gwdkir.Component) map[string]componentJSON {
	if len(components) == 0 {
		return nil
	}
	out := map[string]componentJSON{}
	for _, component := range components {
		out[component.Name] = componentJSON{
			Source:    component.Source,
			Kind:      "component",
			Package:   component.Package,
			Imports:   importsJSON(component.Imports),
			Uses:      usesJSON(component.Uses),
			CSS:       append([]string(nil), component.CSS...),
			JS:        append([]string(nil), component.JS...),
			InlineJS:  inlineScriptNames(component.InlineJS),
			Assets:    append([]string(nil), component.Assets...),
			Props:     propsJSON(component.Props),
			PropsType: goRefJSON(component.PropsType),
			State:     stateContractJSON(component.State),
			Exports:   exportsJSON(component.Exports),
			Emits:     emitsJSON(component.Emits),
		}
	}
	return out
}

func inlineScriptNames(scripts []source.InlineScript) []string {
	if len(scripts) == 0 {
		return nil
	}
	out := make([]string, 0, len(scripts))
	for index, script := range scripts {
		name := script.Name
		if name == "" {
			name = source.InlineScriptName(index)
		}
		out = append(out, name)
	}
	return out
}

func layoutsJSON(layouts []gwdkir.Layout) map[string]layoutJSON {
	if len(layouts) == 0 {
		return nil
	}
	out := map[string]layoutJSON{}
	for _, layout := range layouts {
		out[layout.ID] = layoutJSON{
			Source:  layout.Source,
			Kind:    "layout",
			Package: layout.Package,
			Uses:    usesJSON(layout.Uses),
		}
	}
	return out
}

func propsJSON(props []gwdkir.Prop) []propJSON {
	if len(props) == 0 {
		return nil
	}
	out := make([]propJSON, 0, len(props))
	for _, prop := range props {
		out = append(out, propJSON{Name: prop.Name, Type: prop.Type, Default: prop.Default, DefaultSet: prop.DefaultSet})
	}
	return out
}

func exportsJSON(exports []gwdkir.Export) []propJSON {
	if len(exports) == 0 {
		return nil
	}
	out := make([]propJSON, 0, len(exports))
	for _, export := range exports {
		out = append(out, propJSON{Name: export.Name, Type: export.Type})
	}
	return out
}

func emitsJSON(emits []gwdkir.Emit) []emitJSON {
	if len(emits) == 0 {
		return nil
	}
	out := make([]emitJSON, 0, len(emits))
	for _, emit := range emits {
		out = append(out, emitJSON{Name: emit.Name, Params: emitParamsJSON(emit.Params)})
	}
	return out
}

func emitParamsJSON(params []gwdkir.EmitParam) []emitParamJSON {
	if len(params) == 0 {
		return nil
	}
	out := make([]emitParamJSON, 0, len(params))
	for _, param := range params {
		out = append(out, emitParamJSON{Name: param.Name, Type: param.Type})
	}
	return out
}

func goRefJSON(ref gwdkir.GoRef) *goTypeJSON {
	if ref.Name == "" {
		return nil
	}
	return &goTypeJSON{Alias: ref.Alias, Name: ref.Name}
}

func stateContractJSON(state gwdkir.StateContract) *stateJSON {
	if state.Type.Name == "" {
		return nil
	}
	return &stateJSON{
		Type: goTypeJSON{Alias: state.Type.Alias, Name: state.Type.Name},
		Init: goFuncJSON{Alias: state.Init.Alias, Name: state.Init.Name},
	}
}
