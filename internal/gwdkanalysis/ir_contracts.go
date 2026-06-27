package gwdkanalysis

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewanalysis"
)

func appendContractReferences(program *gwdkir.Program, template gwdkir.Template) {
	refs, err := templateContractReferences(template)
	if err != nil {
		program.Diagnostics = append(program.Diagnostics, gwdkir.Diagnostic{
			Code:    "contract_reference_parse_error",
			Source:  template.Source,
			Span:    template.Span,
			Message: fmt.Sprintf("parse contract references in %s %s view: %v", template.OwnerKind, template.OwnerID, err),
		})
		return
	}
	for _, ref := range refs {
		method := source.BackendRouteMethod(ref.Method)
		path := ref.Path
		if path == "" && template.Route != "" {
			routeIsDynamic := routeHasDynamicParams(template.Route)
			switch {
			case routeIsDynamic:
				program.Diagnostics = append(program.Diagnostics, gwdkir.Diagnostic{
					Code:    "contract_route_invalid",
					Source:  template.Source,
					Span:    templateOffsetSpan(template, ref.Start, ref.End),
					Message: fmt.Sprintf("%s %s must declare an explicit route on dynamic page route %q", irContractReferenceKind(ref.Kind), ref.Name, template.Route),
				})
			case ref.Kind == viewanalysis.ContractReferenceQuery:
				method = "GET"
			case method == "":
				method = "POST"
			}
			if !routeIsDynamic {
				path = template.Route
			}
		}
		importAlias, contractType := splitContractReferenceName(ref.Name)
		importPath := contractReferenceImportPath(template.Imports, importAlias)
		program.ContractRefs = append(program.ContractRefs, gwdkir.ContractReference{
			Kind:        irContractReferenceKind(ref.Kind),
			Name:        ref.Name,
			ImportAlias: importAlias,
			ImportPath:  importPath,
			Type:        contractType,
			Guards:      append([]string(nil), template.Guards...),
			Method:      method,
			Path:        path,
			OwnerKind:   template.OwnerKind,
			OwnerID:     template.OwnerID,
			Package:     template.Package,
			Source:      template.Source,
			Span:        templateOffsetSpan(template, ref.Start, ref.End),
		})
	}
}

func appendRealtimeSubscriptions(program *gwdkir.Program, template gwdkir.Template) {
	refs, err := templateSubscriptionReferences(template)
	if err != nil {
		program.Diagnostics = append(program.Diagnostics, gwdkir.Diagnostic{
			Code:    "realtime_subscription_parse_error",
			Source:  template.Source,
			Span:    template.Span,
			Message: fmt.Sprintf("parse realtime subscriptions in %s %s view: %v", template.OwnerKind, template.OwnerID, err),
		})
		return
	}
	for _, ref := range refs {
		queryAlias, queryType := splitContractReferenceName(ref.Query)
		eventAlias, eventType := splitContractReferenceName(ref.Event)
		program.RealtimeSubscriptions = append(program.RealtimeSubscriptions, gwdkir.RealtimeSubscription{
			Query:            ref.Query,
			QueryImportAlias: queryAlias,
			QueryImportPath:  contractReferenceImportPath(template.Imports, queryAlias),
			QueryType:        queryType,
			Event:            ref.Event,
			EventImportAlias: eventAlias,
			EventImportPath:  contractReferenceImportPath(template.Imports, eventAlias),
			EventType:        eventType,
			Guards:           append([]string(nil), template.Guards...),
			OwnerKind:        template.OwnerKind,
			OwnerID:          template.OwnerID,
			Package:          template.Package,
			Source:           template.Source,
			Span:             templateOffsetSpan(template, ref.EventStart, ref.EventEnd),
			QuerySpan:        templateOffsetSpan(template, ref.QueryStart, ref.QueryEnd),
		})
	}
}

func templateContractReferences(template gwdkir.Template) ([]viewanalysis.ContractReference, error) {
	if len(template.Nodes) > 0 {
		return viewanalysis.ContractReferencesFromNodes(template.Nodes)
	}
	return viewanalysis.ContractReferences(template.Body)
}

func templateSubscriptionReferences(template gwdkir.Template) ([]viewanalysis.SubscriptionReference, error) {
	if len(template.Nodes) > 0 {
		return viewanalysis.SubscriptionReferencesFromNodes(template.Nodes)
	}
	return viewanalysis.SubscriptionReferences(template.Body)
}

func routeHasDynamicParams(route string) bool {
	return strings.Contains(route, "{")
}

func splitContractReferenceName(name string) (string, string) {
	before, after, ok := strings.Cut(name, ".")
	if !ok {
		return "", name
	}
	return before, after
}

func contractReferenceImportPath(imports []gwdkir.Import, alias string) string {
	if alias == "" {
		return ""
	}
	for _, item := range imports {
		if item.Alias == alias {
			return item.Path
		}
	}
	return ""
}

func irContractReferenceKind(kind viewanalysis.ContractReferenceKind) gwdkir.ContractKind {
	switch kind {
	case viewanalysis.ContractReferenceQuery:
		return gwdkir.ContractQuery
	default:
		return gwdkir.ContractCommand
	}
}

func templateOffsetSpan(template gwdkir.Template, start int, end int) source.SourceSpan {
	if start < 0 || end <= start || start >= len([]rune(template.Body)) {
		return template.Span
	}
	startPos := templateOffsetPosition(template, start)
	endPos := templateOffsetPosition(template, end)
	if startPos.Line == 0 || endPos.Line == 0 {
		return template.Span
	}
	return source.SourceSpan{Start: startPos, End: endPos}
}

func templateOffsetPosition(template gwdkir.Template, offset int) source.SourcePosition {
	line := template.BodyStart.Line
	column := template.BodyStart.Column
	if line == 0 {
		line = template.Span.Start.Line + 1
		column = 1
	}
	for index, char := range []rune(template.Body) {
		if index == offset {
			return source.SourcePosition{Line: line, Column: column}
		}
		if char == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	if offset == len([]rune(template.Body)) {
		return source.SourcePosition{Line: line, Column: column}
	}
	return source.SourcePosition{}
}
