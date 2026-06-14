package view

import "github.com/cssbruno/gowdk/internal/clientlang"

type renderContext struct {
	renderComponentContext
	renderDataContext
	renderClientContext
	conditional  *conditionalRender
	ids          *renderIDAllocator
	loopItem     *loopItemRender
	templateLoop *templateLoopRender
}

type renderComponentContext struct {
	components   map[string]Component
	ownerPackage string
	uses         map[string]string
	stack        map[string]bool
	slotHTML     string
	slots        map[string]slotContent
	scopeIDs     []string
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
}

type loopItemRender struct {
	Group    string
	KeyExpr  string
	KeyValue string
}

type templateLoopRender struct{}
