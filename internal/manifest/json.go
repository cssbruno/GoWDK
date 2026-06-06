package manifest

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/view"
)

// PublicSchemaVersion is the current gowdk manifest JSON schema version.
const PublicSchemaVersion = 1

type manifestJSON struct {
	Version         int                      `json:"version"`
	Pages           map[string]pageJSON      `json:"pages"`
	Components      map[string]componentJSON `json:"components,omitempty"`
	Layouts         map[string]layoutJSON    `json:"layouts,omitempty"`
	BackendBindings []backendBindingJSON     `json:"backendBindings,omitempty"`
}

type pageJSON struct {
	Source          string           `json:"source,omitempty"`
	Kind            string           `json:"kind"`
	Package         string           `json:"package,omitempty"`
	Route           string           `json:"route"`
	Render          gowdk.RenderMode `json:"render"`
	Cache           string           `json:"cache,omitempty"`
	Metadata        *metadataJSON    `json:"metadata,omitempty"`
	Imports         []importJSON     `json:"imports,omitempty"`
	Uses            []useJSON        `json:"uses,omitempty"`
	Layouts         []string         `json:"layouts,omitempty"`
	DynamicParams   []string         `json:"dynamicParams,omitempty"`
	Paths           bool             `json:"paths,omitempty"`
	Guard           []string         `json:"guard,omitempty"`
	CSS             []string         `json:"css,omitempty"`
	Blocks          blocksJSON       `json:"blocks"`
	Actions         []actionJSON     `json:"actions,omitempty"`
	APIs            []apiJSON        `json:"apis,omitempty"`
	Components      []string         `json:"components,omitempty"`
	Assets          []string         `json:"assets,omitempty"`
	CSSClasses      []string         `json:"cssClasses,omitempty"`
	StyleAttributes []string         `json:"styleAttributes,omitempty"`
	Artifacts       []artifactJSON   `json:"artifacts,omitempty"`
}

type componentJSON struct {
	Source    string       `json:"source,omitempty"`
	Kind      string       `json:"kind"`
	Package   string       `json:"package,omitempty"`
	Imports   []importJSON `json:"imports,omitempty"`
	Uses      []useJSON    `json:"uses,omitempty"`
	CSS       []string     `json:"css,omitempty"`
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
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Canonical   string `json:"canonical,omitempty"`
	Image       string `json:"image,omitempty"`
}

type propJSON struct {
	Name string `json:"name"`
	Type string `json:"type"`
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
	Paths   bool     `json:"paths"`
	Build   bool     `json:"build"`
	Load    bool     `json:"load"`
	View    bool     `json:"view"`
	Actions []string `json:"actions,omitempty"`
	APIs    []string `json:"apis,omitempty"`
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
	Kind         string               `json:"kind"`
	PageID       string               `json:"pageId"`
	Source       string               `json:"source,omitempty"`
	BlockName    string               `json:"blockName"`
	Method       string               `json:"method,omitempty"`
	Route        string               `json:"route"`
	ImportPath   string               `json:"importPath,omitempty"`
	PackageName  string               `json:"packageName,omitempty"`
	FunctionName string               `json:"functionName"`
	Signature    BackendSignatureKind `json:"signature,omitempty"`
	InputType    string               `json:"inputType,omitempty"`
	InputPointer bool                 `json:"inputPointer,omitempty"`
	InputFields  []backendInputJSON   `json:"inputFields,omitempty"`
	Status       BackendBindingStatus `json:"status"`
	Message      string               `json:"message,omitempty"`
}

type backendInputJSON struct {
	FieldName string `json:"fieldName"`
	FormName  string `json:"formName"`
	Type      string `json:"type"`
}

func metadataJSONFor(metadata PageMetadata) *metadataJSON {
	if metadata.Title == "" && metadata.Description == "" && metadata.Canonical == "" && metadata.Image == "" {
		return nil
	}
	return &metadataJSON{
		Title:       metadata.Title,
		Description: metadata.Description,
		Canonical:   metadata.Canonical,
		Image:       metadata.Image,
	}
}

// MarshalJSON emits the route manifest shape consumed by generated binaries.
func (app Manifest) MarshalJSON() ([]byte, error) {
	pages := map[string]pageJSON{}
	for _, page := range app.Pages {
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
			Paths:           page.Paths,
			Guard:           page.Guard,
			CSS:             page.CSS,
			Blocks:          blocksJSONFor(page),
			Actions:         actionsJSON(page.Blocks.Actions),
			APIs:            apisJSON(page.Blocks.APIs),
			Components:      pageComponents(page),
			Assets:          dependencies.Assets,
			CSSClasses:      dependencies.CSSClasses,
			StyleAttributes: dependencies.StyleAttributes,
			Artifacts:       artifactsJSON(page),
		}
	}
	return json.Marshal(manifestJSON{
		Version:         PublicSchemaVersion,
		Pages:           pages,
		Components:      componentsJSON(app.Components),
		Layouts:         layoutsJSON(app.Layouts),
		BackendBindings: backendBindingsJSON(app.BackendBindings),
	})
}

func backendBindingsJSON(bindings []BackendBinding) []backendBindingJSON {
	if len(bindings) == 0 {
		return nil
	}
	out := make([]backendBindingJSON, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, backendBindingJSON{
			Kind:         binding.Kind,
			PageID:       binding.PageID,
			Source:       binding.Source,
			BlockName:    binding.BlockName,
			Method:       binding.Method,
			Route:        binding.Route,
			ImportPath:   binding.ImportPath,
			PackageName:  binding.PackageName,
			FunctionName: binding.FunctionName,
			Signature:    binding.Signature,
			InputType:    binding.InputType,
			InputPointer: binding.InputPointer,
			InputFields:  backendInputFieldsJSON(binding.InputFields),
			Status:       binding.Status,
			Message:      binding.Message,
		})
	}
	return out
}

func backendInputFieldsJSON(fields []BackendInputField) []backendInputJSON {
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

func importsJSON(imports []Import) []importJSON {
	if len(imports) == 0 {
		return nil
	}
	out := make([]importJSON, 0, len(imports))
	for _, item := range imports {
		out = append(out, importJSON{Alias: item.Alias, Path: item.Path})
	}
	return out
}

func usesJSON(uses []Use) []useJSON {
	if len(uses) == 0 {
		return nil
	}
	out := make([]useJSON, 0, len(uses))
	for _, item := range uses {
		out = append(out, useJSON{Alias: item.Alias, Package: item.Package})
	}
	return out
}

func blocksJSONFor(page Page) blocksJSON {
	return blocksJSON{
		Paths:   page.Paths,
		Build:   page.Blocks.Build,
		Load:    page.Blocks.Load,
		View:    page.Blocks.View,
		Actions: actionNames(page.Blocks.Actions),
		APIs:    apiNames(page.Blocks.APIs),
	}
}

func actionNames(actions []Action) []string {
	if len(actions) == 0 {
		return nil
	}
	names := make([]string, 0, len(actions))
	for _, action := range actions {
		names = append(names, action.Name)
	}
	return names
}

func actionsJSON(actions []Action) []actionJSON {
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

func fragmentsJSON(fragments []Fragment) []fragmentJSON {
	if len(fragments) == 0 {
		return nil
	}
	out := make([]fragmentJSON, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, fragmentJSON{Target: fragment.Target})
	}
	return out
}

func apiNames(apis []API) []string {
	if len(apis) == 0 {
		return nil
	}
	names := make([]string, 0, len(apis))
	for _, api := range apis {
		names = append(names, api.Name)
	}
	return names
}

func apisJSON(apis []API) []apiJSON {
	if len(apis) == 0 {
		return nil
	}
	out := make([]apiJSON, 0, len(apis))
	for _, api := range apis {
		out = append(out, apiJSON{Name: api.Name, Method: api.Method, Route: api.Route})
	}
	return out
}

func artifactsJSON(page Page) []artifactJSON {
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

func pageComponents(page Page) []string {
	if !page.Blocks.View || page.Blocks.ViewBody == "" {
		return nil
	}
	components, err := view.ComponentReferences(page.Blocks.ViewBody)
	if err != nil {
		return nil
	}
	return components
}

func pageDependencies(page Page) view.Dependencies {
	if !page.Blocks.View || page.Blocks.ViewBody == "" {
		return view.Dependencies{}
	}
	dependencies, err := view.ViewDependencies(page.Blocks.ViewBody)
	if err != nil {
		return view.Dependencies{}
	}
	return dependencies
}

func componentsJSON(components []Component) map[string]componentJSON {
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
			Assets:    append([]string(nil), component.Assets...),
			Props:     propsJSON(component.Props),
			PropsType: goTypeRefJSON(component.PropsType),
			State:     stateContractJSON(component.State),
			Exports:   exportsJSON(component.Exports),
			Emits:     emitsJSON(component.Emits),
		}
	}
	return out
}

func layoutsJSON(layouts []Layout) map[string]layoutJSON {
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

func propsJSON(props []Prop) []propJSON {
	if len(props) == 0 {
		return nil
	}
	out := make([]propJSON, 0, len(props))
	for _, prop := range props {
		out = append(out, propJSON{Name: prop.Name, Type: prop.Type})
	}
	return out
}

func exportsJSON(exports []Export) []propJSON {
	if len(exports) == 0 {
		return nil
	}
	out := make([]propJSON, 0, len(exports))
	for _, export := range exports {
		out = append(out, propJSON{Name: export.Name, Type: export.Type})
	}
	return out
}

func emitsJSON(emits []Emit) []emitJSON {
	if len(emits) == 0 {
		return nil
	}
	out := make([]emitJSON, 0, len(emits))
	for _, emit := range emits {
		out = append(out, emitJSON{Name: emit.Name, Params: emitParamsJSON(emit.Params)})
	}
	return out
}

func emitParamsJSON(params []EmitParam) []emitParamJSON {
	if len(params) == 0 {
		return nil
	}
	out := make([]emitParamJSON, 0, len(params))
	for _, param := range params {
		out = append(out, emitParamJSON{Name: param.Name, Type: param.Type})
	}
	return out
}

func goTypeRefJSON(ref GoTypeRef) *goTypeJSON {
	if ref.Name == "" {
		return nil
	}
	return &goTypeJSON{Alias: ref.Alias, Name: ref.Name}
}

func stateContractJSON(state StateContract) *stateJSON {
	if state.Type.Name == "" {
		return nil
	}
	return &stateJSON{
		Type: goTypeJSON{Alias: state.Type.Alias, Name: state.Type.Name},
		Init: goFuncJSON{Alias: state.Init.Alias, Name: state.Init.Name},
	}
}
