package viewanalysis

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// CommandReferences returns package-qualified command references declared by
// g:command on direct form elements in a view fragment.
func CommandReferences(source string) ([]CommandReference, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return nil, err
	}
	return CommandReferencesFromNodes(nodes)
}

// CommandReferencesFromNodes returns package-qualified command references
// declared by g:command on direct form elements in an already-parsed view
// fragment.
func CommandReferencesFromNodes(nodes []viewmodel.Node) ([]CommandReference, error) {
	var refs []CommandReference
	if err := collectCommandReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

func collectCommandReferences(nodes []viewmodel.Node, refs *[]CommandReference) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Element:
			directives, err := elementDirectiveValues(typed)
			if err != nil {
				return err
			}
			if directives.Command != "" {
				method, path := formMethodPath(typed)
				*refs = append(*refs, CommandReference{Command: directives.Command, Method: method, Path: path, Start: directives.CommandStart, End: directives.CommandEnd})
			}
			if err := collectCommandReferences(typed.Children, refs); err != nil {
				return err
			}
		case viewmodel.ComponentCall:
			for _, attr := range typed.Attrs {
				if attr.Name == "g:event" {
					return fmt.Errorf("component %s must not declare g:event; domain and integration events are backend-owned facts", typed.Name)
				}
			}
			if err := collectCommandReferences(typed.Children, refs); err != nil {
				return err
			}
		}
	}
	return nil
}

// QueryReferences returns package-qualified query references declared by
// g:query on direct HTML elements in a view fragment.
func QueryReferences(source string) ([]QueryReference, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return nil, err
	}
	return QueryReferencesFromNodes(nodes)
}

// QueryReferencesFromNodes returns package-qualified query references declared
// by g:query on direct HTML elements in an already-parsed view fragment.
func QueryReferencesFromNodes(nodes []viewmodel.Node) ([]QueryReference, error) {
	var refs []QueryReference
	if err := collectQueryReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

func collectQueryReferences(nodes []viewmodel.Node, refs *[]QueryReference) error {
	contracts, err := contractReferencesFromNodes(nodes)
	if err != nil {
		return err
	}
	for _, ref := range contracts {
		if ref.Kind == ContractReferenceQuery {
			*refs = append(*refs, QueryReference{Query: ref.Name, Start: ref.Start, End: ref.End})
		}
	}
	return nil
}

// SubscriptionReferences returns package-qualified presentation-event
// references declared by g:subscribe on query-owned elements.
func SubscriptionReferences(source string) ([]SubscriptionReference, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return nil, err
	}
	return SubscriptionReferencesFromNodes(nodes)
}

// SubscriptionReferencesFromNodes returns package-qualified presentation-event
// references declared by g:subscribe on query-owned elements in an
// already-parsed view fragment.
func SubscriptionReferencesFromNodes(nodes []viewmodel.Node) ([]SubscriptionReference, error) {
	var refs []SubscriptionReference
	if err := collectSubscriptionReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

func collectSubscriptionReferences(nodes []viewmodel.Node, refs *[]SubscriptionReference) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Element:
			directives, err := elementDirectiveValues(typed)
			if err != nil {
				return err
			}
			if directives.Subscribe != "" {
				*refs = append(*refs, SubscriptionReference{
					Query:      directives.Query,
					QueryStart: directives.QueryStart,
					QueryEnd:   directives.QueryEnd,
					Event:      directives.Subscribe,
					EventStart: directives.SubscribeStart,
					EventEnd:   directives.SubscribeEnd,
				})
			}
			if err := collectSubscriptionReferences(typed.Children, refs); err != nil {
				return err
			}
		case viewmodel.ComponentCall:
			for _, attr := range typed.Attrs {
				if attr.Name == "g:event" {
					return fmt.Errorf("component %s must not declare g:event; domain and integration events are backend-owned facts", typed.Name)
				}
				if attr.Name == "g:subscribe" {
					return fmt.Errorf("component %s must not declare g:subscribe; realtime subscriptions must be declared on query-owned HTML elements", typed.Name)
				}
			}
			if err := collectSubscriptionReferences(typed.Children, refs); err != nil {
				return err
			}
		}
	}
	return nil
}

// ContractReferences returns package-qualified command and query references
// declared by GOWDK view directives.
func ContractReferences(source string) ([]ContractReference, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return nil, err
	}
	return ContractReferencesFromNodes(nodes)
}

// ContractReferencesFromNodes returns package-qualified command and query
// references declared by GOWDK view directives in an already-parsed view
// fragment.
func ContractReferencesFromNodes(nodes []viewmodel.Node) ([]ContractReference, error) {
	var refs []ContractReference
	if err := collectContractReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

func collectContractReferences(nodes []viewmodel.Node, refs *[]ContractReference) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Element:
			directives, err := elementDirectiveValues(typed)
			if err != nil {
				return err
			}
			if directives.Command != "" {
				method, path := formMethodPath(typed)
				*refs = append(*refs, ContractReference{
					Kind:   ContractReferenceCommand,
					Name:   directives.Command,
					Method: method,
					Path:   path,
					Start:  directives.CommandStart,
					End:    directives.CommandEnd,
				})
			}
			if directives.Query != "" {
				*refs = append(*refs, ContractReference{
					Kind:  ContractReferenceQuery,
					Name:  directives.Query,
					Start: directives.QueryStart,
					End:   directives.QueryEnd,
				})
			}
			if err := collectContractReferences(typed.Children, refs); err != nil {
				return err
			}
		case viewmodel.ComponentCall:
			for _, attr := range typed.Attrs {
				if attr.Name == "g:event" {
					return fmt.Errorf("component %s must not declare g:event; domain and integration events are backend-owned facts", typed.Name)
				}
			}
			if err := collectContractReferences(typed.Children, refs); err != nil {
				return err
			}
		}
	}
	return nil
}

func contractReferencesFromNodes(nodes []viewmodel.Node) ([]ContractReference, error) {
	var refs []ContractReference
	if err := collectContractReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

func formMethodPath(node viewmodel.Element) (string, string) {
	if node.Name != "form" {
		return "", ""
	}
	method := "POST"
	path := ""
	for _, attr := range node.Attrs {
		if attr.Expression {
			continue
		}
		switch strings.ToLower(attr.Name) {
		case "method":
			value := strings.TrimSpace(attr.Value)
			if value != "" {
				method = strings.ToUpper(value)
			}
		case "action":
			path = strings.TrimSpace(attr.Value)
		}
	}
	return method, path
}

type viewDirectives struct {
	Action         string
	Command        string
	CommandStart   int
	CommandEnd     int
	Query          string
	QueryStart     int
	QueryEnd       int
	Subscribe      string
	SubscribeStart int
	SubscribeEnd   int
	Target         string
	Swap           string
}

func elementDirectiveValues(node viewmodel.Element) (viewDirectives, error) {
	var directives viewDirectives
	for _, attr := range node.Attrs {
		if !strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if strings.HasPrefix(attr.Name, "g:on:") {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("%s requires an expression value", attr.Name)
			}
			continue
		}
		if attr.Name == "g:if" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("g:if requires an expression value")
			}
			continue
		}
		if attr.Name == "g:else-if" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("g:else-if requires an expression value")
			}
			continue
		}
		if attr.Name == "g:else" {
			if !attr.Boolean && strings.TrimSpace(attr.Value) != "" {
				return viewDirectives{}, fmt.Errorf("g:else must not have a value")
			}
			continue
		}
		if attr.Name == "g:for" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("g:for requires an expression value")
			}
			continue
		}
		if attr.Name == "g:key" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("g:key requires an expression value")
			}
			continue
		}
		if attr.Name == "g:bind:value" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("g:bind:value requires a field value")
			}
			continue
		}
		if attr.Name == "g:bind:checked" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("g:bind:checked requires a field value")
			}
			continue
		}
		if attr.Name == "g:ref" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("g:ref requires a ref name")
			}
			continue
		}
		if attr.Name == "g:unsafe-html" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return viewDirectives{}, fmt.Errorf("g:unsafe-html requires an expression value")
			}
			continue
		}
		if attr.Name == "g:event" {
			return viewDirectives{}, fmt.Errorf("frontend templates must not declare g:event; domain and integration events are backend-owned facts, use g:command for backend intent or g:on:* for local UI events")
		}
		if attr.Name != "g:post" && attr.Name != "g:command" && attr.Name != "g:query" && attr.Name != "g:subscribe" && attr.Name != "g:target" && attr.Name != "g:swap" {
			return viewDirectives{}, fmt.Errorf("unsupported directive attribute %q in SPA build", attr.Name)
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return viewDirectives{}, fmt.Errorf("%s requires a value", attr.Name)
		}
		if attr.Name == "g:query" {
			if directives.Query != "" {
				return viewDirectives{}, fmt.Errorf("element declares multiple g:query directives")
			}
			query := strings.TrimSpace(attr.Value)
			if !isContractReference(query) {
				return viewDirectives{}, fmt.Errorf("g:query %q must be a package-qualified Go contract reference", query)
			}
			directives.Query = query
			directives.QueryStart = attr.Start
			directives.QueryEnd = attr.End
			continue
		}
		if attr.Name == "g:subscribe" {
			if directives.Subscribe != "" {
				return viewDirectives{}, fmt.Errorf("element declares multiple g:subscribe directives")
			}
			event := strings.TrimSpace(attr.Value)
			if !isContractReference(event) {
				return viewDirectives{}, fmt.Errorf("g:subscribe %q must be a package-qualified Go presentation event reference", event)
			}
			directives.Subscribe = event
			directives.SubscribeStart = attr.Start
			directives.SubscribeEnd = attr.End
			continue
		}
		if node.Name != "form" {
			return viewDirectives{}, fmt.Errorf("%s is only supported on <form>", attr.Name)
		}
		switch attr.Name {
		case "g:post":
			if directives.Action != "" {
				return viewDirectives{}, fmt.Errorf("form declares multiple g:post directives")
			}
			directives.Action = strings.TrimSpace(attr.Value)
		case "g:command":
			if directives.Command != "" {
				return viewDirectives{}, fmt.Errorf("form declares multiple g:command directives")
			}
			command := strings.TrimSpace(attr.Value)
			if !isContractReference(command) {
				return viewDirectives{}, fmt.Errorf("g:command %q must be a package-qualified Go contract reference", command)
			}
			directives.Command = command
			directives.CommandStart = attr.Start
			directives.CommandEnd = attr.End
		case "g:target":
			if directives.Target != "" {
				return viewDirectives{}, fmt.Errorf("form declares multiple g:target directives")
			}
			target := strings.TrimSpace(attr.Value)
			if strings.ContainsAny(target, "{}") {
				return viewDirectives{}, fmt.Errorf("g:target %q must be literal", target)
			}
			if !strings.HasPrefix(target, "#") || strings.TrimPrefix(target, "#") == "" || strings.ContainsAny(target, " \t\r\n") {
				return viewDirectives{}, fmt.Errorf("g:target %q must be a literal id selector", target)
			}
			directives.Target = target
		case "g:swap":
			if directives.Swap != "" {
				return viewDirectives{}, fmt.Errorf("form declares multiple g:swap directives")
			}
			swap := strings.TrimSpace(attr.Value)
			if !isSupportedSwapMode(swap) {
				return viewDirectives{}, fmt.Errorf("unsupported g:swap mode %q", swap)
			}
			directives.Swap = swap
		}
	}
	if directives.Command != "" && directives.Query != "" {
		return viewDirectives{}, fmt.Errorf("form must not declare both g:command and g:query")
	}
	if directives.Subscribe != "" && directives.Query == "" {
		return viewDirectives{}, fmt.Errorf("g:subscribe requires g:query on the same element")
	}
	if directives.Swap != "" && directives.Target == "" {
		return viewDirectives{}, fmt.Errorf("g:swap requires g:target")
	}
	return directives, nil
}

func isSupportedSwapMode(value string) bool {
	switch value {
	case "innerHTML", "outerHTML":
		return true
	default:
		return false
	}
}

func isContractReference(source string) bool {
	parts := strings.Split(source, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !isIdentifier(part) {
			return false
		}
	}
	return true
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		switch {
		case char >= 'A' && char <= 'Z':
		case char >= 'a' && char <= 'z':
		case char == '_':
		case index > 0 && char >= '0' && char <= '9':
		default:
			return false
		}
	}
	return true
}
