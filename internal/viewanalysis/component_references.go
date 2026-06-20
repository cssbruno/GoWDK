package viewanalysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// ComponentReferences returns unique component names directly referenced by a
// view markup fragment.
func ComponentReferences(source string) ([]string, error) {
	refs, err := ComponentReferenceSpans(source)
	if err != nil {
		return nil, err
	}
	return componentReferenceNames(refs), nil
}

// ComponentReferencesFromNodes returns unique component names directly
// referenced by an already-parsed view fragment.
func ComponentReferencesFromNodes(nodes []viewmodel.Node) []string {
	return componentReferenceNames(ComponentReferenceSpansFromNodes(nodes))
}

func componentReferenceNames(refs []ComponentReference) []string {
	if len(refs) == 0 {
		return nil
	}
	names := map[string]bool{}
	for _, ref := range refs {
		names[ref.Name] = true
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ComponentReferenceSpans returns component calls directly referenced by a view
// markup fragment, preserving source offsets for diagnostics.
func ComponentReferenceSpans(source string) ([]ComponentReference, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return nil, err
	}
	return ComponentReferenceSpansFromNodes(nodes), nil
}

// ComponentReferenceSpansFromNodes returns component calls from an already-
// parsed view fragment, preserving source offsets for diagnostics.
func ComponentReferenceSpansFromNodes(nodes []viewmodel.Node) []ComponentReference {
	var refs []ComponentReference
	collectComponentReferences(nodes, &refs)
	if len(refs) == 0 {
		return nil
	}
	return refs
}

func collectComponentReferences(nodes []viewmodel.Node, refs *[]ComponentReference) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.ComponentCall:
			*refs = append(*refs, ComponentReference{Name: typed.Name, Start: typed.Start, End: typed.End})
			collectComponentReferences(typed.Children, refs)
		case viewmodel.Element:
			collectComponentReferences(typed.Children, refs)
		case viewmodel.AwaitBlock:
			collectComponentReferences(typed.Pending, refs)
			collectComponentReferences(typed.Then, refs)
			collectComponentReferences(typed.Catch, refs)
		}
	}
}

// ComponentIslandUsages returns component calls that explicitly set g:island.
func ComponentIslandUsages(source string) ([]ComponentIslandUsage, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return nil, err
	}
	return ComponentIslandUsagesFromNodes(nodes)
}

// ComponentIslandUsagesFromNodes returns component calls that explicitly set
// g:island in an already-parsed view fragment.
func ComponentIslandUsagesFromNodes(nodes []viewmodel.Node) ([]ComponentIslandUsage, error) {
	var usages []ComponentIslandUsage
	if err := collectComponentIslandUsages(nodes, &usages); err != nil {
		return nil, err
	}
	return usages, nil
}

func collectComponentIslandUsages(nodes []viewmodel.Node, usages *[]ComponentIslandUsage) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.ComponentCall:
			mode, err := componentCallIslandMode(typed)
			if err != nil {
				return err
			}
			if mode != "" {
				*usages = append(*usages, ComponentIslandUsage{Component: typed.Name, Mode: mode})
			}
			if err := collectComponentIslandUsages(typed.Children, usages); err != nil {
				return err
			}
		case viewmodel.Element:
			if err := collectComponentIslandUsages(typed.Children, usages); err != nil {
				return err
			}
		case viewmodel.AwaitBlock:
			if err := collectComponentIslandUsages(typed.Pending, usages); err != nil {
				return err
			}
			if err := collectComponentIslandUsages(typed.Then, usages); err != nil {
				return err
			}
			if err := collectComponentIslandUsages(typed.Catch, usages); err != nil {
				return err
			}
		}
	}
	return nil
}

// ComponentCallUsages returns component calls with optional g:island metadata.
func ComponentCallUsages(source string) ([]ComponentCallUsage, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return nil, err
	}
	return ComponentCallUsagesFromNodes(nodes)
}

// ComponentCallUsagesFromNodes returns component calls with optional g:island
// metadata from an already-parsed view fragment.
func ComponentCallUsagesFromNodes(nodes []viewmodel.Node) ([]ComponentCallUsage, error) {
	var usages []ComponentCallUsage
	if err := collectComponentCallUsages(nodes, &usages); err != nil {
		return nil, err
	}
	return usages, nil
}

func collectComponentCallUsages(nodes []viewmodel.Node, usages *[]ComponentCallUsage) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.ComponentCall:
			mode, err := componentCallIslandMode(typed)
			if err != nil {
				return err
			}
			*usages = append(*usages, ComponentCallUsage{
				Component:     typed.Name,
				Island:        mode,
				ReactiveProps: componentCallHasReactiveProps(typed),
			})
			if err := collectComponentCallUsages(typed.Children, usages); err != nil {
				return err
			}
		case viewmodel.Element:
			if err := collectComponentCallUsages(typed.Children, usages); err != nil {
				return err
			}
		case viewmodel.AwaitBlock:
			if err := collectComponentCallUsages(typed.Pending, usages); err != nil {
				return err
			}
			if err := collectComponentCallUsages(typed.Then, usages); err != nil {
				return err
			}
			if err := collectComponentCallUsages(typed.Catch, usages); err != nil {
				return err
			}
		}
	}
	return nil
}

func componentCallIslandMode(node viewmodel.ComponentCall) (string, error) {
	mode := ""
	for _, attr := range node.Attrs {
		if attr.Name != "g:island" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return "", fmt.Errorf("component %s g:island requires a value", node.Name)
		}
		value := strings.TrimSpace(attr.Value)
		if value != "wasm" {
			return "", fmt.Errorf("component %s uses unsupported g:island value %q", node.Name, value)
		}
		mode = value
	}
	return mode, nil
}

func componentCallHasReactiveProps(node viewmodel.ComponentCall) bool {
	for _, attr := range node.Attrs {
		if attr.Spread {
			return true
		}
		if strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if attr.Expression {
			return true
		}
	}
	return false
}
