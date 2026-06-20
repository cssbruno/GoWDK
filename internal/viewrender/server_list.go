package viewrender

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
)

// SSRListReplacement is a build-time description of one server-rendered g:for
// list. It is collected during a request-time page render and handed to the app
// generator, which serializes it for the runtime region renderer. The tree
// mirrors nesting: Lists and Conds describe g:for lists and g:if conditionals
// found inside RowTemplate.
type SSRListReplacement struct {
	Placeholder string
	SourcePath  string
	ItemVar     string
	IndexVar    string
	RowTemplate string
	Fields      []SSRListField
	Lists       []SSRListReplacement
	Conds       []SSRCondReplacement
}

// SSRCondReplacement is a build-time description of one server-rendered g:if
// conditional. Its branch renders only when SourcePath resolves to a truthy
// value (negated when Negate is set). The branch shares the enclosing container
// scope.
type SSRCondReplacement struct {
	Placeholder string
	SourcePath  string
	Negate      bool
	// Expr is a full bool expression for a top-level server g:if; when set the
	// runtime evaluates it against the load data instead of SourcePath/Negate.
	Expr     string
	Template string
	Fields   []SSRListField
	Lists    []SSRListReplacement
	Conds    []SSRCondReplacement
}

// SSRListField is one per-render scalar substitution inside a region template.
type SSRListField struct {
	Placeholder string
	Path        string
	Index       bool
	URL         bool
}

// serverScope tracks the active server-lane g:for row or g:if branch while
// rendering its template. itemVar is set for a row scope (interpolations are
// item-relative); when itemVar is empty the scope is a load scope
// (interpolations are server {} field paths validated against tainted). It also
// carries the collectors for the region currently being built.
type serverScope struct {
	itemVar   string
	indexVar  string
	tainted   map[string]bool
	fields    *[]SSRListField
	lists     *[]SSRListReplacement
	conds     *[]SSRCondReplacement
	seen      map[string]string
	seenIndex string
}

// directiveLane is the resolved execution lane of a g:for/g:if directive.
type directiveLane int

const (
	laneClient directiveLane = iota
	laneServer
)

// resolveDirectiveLane decides whether a g:for/g:if whose operand has the given
// root identifier renders server-side (request-time server {} data) or
// client-side (state/store islands). Inside an active server region every nested
// directive inherits the server lane.
func (ctx *renderContext) resolveDirectiveLane(root string) (directiveLane, error) {
	if ctx.serverScope != nil {
		return laneServer, nil
	}
	server := ctx.tainted[root]
	client := ctx.isClientDataRoot(root)
	switch {
	case server && client:
		return laneClient, fmt.Errorf("%q is declared as both request-time server {} data and client state/store; rename one so the directive lane is unambiguous", root)
	case server:
		return laneServer, nil
	default:
		// Client lane covers both declared client state and unknown sources; the
		// client validator (or check-time validation) reports an unknown source.
		return laneClient, nil
	}
}

func (ctx *renderContext) isClientDataRoot(root string) bool {
	if ctx.readFields[root] {
		return true
	}
	if _, ok := ctx.stateTypes[root]; ok {
		return true
	}
	return false
}

// forDirectiveLane resolves the lane of an element's g:for from its collection
// source. A parse failure falls back to the client lane so the client validator
// reports the precise syntax error.
func (ctx *renderContext) forDirectiveLane(node Element) (directiveLane, error) {
	for _, attr := range node.Attrs {
		if attr.Name != "g:for" || attr.Boolean {
			continue
		}
		loop, err := ParseForDirective(strings.TrimSpace(attr.Value))
		if err != nil {
			return laneClient, nil
		}
		return ctx.resolveDirectiveLane(exprRootName(loop.Collection))
	}
	return laneClient, nil
}

// ifDirectiveLane resolves the lane of an element's g:if from its condition's
// root identifier (after stripping a leading server-style negation).
func (ctx *renderContext) ifDirectiveLane(node Element) (directiveLane, error) {
	for _, attr := range node.Attrs {
		if attr.Name != "g:if" || attr.Boolean {
			continue
		}
		expr := strings.TrimSpace(attr.Value)
		expr = strings.TrimSpace(strings.TrimPrefix(expr, "!"))
		return ctx.resolveDirectiveLane(exprRootName(expr))
	}
	return laneClient, nil
}

func elementHasAttr(node Element, name string) bool {
	for _, attr := range node.Attrs {
		if attr.Name == name {
			return true
		}
	}
	return false
}

// renderServerListElement renders a g:for element into a list placeholder plus
// a collected SSRListReplacement. The element's subtree is rendered once as a
// row template in which item interpolations become per-row field placeholders;
// nested g:for and g:if recurse into child specs.
func renderServerListElement(node Element, ctx *renderContext, out *renderOutput) error {
	if ctx.serverScope == nil && ctx.lists == nil {
		return fmt.Errorf("server-lane g:for is only supported in a request-time page view; it cannot be used inside a component, layout, or fragment. Move the server {} data and g:for onto the page")
	}
	if elementHasAttr(node, "g:if") {
		return fmt.Errorf("element cannot combine a server-lane g:for with g:if; place g:if on a child or wrapping element")
	}
	each, err := elementServerForDirective(node)
	if err != nil {
		return err
	}
	sourcePath, err := serverSourcePath(each.Collection, ctx, "g:for")
	if err != nil {
		return err
	}
	templateNode := elementWithoutAttrs(node, "g:for", "g:key")
	if err := validateServerRegionSubtree([]Node{templateNode}); err != nil {
		return err
	}

	fields := []SSRListField{}
	lists := []SSRListReplacement{}
	conds := []SSRCondReplacement{}
	scope := &serverScope{
		itemVar:  each.Var,
		indexVar: each.IndexVar,
		fields:   &fields,
		lists:    &lists,
		conds:    &conds,
		seen:     map[string]string{},
	}

	rowCtx := *ctx
	rowCtx.serverScope = scope
	var rowOut renderOutput
	if err := renderElement(templateNode, &rowCtx, &rowOut); err != nil {
		return err
	}

	replacement := SSRListReplacement{
		Placeholder: "__GOWDK_SSR_LIST_" + ctx.idAllocator().nextListGroup() + "__",
		SourcePath:  sourcePath,
		ItemVar:     each.Var,
		IndexVar:    each.IndexVar,
		RowTemplate: rowOut.string(),
		Fields:      fields,
		Lists:       lists,
		Conds:       conds,
	}
	appendServerList(ctx, replacement)
	out.write(replacement.Placeholder)
	return nil
}

// renderServerConditionalElement renders a g:if element into a conditional
// placeholder plus a collected SSRCondReplacement. The element's subtree is
// rendered once into a branch template in the enclosing container scope.
func renderServerConditionalElement(node Element, ctx *renderContext, out *renderOutput) error {
	if ctx.serverScope == nil && ctx.conds == nil {
		return fmt.Errorf("server-lane g:if is only supported in a request-time page view; it cannot be used inside a component, layout, or fragment. Move the server {} data and g:if onto the page")
	}
	condition, negate, expr, err := elementServerIfDirective(node)
	if err != nil {
		return err
	}
	var sourcePath string
	if expr != "" {
		// A compound expression is evaluated at request time against the load
		// data; it is only supported at the top level (not inside a row), and its
		// referenced roots must be declared server {} fields.
		if parent := ctx.serverScope; parent != nil && parent.itemVar != "" {
			return fmt.Errorf("a nested server-lane g:if supports a single row field, not a compound expression %q; compute compound conditions in Go and expose a bool server {} field", expr)
		}
		if err := validateServerCondExpr(expr, ctx); err != nil {
			return err
		}
	} else {
		sourcePath, err = serverSourcePath(condition, ctx, "g:if")
		if err != nil {
			return err
		}
	}
	templateNode := elementWithoutAttrs(node, "g:if")
	if err := validateServerRegionSubtree([]Node{templateNode}); err != nil {
		return err
	}

	fields := []SSRListField{}
	lists := []SSRListReplacement{}
	conds := []SSRCondReplacement{}
	// The branch renders in the enclosing container scope: a row scope keeps the
	// parent item var; a top-level g:if renders against the load data map.
	scope := &serverScope{
		fields: &fields,
		lists:  &lists,
		conds:  &conds,
		seen:   map[string]string{},
	}
	if parent := ctx.serverScope; parent != nil {
		scope.itemVar = parent.itemVar
		scope.indexVar = parent.indexVar
		scope.tainted = parent.tainted
	} else {
		scope.tainted = ctx.tainted
	}

	branchCtx := *ctx
	branchCtx.serverScope = scope
	var branchOut renderOutput
	if err := renderElement(templateNode, &branchCtx, &branchOut); err != nil {
		return err
	}

	replacement := SSRCondReplacement{
		Placeholder: "__GOWDK_SSR_COND_" + ctx.idAllocator().nextCondGroup() + "__",
		SourcePath:  sourcePath,
		Negate:      negate,
		Expr:        expr,
		Template:    branchOut.string(),
		Fields:      fields,
		Lists:       lists,
		Conds:       conds,
	}
	appendServerCond(ctx, replacement)
	out.write(replacement.Placeholder)
	return nil
}

func appendServerList(ctx *renderContext, replacement SSRListReplacement) {
	if ctx.serverScope != nil {
		*ctx.serverScope.lists = append(*ctx.serverScope.lists, replacement)
		return
	}
	if ctx.lists != nil {
		*ctx.lists = append(*ctx.lists, replacement)
	}
}

func appendServerCond(ctx *renderContext, replacement SSRCondReplacement) {
	if ctx.serverScope != nil {
		*ctx.serverScope.conds = append(*ctx.serverScope.conds, replacement)
		return
	}
	if ctx.conds != nil {
		*ctx.conds = append(*ctx.conds, replacement)
	}
}

// elementServerForDirective parses the g:for collection on a server-lane list
// element (its lane was already resolved from the collection's data source).
func elementServerForDirective(node Element) (ForDirective, error) {
	hasFor := false
	var loop ForDirective
	for _, attr := range node.Attrs {
		if attr.Name != "g:for" {
			continue
		}
		if hasFor {
			return ForDirective{}, fmt.Errorf("element declares multiple g:for directives")
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return ForDirective{}, fmt.Errorf("g:for requires an expression value such as g:for={item in Items}")
		}
		parsed, err := ParseForDirective(attr.Value)
		if err != nil {
			return ForDirective{}, err
		}
		loop = parsed
		hasFor = true
	}
	return loop, nil
}

// elementServerIfDirective parses the g:if condition on a server-lane
// conditional element. A simple field (optionally negated with a leading !) is
// returned as (field, negate, ""); a compound bool expression (comparisons,
// logic, literals) is returned as ("", false, expr) for request-time evaluation.
func elementServerIfDirective(node Element) (field string, negate bool, expr string, err error) {
	hasIf := false
	var raw string
	for _, attr := range node.Attrs {
		if attr.Name != "g:if" {
			continue
		}
		if hasIf {
			return "", false, "", fmt.Errorf("element declares multiple g:if directives")
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return "", false, "", fmt.Errorf("g:if requires an expression value such as g:if={field} or g:if={count > 0}")
		}
		raw = strings.TrimSpace(attr.Value)
		hasIf = true
	}
	for _, attr := range node.Attrs {
		if attr.Name == "g:else-if" || attr.Name == "g:else" {
			return "", false, "", fmt.Errorf("%s is a client-lane directive and cannot follow a server-lane g:if; use a sibling g:if={!field} for the server empty branch", attr.Name)
		}
	}
	if !isSimpleCondition(raw) {
		return "", false, raw, nil
	}
	stripped := raw
	if strings.HasPrefix(stripped, "!") {
		negate = true
		stripped = strings.TrimSpace(stripped[1:])
	}
	if stripped == "" {
		return "", false, "", fmt.Errorf("g:if requires a field after %q", "!")
	}
	return stripped, negate, "", nil
}

// validateServerCondExpr checks a top-level server g:if compound expression: it
// must parse as a bool expression and every identifier it references must be a
// declared server {} field (tracked in the load scope's taint set).
func validateServerCondExpr(expr string, ctx *renderContext) error {
	if _, err := clientlang.ParseExpr(expr); err != nil {
		return fmt.Errorf("server-lane g:if condition %q is not a valid expression: %w", expr, err)
	}
	fields, err := clientlang.ExprFields(expr)
	if err != nil {
		return fmt.Errorf("server-lane g:if condition %q is invalid: %w", expr, err)
	}
	tainted := ctx.tainted
	if ctx.serverScope != nil {
		tainted = ctx.serverScope.tainted
	}
	for _, field := range fields {
		if !tainted[exprRootName(field)] {
			return fmt.Errorf("server-lane g:if condition %q references %q, which is not a declared server {} field", expr, field)
		}
	}
	return nil
}

// isSimpleCondition reports whether a g:if value is a bare field path, optionally
// negated with a single leading !. Anything with operators, comparisons,
// parentheses, quotes, or whitespace is a compound expression.
func isSimpleCondition(raw string) bool {
	stripped := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "!"))
	if stripped == "" {
		return false
	}
	return !strings.ContainsAny(stripped, "!&|=<>(){}\"' \t")
}

// serverSourcePath resolves the load path for a g:for collection or g:if
// condition. A top-level region (or a load-scope branch) must target a declared
// server {} field; a row-scope region must reference its enclosing item.
func serverSourcePath(expr string, ctx *renderContext, directive string) (string, error) {
	scope := ctx.serverScope
	if scope == nil || scope.itemVar == "" {
		tainted := ctx.tainted
		if scope != nil {
			tainted = scope.tainted
		}
		if !tainted[exprRootName(expr)] {
			return "", fmt.Errorf("%s collection/condition %q must be a server {} field; %s renders request-time server data — use the client-state directive for client/island state", directive, expr, directive)
		}
		return expr, nil
	}
	parent := scope.itemVar
	prefix := parent + "."
	if expr == parent {
		return "", fmt.Errorf("nested %s %q cannot be the parent item %q itself; reference a field such as %s.field", directive, expr, parent, parent)
	}
	if !strings.HasPrefix(expr, prefix) {
		return "", fmt.Errorf("nested %s %q must reference the parent item %q (for example %sfield)", directive, expr, parent, prefix)
	}
	return strings.TrimPrefix(expr, prefix), nil
}

// validateServerRegionSubtree rejects constructs that cannot be rendered inside
// a request-time g:for row or g:if branch. Regions support static markup,
// scoped interpolation, nested g:for, and nested g:if only.
func validateServerRegionSubtree(nodes []Node) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			for _, attr := range typed.Attrs {
				if !strings.HasPrefix(attr.Name, "g:") {
					continue
				}
				if attr.Name == "g:for" || attr.Name == "g:key" || attr.Name == "g:if" {
					continue
				}
				return fmt.Errorf("server-lane g:for rows and g:if branches support only static markup, scoped interpolation, nested g:for, and nested g:if; %q is not allowed", attr.Name)
			}
			if err := validateServerRegionSubtree(typed.Children); err != nil {
				return err
			}
		case ComponentCall:
			return fmt.Errorf("g:for rows and g:if branches cannot contain component calls; render request-time markup with static elements, g:for, and g:if")
		case AwaitBlock:
			return fmt.Errorf("g:for rows and g:if branches cannot contain await blocks; render request-time markup with static elements, g:for, and g:if")
		}
	}
	return nil
}

// serverScopeFieldPlaceholder resolves a region interpolation name to a stable
// field placeholder, recording the resolved path on the active scope. In a row
// scope only the row item (and index) are valid; in a load scope only declared
// server {} fields are valid.
func (scope *serverScope) serverScopeFieldPlaceholder(name string, ids *renderIDAllocator) (string, error) {
	path, isIndex, ok := scope.resolvePath(name)
	if !ok {
		return "", fmt.Errorf("server region may only interpolate %s; cannot resolve %q", scope.scopeHint(), name)
	}
	if isIndex {
		if scope.seenIndex == "" {
			scope.seenIndex = "__GOWDK_SSR_FIELD_" + ids.nextListField() + "__"
			*scope.fields = append(*scope.fields, SSRListField{Placeholder: scope.seenIndex, Index: true})
		}
		return scope.seenIndex, nil
	}
	if existing, dup := scope.seen[path]; dup {
		return existing, nil
	}
	placeholder := "__GOWDK_SSR_FIELD_" + ids.nextListField() + "__"
	scope.seen[path] = placeholder
	*scope.fields = append(*scope.fields, SSRListField{Placeholder: placeholder, Path: path})
	return placeholder, nil
}

func (scope *serverScope) resolvePath(name string) (path string, isIndex bool, ok bool) {
	name = strings.TrimSpace(name)
	if scope.itemVar == "" {
		// Load scope: any declared load field is valid; path is the full name.
		if scope.tainted[exprRootName(name)] {
			return name, false, true
		}
		return "", false, false
	}
	if scope.indexVar != "" && name == scope.indexVar {
		return "", true, true
	}
	if name == scope.itemVar {
		return "", false, true
	}
	if strings.HasPrefix(name, scope.itemVar+".") {
		return strings.TrimPrefix(name, scope.itemVar+"."), false, true
	}
	return "", false, false
}

func (scope *serverScope) scopeHint() string {
	if scope.itemVar == "" {
		return "declared server {} fields"
	}
	hint := "the row item " + strconv.Quote(scope.itemVar)
	if scope.indexVar != "" {
		hint += " or index " + scope.indexVar
	}
	return hint
}

func exprRootName(expr string) string {
	expr = strings.TrimSpace(expr)
	if cut := strings.IndexAny(expr, ".[ "); cut >= 0 {
		return expr[:cut]
	}
	return expr
}

func (ids *renderIDAllocator) nextListGroup() string {
	ids.list++
	return fmt.Sprintf("s%d", ids.list)
}

func (ids *renderIDAllocator) nextCondGroup() string {
	ids.cond++
	return fmt.Sprintf("w%d", ids.cond)
}

func (ids *renderIDAllocator) nextListField() string {
	ids.field++
	return fmt.Sprintf("%d", ids.field)
}
