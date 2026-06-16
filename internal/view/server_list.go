package view

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/viewparse"
)

// SSRListReplacement is a build-time description of one server-rendered g:each
// list. It is collected during a request-time page render and handed to the app
// generator, which serializes it for the runtime list renderer. The tree
// mirrors nesting: Children describe g:each lists found inside RowTemplate.
type SSRListReplacement struct {
	Placeholder string
	SourcePath  string
	ItemVar     string
	IndexVar    string
	RowTemplate string
	Fields      []SSRListField
	Children    []SSRListReplacement
}

// SSRListField is one per-row scalar substitution inside a row template.
type SSRListField struct {
	Placeholder string
	Path        string
	Index       bool
}

// EachDirective is a parsed g:each declaration.
type EachDirective = viewparse.EachDirective

// ParseEachDirective parses a g:each value such as "item in Items".
func ParseEachDirective(source string) (EachDirective, error) {
	return viewparse.ParseEachDirective(source)
}

// serverListScope tracks the active g:each row while rendering its template.
type serverListScope struct {
	itemVar   string
	indexVar  string
	fields    *[]SSRListField
	children  *[]SSRListReplacement
	seen      map[string]string
	seenIndex string
}

func elementHasEach(node Element) bool {
	for _, attr := range node.Attrs {
		if attr.Name == "g:each" {
			return true
		}
	}
	return false
}

// renderServerListElement renders a g:each element into a list placeholder plus
// a collected SSRListReplacement. The element's subtree is rendered once as a
// row template in which item interpolations become per-row field placeholders;
// nested g:each elements recurse into child specs.
func renderServerListElement(node Element, ctx *renderContext, out *renderOutput) error {
	each, err := elementEachDirective(node)
	if err != nil {
		return err
	}
	sourcePath, err := serverListSourcePath(each, ctx)
	if err != nil {
		return err
	}
	templateNode := elementWithoutAttrs(node, "g:each", "g:key")
	if err := validateServerListSubtree(templateNode.Children); err != nil {
		return err
	}

	group := ctx.idAllocator().nextListGroup()
	fields := []SSRListField{}
	children := []SSRListReplacement{}
	scope := &serverListScope{
		itemVar:  each.Var,
		indexVar: each.IndexVar,
		fields:   &fields,
		children: &children,
		seen:     map[string]string{},
	}

	rowCtx := *ctx
	rowCtx.serverList = scope
	var rowOut renderOutput
	if err := renderElement(templateNode, &rowCtx, &rowOut); err != nil {
		return err
	}

	replacement := SSRListReplacement{
		Placeholder: "__GOWDK_SSR_LIST_" + group + "__",
		SourcePath:  sourcePath,
		ItemVar:     each.Var,
		IndexVar:    each.IndexVar,
		RowTemplate: rowOut.string(),
		Fields:      fields,
		Children:    children,
	}
	if ctx.serverList != nil {
		*ctx.serverList.children = append(*ctx.serverList.children, replacement)
	} else if ctx.lists != nil {
		*ctx.lists = append(*ctx.lists, replacement)
	}
	out.write(replacement.Placeholder)
	return nil
}

func elementEachDirective(node Element) (EachDirective, error) {
	hasEach := false
	var each EachDirective
	for _, attr := range node.Attrs {
		if attr.Name != "g:each" {
			continue
		}
		if hasEach {
			return EachDirective{}, fmt.Errorf("element declares multiple g:each directives")
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return EachDirective{}, fmt.Errorf("g:each requires an expression value such as g:each={item in Items}")
		}
		parsed, err := ParseEachDirective(attr.Value)
		if err != nil {
			return EachDirective{}, err
		}
		each = parsed
		hasEach = true
	}
	for _, attr := range node.Attrs {
		if attr.Name == "g:for" {
			return EachDirective{}, fmt.Errorf("element cannot combine g:each with g:for; g:each renders request-time server data, g:for binds client/island state")
		}
	}
	return each, nil
}

// serverListSourcePath resolves the load path for a g:each collection. A
// top-level g:each must target an SSR load {} field; a nested g:each must
// reference its parent item so its slice can be resolved per parent row.
func serverListSourcePath(each EachDirective, ctx *renderContext) (string, error) {
	if ctx.serverList == nil {
		if !ctx.tainted[each.Collection] {
			return "", fmt.Errorf("g:each collection %q must be an SSR load {} field; g:each renders request-time server data — use g:for for client/island state", each.Collection)
		}
		return each.Collection, nil
	}
	parent := ctx.serverList.itemVar
	prefix := parent + "."
	if each.Collection == parent {
		return "", fmt.Errorf("nested g:each collection cannot be the parent item %q itself; reference a slice field such as %s.items", parent, parent)
	}
	if !strings.HasPrefix(each.Collection, prefix) {
		return "", fmt.Errorf("nested g:each collection %q must reference the parent item %q (for example %sfield)", each.Collection, parent, prefix)
	}
	return strings.TrimPrefix(each.Collection, prefix), nil
}

// validateServerListSubtree rejects constructs that cannot be rendered inside a
// request-time g:each row. Rows support static markup, item interpolation, and
// nested g:each only; client directives, components, and inline scripts have no
// server-render semantics here.
func validateServerListSubtree(nodes []Node) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			for _, attr := range typed.Attrs {
				if !strings.HasPrefix(attr.Name, "g:") {
					continue
				}
				if attr.Name == "g:each" || attr.Name == "g:key" {
					continue
				}
				return fmt.Errorf("g:each rows support only static markup, item interpolation, and nested g:each; %q is not allowed inside a g:each row", attr.Name)
			}
			if err := validateServerListSubtree(typed.Children); err != nil {
				return err
			}
		case ComponentCall:
			return fmt.Errorf("g:each rows cannot contain component calls; render request-time lists with static markup and nested g:each")
		}
	}
	return nil
}

// serverListFieldPlaceholder resolves a row interpolation name to a stable
// per-row field placeholder, recording the item-relative path (or index) on the
// active scope. It returns an error when the name does not reference the row's
// own item or index variable.
func (scope *serverListScope) serverListFieldPlaceholder(name string, ids *renderIDAllocator) (string, error) {
	path, isIndex, ok := scope.itemRelativePath(name)
	if !ok {
		return "", fmt.Errorf("g:each row may only interpolate its item %q%s; cannot resolve %q", scope.itemVar, scope.indexHint(), name)
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

func (scope *serverListScope) itemRelativePath(name string) (path string, isIndex bool, ok bool) {
	name = strings.TrimSpace(name)
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

func (scope *serverListScope) indexHint() string {
	if scope.indexVar == "" {
		return ""
	}
	return " or index " + scope.indexVar
}

func (ids *renderIDAllocator) nextListGroup() string {
	ids.list++
	return fmt.Sprintf("s%d", ids.list)
}

func (ids *renderIDAllocator) nextListField() string {
	ids.field++
	return fmt.Sprintf("%d", ids.field)
}
