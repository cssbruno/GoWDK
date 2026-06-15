package view

import (
	"fmt"
	"strings"
)

func collectCommandReferences(nodes []Node, refs *[]CommandReference) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			directives, err := typed.directiveValues()
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
		case ComponentCall:
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

func collectQueryReferences(nodes []Node, refs *[]QueryReference) error {
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

func collectSubscriptionReferences(nodes []Node, refs *[]SubscriptionReference) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			directives, err := typed.directiveValues()
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
		case ComponentCall:
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

func collectContractReferences(nodes []Node, refs *[]ContractReference) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			directives, err := typed.directiveValues()
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
		case ComponentCall:
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

func formMethodPath(node Element) (string, string) {
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

func contractReferencesFromNodes(nodes []Node) ([]ContractReference, error) {
	var refs []ContractReference
	if err := collectContractReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}
