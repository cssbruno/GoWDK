package viewparse

import (
	"fmt"
	"strings"
)

// ForDirective is a parsed g:for declaration.
type ForDirective struct {
	Var        string
	IndexVar   string
	Collection string
}

// ParseForDirective parses a g:for value such as "item in Items" or
// "item, i in Items".
func ParseForDirective(source string) (ForDirective, error) {
	match := forDirectivePattern.FindStringSubmatch(strings.TrimSpace(source))
	if match == nil {
		return ForDirective{}, fmt.Errorf("g:for must use \"item in Items\" or \"item, i in Items\" syntax")
	}
	item := strings.TrimSpace(match[1])
	if !isIdentifier(item) {
		return ForDirective{}, fmt.Errorf("g:for item name %q is invalid", item)
	}
	index := strings.TrimSpace(match[2])
	if index != "" {
		if !isIdentifier(index) {
			return ForDirective{}, fmt.Errorf("g:for index name %q is invalid", index)
		}
		if index == item {
			return ForDirective{}, fmt.Errorf("g:for item and index names must differ")
		}
	}
	collection := strings.TrimSpace(match[3])
	if collection == "" {
		return ForDirective{}, fmt.Errorf("g:for collection expression is empty")
	}
	return ForDirective{Var: item, IndexVar: index, Collection: collection}, nil
}

// EachDirective is a parsed g:each declaration. It shares grammar with
// g:for ("item in Items" or "item, i in Items") but selects server-side
// request-time iteration over SSR load {} data rather than client/island state.
type EachDirective = ForDirective

// ParseEachDirective parses a g:each value such as "item in Items" or
// "item, i in Items". The grammar matches g:for; the two differ only in where
// the collection is bound (server load data vs. client state).
func ParseEachDirective(source string) (EachDirective, error) {
	directive, err := ParseForDirective(source)
	if err != nil {
		return EachDirective{}, eachErrorFromFor(err)
	}
	return directive, nil
}

func eachErrorFromFor(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), "g:for", "g:each"))
}
