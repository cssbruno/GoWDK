package view

import (
	"sort"

	"github.com/cssbruno/gowdk/internal/viewanalysis"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

type Attr = viewmodel.Attr
type Component = viewmodel.Component
type ComponentCall = viewmodel.ComponentCall
type Element = viewmodel.Element
type InlineScript = viewmodel.InlineScript
type Node = viewmodel.Node
type Text = viewmodel.Text

// Parse parses a view markup fragment.
func Parse(source string) ([]Node, error) {
	return viewparse.Parse(source)
}

// RenderSPA renders a view markup fragment with escaped text and attrs.
func RenderSPA(source string) (string, error) {
	return RenderWithComponents(source, nil)
}

// RenderWithComponents renders a view markup fragment with component support.
func RenderWithComponents(source string, components map[string]Component) (string, error) {
	return RenderWithData(source, components, nil)
}

// RenderWithData renders a view markup fragment with component support and
// string interpolation data.
func RenderWithData(source string, components map[string]Component, data map[string]string) (string, error) {
	return RenderWithOptions(source, components, data, Options{})
}

// Options configures view rendering.
type Options struct {
	Actions           map[string]string
	ActionInputFields map[string][]ActionInputField
	Package           string
	Uses              map[string]string
	// Tainted names interpolation values that carry request-time,
	// attacker-influenceable data (e.g. server {} fields). Tainted values are
	// rejected in URL-bearing, event-handler, style, and srcdoc attributes the
	// same way route params are, so they cannot smuggle a javascript:/data: URL
	// past HTML-text escaping.
	Tainted                map[string]bool
	RealtimeEventTypeNames map[string]string
	QueryTypeNames         map[string]string
	// ServerListSink and ServerCondSink, when non-nil, receive the top-level
	// g:for lists and g:if conditionals discovered while rendering a
	// request-time page. The caller serializes them for the runtime region
	// renderer.
	ServerListSink *[]SSRListReplacement
	ServerCondSink *[]SSRCondReplacement
}

// ActionInputField describes Go action input metadata available while rendering
// literal controls inside a g:post form.
type ActionInputField struct {
	FormName string
	Type     string
}

// ActionFormField describes one direct literal form field for a g:post action.
type ActionFormField struct {
	Name             string
	Required         bool
	RequiredMessage  string
	MinLength        int
	MinLengthMessage string
	MaxLength        int
	MaxLengthMessage string
	Pattern          string
	PatternMessage   string
}

type Dependencies = viewanalysis.Dependencies
type ComponentIslandUsage = viewanalysis.ComponentIslandUsage
type ComponentCallUsage = viewanalysis.ComponentCallUsage
type ComponentReference = viewanalysis.ComponentReference
type ContractReference = viewanalysis.ContractReference
type ContractReferenceKind = viewanalysis.ContractReferenceKind

const (
	ContractReferenceCommand = viewanalysis.ContractReferenceCommand
	ContractReferenceQuery   = viewanalysis.ContractReferenceQuery
)

type CommandReference = viewanalysis.CommandReference
type QueryReference = viewanalysis.QueryReference
type SubscriptionReference = viewanalysis.SubscriptionReference

// RenderWithOptions renders a view markup fragment with component support,
// interpolation data, and page-scoped action endpoints.
func RenderWithOptions(source string, components map[string]Component, data map[string]string, options Options) (string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return "", err
	}
	return RenderNodesWithOptions(nodes, components, data, options)
}

// RenderNodesWithOptions renders an already-parsed view fragment with component
// support, interpolation data, and page-scoped action endpoints.
func RenderNodesWithOptions(nodes []Node, components map[string]Component, data map[string]string, options Options) (string, error) {
	return renderParsedNodes(nodes, renderContext{
		renderComponentContext: renderComponentContext{
			components:             components,
			ownerPackage:           options.Package,
			uses:                   cloneValues(options.Uses),
			realtimeEventTypeNames: cloneValues(options.RealtimeEventTypeNames),
			queryTypeNames:         cloneValues(options.QueryTypeNames),
			stack:                  map[string]bool{},
		},
		renderDataContext: renderDataContext{
			values:       cloneValues(data),
			tainted:      cloneTaintSet(options.Tainted),
			actions:      cloneValues(options.Actions),
			actionFields: cloneActionInputFields(options.ActionInputFields),
			stateFields:  map[string]bool{},
			readFields:   map[string]bool{},
			bindFields:   map[string]bool{},
		},
		ids:   &renderIDAllocator{},
		lists: options.ServerListSink,
		conds: options.ServerCondSink,
	})
}

// ActionFormFields returns direct literal HTML control names grouped by g:post
// action name. Component-hidden controls are intentionally not inferred in this
// first decoder slice.
func ActionFormFields(source string) (map[string][]string, error) {
	schema, err := ActionFormSchema(source)
	if err != nil {
		return nil, err
	}
	fields := map[string][]string{}
	for action, actionFields := range schema {
		for _, field := range actionFields {
			fields[action] = append(fields[action], field.Name)
		}
	}
	return fields, nil
}

// ViewDependencies returns direct literal asset and style references from a
// view markup fragment. Interpolated and external URLs are not reported.
func ViewDependencies(source string) (Dependencies, error) {
	return viewanalysis.ViewDependencies(source)
}

// ViewDependenciesFromNodes returns direct literal asset and style references
// from an already-parsed view fragment.
func ViewDependenciesFromNodes(nodes []Node) Dependencies {
	return viewanalysis.ViewDependenciesFromNodes(nodes)
}

// ActionFormSchema returns direct literal HTML controls grouped by g:post action
// name. Duplicate field names are merged, and Required is true if any matching
// direct control is required.
func ActionFormSchema(source string) (map[string][]ActionFormField, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	return ActionFormSchemaFromNodes(nodes)
}

// ActionFormSchemaFromNodes returns direct literal HTML controls grouped by
// g:post action name from an already-parsed view fragment.
func ActionFormSchemaFromNodes(nodes []Node) (map[string][]ActionFormField, error) {
	fields := map[string]map[string]ActionFormField{}
	if err := collectActionFormFields(nodes, fields); err != nil {
		return nil, err
	}
	schema := map[string][]ActionFormField{}
	for action := range fields {
		names := make([]string, 0, len(fields[action]))
		for name := range fields[action] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			schema[action] = append(schema[action], fields[action][name])
		}
	}
	return schema, nil
}

// ComponentReferences returns unique component names directly referenced by a
// view markup fragment.
func ComponentReferences(source string) ([]string, error) {
	return viewanalysis.ComponentReferences(source)
}

// ComponentReferencesFromNodes returns unique component names directly
// referenced by an already-parsed view fragment.
func ComponentReferencesFromNodes(nodes []Node) []string {
	return viewanalysis.ComponentReferencesFromNodes(nodes)
}

// ComponentReferenceSpans returns component calls directly referenced by a view
// markup fragment, preserving source offsets for diagnostics.
func ComponentReferenceSpans(source string) ([]ComponentReference, error) {
	return viewanalysis.ComponentReferenceSpans(source)
}

// ComponentReferenceSpansFromNodes returns component calls from an already-
// parsed view fragment, preserving source offsets for diagnostics.
func ComponentReferenceSpansFromNodes(nodes []Node) []ComponentReference {
	return viewanalysis.ComponentReferenceSpansFromNodes(nodes)
}

// ComponentIslandUsages returns component calls that explicitly set g:island.
func ComponentIslandUsages(source string) ([]ComponentIslandUsage, error) {
	return viewanalysis.ComponentIslandUsages(source)
}

// ComponentIslandUsagesFromNodes returns component calls that explicitly set
// g:island in an already-parsed view fragment.
func ComponentIslandUsagesFromNodes(nodes []Node) ([]ComponentIslandUsage, error) {
	return viewanalysis.ComponentIslandUsagesFromNodes(nodes)
}

// ComponentCallUsages returns component calls with optional g:island metadata.
func ComponentCallUsages(source string) ([]ComponentCallUsage, error) {
	return viewanalysis.ComponentCallUsages(source)
}

// ComponentCallUsagesFromNodes returns component calls with optional g:island
// metadata from an already-parsed view fragment.
func ComponentCallUsagesFromNodes(nodes []Node) ([]ComponentCallUsage, error) {
	return viewanalysis.ComponentCallUsagesFromNodes(nodes)
}

// CommandReferences returns package-qualified command references declared by
// g:command on direct form elements in a view fragment.
func CommandReferences(source string) ([]CommandReference, error) {
	return viewanalysis.CommandReferences(source)
}

// CommandReferencesFromNodes returns package-qualified command references
// declared by g:command on direct form elements in an already-parsed view
// fragment.
func CommandReferencesFromNodes(nodes []Node) ([]CommandReference, error) {
	return viewanalysis.CommandReferencesFromNodes(nodes)
}

// QueryReferences returns package-qualified query references declared by
// g:query on direct HTML elements in a view fragment.
func QueryReferences(source string) ([]QueryReference, error) {
	return viewanalysis.QueryReferences(source)
}

// QueryReferencesFromNodes returns package-qualified query references declared
// by g:query on direct HTML elements in an already-parsed view fragment.
func QueryReferencesFromNodes(nodes []Node) ([]QueryReference, error) {
	return viewanalysis.QueryReferencesFromNodes(nodes)
}

// SubscriptionReferences returns package-qualified presentation-event
// references declared by g:subscribe on query-owned elements.
func SubscriptionReferences(source string) ([]SubscriptionReference, error) {
	return viewanalysis.SubscriptionReferences(source)
}

// SubscriptionReferencesFromNodes returns package-qualified presentation-event
// references declared by g:subscribe on query-owned elements in an
// already-parsed view fragment.
func SubscriptionReferencesFromNodes(nodes []Node) ([]SubscriptionReference, error) {
	return viewanalysis.SubscriptionReferencesFromNodes(nodes)
}

// ContractReferences returns package-qualified command and query references
// declared by GOWDK view directives.
func ContractReferences(source string) ([]ContractReference, error) {
	return viewanalysis.ContractReferences(source)
}

// ContractReferencesFromNodes returns package-qualified command and query
// references declared by GOWDK view directives in an already-parsed view
// fragment.
func ContractReferencesFromNodes(nodes []Node) ([]ContractReference, error) {
	return viewanalysis.ContractReferencesFromNodes(nodes)
}

// Canonical returns a deterministic AST-backed representation of a view body.
