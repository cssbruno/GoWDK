package manifest

import (
	"encoding/json"

	"github.com/gowdk/gowdk"
)

type manifestJSON struct {
	Pages map[string]pageJSON `json:"pages"`
}

type pageJSON struct {
	Route   string           `json:"route"`
	Render  gowdk.RenderMode `json:"render"`
	Layouts []string         `json:"layouts,omitempty"`
	Paths   bool             `json:"paths,omitempty"`
	Guard   []string         `json:"guard,omitempty"`
	Actions []actionJSON     `json:"actions,omitempty"`
}

type actionJSON struct {
	Name           string `json:"name"`
	InputName      string `json:"inputName,omitempty"`
	InputType      string `json:"inputType,omitempty"`
	ValidatesInput bool   `json:"validatesInput,omitempty"`
	Redirect       string `json:"redirect,omitempty"`
}

// MarshalJSON emits the route manifest shape consumed by generated binaries.
func (app Manifest) MarshalJSON() ([]byte, error) {
	pages := map[string]pageJSON{}
	for _, page := range app.Pages {
		pages[page.ID] = pageJSON{
			Route:   page.Route,
			Render:  page.RenderMode(gowdk.Static),
			Layouts: page.Layouts,
			Paths:   page.Paths,
			Guard:   page.Guard,
			Actions: actionsJSON(page.Blocks.Actions),
		}
	}
	return json.Marshal(manifestJSON{Pages: pages})
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
		})
	}
	return out
}
