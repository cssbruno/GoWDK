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
	Version    int                      `json:"version"`
	Pages      map[string]pageJSON      `json:"pages"`
	Components map[string]componentJSON `json:"components,omitempty"`
	Layouts    map[string]layoutJSON    `json:"layouts,omitempty"`
}

type pageJSON struct {
	Source          string           `json:"source,omitempty"`
	Kind            string           `json:"kind"`
	Route           string           `json:"route"`
	Render          gowdk.RenderMode `json:"render"`
	Imports         []importJSON     `json:"imports,omitempty"`
	Layouts         []string         `json:"layouts,omitempty"`
	DynamicParams   []string         `json:"dynamicParams,omitempty"`
	Paths           bool             `json:"paths,omitempty"`
	Guard           []string         `json:"guard,omitempty"`
	CSS             []string         `json:"css,omitempty"`
	Blocks          blocksJSON       `json:"blocks"`
	Actions         []actionJSON     `json:"actions,omitempty"`
	APIs            []apiJSON        `json:"apis,omitempty"`
	Components      []string         `json:"components,omitempty"`
	StaticAssets    []string         `json:"staticAssets,omitempty"`
	CSSClasses      []string         `json:"cssClasses,omitempty"`
	StyleAttributes []string         `json:"styleAttributes,omitempty"`
	Artifacts       []artifactJSON   `json:"artifacts,omitempty"`
}

type componentJSON struct {
	Source    string       `json:"source,omitempty"`
	Kind      string       `json:"kind"`
	Imports   []importJSON `json:"imports,omitempty"`
	Props     []propJSON   `json:"props,omitempty"`
	PropsType *goTypeJSON  `json:"propsType,omitempty"`
	State     *stateJSON   `json:"state,omitempty"`
}

type layoutJSON struct {
	Source string `json:"source,omitempty"`
	Kind   string `json:"kind"`
}

type propJSON struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type importJSON struct {
	Alias string `json:"alias,omitempty"`
	Path  string `json:"path"`
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

// MarshalJSON emits the route manifest shape consumed by generated binaries.
func (app Manifest) MarshalJSON() ([]byte, error) {
	pages := map[string]pageJSON{}
	for _, page := range app.Pages {
		dependencies := pageDependencies(page)
		pages[page.ID] = pageJSON{
			Source:          page.Source,
			Kind:            "page",
			Route:           page.Route,
			Render:          page.RenderMode(gowdk.Static),
			Imports:         importsJSON(page.Imports),
			Layouts:         page.Layouts,
			DynamicParams:   page.DynamicParams(),
			Paths:           page.Paths,
			Guard:           page.Guard,
			CSS:             page.CSS,
			Blocks:          blocksJSONFor(page),
			Actions:         actionsJSON(page.Blocks.Actions),
			APIs:            apisJSON(page.Blocks.APIs),
			Components:      pageComponents(page),
			StaticAssets:    dependencies.StaticAssets,
			CSSClasses:      dependencies.CSSClasses,
			StyleAttributes: dependencies.StyleAttributes,
			Artifacts:       artifactsJSON(page),
		}
	}
	return json.Marshal(manifestJSON{
		Version:    PublicSchemaVersion,
		Pages:      pages,
		Components: componentsJSON(app.Components),
		Layouts:    layoutsJSON(app.Layouts),
	})
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
	switch page.RenderMode(gowdk.Static) {
	case gowdk.Static, gowdk.Action:
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
			Imports:   importsJSON(component.Imports),
			Props:     propsJSON(component.Props),
			PropsType: goTypeRefJSON(component.PropsType),
			State:     stateContractJSON(component.State),
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
			Source: layout.Source,
			Kind:   "layout",
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
