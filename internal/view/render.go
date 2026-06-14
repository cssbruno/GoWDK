package view

import (
	"fmt"
	"strings"
)

func render(source string, ctx renderContext) (string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return "", err
	}
	return renderParsedNodes(nodes, ctx)
}

func renderParsedNodes(nodes []Node, ctx renderContext) (string, error) {
	if err := validateFragmentTargetReferences(nodes); err != nil {
		return "", err
	}
	if ctx.ids == nil {
		ctx.ids = &renderIDAllocator{}
	}
	return renderNodes(nodes, &ctx)
}

func validateFragmentTargetReferences(nodes []Node) error {
	ids := map[string]bool{}
	targets := map[string]bool{}
	collectIDsAndTargets(nodes, ids, targets)
	for target := range targets {
		id := strings.TrimPrefix(target, "#")
		if !ids[id] {
			return fmt.Errorf("g:target %q does not reference a literal id in this view", target)
		}
	}
	return nil
}

func collectIDsAndTargets(nodes []Node, ids map[string]bool, targets map[string]bool) {
	for _, node := range nodes {
		element, ok := node.(Element)
		if !ok {
			continue
		}
		hasPost := false
		for _, attr := range element.Attrs {
			if attr.Name == "g:post" {
				hasPost = true
				break
			}
		}
		for _, attr := range element.Attrs {
			if attr.Boolean {
				continue
			}
			switch attr.Name {
			case "id":
				id := strings.TrimSpace(attr.Value)
				if id != "" && !strings.ContainsAny(id, "{}") {
					ids[id] = true
				}
			case "g:target":
				target := strings.TrimSpace(attr.Value)
				if hasPost && target != "" && !strings.ContainsAny(target, "{}") {
					targets[target] = true
				}
			}
		}
		collectIDsAndTargets(element.Children, ids, targets)
	}
}
