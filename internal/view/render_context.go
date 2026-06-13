package view

import "github.com/cssbruno/gowdk/internal/clientlang"

type renderContext struct {
	components   map[string]Component
	ownerPackage string
	uses         map[string]string
	values       map[string]string
	tainted      map[string]bool
	actions      map[string]string
	actionFields map[string][]ActionInputField
	formAction   string
	stack        map[string]bool
	slotHTML     string
	slots        map[string]slotContent
	stateFields  map[string]bool
	readFields   map[string]bool
	bindFields   map[string]bool
	conditional  *conditionalRender
	handlers     map[string]clientlang.Handler
	stateTypes   map[string]clientlang.ValueType
	refs         map[string]clientlang.Ref
	emits        map[string]clientlang.Emit
	loopSeq      *int
	bindingSeq   *int
	islandSeq    *int
	loopItem     *loopItemRender
	templateLoop *templateLoopRender
	scopeIDs     []string
	selectBound  bool
	selectValue  string
}

type loopItemRender struct {
	Group    string
	KeyExpr  string
	KeyValue string
}

type templateLoopRender struct{}
