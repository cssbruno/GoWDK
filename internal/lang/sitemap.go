package lang

import (
	"encoding/json"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// SiteMap is an editor-facing route and file map.
type SiteMap struct {
	Pages []SiteMapPage `json:"pages"`
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

// BuildSiteMap converts a manifest into the editor-facing site map.
func BuildSiteMap(config gowdk.Config, app manifest.Manifest) SiteMap {
	pages := make([]SiteMapPage, 0, len(app.Pages))
	for _, page := range app.Pages {
		pages = append(pages, SiteMapPage{
			ID:            page.ID,
			Route:         page.Route,
			Source:        page.Source,
			Render:        page.RenderMode(config.Render.DefaultMode()),
			Layouts:       page.Layouts,
			Guard:         page.Guard,
			DynamicParams: page.DynamicParams(),
			Blocks: SiteMapBlocks{
				Paths:   page.Paths,
				Build:   page.Blocks.Build,
				Load:    page.Blocks.Load,
				View:    page.Blocks.View,
				Actions: actionNames(page.Blocks.Actions),
				APIs:    apiNames(page.Blocks.APIs),
			},
		})
	}
	return SiteMap{Pages: pages}
}

// SiteMapJSON returns the JSON site map for parsed and validated files.
func SiteMapJSON(config gowdk.Config, paths []string) ([]byte, Diagnostics) {
	app, diagnostics := CheckFiles(config, paths)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	payload, err := json.MarshalIndent(BuildSiteMap(config, app), "", "  ")
	if err != nil {
		return nil, Diagnostics{{Severity: "error", Message: err.Error()}}
	}
	return append(payload, '\n'), diagnostics
}

func actionNames(actions []manifest.Action) []string {
	if len(actions) == 0 {
		return nil
	}
	names := make([]string, 0, len(actions))
	for _, action := range actions {
		names = append(names, action.Name)
	}
	return names
}

func apiNames(apis []manifest.API) []string {
	if len(apis) == 0 {
		return nil
	}
	names := make([]string, 0, len(apis))
	for _, api := range apis {
		names = append(names, api.Name)
	}
	return names
}
