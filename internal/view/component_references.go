package view

import "strings"

func collectComponentReferences(nodes []Node, names map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			names[typed.Name] = true
			collectComponentReferences(typed.Children, names)
		case Element:
			collectComponentReferences(typed.Children, names)
		}
	}
}

func collectComponentIslandUsages(nodes []Node, usages *[]ComponentIslandUsage) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			mode, err := typed.islandMode()
			if err != nil {
				return err
			}
			if mode != "" {
				*usages = append(*usages, ComponentIslandUsage{Component: typed.Name, Mode: mode})
			}
			if err := collectComponentIslandUsages(typed.Children, usages); err != nil {
				return err
			}
		case Element:
			if err := collectComponentIslandUsages(typed.Children, usages); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectComponentCallUsages(nodes []Node, usages *[]ComponentCallUsage) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			mode, err := typed.islandMode()
			if err != nil {
				return err
			}
			*usages = append(*usages, ComponentCallUsage{
				Component:     typed.Name,
				Island:        mode,
				ReactiveProps: typed.hasReactiveProps(),
			})
			if err := collectComponentCallUsages(typed.Children, usages); err != nil {
				return err
			}
		case Element:
			if err := collectComponentCallUsages(typed.Children, usages); err != nil {
				return err
			}
		}
	}
	return nil
}

func (node ComponentCall) hasReactiveProps() bool {
	for _, attr := range node.Attrs {
		if strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if attr.Expression {
			return true
		}
	}
	return false
}
