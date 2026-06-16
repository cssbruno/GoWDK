package view

import (
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"strconv"
	"strings"
)

func renderNodes(nodes []Node, ctx *renderContext) (string, error) {
	if len(nodes) == 0 {
		return "", nil
	}
	var out renderOutput
	groupSeq := 0
	inChain := false
	chainMatched := false
	chainIndex := 0
	for _, node := range nodes {
		nodeCtx := ctx
		if element, ok := node.(Element); ok {
			// A server-lane g:if is a structural request-time region, not a
			// client conditional chain: let renderElement dispatch it and do not
			// open or continue a client if/else-if/else chain. g:else-if/g:else
			// are client-only, so only a standalone g:if can take the server lane.
			if elementHasAttr(element, "g:if") && !elementHasAttr(element, "g:else-if") && !elementHasAttr(element, "g:else") {
				lane, err := ctx.ifDirectiveLane(element)
				if err != nil {
					return "", err
				}
				if lane == laneServer {
					inChain = false
					chainMatched = false
					chainIndex = 0
					if err := renderNode(node, ctx, &out); err != nil {
						return "", err
					}
					continue
				}
			}
			branch, err := conditionalBranch(element, ctx)
			if err != nil {
				return "", err
			}
			switch branch.Kind {
			case "if":
				groupSeq++
				inChain = true
				chainMatched = branch.Visible
				chainIndex = 0
				next := *ctx
				next.conditional = &conditionalRender{
					Group:     fmt.Sprintf("c%d", groupSeq),
					Index:     chainIndex,
					Condition: branch.Condition,
					Visible:   branch.Visible,
				}
				nodeCtx = &next
			case "else-if":
				if !inChain {
					return "", fmt.Errorf("g:else-if must follow a sibling g:if or g:else-if")
				}
				chainIndex++
				visible := !chainMatched && branch.Visible
				if visible {
					chainMatched = true
				}
				next := *ctx
				next.conditional = &conditionalRender{
					Group:     fmt.Sprintf("c%d", groupSeq),
					Index:     chainIndex,
					Condition: branch.Condition,
					Visible:   visible,
				}
				nodeCtx = &next
			case "else":
				if !inChain {
					return "", fmt.Errorf("g:else must follow a sibling g:if or g:else-if")
				}
				chainIndex++
				visible := !chainMatched
				chainMatched = true
				next := *ctx
				next.conditional = &conditionalRender{
					Group:   fmt.Sprintf("c%d", groupSeq),
					Index:   chainIndex,
					Visible: visible,
				}
				nodeCtx = &next
				inChain = false
			default:
				inChain = false
				chainMatched = false
				chainIndex = 0
			}
		} else if !ignorableConditionalSeparator(node) {
			inChain = false
			chainMatched = false
			chainIndex = 0
		}
		if err := renderNode(node, nodeCtx, &out); err != nil {
			return "", err
		}
	}
	return out.string(), nil
}

func renderNode(node Node, ctx *renderContext, out *renderOutput) error {
	switch typed := node.(type) {
	case Text:
		return renderTextNode(typed, ctx, out)
	case Element:
		return renderElement(typed, ctx, out)
	case ComponentCall:
		return renderComponentCall(typed, ctx, out)
	default:
		return fmt.Errorf("unsupported view node %T", node)
	}
}

type conditionalRender struct {
	Group     string
	Index     int
	Condition string
	Visible   bool
}

func (conditional conditionalRender) Marker() string {
	return conditional.Group + "-" + strconv.Itoa(conditional.Index)
}

type conditionalBranchInfo struct {
	Kind      string
	Condition string
	Visible   bool
}

func conditionalBranch(node Element, ctx *renderContext) (conditionalBranchInfo, error) {
	hasIf := false
	hasElseIf := false
	hasElse := false
	var condition string
	for _, attr := range node.Attrs {
		switch attr.Name {
		case "g:if":
			hasIf = true
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return conditionalBranchInfo{}, fmt.Errorf("g:if requires an expression value")
			}
			condition = strings.TrimSpace(attr.Value)
		case "g:else-if":
			hasElseIf = true
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return conditionalBranchInfo{}, fmt.Errorf("g:else-if requires an expression value")
			}
			condition = strings.TrimSpace(attr.Value)
		case "g:else":
			hasElse = true
			if !attr.Boolean && strings.TrimSpace(attr.Value) != "" {
				return conditionalBranchInfo{}, fmt.Errorf("g:else must not have a value")
			}
		}
	}
	count := 0
	for _, set := range []bool{hasIf, hasElseIf, hasElse} {
		if set {
			count++
		}
	}
	if count == 0 {
		return conditionalBranchInfo{}, nil
	}
	if count > 1 {
		return conditionalBranchInfo{}, fmt.Errorf("element cannot combine g:if, g:else-if, and g:else")
	}
	if hasElse {
		return conditionalBranchInfo{Kind: "else"}, nil
	}
	if err := ValidateIslandBoolExpression(condition, ctx.readFields); err != nil {
		if hasIf {
			return conditionalBranchInfo{}, fmt.Errorf("g:if: %w", err)
		}
		return conditionalBranchInfo{}, fmt.Errorf("g:else-if: %w", err)
	}
	visible := false
	if evaluated, err := clientlang.EvalBool(condition, ctx.values); err == nil {
		visible = evaluated
	}
	if hasIf {
		return conditionalBranchInfo{Kind: "if", Condition: condition, Visible: visible}, nil
	}
	return conditionalBranchInfo{Kind: "else-if", Condition: condition, Visible: visible}, nil
}

func ignorableConditionalSeparator(node Node) bool {
	text, ok := node.(Text)
	return ok && strings.TrimSpace(text.Value) == ""
}
