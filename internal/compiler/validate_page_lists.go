package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// validatePageServerLists mirrors, at check time, the server-lane rules the
// renderer enforces at build time so the diagnostics match. In the lane model
// g:for/g:if render server-side when their operand is a server {} request-time
// field (or, when nested, the enclosing row item); this validates those
// server-lane uses. A g:for/g:if over client state/store is the client lane and
// is validated by the island validator, not here.
func validatePageServerLists(page gwdkir.Page) []ValidationError {
	nodes := pageViewNodes(page)
	if len(nodes) == 0 {
		return nil
	}
	loads := collectPageLoads(page)
	var diagnostics []ValidationError
	validateServerLoadFieldConflicts(page, loads, &diagnostics)
	validateServerLoadResultFields(page, loads, &diagnostics)
	walkPageListNodes(page, nodes, loads, nil, false, &diagnostics)
	return diagnostics
}

func pageViewNodes(page gwdkir.Page) []viewmodel.Node {
	if len(page.Blocks.ViewNodes) > 0 {
		return page.Blocks.ViewNodes
	}
	if strings.TrimSpace(page.Blocks.ViewBody) == "" {
		return nil
	}
	nodes, err := viewparse.Parse(page.Blocks.ViewBody)
	if err != nil {
		return nil
	}
	return nodes
}

// pageLoads describes a page's declared server {} fields. fields holds the exact
// declared paths (e.g. "columns", "user.name"); roots holds the leading
// identifiers (e.g. "user"). The renderer resolves a top-level region's lane by
// the operand's root, so root membership decides whether a g:for/g:if is the
// server lane.
type pageLoads struct {
	fields map[string]bool
	roots  map[string]bool
}

func collectPageLoads(page gwdkir.Page) pageLoads {
	loads := pageLoads{fields: map[string]bool{}, roots: map[string]bool{}}
	if !page.Blocks.Server {
		return loads
	}
	for _, field := range page.Blocks.ServerFields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		loads.fields[field] = true
		loads.roots[exprRoot(field)] = true
	}
	return loads
}

func walkPageListNodes(page gwdkir.Page, nodes []viewmodel.Node, loads pageLoads, eachVars []string, inServerRegion bool, diagnostics *[]ValidationError) {
	for _, node := range nodes {
		element, ok := node.(viewmodel.Element)
		if !ok {
			if call, isCall := node.(viewmodel.ComponentCall); isCall {
				if inServerRegion || len(eachVars) > 0 {
					*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_region_directive",
						fmt.Sprintf("%s: server-lane g:for rows and g:if branches cannot contain component calls; render request-time markup with static elements, g:for, and g:if", page.ID)))
					continue
				}
				walkPageListNodes(page, call.Children, loads, eachVars, inServerRegion, diagnostics)
			}
			continue
		}
		inRow := len(eachVars) > 0
		childVars := eachVars
		if attr, has := elementAttr(element, "g:for"); has {
			pushed, ok := validatePageServerForDirective(page, attr, loads, eachVars, diagnostics)
			if ok {
				childVars = append(append([]string(nil), eachVars...), pushed)
			}
		}
		serverBranch := inRow
		if attr, has := elementAttr(element, "g:if"); has {
			if validatePageServerIfDirective(page, attr, loads, eachVars, diagnostics) {
				serverBranch = true
			}
		}
		serverRegionVars := childVars
		if !serverBranch && len(childVars) == len(eachVars) {
			serverRegionVars = eachVars
		}
		currentServerRegion := inServerRegion || serverBranch || len(childVars) > len(eachVars)
		validatePageServerRegionElement(page, element, loads, currentServerRegion, serverRegionVars, diagnostics)
		walkPageListNodes(page, element.Children, loads, childVars, currentServerRegion, diagnostics)
	}
}

// serverLaneForCollection reports whether a g:for collection is the server lane:
// nested inside a server row (inherits), or a top-level collection whose root is
// a declared server {} field.
func serverLaneForCollection(collection string, loads pageLoads, eachVars []string) bool {
	if len(eachVars) > 0 {
		return true
	}
	return loads.roots[exprRoot(collection)]
}

func serverLaneForCondition(expr string, loads pageLoads, eachVars []string) bool {
	if len(eachVars) > 0 {
		return true
	}
	return loads.roots[exprRoot(expr)]
}

func validatePageServerForDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, eachVars []string, diagnostics *[]ValidationError) (string, bool) {
	loop, err := viewparse.ParseForDirective(attr.Value)
	if err != nil {
		if serverLaneForCollection(strings.TrimSpace(attr.Value), loads, eachVars) {
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_for_invalid", fmt.Sprintf("%s: %v", page.ID, err)))
		}
		return "", false
	}
	if !serverLaneForCollection(loop.Collection, loads, eachVars) {
		// Client lane: a g:for over client state/store is validated by the island
		// validator. Do not push a client loop variable into the server row scope.
		return "", false
	}
	if len(eachVars) == 0 {
		return loop.Var, true
	}
	parent := eachVars[len(eachVars)-1]
	if loop.Collection == parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_for_nested_scope",
			fmt.Sprintf("%s: nested g:for collection cannot be the parent row item %q itself; reference a slice field such as %s.items", page.ID, parent, parent)))
		return loop.Var, true
	}
	if exprRoot(loop.Collection) != parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_for_nested_scope",
			fmt.Sprintf("%s: nested g:for collection %q must reference the parent row item %q (for example %s.field)", page.ID, loop.Collection, parent, parent)))
	}
	return loop.Var, true
}

func validatePageServerIfDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, eachVars []string, diagnostics *[]ValidationError) bool {
	raw := strings.TrimSpace(attr.Value)
	if raw == "" {
		return false
	}
	if !simpleConditionField(raw) {
		// Compound expression: server-lane only at the top level, and every
		// referenced root must be a declared server {} field.
		return validatePageServerCondExpr(page, raw, loads, eachVars, diagnostics)
	}
	condition := strings.TrimSpace(strings.TrimPrefix(raw, "!"))
	if !serverLaneForCondition(condition, loads, eachVars) {
		// Client lane: validated by the island validator.
		return false
	}
	if len(eachVars) == 0 {
		return true
	}
	parent := eachVars[len(eachVars)-1]
	if condition == parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_if_nested_scope",
			fmt.Sprintf("%s: nested g:if condition cannot be the row item %q itself; reference a field such as %s.field", page.ID, parent, parent)))
		return true
	}
	if exprRoot(condition) != parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_if_nested_scope",
			fmt.Sprintf("%s: nested g:if condition %q must reference the enclosing row item %q (for example %s.field)", page.ID, condition, parent, parent)))
	}
	return true
}

// validatePageServerCondExpr mirrors the renderer's compound server g:if rule: a
// compound expression is the server lane only when it references a server {}
// field, is rejected inside a row, and must parse with all roots declared.
func validatePageServerCondExpr(page gwdkir.Page, raw string, loads pageLoads, eachVars []string, diagnostics *[]ValidationError) bool {
	fields, err := clientlang.ExprFields(raw)
	if err != nil {
		// Unparseable: only our concern when it is clearly the server lane; leave
		// client-lane syntax errors to the island validator.
		return false
	}
	references := false
	for _, field := range fields {
		if loads.roots[exprRoot(field)] {
			references = true
			break
		}
	}
	if !references {
		return false
	}
	if len(eachVars) > 0 {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_if_invalid",
			fmt.Sprintf("%s: a nested server-lane g:if supports a single row field, not a compound expression %q; compute compound conditions in Go and expose a bool server {} field", page.ID, raw)))
		return true
	}
	if _, err := clientlang.ParseExpr(raw); err != nil {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_if_invalid",
			fmt.Sprintf("%s: server-lane g:if condition %q is not a valid expression: %v", page.ID, raw, err)))
		return true
	}
	for _, field := range fields {
		if !loads.roots[exprRoot(field)] {
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_if_invalid",
				fmt.Sprintf("%s: server-lane g:if condition %q references %q, which is not a declared server {} field", page.ID, raw, field)))
			return true
		}
	}
	return true
}

// simpleConditionField reports whether a g:if value is a bare field path,
// optionally negated with a single leading !.
func simpleConditionField(raw string) bool {
	stripped := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "!"))
	if stripped == "" {
		return false
	}
	return !strings.ContainsAny(stripped, "!&|=<>(){}\"' \t")
}

func validatePageRawHTMLDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, inRow bool, diagnostics *[]ValidationError) {
	expr := strings.TrimSpace(attr.Value)
	if expr == "" {
		return
	}
	if inRow {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "ghtml_over_load_data",
			fmt.Sprintf("%s: g:unsafe-html is not supported inside a server-lane g:for row; row data is attacker-influenceable and bypasses escape-by-default. Render row text with escape-by-default interpolation instead of raw HTML", page.ID)))
		return
	}
	if loads.roots[exprRoot(expr)] {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "ghtml_over_load_data",
			fmt.Sprintf("%s: g:unsafe-html cannot render request-time server {} data %q; server {} fields are attacker-influenceable and bypass escape-by-default. Render request-time text with escape-by-default interpolation (e.g. inside g:for) instead of raw HTML", page.ID, expr)))
	}
}

func validateServerLoadFieldConflicts(page gwdkir.Page, loads pageLoads, diagnostics *[]ValidationError) {
	if len(loads.fields) == 0 {
		return
	}
	routeParams := map[string]bool{}
	for _, param := range page.DynamicParams() {
		routeParams[param] = true
	}
	buildFields := pageBuildFields(page)
	for field := range loads.fields {
		topLevel, _, _ := strings.Cut(field, ".")
		if routeParams[field] || routeParams[topLevel] {
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_load_field_conflict",
				fmt.Sprintf("%s: server {} load field %q conflicts with build data or route params; rename the server field or read route params through the request-time load context", page.ID, field)))
			continue
		}
		if buildFields[field] || buildFields[topLevel] {
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_load_field_conflict",
				fmt.Sprintf("%s: server {} load field %q conflicts with build data or route params; rename either the build field or the server field", page.ID, field)))
		}
	}
}

func validateServerLoadResultFields(page gwdkir.Page, loads pageLoads, diagnostics *[]ValidationError) {
	if len(loads.fields) == 0 || page.LoadBinding.Status != source.BackendBindingBound || page.LoadBinding.ResultType == "" {
		return
	}
	known := map[string]bool{}
	for _, field := range page.LoadBinding.ResultFields {
		if field.Path != "" {
			known[field.Path] = true
		}
	}
	for field := range loads.fields {
		if known[field] {
			continue
		}
		*diagnostics = append(*diagnostics, pageServerLoadDiagnostic(page, "server_load_field_unknown",
			fmt.Sprintf("%s: server {} load field %q is not exported by typed load result %s; update the field path, add a json tag, or expose the field from %s", page.ID, field, page.LoadBinding.ResultType, page.LoadBinding.ResultType)))
	}
}

func pageBuildFields(page gwdkir.Page) map[string]bool {
	fields := map[string]bool{}
	for _, record := range page.Blocks.BuildRecords {
		for name := range record.Fields {
			fields[name] = true
		}
		for name := range record.Expressions {
			fields[name] = true
		}
	}
	return fields
}

func validatePageServerRegionElement(page gwdkir.Page, element viewmodel.Element, loads pageLoads, inServerRegion bool, eachVars []string, diagnostics *[]ValidationError) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:unsafe-html" {
			validatePageRawHTMLDirective(page, attr, loads, inServerRegion, diagnostics)
			continue
		}
		if inServerRegion && strings.HasPrefix(attr.Name, "g:") && !allowedServerRegionDirective(attr.Name) {
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_region_directive",
				fmt.Sprintf("%s: server-lane g:for rows and g:if branches support only static markup, scoped interpolation, nested g:for, and nested g:if; %q is not allowed", page.ID, attr.Name)))
			continue
		}
		if unsafeRequestTimeAttr(attr.Name) && attrReferencesRequestTime(page, attr, loads, inServerRegion, eachVars) {
			if allowRequestTimeURLAttrTemplate(attr.Name, attr.Value) {
				continue
			}
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_url_tainted",
				fmt.Sprintf("%s: request-time interpolation (route param or load field) is not allowed in %q attributes", page.ID, attr.Name)))
		}
	}
}

func allowedServerRegionDirective(name string) bool {
	switch name {
	case "g:for", "g:key", "g:if":
		return true
	default:
		return false
	}
}

func unsafeRequestTimeAttr(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "on") && len(name) > 2 {
		return true
	}
	if urlBearingRequestTimeAttr(name) {
		return true
	}
	switch name {
	case "style", "srcdoc":
		return true
	default:
		return false
	}
}

func allowRequestTimeURLAttrTemplate(name, value string) bool {
	if !urlBearingRequestTimeAttr(name) {
		return false
	}
	value = strings.TrimSpace(value)
	if strings.EqualFold(strings.TrimSpace(name), "srcset") {
		return allowRequestTimeSrcsetURLAttrTemplate(value)
	}
	return allowSingleRequestTimeURLAttrTemplate(value)
}

func allowRequestTimeSrcsetURLAttrTemplate(value string) bool {
	for _, candidate := range strings.Split(value, ",") {
		fields := strings.Fields(strings.TrimSpace(candidate))
		if len(fields) == 0 || !strings.Contains(fields[0], "{") {
			continue
		}
		if !allowSingleRequestTimeURLAttrTemplate(fields[0]) {
			return false
		}
	}
	return true
}

func allowSingleRequestTimeURLAttrTemplate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value[0] != '/' {
		return false
	}
	if len(value) > 1 && (value[1] == '/' || value[1] == '\\' || value[1] == '{') {
		return false
	}
	if strings.Contains(value, "\\") || containsRequestTimeURLControl(value) {
		return false
	}
	return true
}

func urlBearingRequestTimeAttr(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "href", "src", "srcset", "action", "formaction", "poster", "cite", "data", "longdesc", "manifest", "xlink:href":
		return true
	default:
		return false
	}
}

func containsRequestTimeURLControl(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}

func attrReferencesRequestTime(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, inServerRegion bool, eachVars []string) bool {
	for _, expr := range attrInterpolations(attr) {
		if _, ok := routeParamExpression(expr); ok {
			return true
		}
		if isPageRouteParam(page, expr) {
			return true
		}
		if loads.fields[expr] {
			return true
		}
		if inServerRegion {
			root := exprRoot(expr)
			for _, eachVar := range eachVars {
				if root == eachVar {
					return true
				}
			}
			if loads.roots[root] {
				return true
			}
		}
	}
	return false
}

func attrInterpolations(attr viewmodel.Attr) []string {
	if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
		return nil
	}
	if attr.Expression {
		return []string{stripBracedExpression(attr.Value)}
	}
	var out []string
	value := attr.Value
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			return out
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return out
		}
		end += start
		expr := strings.TrimSpace(value[start+1 : end])
		if expr != "" {
			out = append(out, expr)
		}
		value = value[end+1:]
	}
}

func stripBracedExpression(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "{"), "}"))
	}
	return value
}

func routeParamExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `param("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `param("`)
	if name == "" || strings.ContainsAny(name, " \t\n\r{}()\"'") {
		return "", false
	}
	return name, true
}

func isPageRouteParam(page gwdkir.Page, expr string) bool {
	for _, param := range page.DynamicParams() {
		if expr == param {
			return true
		}
	}
	return false
}

func pageListDiagnostic(page gwdkir.Page, code, message string) ValidationError {
	return ValidationError{
		Code:    code,
		PageID:  page.ID,
		Source:  page.Source,
		Span:    firstSpan(page.Blocks.Spans.View, page.Spans.Page),
		Message: message,
	}
}

func pageServerLoadDiagnostic(page gwdkir.Page, code, message string) ValidationError {
	return ValidationError{
		Code:    code,
		PageID:  page.ID,
		Source:  page.Source,
		Span:    firstSpan(page.Blocks.Spans.Server, page.Blocks.Spans.View, page.Spans.Page),
		Message: message,
	}
}

func elementAttr(element viewmodel.Element, name string) (viewmodel.Attr, bool) {
	for _, attr := range element.Attrs {
		if attr.Name == name {
			return attr, true
		}
	}
	return viewmodel.Attr{}, false
}

// exprRoot returns the leading identifier of a dotted/indexed expression, e.g.
// "columns" from "columns" and "col" from "col.issues".
func exprRoot(expr string) string {
	expr = strings.TrimSpace(expr)
	if cut := strings.IndexAny(expr, ".[ "); cut >= 0 {
		return expr[:cut]
	}
	return expr
}
