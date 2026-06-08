package diagnostics

import (
	"sort"
	"strings"
)

// Explanation is the user-facing diagnostic explanation payload.
type Explanation struct {
	Code      string    `json:"code"`
	Area      string    `json:"area"`
	Stability Stability `json:"stability"`
	Summary   string    `json:"summary"`
	Details   string    `json:"details,omitempty"`
	NextSteps []string  `json:"nextSteps,omitempty"`
	Invalid   string    `json:"invalid,omitempty"`
	Fixed     string    `json:"fixed,omitempty"`
}

type explanationDetail struct {
	Details   string
	NextSteps []string
	Invalid   string
	Fixed     string
}

var explanationDetails = map[string]explanationDetail{
	"guard_requires_request_render": {
		Details: "Protected page guards must gate the page GET route at request time. A build-time SPA page emits plain static HTML, so it cannot enforce frontend page access.",
		NextSteps: []string{
			"Add load {} or go ssr {} and enable the SSR addon when the page should be protected.",
			"Use @guard public when the page is intentionally public and keep backend authorization in Go handlers.",
		},
		Invalid: `@page dashboard
@route "/dashboard"
@guard auth.required
`,
		Fixed: `@page dashboard
@route "/dashboard"
@guard auth.required

load {
}
`,
	},
	"missing_ssr_addon": {
		Details: "The source selects request-time page rendering through load {}, go ssr {}, SSR render mode, or hybrid render mode, but the loaded config does not enable the SSR addon.",
		NextSteps: []string{
			"Enable ssr.Addon() in gowdk.config.go when request-time page rendering is intentional.",
			"Remove load {} or go ssr {} when the page should stay build-time SPA output.",
		},
		Invalid: `package pages

@page dashboard
@route "/dashboard"

load {
}

view {
  <main>Dashboard</main>
}
`,
		Fixed: `var Config = gowdk.Config{
  Addons: []gowdk.Addon{
    ssr.Addon(),
  },
}
`,
	},
	"spa_dynamic_route_missing_paths": {
		Details: "Build-time SPA pages with dynamic route params need concrete paths at build time. Request-time pages can skip paths because params are decoded per request.",
		NextSteps: []string{
			"Add paths { ... } with concrete param values for every static output path.",
			"Use load {} or go ssr {} with the SSR addon when the route should render per request.",
		},
		Invalid: `@page post
@route "/blog/{slug}"

view {
  <main>{param("slug")}</main>
}
`,
		Fixed: `paths {
  => { slug: "hello-gowdk" }
}
`,
	},
	"missing_view_block": {
		Details: "Current page files own a GET page route and must render HTML for that route. API-only source files are not a stable file kind yet.",
		NextSteps: []string{
			"Add view {} to page files.",
			"Move API-only behavior into supported endpoint declarations on a page or normal Go handlers.",
		},
		Invalid: `@page home
@route "/"
`,
		Fixed: `view {
  <main>Home</main>
}
`,
	},
	"old_action_block_syntax": {
		Details: "Action behavior is declared as endpoint metadata now. The old act name { ... } block form is rejected so generated adapters can bind exact Go handler symbols.",
		NextSteps: []string{
			"Replace act Name { ... } with act Name POST \"/path\".",
			"Move behavior into an exported Go handler named by the action declaration.",
		},
		Invalid: `act Submit {
}
`,
		Fixed: `act Submit POST "/signup"
`,
	},
	"old_api_block_syntax": {
		Details: "API behavior is declared as endpoint metadata now. The old api name { ... } block form is rejected so generated adapters can bind exact Go handler symbols.",
		NextSteps: []string{
			"Replace api Name { ... } with api Name METHOD \"/path\".",
			"Move behavior into an exported Go handler named by the API declaration.",
		},
		Invalid: `api Health {
}
`,
		Fixed: `api Health GET "/api/health"
`,
	},
	"parse_error": {
		Details: "The current parser is still line-oriented. It emits parse_error when source is outside the supported grammar and no more specific stable parser code exists.",
		NextSteps: []string{
			"Check the line reported by the diagnostic against docs/language/spec.md.",
			"Use current endpoint declarations and supported block forms instead of planned syntax.",
		},
	},
	"missing_page_guard": {
		Details: "The current guard hardening slice requires page authors to state whether a page is intentionally public or protected by application-owned guard IDs.",
		NextSteps: []string{
			"Add @guard public when the page is intentionally public.",
			"Add protected guard IDs such as @guard auth.required for private pages.",
		},
		Invalid: `@page home
@route "/"
`,
		Fixed: `@page home
@route "/"
@guard public
`,
	},
	"public_guard_exclusive": {
		Details: "The public guard means no protected guard should run for that page, so it cannot be mixed with other guard IDs.",
		NextSteps: []string{
			"Keep @guard public by itself for public pages.",
			"Remove public and keep protected guard IDs for guarded pages.",
		},
		Invalid: `@page dashboard
@route "/dashboard"
@guard public auth.required
`,
		Fixed: `@page dashboard
@route "/dashboard"
@guard auth.required
`,
	},
}

// Explain returns a user-facing explanation for code.
func Explain(code string) (Explanation, bool) {
	entry, ok := Lookup(code)
	if !ok {
		return Explanation{}, false
	}
	explanation := Explanation{
		Code:      entry.Code,
		Area:      entry.Area,
		Stability: entry.Stability,
		Summary:   entry.Summary,
		NextSteps: defaultNextSteps(entry),
	}
	if detail, ok := explanationDetails[entry.Code]; ok {
		explanation.Details = detail.Details
		explanation.Invalid = detail.Invalid
		explanation.Fixed = detail.Fixed
		if len(detail.NextSteps) > 0 {
			explanation.NextSteps = detail.NextSteps
		}
	}
	return explanation, true
}

func defaultNextSteps(entry Code) []string {
	switch entry.Stability {
	case StabilityStable:
		return []string{"Use the summary, source range, and docs/reference/diagnostics.md to fix the reported source."}
	case StabilityExperimental:
		return []string{"This code belongs to a partial feature slice; check docs for current limits before relying on the behavior."}
	case StabilityAddon:
		return []string{"Check the addon that emitted this diagnostic for addon-specific guidance."}
	default:
		return nil
	}
}

// Suggestions returns close diagnostic-code matches for an unknown code.
func Suggestions(code string, limit int) []string {
	code = strings.TrimSpace(code)
	if code == "" || limit <= 0 {
		return nil
	}
	type candidate struct {
		code     string
		distance int
	}
	var candidates []candidate
	for _, entry := range Registry {
		distance := levenshtein(code, entry.Code)
		if strings.Contains(entry.Code, code) || strings.Contains(code, entry.Code) {
			distance--
		}
		candidates = append(candidates, candidate{code: entry.Code, distance: distance})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].code < candidates[j].code
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	suggestions := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		suggestions = append(suggestions, candidate.code)
	}
	return suggestions
}

func levenshtein(left, right string) int {
	if left == right {
		return 0
	}
	if left == "" {
		return len(right)
	}
	if right == "" {
		return len(left)
	}
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for i := 1; i <= len(left); i++ {
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 0
			if left[i-1] != right[j-1] {
				cost = 1
			}
			current[j] = minInt(
				current[j-1]+1,
				previous[j]+1,
				previous[j-1]+cost,
			)
		}
		previous, current = current, previous
	}
	return previous[len(right)]
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}
