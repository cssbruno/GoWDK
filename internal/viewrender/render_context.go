package viewrender

import "github.com/cssbruno/gowdk/internal/clientlang"

type renderContext struct {
	renderComponentContext
	renderDataContext
	renderClientContext
	conditional  *conditionalRender
	ids          *renderIDAllocator
	loopItem     *loopItemRender
	templateLoop *templateLoopRender
	// serverScope is the active g:for row or g:if branch scope, set while
	// rendering a request-time server region template. It is nil outside regions.
	serverScope *serverScope
	// lists and conds collect the top-level g:for lists and g:if conditionals
	// discovered during a render. They are shared pointers so they survive
	// renderContext value copies.
	lists *[]SSRListReplacement
	conds *[]SSRCondReplacement
}

type renderComponentContext struct {
	components             map[string]Component
	ownerPackage           string
	uses                   map[string]string
	realtimeEventTypeNames map[string]string
	queryTypeNames         map[string]string
	stack                  map[string]bool
	slotHTML               string
	slots                  map[string]slotContent
	scopeIDs               []string
}

type renderDataContext struct {
	values       map[string]string
	tainted      map[string]bool
	actions      map[string]string
	actionFields map[string][]ActionInputField
	formAction   string
	propFields   map[string]bool
	stateFields  map[string]bool
	readFields   map[string]bool
	bindFields   map[string]bool
	selectBound  bool
	selectValue  string
}

type renderClientContext struct {
	handlers   map[string]clientlang.Handler
	stateTypes map[string]clientlang.ValueType
	refs       map[string]clientlang.Ref
	emits      map[string]clientlang.Emit
}

type renderIDAllocator struct {
	loop    int
	binding int
	island  int
	list    int
	cond    int
	field   int
}

type loopItemRender struct {
	Group    string
	KeyExpr  string
	KeyValue string
}

type templateLoopRender struct{}
