package lang

// Completion describes one language completion shared by editor integrations.
type Completion struct {
	Label  string
	Detail string
}

// Completions returns the core .gwdk language keywords known to editor tools.
func Completions() []Completion {
	return []Completion{
		{Label: "@page", Detail: "Declare the page id."},
		{Label: "@route", Detail: "Declare the route path."},
		{Label: "@layout", Detail: "Declare one or more layout ids."},
		{Label: "@render", Detail: "Declare render mode: spa, action, hybrid, or ssr."},
		{Label: "@guard", Detail: "Declare route guards."},
		{Label: "@css", Detail: "Select page CSS inputs: default, page, none, or discovered CSS names."},
		{Label: "spa", Detail: "Non-SSR app render mode."},
		{Label: "action", Detail: "Non-SSR app page with backend actions."},
		{Label: "hybrid", Detail: "App output with selected request-time behavior."},
		{Label: "ssr", Detail: "Request-time full-page rendering through the SSR addon."},
		{Label: "paths", Detail: "Build-time dynamic route path block."},
		{Label: "build", Detail: "Build-time data block."},
		{Label: "load", Detail: "Request-time data block."},
		{Label: "act", Detail: "Action block for POST/form behavior."},
		{Label: "api", Detail: "API handler block."},
		{Label: "view", Detail: "Markup render block."},
		{Label: "g:post", Detail: "Bind a form to an action."},
		{Label: "g:target", Detail: "Select partial update target."},
		{Label: "g:swap", Detail: "Select partial update swap behavior."},
	}
}
