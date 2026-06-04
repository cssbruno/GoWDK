package manifest

import (
	"regexp"
	"sort"

	"github.com/gowdk/gowdk"
)

var routeParamPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Manifest is the compiler's normalized view of discovered .gwdk files.
type Manifest struct {
	Pages      []Page
	Components []Component
}

// Page describes a .gwdk page after parsing and normalization.
type Page struct {
	Source  string
	ID      string
	Route   string
	Render  gowdk.RenderMode
	Layouts []string
	Guard   []string
	Paths   bool
	Blocks  Blocks
}

// Blocks records the source blocks declared by a page.
type Blocks struct {
	PathsBody string
	Build     bool
	BuildBody string
	Load      bool
	View      bool
	ViewBody  string
	Actions   []Action
	APIs      []API
}

// Component describes a .cmp.gwdk component after parsing and normalization.
type Component struct {
	Source string
	Name   string
	Props  []Prop
	Blocks Blocks
}

// Prop describes one component prop declaration.
type Prop struct {
	Name string
	Type string
}

// Action describes an act block.
type Action struct {
	Name           string
	Body           string
	InputName      string
	InputType      string
	ValidatesInput bool
	Redirect       string
}

// API describes an api block.
type API struct {
	Name   string
	Method string
	Route  string
}

// RenderMode returns the effective render mode for a page.
func (page Page) RenderMode(defaultMode gowdk.RenderMode) gowdk.RenderMode {
	if page.Render != "" {
		return page.Render
	}
	if defaultMode == "" {
		return gowdk.Static
	}
	return defaultMode
}

// DynamicParams returns route parameters declared with /path/{param} syntax.
func (page Page) DynamicParams() []string {
	matches := routeParamPattern.FindAllStringSubmatch(page.Route, -1)
	if len(matches) == 0 {
		return nil
	}

	params := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		param := match[1]
		if !seen[param] {
			seen[param] = true
			params = append(params, param)
		}
	}
	sort.Strings(params)
	return params
}
