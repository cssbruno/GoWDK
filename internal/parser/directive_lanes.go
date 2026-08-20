package parser

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewmodel"
)

func validateDirectiveLaneDeclarations(nodes []viewmodel.Node, span source.SourceSpan) error {
	for _, node := range nodes {
		var attrs []viewmodel.Attr
		var children []viewmodel.Node
		switch typed := node.(type) {
		case viewmodel.Element:
			attrs, children = typed.Attrs, typed.Children
		case viewmodel.ComponentCall:
			attrs, children = typed.Attrs, typed.Children
		case viewmodel.AwaitBlock:
			if err := validateDirectiveLaneDeclarations(typed.Pending, span); err != nil {
				return err
			}
			if err := validateDirectiveLaneDeclarations(typed.Then, span); err != nil {
				return err
			}
			if err := validateDirectiveLaneDeclarations(typed.Catch, span); err != nil {
				return err
			}
			continue
		default:
			continue
		}
		directive := ""
		lane := ""
		laneCount := 0
		laneValidShape := false
		for _, attr := range attrs {
			switch attr.Name {
			case "g:for", "g:if":
				if directive == "" {
					directive = attr.Name
				}
			case "g:lane":
				laneCount++
				lane = strings.TrimSpace(attr.Value)
				laneValidShape = !attr.Boolean && !attr.Expression
			}
		}
		if directive != "" {
			switch {
			case laneCount == 0:
				return &DiagnosticError{Code: DiagnosticDirectiveLaneRequired, Span: span, Message: fmt.Sprintf("%s requires g:lane=\"server\" or g:lane=\"client\"; implicit directive lanes were removed", directive)}
			case laneCount > 1:
				return &DiagnosticError{Code: DiagnosticDirectiveLaneInvalid, Span: span, Message: "element declares multiple g:lane attributes"}
			case !laneValidShape || lane != "server" && lane != "client":
				return &DiagnosticError{Code: DiagnosticDirectiveLaneInvalid, Span: span, Message: fmt.Sprintf("g:lane must be the string literal \"server\" or \"client\", got %q", lane)}
			}
		} else if laneCount > 0 {
			return &DiagnosticError{Code: DiagnosticDirectiveLaneInvalid, Span: span, Message: "g:lane requires g:for or g:if on the same element"}
		}
		if err := validateDirectiveLaneDeclarations(children, span); err != nil {
			return err
		}
	}
	return nil
}
