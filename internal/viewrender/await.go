package viewrender

import (
	"fmt"

	"github.com/cssbruno/gowdk/internal/clientlang"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

func renderAwaitBlock(block AwaitBlock, ctx *renderContext, out *renderOutput) error {
	if !ctx.awaitAllowed {
		return fmt.Errorf("await blocks are only supported inside component views rendered as client islands")
	}
	if ctx.templateLoop != nil {
		return fmt.Errorf("await blocks are not supported inside g:for templates")
	}
	if ctx.serverScope != nil {
		return fmt.Errorf("await blocks are not supported inside server-rendered regions")
	}
	if _, err := clientlang.ValidateAwaitFetchExpression(block.Expression, ctx.readSymbols(), nil); err != nil {
		return fmt.Errorf("await block: %w", err)
	}
	if ctx.awaitUsed != nil {
		*ctx.awaitUsed = true
	}

	id := ctx.nextAwaitID()
	out.write("<gowdk-await")
	out.write(gowhtml.Attr("data-gowdk-await", id))
	out.write(gowhtml.Attr("data-gowdk-binding-await", ctx.nextBindingID()))
	out.write(gowhtml.Attr("data-gowdk-await-expr", block.Expression))
	out.write(gowhtml.Attr("data-gowdk-await-result", block.ResultName))
	if block.ErrorName != "" {
		out.write(gowhtml.Attr("data-gowdk-await-error", block.ErrorName))
	}
	out.write(">")
	if err := renderAwaitBranchTemplate(out, "pending", block.Pending, ctx, nil); err != nil {
		return err
	}
	if err := renderAwaitBranchTemplate(out, "then", block.Then, ctx, awaitThenSymbols(block.ResultName)); err != nil {
		return err
	}
	if block.ErrorName != "" {
		if err := renderAwaitBranchTemplate(out, "catch", block.Catch, ctx, awaitCatchSymbols(block.ErrorName)); err != nil {
			return err
		}
	}
	out.write("</gowdk-await>")
	return nil
}

func renderAwaitBranchTemplate(out *renderOutput, name string, nodes []Node, ctx *renderContext, symbols map[string]clientlang.ValueType) error {
	branchCtx := *ctx
	branchCtx.conditional = nil
	if len(symbols) > 0 {
		branchCtx.templateLoop = &templateLoopRender{}
		branchCtx.readFields = mergeBoolSets(ctx.readFields, boolSet(keysFromTypes(symbols)))
		branchCtx.stateTypes = mergeClientSymbols(ctx.stateTypes, symbols)
	}
	html, err := renderNodes(nodes, &branchCtx)
	if err != nil {
		return err
	}
	out.write(`<template data-gowdk-await-branch="`)
	out.write(name)
	out.write(`">`)
	out.write(html)
	out.write("</template>")
	return nil
}

func awaitThenSymbols(name string) map[string]clientlang.ValueType {
	return map[string]clientlang.ValueType{name: clientlang.TypeUnknown}
}

func awaitCatchSymbols(name string) map[string]clientlang.ValueType {
	return map[string]clientlang.ValueType{
		name:              clientlang.TypeObject,
		name + ".message": clientlang.TypeString,
	}
}
