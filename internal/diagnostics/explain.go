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
	Severity  Severity  `json:"severity"`
	Fix       *Fix      `json:"fix,omitempty"`
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
	"page_store_persist_key_conflict": {
		Details: "Two pages declare a persisted store with the same name but different struct shapes. Persistence is keyed by store name (gowdk:store:<name>), so both pages read and write the same browser storage slot. Because their embedded schema hashes differ, navigating from one page to the other discards the saved value every time. Either rename one store so each owns its own key, or give them the same shape so sharing is intentional.",
		NextSteps: []string{
			"Rename one of the stores so each persisted store has a unique name.",
			"Give both stores the same Go type when sharing one persisted value across pages is intended.",
		},
		Invalid: `// pages/shop.page.gwdk
store cart ui.CartState = ui.NewCartState() persist "local"
// pages/admin.page.gwdk  (different shape, same name)
store cart admin.AuditState = admin.NewAuditState() persist "local"`,
		Fixed: `// pages/shop.page.gwdk
store cart  ui.CartState     = ui.NewCartState()     persist "local"
// pages/admin.page.gwdk
store audit admin.AuditState = admin.NewAuditState() persist "local"`,
	},
	"page_store_persist_scope_conflict": {
		Details: "Two pages declare a persisted store with the same name and the same struct shape but different persist scopes (one local, one session). Persistence is keyed by store name (gowdk:store:<name>), so they share one storage slot, but local uses window.localStorage and session uses window.sessionStorage. The runtime keeps whichever scope initialized first, so the effective backend — and whether the value survives a browser restart — depends on which route the user visited first. Use the same scope on both pages, or rename one store.",
		NextSteps: []string{
			"Declare the same persist scope (both \"local\" or both \"session\") wherever the store is persisted.",
			"Rename one of the stores so each persisted store owns its own scope and storage key.",
		},
		Invalid: `// pages/shop.page.gwdk
store cart ui.CartState = ui.NewCartState() persist "local"
// pages/checkout.page.gwdk  (same shape, different scope)
store cart ui.CartState = ui.NewCartState() persist "session"`,
		Fixed: `// pages/shop.page.gwdk
store cart ui.CartState = ui.NewCartState() persist "local"
// pages/checkout.page.gwdk
store cart ui.CartState = ui.NewCartState() persist "local"`,
	},
	"page_store_persist_scope_invalid": {
		Details: "A page store may opt into browser persistence with persist \"local\" or persist \"session\". local uses window.localStorage (survives a browser restart); session uses window.sessionStorage (survives reload and SPA navigation, cleared when the tab closes). No other scope is supported.",
		NextSteps: []string{
			"Use persist \"local\" to keep the store across browser restarts.",
			"Use persist \"session\" to keep the store for the life of the tab.",
			"Remove the persist modifier when the store should reset on reload.",
		},
		Invalid: `store cart ui.CartState = ui.NewCartState() persist "disk"`,
		Fixed:   `store cart ui.CartState = ui.NewCartState() persist "local"`,
	},
	"page_store_persist_secret_field": {
		Details: "A persisted store field name resembles a secret (for example token, password, secret, or auth). Persisted store state is written to browser storage, which is readable by any script on the same origin, so it must never hold credentials, session tokens, or trusted authorization state. This is a warning, not an error: rename the field if it is not actually a secret.",
		NextSteps: []string{
			"Keep secrets, session tokens, and authorization state in server-owned Go and never in a persisted store.",
			"Rename the field if its name only resembles a secret but holds plain UI state.",
			"Drop the persist modifier if the store legitimately needs sensitive values in memory but not on disk.",
		},
		Invalid: `store session ui.SessionState = ui.NewSession() persist "local"
// SessionState has a Token field`,
		Fixed: `store prefs ui.UIPrefs = ui.DefaultPrefs() persist "local"
// UIPrefs holds only non-sensitive UI state`,
	},
	"guard_requires_request_render": {
		Details: "Protected page guards must gate the page GET route at request time. A build-time SPA page emits plain static HTML, so it cannot enforce frontend page access.",
		NextSteps: []string{
			"Add server {} or go server {} and enable the SSR addon when the page should be protected.",
			"Use guard public when the page is intentionally public and keep backend authorization in Go handlers.",
		},
		Invalid: `page dashboard
route "/dashboard"
guard auth.required
`,
		Fixed: `page dashboard
route "/dashboard"
guard auth.required

load {
}
`,
	},
	"missing_ssr_addon": {
		Details: "The source selects request-time page rendering through server {}, go server {}, SSR render mode, or hybrid render mode, but the loaded config does not enable the SSR addon.",
		NextSteps: []string{
			"Enable ssr.Addon() in gowdk.config.go when request-time page rendering is intentional.",
			"Remove server {} or go server {} when the page should stay build-time SPA output.",
		},
		Invalid: `package pages

page dashboard
route "/dashboard"

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
	"missing_realtime_addon": {
		Details: "The source declares g:subscribe for a browser-facing presentation event, but the loaded config does not enable the realtime addon.",
		NextSteps: []string{
			"Enable realtime.Addon() in gowdk.config.go when browser presentation-event fanout is intentional.",
			"Remove g:subscribe when the query region should stay static or update through normal fragments/actions.",
		},
		Invalid: `view {
  <section g:query="patients.GetPatientPage" g:subscribe="patients.PatientNotice"></section>
}
`,
		Fixed: `var Config = gowdk.Config{
  Addons: []gowdk.Addon{
    realtime.Addon(),
  },
}
`,
	},
	"ssr_command_no_client": {
		Details: "A request-time page (server {}, SSR, or hybrid) declares a g:command write form but renders no g:query region the command can refresh. Request-time pages now ship the small client runtime, so a g:command submit posts in the background and applies the single-flight region refresh the adapter names in the X-GOWDK-Queries response header. With no reactive read region there is nothing to refresh: the submit only fires a gowdk:command-success event, and with client JavaScript disabled the bare POST navigates to the adapter's raw JSON. This is a warning, not a build failure: the write still compiles, it just has no visible reactive effect and no no-JS fallback.",
		NextSteps: []string{
			"Add a g:query region the command's domain events invalidate, so the single-flight refresh has a region to apply.",
			"Or replace g:command with a g:post action handler that calls the same contract and returns a response.Response such as response.RedirectTo, so the write path works without client JavaScript.",
		},
		Invalid: `page board
route "/board"
server { => { columns } }

view {
  <form g:command="issues.CreateIssue"><input name="title" /></form>
}
`,
		Fixed: `page board
route "/board"
server { => { columns } }

view {
  <section g:query="issues.GetBoard">{columns}</section>
  <form g:command="issues.CreateIssue"><input name="title" /></form>
}
`,
	},
	"spa_dynamic_route_missing_paths": {
		Details: "Build-time SPA pages with dynamic route params need concrete paths at build time. Request-time pages can skip paths because params are decoded per request.",
		NextSteps: []string{
			"Add paths { ... } with concrete param values for every static output path.",
			"Use server {} or go server {} with the SSR addon when the route should render per request.",
		},
		Invalid: `page post
route "/blog/{slug}"

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
		Invalid: `page home
route "/"
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
		Details: "The parser emits parse_error when source is outside the supported grammar and no more specific stable parser code exists.",
		NextSteps: []string{
			"Check the line reported by the diagnostic against docs/language/spec.md.",
			"Use current endpoint declarations and supported block forms instead of planned syntax.",
		},
	},
	"missing_page_guard": {
		Details: "A page that declares no guard is not public by default. This is a warning, not a build failure: the page's route is denied (403) at request time until the author states intent. A static page is denied through the generated app's deny registry; a request-time page is denied in its own handler. Add guard public to expose the page on purpose so access is never granted by omission.",
		NextSteps: []string{
			"Add guard public when the page is intentionally public.",
			"Add protected guard IDs such as guard auth.required for private pages.",
		},
		Invalid: `page home
route "/"
`,
		Fixed: `page home
route "/"
guard public
`,
	},
	"unsupported_markup_syntax": {
		Details: "view {} markup expands only through GOWDK-owned AST nodes and g: directives. Foreign template blocks such as {#if}, {#each}, {#snippet}, {@html}, {@const}, and {@debug} are rejected with guidance instead of being translated implicitly. These rejections currently surface through the view_parse_error carrier code with this canonical message text.",
		NextSteps: []string{
			"Use g:if/g:else-if/g:else, g:for with g:key, component slots, or build/load data instead of foreign template blocks.",
			"Use the explicit g:unsafe-html={Expr} directive when trusted raw HTML output is intentional.",
		},
	},
	"unsupported_markup_directive": {
		Details: "view {} markup accepts only the documented g: directive set. Unknown g: attributes, and deferred families such as DOM/document/window/body targets, g:await/g:async placeholder directives, and DOM actions, are rejected at parse time. These rejections currently surface through the view_parse_error carrier code with this canonical message text.",
		NextSteps: []string{
			"Use a supported directive from docs/language/markup.md.",
			"Deferred behavior (document targets, DOM actions, and directive-form async placeholders) belongs to page metadata, component client blocks, or future addon contracts.",
		},
	},
	"audit_action_missing_csrf": {
		Details: "gowdk audit derives an action endpoint that decodes a request body without CSRF enforcement. The built-in security baseline (and any require csrf policy) treats this as an error because action POSTs are cross-site-forgeable.",
		NextSteps: []string{
			"Remove Build.CSRF.Disabled and provide a runtime CSRF secret so generated actions validate tokens before decoding.",
			"Override the built-in baseline.actions policy in a *.audit.gwdk file if the endpoint is intentionally exempt.",
		},
	},
	"audit_api_missing_csrf": {
		Details: "gowdk audit derives a state-changing API endpoint without CSRF enforcement. The built-in security baseline treats unsafe browser-reachable API methods as cross-site-forgeable unless another policy explicitly overrides that posture.",
		NextSteps: []string{
			"Remove Build.CSRF.Disabled and provide a runtime CSRF secret so generated state-changing APIs validate tokens before user handlers run.",
			"Override the built-in baseline.api policy in a *.audit.gwdk file only when the API uses another cross-site request strategy.",
		},
	},
	"audit_api_public_by_omission": {
		Details: "An API endpoint inherits no protective guard, so it would be callable without authorization. The baseline forbids public-by-omission APIs; access must be stated, not granted by omission.",
		NextSteps: []string{
			"Add a guard such as guard permission:resource.read to the page that declares the API endpoint.",
			"Add guard public only when the API is intentionally unauthenticated, and confirm the policy allows it.",
		},
	},
	"audit_bundle_secret": {
		Details: "The embedded build output or literal build-time data contains a value that matches a secret-shaped pattern (for example an env file, a private key, or a token). Secrets must not ship inside generated artifacts.",
		NextSteps: []string{
			"Move the secret to a runtime environment variable and read it in Go, not at build time.",
			"Exclude the offending file from the embedded asset set.",
		},
	},
	"audit_client_route_unguarded": {
		Details: "A client or SPA route declares no guard, so it is protected only by the generated runtime default-deny gate (HTTP 403). Under pure static hosting that gate is absent and the route's HTML could be served. The static-export caveat in docs/language/guards.md applies.",
		NextSteps: []string{
			"State the route's access with guard public or a protective guard so it joins the deny registry.",
			"Serve the route through the generated Go server, which enforces the deny registry.",
		},
	},
	"audit_command_missing_csrf": {
		Details: "gowdk audit derives a generated command endpoint that accepts a state-changing web request without CSRF enforcement. The built-in security baseline treats this as an error because command POSTs are cross-site-forgeable in the same way as action POSTs.",
		NextSteps: []string{
			"Remove Build.CSRF.Disabled and provide a runtime CSRF secret so generated command endpoints validate tokens before decoding.",
			"Override the built-in baseline.contract_commands policy in a *.audit.gwdk file if the endpoint is intentionally exempt.",
		},
	},
	"audit_contract_roleless": {
		Details: "A command or query contract is exposed to the web surface but declares no roles. The contract layer is the source of truth for authorization, so a roleless contract has no role to admit: the runtime data-layer gate fails closed and denies every web caller, which makes the endpoint unreachable, while a developer following the \"authz in contracts, guards as redundancy\" model may believe it is protected by the page guard alone.",
		NextSteps: []string{
			"Declare the roles permitted to execute the contract at registration (for example RoleWeb or RoleAdmin).",
			"Declare RoleAny only when the contract is intentionally callable by every role, including unauthenticated web callers.",
		},
	},
	"audit_guard_unverified": {
		Details: "A route or endpoint declares an app-owned guard whose implementation and failure behavior are not verified by audit fixture evidence. The static posture records the guard ID, but it cannot prove the app-owned authorization logic.",
		NextSteps: []string{
			"Add generated-app audit fixtures that exercise the real guard behavior.",
			"Use auth.Addon, role:, or permission: guards when the intended check is covered by GOWDK-native guard behavior.",
		},
	},
	"audit_guardless_endpoint_page": {
		Details: "A page that declares backend endpoints has no guard. Actions, fragments, commands, queries, and APIs would be publicly callable even when the page GET route is denied, which contradicts default-deny.",
		NextSteps: []string{
			"Add a guard to the page so its derived endpoints inherit it.",
			"Use guard public only when every derived endpoint is intentionally unauthenticated.",
		},
	},
	"audit_headers_missing": {
		Details: "An audit policy requires a security response header (for example Content-Security-Policy) but the generated app is not configured to emit it. This is a static-posture warning; runtime verification is a separate error.",
		NextSteps: []string{
			"Enable Build.SecurityHeaders and configure the required header in gowdk.config.go.",
			"Remove the require header rule if the header is owned by an upstream proxy.",
		},
	},
	"audit_headers_runtime_missing": {
		Details: "gowdk audit --run started the generated app and a required security response header was absent from a served response. The runtime contradicts the declared posture.",
		NextSteps: []string{
			"Confirm Build.SecurityHeaders emits the header on the served route and fragment responses.",
			"Check for middleware or a reverse proxy stripping the header in the run environment.",
		},
	},
	"audit_max_body_exceeds_policy": {
		Details: "An endpoint's configured request body limit is larger than the maximum a policy allows. Oversized limits widen the denial-of-service surface.",
		NextSteps: []string{
			"Lower Build.BodyLimits (or the per-target limit) to the policy maximum.",
			"Raise the policy max_body rule if the larger limit is intentional and justified.",
		},
	},
	"audit_observability_absolute_source": {
		Details: "Generated observability data can expose absolute source paths through span source metadata. Absolute paths can leak local user, checkout, or build-agent details.",
		NextSteps: []string{
			"Normalize source references before exporting span data.",
			"Keep generated observability endpoints debug-only and loopback-only.",
		},
	},
	"audit_observability_batch_limit_missing": {
		Details: "The browser trace ingestion surface does not declare a span batch limit. Without a batch cap, one request can consume disproportionate memory or processing time.",
		NextSteps: []string{
			"Declare and enforce a maximum number of spans per ingestion request.",
			"Disable browser trace ingestion where it is not needed.",
		},
	},
	"audit_observability_body_limit_missing": {
		Details: "The browser trace ingestion surface does not declare a request body limit. Trace collection endpoints should fail closed on oversized payloads.",
		NextSteps: []string{
			"Declare and enforce a bounded request body limit for trace ingestion.",
			"Keep generated trace endpoints behind the debug-only loopback gate.",
		},
	},
	"audit_observability_content_type_missing": {
		Details: "The browser trace ingestion surface does not require a JSON content type. Content-type checks reduce accidental cross-surface ingestion and make request handling easier to audit.",
		NextSteps: []string{
			"Require application/json for browser trace ingestion.",
			"Reject unsupported content types before reading the request body.",
		},
	},
	"audit_observability_origin_unchecked": {
		Details: "An observability endpoint lacks a loopback or origin access policy. Trace data can include route, source, and application timing metadata, so generated trace surfaces must not be broadly exposed.",
		NextSteps: []string{
			"Keep generated trace endpoints loopback-only.",
			"Add an explicit app-owned access gate before exposing trace data beyond local development.",
		},
	},
	"audit_observability_production_exposed": {
		Details: "An observability endpoint is mounted outside the debug-only lane. Trace viewer, JSON, SSE, or browser-ingestion surfaces can expose internal metadata and should not be production-public by default.",
		NextSteps: []string{
			"Disable generated observability for production output.",
			"Mount trace surfaces behind an app-owned authenticated access gate if production access is intentional.",
		},
	},
	"audit_public_not_allowed": {
		Details: "A target route or endpoint is public (guard public or no protective guard) but a policy deny public rule forbids public access for its selector.",
		NextSteps: []string{
			"Add a protective guard to the target so it is no longer public.",
			"Narrow the policy selector if the target is intentionally public.",
		},
	},
	"audit_raw_html_sink": {
		Details: "A view renders raw, unescaped HTML through g:unsafe-html (or an equivalent raw sink). Raw sinks are an XSS surface and must be explicitly allowlisted so each one is a reviewed decision.",
		NextSteps: []string{
			"Render escaped interpolation instead of g:unsafe-html when raw HTML is not required.",
			"Add the sink (source:field) to the policy raw-HTML allowlist when raw output is intentional and the input is trusted.",
		},
	},
	"audit_required_guard_missing": {
		Details: "A policy requires a role or permission guard (for example require role:admin) on the selected target, but the target does not declare it. The static posture does not satisfy the declared permission.",
		NextSteps: []string{
			"Add the required guard ID to the matched page so its routes and endpoints inherit it.",
			"Adjust the policy selector or required guard if the rule is too broad.",
		},
	},
	"audit_runtime_mismatch": {
		Details: "gowdk audit --run observed runtime behavior that contradicts the declared static posture (for example a route that should be denied returned a success status).",
		NextSteps: []string{
			"Reconcile the generated handler behavior with the declared guard, CSRF, or limit metadata.",
			"File the mismatch with the route or endpoint ID reported by the finding.",
		},
	},
	"audit_test_failed": {
		Details: "An audit integration test expectation declared in a *.audit.gwdk file did not hold when run against the generated app.",
		NextSteps: []string{
			"Inspect the reported request, expected value, and actual value to locate the divergence.",
			"Fix the handler, the guard configuration, or the test expectation as appropriate.",
		},
	},
	"policy_duplicate_name": {
		Details: "Two audit policies declare the same name. Policy names must be unique so extends and override resolution is unambiguous.",
		NextSteps: []string{
			"Rename one of the conflicting policies.",
		},
	},
	"policy_extends_cycle": {
		Details: "An audit policy extends chain forms a cycle (for example A extends B and B extends A), so composition cannot be resolved.",
		NextSteps: []string{
			"Break the cycle so the extends graph is acyclic.",
		},
	},
	"policy_selector_matched_nothing": {
		Details: "An audit policy selector matched no routes or endpoints. The rule has no effect, which often signals a typo or a stale selector.",
		NextSteps: []string{
			"Check the selector glob or kind against gowdk routes and gowdk endpoints output.",
			"Remove the policy or rule if the target no longer exists.",
		},
	},
	"policy_unknown_extends": {
		Details: "An audit policy extends a policy name that is not defined in any loaded *.audit.gwdk file or the built-in baseline.",
		NextSteps: []string{
			"Define the referenced policy, or correct the extends name.",
		},
	},
	"policy_unknown_selector": {
		Details: "An audit policy uses a selector form gowdk audit does not recognize. Supported forms are route globs (for example /admin/**) and kind selectors (act:*, api:*, fragment:*, and contract kinds).",
		NextSteps: []string{
			"Use a supported selector form from docs/language/audit.md.",
		},
	},
	"public_guard_exclusive": {
		Details: "The public guard means no protected guard should run for that page, so it cannot be mixed with other guard IDs.",
		NextSteps: []string{
			"Keep guard public by itself for public pages.",
			"Remove public and keep protected guard IDs for guarded pages.",
		},
		Invalid: `page dashboard
route "/dashboard"
guard public auth.required
`,
		Fixed: `page dashboard
route "/dashboard"
guard auth.required
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
		Severity:  entry.Severity,
		Fix:       entry.Fix,
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
